import type { Bot } from "grammy";
import { registerRuleCommands } from "./commands";

export function registerRuleFeature(bot: Bot, db: D1Database): void {
	registerRuleCommands(bot, db);
}
