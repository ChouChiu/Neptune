import type { Bot } from "grammy";
import { registerReportCommand } from "./commands";

export function registerReportFeature(bot: Bot, db: D1Database): void {
	registerReportCommand(bot, db);
}
