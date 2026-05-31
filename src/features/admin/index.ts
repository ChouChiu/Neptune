import type { Bot } from "grammy";
import { registerAdminCommands } from "./commands";

export function registerAdminFeature(bot: Bot, db: D1Database): void {
	registerAdminCommands(bot, db);
}
