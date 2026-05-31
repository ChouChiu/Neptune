import type { Bot } from "grammy";
import { addWarning, getWarningCount } from "../../shared/db/queries";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { getNickname } from "../../shared/utils/nickname";
import { checkAdminPermission } from "../../shared/utils/permissions";
import { replyOptionsWithParse } from "../../shared/utils/reply";

export function registerWarnCommand(bot: Bot, db: D1Database): void {
	bot.command("warn", async (ctx) => {
		if (ctx.chat.type === "private") {
			await ctx.reply(
				escapeMarkdown("此命令只能在群组中使用。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply(
				escapeMarkdown("你没有权限执行此操作。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const replyMsg = ctx.message?.reply_to_message;
		if (!replyMsg?.from) {
			await ctx.reply(
				escapeMarkdown("请回复目标用户的消息以进行警告。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const targetUser = replyMsg.from;
		if (targetUser.is_bot) {
			await ctx.reply(
				escapeMarkdown("涅普不能警告机器人哦～"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		try {
			const targetMember = await ctx.api.getChatMember(
				ctx.chat.id,
				targetUser.id,
			);
			if (["administrator", "creator"].includes(targetMember.status)) {
				await ctx.reply(
					escapeMarkdown("涅普不能警告管理员哦～"),
					replyOptionsWithParse(ctx),
				);
				return;
			}
		} catch {
			// User may have left; allow warning to proceed
		}

		const from = ctx.from;
		if (!from) return;

		const reason = ctx.match?.toString().trim() ?? "";

		await addWarning(db, groupId, targetUser.id, from.id, reason);

		const count = await getWarningCount(db, groupId, targetUser.id);
		const nickname = getNickname(targetUser);

		let text: string;
		if (reason) {
			text = `⚠️ 涅普警告了 ${escapeMarkdown(nickname)}！\n\n`;
			text += `📝 原因：${escapeMarkdown(reason)}\n`;
			text += `📊 这是该用户的第 ${count} 次警告～`;
		} else {
			text = `⚠️ 涅普警告了 ${escapeMarkdown(nickname)}！\n`;
			text += `📊 这是该用户的第 ${count} 次警告～`;
		}

		await ctx.reply(text, replyOptionsWithParse(ctx));
	});
}
