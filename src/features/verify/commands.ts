import type { Bot } from "grammy";
import {
	getGroupConfig,
	updateVerifyButtonText,
	updateVerifyTimeout,
} from "../../shared/db/queries";
import { getBotUsername } from "../../shared/utils/botInfo";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { getNickname } from "../../shared/utils/nickname";
import { checkAdminPermission } from "../../shared/utils/permissions";
import { replyOptions, replyOptionsWithParse } from "../../shared/utils/reply";

export function registerVerifyCommands(bot: Bot, db: D1Database): void {
	bot.command("setverifybutton", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const text = ctx.match?.toString().trim();
		if (!text) {
			await ctx.reply("用法: /setverifybutton <按钮文案>", replyOptions(ctx));
			return;
		}

		await updateVerifyButtonText(db, groupId, text);
		await ctx.reply(`✅ 认证按钮文案已更新为: ${text}`, replyOptions(ctx));
	});

	bot.command("setverifytimeout", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const timeoutStr = ctx.match?.toString().trim();
		if (!timeoutStr) {
			await ctx.reply("用法: /setverifytimeout <秒数>", replyOptions(ctx));
			return;
		}

		const timeout = Number.parseInt(timeoutStr, 10);
		if (Number.isNaN(timeout) || timeout <= 0) {
			await ctx.reply("请输入有效的秒数。", replyOptions(ctx));
			return;
		}

		await updateVerifyTimeout(db, groupId, timeout);
		await ctx.reply(
			`✅ 认证超时时间已更新为: ${timeout} 秒`,
			replyOptions(ctx),
		);
	});

	bot.command("testverify", async (ctx) => {
		if (ctx.chat.type === "private") {
			await ctx.reply(
				escapeMarkdown("请在群组中使用此命令。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		// 检查管理员权限
		const { allowed } = await checkAdminPermission(db, ctx);
		if (!allowed) {
			await ctx.reply(
				escapeMarkdown("只有管理员才能使用此命令。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const groupId = ctx.chat.id;
		const config = await getGroupConfig(db, groupId);
		if (!config?.welcome_enabled) {
			await ctx.reply(
				escapeMarkdown("请先使用 /enablewelcome 启用入群欢迎。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		if (!ctx.from) return;

		const userId = ctx.from.id;
		const nickname = getNickname(ctx.from);

		const botUsername = await getBotUsername(ctx.api);
		const verifyUrl = `https://t.me/${botUsername}?start=verify${groupId}_${userId}`;

		await ctx.reply(`测试验证消息\n\n欢迎 ${nickname} 来到群组！`, {
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
	});
}
