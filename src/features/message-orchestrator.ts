import type { Bot } from "grammy";
import type { Env } from "../types";
import { handleAiChat } from "./ai-chat/handlers";
import { handleKeywordMatch } from "./keywords/handlers";
import { handleCaptchaReply } from "./verify/captcha-handler";

export function registerMessageOrchestrator(
	bot: Bot,
	env: Env,
	executionCtx?: ExecutionContext,
): void {
	bot.on("message", async (ctx) => {
		if (!ctx.message || !ctx.from) return;

		if (ctx.chat?.type === "private") {
			if (await handleCaptchaReply(ctx, env.db)) return;
		}

		if (ctx.chat?.type === "group" || ctx.chat?.type === "supergroup") {
			if (
				await handleAiChat(
					ctx,
					bot,
					env.db,
					env.MIMO_API_KEY,
					env.aiContext,
					executionCtx,
				)
			)
				return;
			await handleKeywordMatch(ctx, env.db);
		}
	});
}
