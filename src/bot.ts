import { Bot } from "grammy";
import { registerAdminCommands } from "./commands/admin";
import { registerHelpCommand } from "./commands/help";
import { registerKeywordCommands } from "./commands/keywords";
import { registerPingCommand } from "./commands/ping";
import { registerVerifyCommands } from "./commands/verify";
import { registerVotekickCommands } from "./commands/votekick";
import { registerWelcomeCommands } from "./commands/welcome";
import { registerChatMemberHandler } from "./handlers/chatMember";
import { registerMessageHandler } from "./handlers/message";
import { registerVotekickHandler } from "./handlers/votekick";
import type { Env } from "./types";

export function createBot(env: Env): Bot {
	const bot = new Bot(env.BOT_TOKEN);

	registerHelpCommand(bot);
	registerPingCommand(bot);
	registerAdminCommands(bot, env.db);
	registerWelcomeCommands(bot, env.db);
	registerVerifyCommands(bot, env.db);
	registerKeywordCommands(bot, env.db);
	registerVotekickCommands(bot, env.db);
	registerChatMemberHandler(
		bot,
		env.db,
		env.captcha,
		env.REUSE_CAPTCHA === "true",
	);
	registerMessageHandler(bot, env.db, env.MIMO_API_KEY, env.aiContext);
	registerVotekickHandler(bot, env.db);

	return bot;
}
