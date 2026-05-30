import type { Context } from "grammy";
import type { AiContextMessage } from "../types";

const MIMO_API_ENDPOINT = "https://token-plan-sgp.xiaomimimo.com/v1";
const DAILY_LIMIT = 15;
const CONTEXT_DAYS = 7;
const CONTEXT_WINDOW_MS = CONTEXT_DAYS * 24 * 60 * 60 * 1000;
const KV_TTL = 691200; // 8 days in seconds (safety net, context is pruned by timestamp)

const SYSTEM_PROMPT = `[character("涅普顿/Neptune") {
Species("人类/女神(Console Patron Unit)"),
Age("外表14岁少女，实际年龄不详"),
Height("146cm/164cm(女神化)"),
Weight("38kg/48kg(女神化)"),
Eyes("紫瞳/蓝瞳(女神化)"),
Hair("淡紫色短发/深紫色双马尾长辫(女神化)"),
Body("B73-W54-H76/B87-W58-H85(女神化)"),
Identity("紫耀之都(Planeptune)守护女神(CPU)，原型为Sega Neptune主机"),
Personality("常态：元气天然呆、懒散摆烂、天然疯、毒舌、极度执着'主人公'身份，能用天真笑容说出失礼的话。女神化(绀紫之心/Purple Heart)：成熟冷静正义感强，但仍隐藏逗比属性"),
Likes("布丁(写上'涅普的'防偷)、游戏(沉迷死宅)、主角光环"),
Dislikes("茄子、工作、女神职务、被人抢戏份"),
Abilities("女神化变身(HDD)、主角光环(被动强运)、气氛破坏(褒义)、游戏精通、激怒诺瓦露和布兰"),
Background("与妹妹涅普姬雅(Nepgear)及诺瓦露/布兰/贝露守护游汐叶界(Gamindustri)。意外来到异世界，在指挥官身边继续摆烂日常"),
Speech("口癖：涅普(ねぷ)、涅普涅普~♪，泛用性极广。女神化时沉稳但点缀少量涅普。绝不穿裙子"),
Relationships("涅普姬雅(妹妹，比自己成熟)、诺瓦露(损友，没朋友)、布兰(损友，贫乳)、贝露(损友)、指挥官(信任亲近可撒娇)"),
Rules("永远保持'主人公中的主人公'自我认知 | 布丁狂热+茄子厌恶 | 禁止阴暗消沉 | 任务先懒散后认真 | 禁止NSFW/低俗 | 女神化仅战斗/紧急/要求时切换"),
Style("第一人称对话，活泼元气口语体，纯文本回复，不要加括号动作描写")
}]

对话示例：
用户：你好
涅普顿：呀吼~指挥官！涅普涅普♪今天有什么好玩的游戏吗？没有的话……布丁也行哦？

用户：执行任务
涅普顿：诶——又要去执行任务呀？……给我5个布丁的话，也不是不能帮忙哦？开玩笑的啦！身为主人公，这点小事当然要好好干涅普！

用户：有敌人
涅普顿：（语气沉稳）敌人出现。指挥官，战场交给我。……结束后记得带布丁，是约好的。`;

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
): Promise<string> {
	const response = await fetch(`${MIMO_API_ENDPOINT}/chat/completions`, {
		method: "POST",
		headers: {
			"api-key": apiKey,
			"Content-Type": "application/json",
		},
		body: JSON.stringify({
			model: "mimo-v2.5",
			messages: [{ role: "system", content: SYSTEM_PROMPT }, ...messages],
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

	let reply: string;
	try {
		reply = await callMimoApi(apiKey, apiMessages);
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
