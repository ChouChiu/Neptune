import type { Bot } from "grammy";
import { replyOptions } from "../utils/reply";

export function registerPingCommand(bot: Bot): void {
	bot.command("ping", async (ctx) => {
		await ctx.reply("🏓 Pong!", replyOptions(ctx));
	});
}
