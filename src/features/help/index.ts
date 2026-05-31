import type { Bot } from "grammy";
import { registerHelpCommand } from "./commands";

export function registerHelpFeature(bot: Bot): void {
	registerHelpCommand(bot);
}
