import type { Bot } from "grammy";
import { registerWarnCommand } from "./commands";

export function registerWarnFeature(bot: Bot, db: D1Database): void {
	registerWarnCommand(bot, db);
}
