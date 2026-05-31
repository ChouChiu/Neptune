import type { Bot } from "grammy";
import { registerKeywordCommands } from "./commands";

export { handleKeywordMatch } from "./handlers";

export function registerKeywordsFeature(bot: Bot, db: D1Database): void {
	registerKeywordCommands(bot, db);
}
