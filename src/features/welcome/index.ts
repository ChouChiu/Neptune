import type { Bot } from "grammy";
import { registerWelcomeCommands } from "./commands";
import { registerWelcomeHandlers } from "./handlers";

export function registerWelcomeFeature(bot: Bot, db: D1Database): void {
	registerWelcomeCommands(bot, db);
	registerWelcomeHandlers(bot, db);
}
