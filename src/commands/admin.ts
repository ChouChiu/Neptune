import type { Bot, CommandContext, Context } from "grammy";
import {
	connectAdmin,
	getAdminGroupId,
	getAdminGroups,
	getGroupConfig,
	setCurrentGroup,
} from "../db/queries";
import { getBotUsername } from "../utils/botInfo";
import { getNickname } from "../utils/nickname";
import { checkAdminPermission } from "../utils/permissions";
import { replyOptions, replyOptionsWithParse } from "../utils/reply";

export function registerAdminCommands(bot: Bot, db: D1Database): void {
	bot.command("id", async (ctx) => {
		const chat = ctx.chat;
		if (chat.type === "private") {
			await ctx.reply(
				"请在群组中使用此命令获取群组 ID。",
				replyOptionsWithParse(ctx),
			);
			return;
		}
		// 自动关联群组
		if (ctx.from) {
			await connectAdmin(db, ctx.from.id, chat.id);
		}
		await ctx.reply(`当前群组 ID: \`${chat.id}\``, replyOptionsWithParse(ctx));
	});

	bot.command("connect", async (ctx) => {
		const chat = ctx.chat;
		if (chat.type !== "private") {
			await ctx.reply("请在私聊中使用此命令。", replyOptionsWithParse(ctx));
			return;
		}

		const args = ctx.match?.toString().trim();
		if (!args) {
			await ctx.reply("用法: /connect <群组ID>", replyOptionsWithParse(ctx));
			return;
		}

		const groupId = Number.parseInt(args, 10);
		if (Number.isNaN(groupId)) {
			await ctx.reply("无效的群组 ID。", replyOptionsWithParse(ctx));
			return;
		}

		if (!ctx.from) {
			await ctx.reply("无法获取用户信息。", replyOptionsWithParse(ctx));
			return;
		}

		// 检查用户是否是群组的 Telegram 管理员
		try {
			const chatMember = await ctx.api.getChatMember(groupId, ctx.from.id);
			if (!["administrator", "creator"].includes(chatMember.status)) {
				await ctx.reply(
					"你不是该群组的管理员，无法绑定。",
					replyOptionsWithParse(ctx),
				);
				return;
			}
		} catch (error) {
			await ctx.reply(
				`无法验证群组权限: ${error instanceof Error ? error.message : String(error)}`,
				replyOptionsWithParse(ctx),
			);
			return;
		}

		await connectAdmin(db, ctx.from.id, groupId);
		await ctx.reply(
			`已绑定到群组 ${groupId}。现在可以在私聊中管理该群组。`,
			replyOptionsWithParse(ctx),
		);
	});

	bot.command("switch", async (ctx) => {
		if (ctx.chat.type !== "private") {
			await ctx.reply("请在私聊中使用此命令。", replyOptions(ctx));
			return;
		}

		if (!ctx.from) return;

		const groupIds = await getAdminGroups(db, ctx.from.id);
		if (groupIds.length === 0) {
			await ctx.reply(
				"你还没有绑定任何群组。使用 /connect <群组ID> 绑定。",
				replyOptions(ctx),
			);
			return;
		}

		const currentGroupId = await getAdminGroupId(db, ctx.from.id);

		const buttons = [];
		for (const gid of groupIds) {
			const label = `${gid === currentGroupId ? "✅ " : ""}${gid}`;
			buttons.push([{ text: label, callback_data: `switch:${gid}` }]);
		}

		await ctx.reply("选择当前管理的群组：", {
			reply_markup: { inline_keyboard: buttons },
		});
	});

	bot.command("testverify", async (ctx) => {
		if (ctx.chat.type === "private") {
			await ctx.reply("请在群组中使用此命令。", replyOptionsWithParse(ctx));
			return;
		}

		// 检查管理员权限
		const { allowed } = await checkAdminPermission(db, ctx);
		if (!allowed) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptionsWithParse(ctx));
			return;
		}

		const groupId = ctx.chat.id;
		const config = await getGroupConfig(db, groupId);
		if (!config?.welcome_enabled) {
			await ctx.reply(
				"请先使用 /enablewelcome 启用入群欢迎。",
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

	bot.callbackQuery(/^switch:(-?\d+)$/, async (ctx) => {
		const match = ctx.match;
		if (!match?.[1]) return;

		const groupId = Number.parseInt(match[1], 10);
		if (Number.isNaN(groupId)) return;

		if (!ctx.from) return;

		await setCurrentGroup(db, ctx.from.id, groupId);
		await ctx.answerCallbackQuery({ text: `已切换到群组 ${groupId}` });

		// 更新消息
		const groupIds = await getAdminGroups(db, ctx.from.id);
		const buttons = [];
		for (const gid of groupIds) {
			buttons.push([
				{
					text: `${gid === groupId ? "✅ " : ""}${gid}`,
					callback_data: `switch:${gid}`,
				},
			]);
		}

		await ctx.editMessageText("选择当前管理的群组：", {
			reply_markup: { inline_keyboard: buttons },
		});
	});
}

export async function resolveGroupId(
	db: D1Database,
	ctx: CommandContext<Context>,
): Promise<number | null> {
	if (ctx.chat.type === "private") {
		if (!ctx.from) return null;
		const groupId = await getAdminGroupId(db, ctx.from.id);
		if (!groupId) {
			return null;
		}
		return groupId;
	}
	// 群组中自动添加关联
	if (ctx.from) {
		await connectAdmin(db, ctx.from.id, ctx.chat.id);
	}
	return ctx.chat.id;
}
