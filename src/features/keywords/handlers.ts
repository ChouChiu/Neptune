import { sify } from "chinese-conv";
import type { Context } from "grammy";
import { getKeywords } from "../../shared/db/queries";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { getNickname } from "../../shared/utils/nickname";
import { replacePlaceholders } from "../../shared/utils/placeholders";
import { replyOptionsWithParse } from "../../shared/utils/reply";
import type { KeywordRule } from "../../types";

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

export async function handleKeywordMatch(
	ctx: Context,
	db: D1Database,
): Promise<boolean> {
	if (ctx.chat?.type !== "group" && ctx.chat?.type !== "supergroup")
		return false;

	const groupId = ctx.chat.id;
	const text = ctx.message?.text;
	if (!text) return false;

	const keywords = await getCachedKeywords(db, groupId);
	if (keywords.length === 0) return false;

	const matchedRule = matchKeyword(keywords, text);
	if (!matchedRule) return false;

	const nickname = ctx.from ? getNickname(ctx.from) : "unknown";

	const replyContent = replacePlaceholders(
		escapeMarkdown(matchedRule.reply_content),
		{
			nickname: escapeMarkdown(nickname),
			userid: ctx.from?.id,
			groupname: ctx.chat.title ? escapeMarkdown(ctx.chat.title) : undefined,
		},
	);

	await ctx.reply(replyContent, replyOptionsWithParse(ctx));
	return true;
}
