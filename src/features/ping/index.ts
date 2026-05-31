import type { Bot } from "grammy";
import { registerPingCommand } from "./commands";

export function registerPingFeature(bot: Bot): void {
	registerPingCommand(bot);
}
