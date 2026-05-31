import type { Bot } from "grammy";
import type { Env } from "../types";
import { registerAdminFeature } from "./admin";
import { registerAiChatFeature } from "./ai-chat";
import { registerHelpFeature } from "./help";
import { registerKeywordsFeature } from "./keywords";
import { registerMessageOrchestrator } from "./message-orchestrator";
import { registerPingFeature } from "./ping";
import { registerReportFeature } from "./report";
import { registerRuleFeature } from "./rule";
import { registerVerifyFeature } from "./verify";
import { registerVotekickFeature } from "./votekick";
import { registerWarnFeature } from "./warn";
import { registerWelcomeFeature } from "./welcome";

export function registerFeatures(bot: Bot, env: Env): void {
	registerHelpFeature(bot);
	registerPingFeature(bot);
	registerAdminFeature(bot, env.db);
	registerWelcomeFeature(bot, env.db);
	registerVerifyFeature(bot, env.db, env.captcha, env.REUSE_CAPTCHA === "true");
	registerRuleFeature(bot, env.db);
	registerKeywordsFeature(bot, env.db);
	registerAiChatFeature(bot, env.db, env.MIMO_API_KEY, env.aiContext);
	registerVotekickFeature(bot, env.db);
	registerWarnFeature(bot, env.db);
	registerReportFeature(bot, env.db);
	registerMessageOrchestrator(bot, env);
}
