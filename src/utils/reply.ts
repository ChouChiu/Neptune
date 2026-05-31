import type { Context } from "grammy";

export function replyOptions(ctx: Context) {
	return {
		reply_parameters: ctx.message
			? { message_id: ctx.message.message_id }
			: undefined,
	};
}

export function replyOptionsWithParse(ctx: Context) {
	return {
		parse_mode: "MarkdownV2" as const,
		reply_parameters: ctx.message
			? { message_id: ctx.message.message_id }
			: undefined,
	};
}
