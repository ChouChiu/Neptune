import type { Bot } from "grammy";
import { addKeyword, getKeywords, removeKeyword } from "../db/queries";
import { checkAdminPermission } from "../utils/permissions";
import {
	escapeMarkdown,
	replyOptions,
	replyOptionsWithParse,
} from "../utils/reply";

export function registerKeywordCommands(bot: Bot, db: D1Database): void {
	bot.command("addkeyword", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const args = ctx.match?.toString().trim();
		if (!args) {
			await ctx.reply(
				"用法: /addkeyword <关键词> <回复内容>\n支持占位符: {nickname} {userid}",
				replyOptions(ctx),
			);
			return;
		}

		const spaceIndex = args.indexOf(" ");
		if (spaceIndex === -1) {
			await ctx.reply("请同时提供关键词和回复内容。", replyOptions(ctx));
			return;
		}

		const keyword = args.slice(0, spaceIndex);
		const reply = args.slice(spaceIndex + 1);

		if (keyword.length > 200) {
			await ctx.reply("关键词过长（最大 200 字符）。", replyOptions(ctx));
			return;
		}

		if (reply.length > 4096) {
			await ctx.reply("回复内容过长（最大 4096 字符）。", replyOptions(ctx));
			return;
		}

		await addKeyword(db, groupId, keyword, false, reply);
		await ctx.reply(`✅ 已添加关键词规则: ${keyword}`, replyOptions(ctx));
	});

	bot.command("addregex", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const args = ctx.match?.toString().trim();
		if (!args) {
			await ctx.reply(
				"用法: /addregex <正则表达式> <回复内容>",
				replyOptions(ctx),
			);
			return;
		}

		const spaceIndex = args.indexOf(" ");
		if (spaceIndex === -1) {
			await ctx.reply("请同时提供正则表达式和回复内容。", replyOptions(ctx));
			return;
		}

		const pattern = args.slice(0, spaceIndex);
		const reply = args.slice(spaceIndex + 1);

		if (pattern.length > 200) {
			await ctx.reply("正则表达式过长（最大 200 字符）。", replyOptions(ctx));
			return;
		}

		try {
			const regex = new RegExp(pattern);
			const testStart = Date.now();
			regex.test("a".repeat(1000));
			if (Date.now() - testStart > 100) {
				await ctx.reply(
					"正则表达式过于复杂，可能导致性能问题。",
					replyOptions(ctx),
				);
				return;
			}
			await addKeyword(db, groupId, pattern, true, reply);
			await ctx.reply(`✅ 已添加正则规则: ${pattern}`, replyOptions(ctx));
		} catch {
			await ctx.reply("无效的正则表达式。", replyOptions(ctx));
		}
	});

	bot.command("listkeywords", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const keywords = await getKeywords(db, groupId);
		if (keywords.length === 0) {
			await ctx.reply("暂无关键词规则。", replyOptions(ctx));
			return;
		}

		const lines = keywords.map(
			(k, i) =>
				`${i + 1}. ${k.is_regex ? "🔍" : "🔤"} ${escapeMarkdown(k.pattern)} → ${escapeMarkdown(k.reply_content)}`,
		);
		await ctx.reply(
			`📋 *关键词规则*\n\n${lines.join("\n")}`,
			replyOptionsWithParse(ctx),
		);
	});

	bot.command("removekeyword", async (ctx) => {
		// 检查管理员权限
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const keyword = ctx.match?.toString().trim();
		if (!keyword) {
			await ctx.reply("用法: /removekeyword <关键词>", replyOptions(ctx));
			return;
		}

		const removed = await removeKeyword(db, groupId, keyword);
		if (removed) {
			await ctx.reply(`✅ 已删除关键词: ${keyword}`, replyOptions(ctx));
		} else {
			await ctx.reply(`未找到关键词: ${keyword}`, replyOptions(ctx));
		}
	});
}
