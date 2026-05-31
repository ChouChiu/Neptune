import type { Bot } from "grammy";
import { addReport } from "../../shared/db/queries";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { replyOptionsWithParse } from "../../shared/utils/reply";

export function registerReportCommand(bot: Bot, db: D1Database): void {
	bot.command("report", async (ctx) => {
		if (ctx.chat.type === "private") {
			await ctx.reply(
				escapeMarkdown("此命令只能在群组中使用。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const replyMsg = ctx.message?.reply_to_message;
		if (!replyMsg?.from) {
			await ctx.reply(
				escapeMarkdown("请回复目标用户的消息以提交举报。"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

		const targetUser = replyMsg.from;
		if (targetUser.is_bot) {
			await ctx.reply(
				escapeMarkdown("涅普不能举报机器人哦～"),
				replyOptionsWithParse(ctx),
			);
			return;
		}

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

		await addReport(
			db,
			ctx.chat.id,
			from.id,
			targetUser.id,
			replyMsg.message_id,
			content,
		);

		const text =
			`✉️ 涅普已经收到举报啦，会尽快处理～\n\n` +
			`📝 举报内容：${escapeMarkdown(content)}`;

		await ctx.reply(text, replyOptionsWithParse(ctx));
	});
}
