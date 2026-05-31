import type { Bot } from "grammy";
import { registerVotekickCommands } from "./commands";
import { registerVotekickHandler } from "./handlers";

export function registerVotekickFeature(bot: Bot, db: D1Database): void {
	registerVotekickCommands(bot, db);
	registerVotekickHandler(bot, db);
}
