import type { Bot } from "grammy";
import { addReport } from "../../shared/db/queries";
import {
	requireGroup,
	requireNonBot,
	requireReplyTarget,
} from "../../shared/utils/command-guards";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { replyOptionsWithParse } from "../../shared/utils/reply";

export function registerReportCommand(bot: Bot, db: D1Database): void {
	bot.command("report", async (ctx) => {
		const { allowed: groupAllowed, groupId } = await requireGroup(ctx);
		if (!groupAllowed || !groupId) return;

		const { allowed: replyAllowed, target } = requireReplyTarget(ctx);
		if (!replyAllowed || !target) return;

		if (!(await requireNonBot(ctx, target))) return;

		const from = ctx.from;
		if (!from) return;

		const content = ctx.match?.toString().trim() ?? "";
		if (!content) {
			await ctx.reply(
				escapeMarkdown("请填写举报内容。用法: /report <举报原因>"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const reportedText =
			ctx.message?.reply_to_message?.text ||
			ctx.message?.reply_to_message?.caption ||
			"";

		await addReport(
			db,
			groupId,
			from.id,
			target.id,
			ctx.message?.reply_to_message?.message_id ?? 0,
			reportedText,
			content,
		);

		const text =
			`✉️ 涅普已经收到举报啦，会尽快处理～\n\n` +
			`📝 举报内容：${escapeMarkdown(content)}`;

		await ctx.reply(text, replyOptionsWithParse(ctx));
	});
}
