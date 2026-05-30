import type { Bot } from "grammy";
import { getGroupConfig, updateGroupRule } from "../db/queries";
import { checkAdminPermission } from "../utils/permissions";
import { replyOptions } from "../utils/reply";

export function registerRuleCommands(bot: Bot, db: D1Database): void {
	bot.command("rule", async (ctx) => {
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const rule = ctx.match?.toString().trim();

		if (!rule) {
			const config = await getGroupConfig(db, groupId);
			if (config?.rule) {
				await ctx.reply(
					`当前群规:\n\n${config.rule}\n\n使用 /rule <内容> 修改群规\n使用 /rule off 清除群规`,
					replyOptions(ctx),
				);
			} else {
				await ctx.reply(
					"当前未设置群规。\n\n使用 /rule <内容> 设置群规",
					replyOptions(ctx),
				);
			}
			return;
		}

		if (rule === "off") {
			await updateGroupRule(db, groupId, "");
			await ctx.reply("✅ 群规已清除。", replyOptions(ctx));
			return;
		}

		await updateGroupRule(db, groupId, rule);
		await ctx.reply(
			"✅ 群规已设置。入群认证时将强制阅读群规。",
			replyOptions(ctx),
		);
	});
}
