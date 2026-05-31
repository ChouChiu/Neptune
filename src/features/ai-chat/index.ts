import type { Bot } from "grammy";

export { handleAiChat } from "./handlers";

export function registerAiChatFeature(
	_bot: Bot,
	_db: D1Database,
	_apiKey: string,
	_kv: KVNamespace,
): void {
	// AI chat is triggered via message-orchestrator, no standalone registration needed
}
