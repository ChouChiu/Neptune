import { sify } from "chinese-conv";
import type { Bot } from "grammy";
import {
	getKeywords,
	isAdminConnected,
	removePendingVerification,
} from "../db/queries";
import type { KeywordRule } from "../types";
import { getChatResponse, shouldTriggerAi } from "../utils/ai-chat";
import { getNickname } from "../utils/nickname";
import { replacePlaceholders } from "../utils/placeholders";
import {
	escapeMarkdown,
	replyOptions,
	replyOptionsWithParse,
} from "../utils/reply";

interface CompiledKeywordRule {
	rule: KeywordRule;
	regex?: RegExp;
}

interface KeywordCacheEntry {
	rules: CompiledKeywordRule[];
	expiresAt: number;
}

const keywordCache = new Map<number, KeywordCacheEntry>();
const KEYWORD_CACHE_TTL = 60_000;

async function getCachedKeywords(
	db: D1Database,
	groupId: number,
): Promise<CompiledKeywordRule[]> {
	const cached = keywordCache.get(groupId);
	if (cached && cached.expiresAt > Date.now()) {
		return cached.rules;
	}
	const rawRules = await getKeywords(db, groupId);
	const rules: CompiledKeywordRule[] = rawRules.map((rule) => {
		if (rule.is_regex) {
			try {
				return { rule, regex: new RegExp(rule.pattern, "i") };
			} catch {
				return { rule };
			}
		}
		return { rule };
	});
	keywordCache.set(groupId, {
		rules,
		expiresAt: Date.now() + KEYWORD_CACHE_TTL,
	});
	return rules;
}

export function registerMessageHandler(
	bot: Bot,
	db: D1Database,
	apiKey: string,
	kv: KVNamespace,
): void {
	bot.on("message", async (ctx) => {
		if (!ctx.message || !ctx.from) return;

		const userId = ctx.from.id;

		if (ctx.chat?.type === "private" && ctx.message.text) {
			const text = ctx.message.text;

			// 跳过命令
			if (text.startsWith("/")) return;

			const verifications = await db
				.prepare(
					"SELECT * FROM pending_verifications WHERE user_id = ? AND expires_at > ?",
				)
				.bind(userId, Math.floor(Date.now() / 1000))
				.all<{
					user_id: number;
					group_id: number;
					captcha_text: string;
					expires_at: number;
					welcome_message_id: number | null;
					attempts: number;
				}>();

			const MAX_ATTEMPTS = 5;

			for (const verification of verifications.results) {
				const groupId = verification.group_id;

				if (verification.attempts >= MAX_ATTEMPTS) {
					await removePendingVerification(db, userId, groupId);
					await ctx.reply(
						"验证失败次数过多，请重新加入群组。",
						replyOptions(ctx),
					);
					continue;
				}

				if (text.toUpperCase() === verification.captcha_text.toUpperCase()) {
					// 删除欢迎消息
					if (verification.welcome_message_id) {
						try {
							await ctx.api.deleteMessage(
								groupId,
								verification.welcome_message_id,
							);
						} catch (error) {
							console.error("Failed to delete welcome message:", error);
						}
					}

					await removePendingVerification(db, userId, groupId);

					try {
						await ctx.api.restrictChatMember(groupId, userId, {
							can_send_messages: true,
							can_send_audios: true,
							can_send_documents: true,
							can_send_photos: true,
							can_send_videos: true,
							can_send_video_notes: true,
							can_send_voice_notes: true,
							can_send_polls: true,
							can_send_other_messages: true,
							can_add_web_page_previews: true,
							can_change_info: true,
							can_invite_users: true,
							can_pin_messages: true,
							can_manage_topics: true,
						});
						await ctx.reply(
							"✅ 验证成功！你现在可以在群组中发言了。",
							replyOptions(ctx),
						);
					} catch (error) {
						console.error("Failed to unrestrict user:", error);
						await ctx.reply(
							"验证成功，但解除限制失败。请联系管理员。",
							replyOptions(ctx),
						);
					}
				} else {
					await db
						.prepare(
							"UPDATE pending_verifications SET attempts = attempts + 1 WHERE user_id = ? AND group_id = ?",
						)
						.bind(userId, groupId)
						.run();
					const remaining = MAX_ATTEMPTS - verification.attempts - 1;
					if (remaining > 0) {
						await ctx.reply(
							`❌ 验证码错误，还剩 ${remaining} 次机会。`,
							replyOptions(ctx),
						);
					} else {
						await removePendingVerification(db, userId, groupId);
						await ctx.reply(
							"❌ 验证失败次数过多，请重新加入群组。",
							replyOptions(ctx),
						);
					}
				}
				return;
			}
		}

		if (ctx.chat?.type === "group" || ctx.chat?.type === "supergroup") {
			const groupId = ctx.chat.id;
			const text = ctx.message.text;

			if (!text) return;

			const botId = ctx.me?.id ?? bot.botInfo.id;
			if (shouldTriggerAi(ctx, botId)) {
				try {
					const userMessage = text.replace(/@\w+/g, "").trim();
					if (!userMessage) return;

					const quoted = ctx.message.reply_to_message;
					const quotedText =
						quoted?.text ?? (quoted as { caption?: string })?.caption;
					const finalMessage = quotedText
						? `[引用消息] ${quotedText}\n\n${userMessage}`
						: userMessage;

					await ctx.replyWithChatAction("typing");

					const chatMember = await ctx.getChatMember(userId);
					const isAdmin =
						chatMember.status === "creator" ||
						chatMember.status === "administrator" ||
						(await isAdminConnected(db, userId, groupId));

					let memberCount: number | undefined;
					try {
						memberCount = await ctx.getChatMemberCount();
					} catch {}

					console.log("AI chat request:", {
						groupId,
						userId,
						isAdmin,
						messageLength: finalMessage.length,
					});

					const reply = await getChatResponse(
						db,
						kv,
						apiKey,
						groupId,
						userId,
						finalMessage,
						isAdmin,
						{
							title: ctx.chat.title,
							memberCount,
						},
					);

					console.log("AI chat response:", {
						length: reply.length,
						preview: reply.substring(0, 50),
					});
					await ctx.reply(reply, replyOptionsWithParse(ctx));
				} catch (error) {
					console.error("AI chat error:", error);
					try {
						await ctx.reply(
							"涅普？！出了点状况，主角光环暂时失效了……再试一次吧！",
							replyOptions(ctx),
						);
					} catch (replyError) {
						console.error("Failed to send error reply:", replyError);
					}
				}
				return;
			}

			const keywords = await getCachedKeywords(db, groupId);
			if (keywords.length === 0) return;

			const matchedRule = matchKeyword(keywords, text);
			if (!matchedRule) return;

			const nickname = ctx.from ? getNickname(ctx.from) : "unknown";

			const replyContent = replacePlaceholders(matchedRule.reply_content, {
				nickname: escapeMarkdown(nickname),
				userid: ctx.from?.id,
				groupname: ctx.chat.title ? escapeMarkdown(ctx.chat.title) : undefined,
			});

			await ctx.reply(replyContent, replyOptionsWithParse(ctx));
		}
	});
}

function matchKeyword(
	keywords: CompiledKeywordRule[],
	text: string,
): KeywordRule | null {
	const lowerText = text.toLowerCase();
	const simplifiedText = sify(text).toLowerCase();
	const hasTraditionalText = simplifiedText !== lowerText;

	for (const compiled of keywords) {
		const rule = compiled.rule;
		if (rule.is_regex) {
			try {
				const regex = compiled.regex ?? new RegExp(rule.pattern, "i");
				if (regex.test(text)) return rule;
				if (hasTraditionalText && regex.test(simplifiedText)) return rule;
				const simplifiedPattern = sify(rule.pattern);
				if (simplifiedPattern !== rule.pattern) {
					const regex2 = new RegExp(simplifiedPattern, "i");
					if (regex2.test(lowerText) || regex2.test(simplifiedText))
						return rule;
				}
			} catch {}
		} else {
			if (simplifiedText.includes(sify(rule.pattern).toLowerCase())) {
				return rule;
			}
		}
	}
	return null;
}
