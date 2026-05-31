import type { Bot, Context } from "grammy";
import { isAdminConnected } from "../../shared/db/queries";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { replyOptions, replyOptionsWithParse } from "../../shared/utils/reply";
import { getChatResponse, shouldTriggerAi } from "./ai-chat";

export async function handleAiChat(
	ctx: Context,
	bot: Bot,
	db: D1Database,
	apiKey: string,
	kv: KVNamespace,
): Promise<boolean> {
	if (ctx.chat?.type !== "group" && ctx.chat?.type !== "supergroup")
		return false;

	const text = ctx.message?.text;
	if (!text) return false;

	const userId = ctx.from?.id;
	if (!userId) return false;

	const botId = ctx.me?.id ?? bot.botInfo.id;
	if (!shouldTriggerAi(ctx, botId)) return false;

	try {
		const userMessage = text.replace(/@\w+/g, "").trim();
		if (!userMessage) return false;

		const quoted = ctx.message?.reply_to_message;
		const quotedText =
			quoted?.text ?? (quoted as { caption?: string })?.caption;
		const finalMessage = quotedText
			? `[引用消息] ${quotedText}\n\n${userMessage}`
			: userMessage;

		await ctx.replyWithChatAction("typing");

		const groupId = ctx.chat.id;
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
		await ctx.reply(escapeMarkdown(reply), replyOptionsWithParse(ctx));
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
	return true;
}
