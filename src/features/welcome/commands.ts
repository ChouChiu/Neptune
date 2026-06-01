import type { Bot } from "grammy";
import {
	setWelcomeEnabled,
	updateWelcomeMessage,
} from "../../shared/db/queries";
import { requireAdmin } from "../../shared/utils/command-guards";
import { replyOptions } from "../../shared/utils/reply";

export function registerWelcomeCommands(bot: Bot, db: D1Database): void {
	bot.command("setwelcome", async (ctx) => {
		const { allowed, groupId } = await requireAdmin(db, ctx);
		if (!allowed || !groupId) return;

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
		const { allowed, groupId } = await requireAdmin(db, ctx);
		if (!allowed || !groupId) return;

		await setWelcomeEnabled(db, groupId, true);
		await ctx.reply("✅ 入群欢迎已启用。", replyOptions(ctx));
	});

	bot.command("disablewelcome", async (ctx) => {
		const { allowed, groupId } = await requireAdmin(db, ctx);
		if (!allowed || !groupId) return;

		await setWelcomeEnabled(db, groupId, false);
		await ctx.reply("✅ 入群欢迎已禁用。", replyOptions(ctx));
	});
}
