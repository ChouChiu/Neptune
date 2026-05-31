import { Bot } from "grammy";
import { registerFeatures } from "./features";
import type { Env } from "./types";

export function createBot(env: Env): Bot {
	const bot = new Bot(env.BOT_TOKEN);
	registerFeatures(bot, env);
	return bot;
}
