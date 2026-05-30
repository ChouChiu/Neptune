import type { Bot } from "grammy";
import { setWelcomeEnabled, updateWelcomeMessage } from "../db/queries";
import { checkAdminPermission } from "../utils/permissions";
import { replyOptions } from "../utils/reply";

export function registerWelcomeCommands(bot: Bot, db: D1Database): void {
	bot.command("setwelcome", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const message = ctx.match?.toString().trim();
		if (!message) {
			await ctx.reply(
				"用法: /setwelcome <消息>\n支持占位符: {nickname} {userid} {groupname}",
				replyOptions(ctx),
			);
			return;
		}

		if (message.length > 4096) {
			await ctx.reply("欢迎消息过长（最大 4096 字符）。", replyOptions(ctx));
			return;
		}

		await updateWelcomeMessage(db, groupId, message);
		await ctx.reply("✅ 欢迎消息已更新。", replyOptions(ctx));
	});

	bot.command("enablewelcome", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		await setWelcomeEnabled(db, groupId, true);
		await ctx.reply("✅ 入群欢迎已启用。", replyOptions(ctx));
	});

	bot.command("disablewelcome", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		await setWelcomeEnabled(db, groupId, false);
		await ctx.reply("✅ 入群欢迎已禁用。", replyOptions(ctx));
	});
}
