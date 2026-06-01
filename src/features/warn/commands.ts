import type { Bot } from "grammy";
import { addWarning, getWarningCount } from "../../shared/db/queries";
import {
	requireAdmin,
	requireGroup,
	requireNonBot,
	requireReplyTarget,
} from "../../shared/utils/command-guards";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { getNickname } from "../../shared/utils/nickname";
import { replyOptionsWithParse } from "../../shared/utils/reply";

export function registerWarnCommand(bot: Bot, db: D1Database): void {
	bot.command("warn", async (ctx) => {
		const { allowed: groupAllowed, groupId } = await requireGroup(ctx);
		if (!groupAllowed || !groupId) return;

		const { allowed, groupId: adminGroupId } = await requireAdmin(db, ctx);
		if (!allowed || !adminGroupId) return;

		const { allowed: replyAllowed, target } = requireReplyTarget(ctx);
		if (!replyAllowed || !target) return;

		if (!(await requireNonBot(ctx, target))) return;

		try {
			const targetMember = await ctx.api.getChatMember(ctx.chat.id, target.id);
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

		await addWarning(db, adminGroupId, target.id, from.id, reason);

		const count = await getWarningCount(db, adminGroupId, target.id);
		const nickname = getNickname(target);

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
