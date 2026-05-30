import type { Context } from "grammy";
import type { AiContextMessage } from "../types";
import { DEFAULT_SKILL, matchSkills, skillToText } from "./skills";
import systemPromptData from "./system-prompt.json";

const MIMO_API_ENDPOINT = "https://token-plan-sgp.xiaomimimo.com/v1";
const DAILY_LIMIT = 15;
const CONTEXT_DAYS = 7;
const CONTEXT_WINDOW_MS = CONTEXT_DAYS * 24 * 60 * 60 * 1000;
const KV_TTL = 691200; // 8 days in seconds (safety net, context is pruned by timestamp)

interface SystemPromptData {
	character: Record<string, unknown>;
	examples: { user: string; reply: string }[];
}

function formatCharacterField(key: string, value: unknown): string {
	if (Array.isArray(value)) return `${key}(${value.join(" | ")})`;
	if (typeof value === "object" && value !== null) {
		const inner = Object.entries(value)
			.map(([k, v]) => `${k}: ${v}`)
			.join("；");
		return `${key}(${inner})`;
	}
	return `${key}(${value})`;
}

function systemPromptToText(data: SystemPromptData): string {
	const char = data.character;
	const lines = Object.entries(char).map(([key, value]) =>
		formatCharacterField(key.charAt(0).toUpperCase() + key.slice(1), value),
	);
	const charBlock = `[character("${char.name}") {\n${lines.slice(1).join(",\n")}\n}]`;

	const examples = data.examples
		.map((ex) => `用户：${ex.user}\n涅普顿：${ex.reply}`)
		.join("\n\n");

	return `${charBlock}\n\n对话示例：\n${examples}`;
}

const data = systemPromptData as SystemPromptData;
const SYSTEM_PROMPT = systemPromptToText(data);

export function getTodayDate(): string {
	const now = new Date();
	return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

export function shouldTriggerAi(ctx: Context, botId: number): boolean {
	const message = ctx.message;
	if (!message) return false;

	if (message.entities) {
		for (const entity of message.entities) {
			if (entity.type === "mention" || entity.type === "text_mention") {
				if (entity.type === "mention") {
					const mentionedText = message.text?.substring(
						entity.offset,
						entity.offset + entity.length,
					);
					if (mentionedText && ctx.me?.username) {
						if (
							mentionedText.toLowerCase() ===
							`@${ctx.me.username.toLowerCase()}`
						) {
							return true;
						}
					}
				} else if (
					entity.type === "text_mention" &&
					entity.user?.id === botId
				) {
					return true;
				}
			}
		}
	}

	if (message.reply_to_message?.from?.id === botId) {
		const repliedText = message.reply_to_message.text || "";
		if (
			repliedText.includes("验证") ||
			repliedText.includes("欢迎") ||
			repliedText.includes("命令") ||
			repliedText.includes("踢人") ||
			repliedText.includes("投票")
		) {
			return false;
		}
		return true;
	}

	return false;
}

export async function getAiContext(
	kv: KVNamespace,
	groupId: number,
): Promise<AiContextMessage[]> {
	const key = `ai:context:${groupId}`;
	const data = await kv.get(key, "json");
	if (!data) return [];
	return data as AiContextMessage[];
}

export async function updateAiContext(
	kv: KVNamespace,
	groupId: number,
	messages: AiContextMessage[],
): Promise<void> {
	const key = `ai:context:${groupId}`;
	const cutoff = Date.now() - CONTEXT_WINDOW_MS;
	const trimmed = messages.filter((msg) => (msg.timestamp ?? 0) >= cutoff);
	await kv.put(key, JSON.stringify(trimmed), { expirationTtl: KV_TTL });
}

export async function getAiUsageCount(
	db: D1Database,
	userId: number,
	groupId: number,
	date: string,
): Promise<number> {
	const result = await db
		.prepare(
			"SELECT count FROM ai_chat_usage WHERE user_id = ? AND group_id = ? AND date = ?",
		)
		.bind(userId, groupId, date)
		.first<{ count: number }>();
	return result?.count ?? 0;
}

export async function incrementAiUsage(
	db: D1Database,
	userId: number,
	groupId: number,
	date: string,
): Promise<void> {
	await db
		.prepare(
			`INSERT INTO ai_chat_usage (user_id, group_id, date, count)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(user_id, group_id, date)
			DO UPDATE SET count = count + 1`,
		)
		.bind(userId, groupId, date)
		.run();
}

export async function callMimoApi(
	apiKey: string,
	messages: { role: string; content: string }[],
	systemPrompt: string = SYSTEM_PROMPT,
): Promise<string> {
	const response = await fetch(`${MIMO_API_ENDPOINT}/chat/completions`, {
		method: "POST",
		headers: {
			"api-key": apiKey,
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			model: "mimo-v2.5",
			messages: [{ role: "system", content: systemPrompt }, ...messages],
			stream: false,
			temperature: 1.0,
			top_p: 0.95,
			max_completion_tokens: 2048,
		}),
	});

	if (!response.ok) {
		const errorText = await response.text();
		throw new Error(`MiMo API error: ${response.status} - ${errorText}`);
	}

	const data = (await response.json()) as {
		choices: { message: { content: string } }[];
	};
	return data.choices[0]?.message?.content ?? "";
}

export async function getChatResponse(
	db: D1Database,
	kv: KVNamespace,
	apiKey: string,
	groupId: number,
	userId: number,
	userMessage: string,
	isAdmin: boolean = false,
	groupContext?: { title?: string; memberCount?: number },
): Promise<string> {
	const today = getTodayDate();
	const usage = await getAiUsageCount(db, userId, groupId, today);

	if (!isAdmin && usage >= DAILY_LIMIT) {
		return "涅普涅普~今天的主角光环能量用完啦！明天再来找涅普玩吧~♪（每日限额15次）";
	}

	const context = await getAiContext(kv, groupId);

	context.push({
		role: "user",
		content: userMessage,
		userId,
		timestamp: Date.now(),
	});

	const apiMessages = context.map((msg) => ({
		role: msg.role,
		content: msg.content,
	}));

	const matchedSkills = matchSkills(userMessage);
	let systemPrompt = `${SYSTEM_PROMPT}\n${skillToText(DEFAULT_SKILL)}`;
	if (matchedSkills.length > 0) {
		systemPrompt += `\n${matchedSkills.map((s) => skillToText(s)).join("\n")}`;
	}
	if (groupContext) {
		systemPrompt += `\n\n[当前群组信息]\n群组名称：${groupContext.title ?? "未知群组"}\n群组ID：${groupId}${groupContext.memberCount ? `\n成员数：${groupContext.memberCount}` : ""}\n\n请根据群组氛围自然地回应，可以适当提及群组相关的话题。`;
	}

	let reply: string;
	try {
		reply = await callMimoApi(apiKey, apiMessages, systemPrompt);
	} catch (error) {
		console.error("MiMo API call failed:", error);
		return "涅普？！刚才好像有什么东西掉线了……主角的网络冒险失败了一次，再试一次吧！";
	}

	const MAX_REPLY_LENGTH = 2048;
	if (reply.length > MAX_REPLY_LENGTH) {
		reply = `${reply.substring(0, MAX_REPLY_LENGTH - 20)}……涅普！说得太多了啦~`;
	}

	context.push({
		role: "assistant",
		content: reply,
		timestamp: Date.now(),
	});

	await updateAiContext(kv, groupId, context);
	if (!isAdmin) await incrementAiUsage(db, userId, groupId, today);

	if (isAdmin) return reply;

	const remaining = DAILY_LIMIT - usage - 1;
	return `${reply}\n\n_剩余次数: ${remaining}/${DAILY_LIMIT}_`;
}
