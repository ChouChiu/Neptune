import type { Bot } from "grammy";
import { updateVerifyButtonText, updateVerifyTimeout } from "../db/queries";
import { checkAdminPermission } from "../utils/permissions";
import { replyOptions } from "../utils/reply";

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
}
