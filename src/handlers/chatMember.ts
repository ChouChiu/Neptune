import { type Bot, InputFile } from "grammy";
import {
	addPendingVerification,
	cleanExpiredVerifications,
	getGroupConfig,
	getPendingVerification,
} from "../db/queries";
import { getBotUsername } from "../utils/botInfo";
import { generateCaptcha, uploadCaptchaToR2 } from "../utils/captcha";
import { getNickname } from "../utils/nickname";
import { replacePlaceholders } from "../utils/placeholders";
import { escapeMarkdown, replyOptions } from "../utils/reply";

const RULE_ACK_WAIT_SECONDS = 10;

function buildRuleText(rule: string, remaining: number): string {
	const countdown =
		remaining > 0
			? `\n\n⏱️ ${remaining} 秒后可点击下方按钮`
			: "\n\n✅ 阅读时间已到，请点击下方按钮";
	return `📋 *群规*\n\n${rule}${countdown}`;
}

export function registerChatMemberHandler(
	bot: Bot,
	db: D1Database,
	bucket: R2Bucket,
	reuseCaptcha = false,
): void {
	bot.on("message:new_chat_members", async (ctx) => {
		const newMembers = ctx.message.new_chat_members;
		if (!newMembers) return;

		const groupId = ctx.chat.id;

		const config = await getGroupConfig(db, groupId);
		if (!config?.welcome_enabled) return;

		await cleanExpiredVerifications(db);

		try {
			await ctx.api.deleteMessage(groupId, ctx.message.message_id);
		} catch (error) {
			console.error("Failed to delete join message:", error);
		}

		for (const newMember of newMembers) {
			if (newMember.is_bot) continue;

			const userId = newMember.id;
			const nickname = getNickname(newMember);

			const welcomeText = replacePlaceholders(config.welcome_message, {
				nickname: escapeMarkdown(nickname),
				userid: userId,
				groupname: ctx.chat.title ? escapeMarkdown(ctx.chat.title) : undefined,
			});

			const botUsername = await getBotUsername(ctx.api);
			const verifyUrl = `https://t.me/${botUsername}?start=verify${groupId}_${userId}`;

			const welcomeMsg = await ctx.reply(welcomeText, {
				parse_mode: "Markdown",
				reply_markup: {
					inline_keyboard: [
						[
							{
								text: config.verify_button_text,
								url: verifyUrl,
							},
						],
					],
				},
			});

			const timeout = config.verify_timeout;
			const expiresAt = Math.floor(Date.now() / 1000) + timeout + 300;
			await addPendingVerification(
				db,
				userId,
				groupId,
				"",
				expiresAt,
				welcomeMsg.message_id,
			);

			try {
				await ctx.api.restrictChatMember(groupId, userId, {
					can_send_messages: false,
					can_send_audios: false,
					can_send_documents: false,
					can_send_photos: false,
					can_send_videos: false,
					can_send_video_notes: false,
					can_send_voice_notes: false,
					can_send_polls: false,
					can_send_other_messages: false,
					can_add_web_page_previews: false,
					can_change_info: false,
					can_invite_users: false,
					can_pin_messages: false,
					can_manage_topics: false,
				});
			} catch (error) {
				console.error(
					"Failed to restrict user:",
					error instanceof Error ? error.message : error,
				);
			}
		}
	});

	bot.command("start", async (ctx) => {
		if (ctx.chat.type !== "private") return;

		const args = ctx.message?.text?.split(" ");
		const payload = args?.[1];

		if (!payload?.startsWith("verify")) {
			await ctx.reply(
				"欢迎使用 Neptune！发送 /help 查看命令列表。",
				replyOptions(ctx),
			);
			return;
		}

		const parts = payload.slice(6).split("_");
		if (parts.length < 2) return;
		const groupId = Number.parseInt(parts[0] ?? "", 10);
		const targetUserId = Number.parseInt(parts[1] ?? "", 10);
		if (Number.isNaN(groupId) || Number.isNaN(targetUserId)) return;
		if (!ctx.from) return;

		if (ctx.from.id !== targetUserId) {
			await ctx.reply("这不是你的验证链接。", replyOptions(ctx));
			return;
		}

		const userId = ctx.from.id;

		const config = await getGroupConfig(db, groupId);
		if (!config) {
			await ctx.reply("群组配置错误。", replyOptions(ctx));
			return;
		}

		const existing = await getPendingVerification(db, userId, groupId);
		const welcomeMsgId = existing?.welcome_message_id ?? undefined;

		if (config.rule) {
			const showTime = Math.floor(Date.now() / 1000);
			await addPendingVerification(
				db,
				userId,
				groupId,
				"",
				Math.floor(Date.now() / 1000) + config.verify_timeout,
				welcomeMsgId,
			);

			await ctx.reply(buildRuleText(config.rule, RULE_ACK_WAIT_SECONDS), {
				parse_mode: "Markdown",
				reply_markup: {
					inline_keyboard: [
						[
							{
								text: "我以知晓",
								callback_data: `rule_ack:${groupId}:${showTime}`,
							},
						],
					],
				},
			});
			return;
		}

		await sendCaptcha(
			ctx.api,
			db,
			bucket,
			userId,
			groupId,
			config,
			reuseCaptcha,
			welcomeMsgId,
		);
	});

	bot.callbackQuery(/^rule_ack:(-?\d+):(\d+)$/, async (ctx) => {
		const match = ctx.match;
		if (!match?.[1] || !match[2]) return;

		const groupId = Number.parseInt(match[1], 10);
		const showTime = Number.parseInt(match[2], 10);
		const userId = ctx.from.id;

		const config = await getGroupConfig(db, groupId);
		if (!config?.rule) {
			await ctx.answerCallbackQuery({ text: "配置错误。" });
			return;
		}

		const now = Math.floor(Date.now() / 1000);
		const elapsed = now - showTime;

		if (elapsed < RULE_ACK_WAIT_SECONDS) {
			const remaining = RULE_ACK_WAIT_SECONDS - elapsed;

			try {
				await ctx.editMessageText(buildRuleText(config.rule, remaining), {
					parse_mode: "Markdown",
					reply_markup: {
						inline_keyboard: [
							[
								{
									text: "我以知晓",
									callback_data: `rule_ack:${groupId}:${showTime}`,
								},
							],
						],
					},
				});
			} catch {}

			await ctx.answerCallbackQuery({
				text: `还需等待 ${remaining} 秒`,
			});
			return;
		}

		const existing = await getPendingVerification(db, userId, groupId);
		const welcomeMsgId = existing?.welcome_message_id ?? undefined;

		await ctx.answerCallbackQuery({ text: "正在生成验证码..." });

		try {
			await ctx.editMessageReplyMarkup({
				reply_markup: { inline_keyboard: [] },
			});
		} catch {}

		await sendCaptcha(
			ctx.api,
			db,
			bucket,
			userId,
			groupId,
			config,
			reuseCaptcha,
			welcomeMsgId,
		);
	});
}

async function sendCaptcha(
	api: Bot["api"],
	db: D1Database,
	bucket: R2Bucket,
	userId: number,
	groupId: number,
	config: { verify_timeout: number },
	reuseCaptcha: boolean,
	welcomeMsgId?: number,
): Promise<void> {
	const captcha = await generateCaptcha(bucket, 5, reuseCaptcha);
	const captchaKey = `captcha/${groupId}/${userId}.bmp`;
	await uploadCaptchaToR2(bucket, captchaKey, captcha.bmp);

	const timeout = config.verify_timeout;
	const expiresAt = Math.floor(Date.now() / 1000) + timeout;
	await addPendingVerification(
		db,
		userId,
		groupId,
		captcha.text,
		expiresAt,
		welcomeMsgId,
	);

	try {
		await api.sendPhoto(userId, new InputFile(captcha.bmp, "captcha.bmp"), {
			caption: `请回复图片中的文字完成验证（${timeout}秒内有效）`,
		});
	} catch (error) {
		console.error(
			"Failed to send captcha:",
			error instanceof Error ? error.message : error,
		);
	}
}
