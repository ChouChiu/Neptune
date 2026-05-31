import type { Bot, Context } from "grammy";
import { isAdminConnected } from "../../shared/db/queries";
import { formatGeneratedMarkdownV2 } from "../../shared/utils/markdown";
import { replyOptions } from "../../shared/utils/reply";
import { enqueueTelegramSend } from "../../shared/utils/telegram-send-queue";
import { getChatResponse, shouldTriggerAi } from "./ai-chat";

const TYPING_REFRESH_MS = 4000;

interface AiChatJob {
	bot: Bot;
	db: D1Database;
	apiKey: string;
	kv: KVNamespace;
	groupId: number;
	groupTitle?: string;
	replyToMessageId: number;
	userId: number;
	message: string;
}

function startTypingIndicator(bot: Bot, groupId: number): () => void {
	let stopped = false;

	const sendTyping = async () => {
		if (stopped) return;
		try {
			await bot.api.sendChatAction(groupId, "typing");
		} catch (error) {
			console.error("Failed to send AI chat typing action:", error);
		}
	};

	void sendTyping();
	const timer = setInterval(() => {
		void sendTyping();
	}, TYPING_REFRESH_MS);

	return () => {
		stopped = true;
		clearInterval(timer);
	};
}

export async function handleAiChat(
	ctx: Context,
	bot: Bot,
	db: D1Database,
	apiKey: string,
	kv: KVNamespace,
	executionCtx?: ExecutionContext,
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

		const groupId = ctx.chat.id;

		console.log("AI chat request:", {
			groupId,
			userId,
			messageLength: finalMessage.length,
			queued: true,
		});

		const job = processAiChat({
			bot,
			db,
			apiKey,
			kv,
			groupId,
			groupTitle: ctx.chat.title,
			replyToMessageId: ctx.message.message_id,
			userId,
			message: finalMessage,
		});

		if (executionCtx) {
			executionCtx.waitUntil(job);
		} else {
			await job;
		}
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

async function processAiChat(job: AiChatJob): Promise<void> {
	const {
		bot,
		db,
		apiKey,
		kv,
		groupId,
		groupTitle,
		replyToMessageId,
		userId,
		message,
	} = job;

	const stopTyping = startTypingIndicator(bot, groupId);

	try {
		const chatMember = await bot.api.getChatMember(groupId, userId);
		const isAdmin =
			chatMember.status === "creator" ||
			chatMember.status === "administrator" ||
			(await isAdminConnected(db, userId, groupId));

		let memberCount: number | undefined;
		try {
			memberCount = await bot.api.getChatMemberCount(groupId);
		} catch {}

		const reply = await getChatResponse(
			db,
			kv,
			apiKey,
			groupId,
			userId,
			message,
			isAdmin,
			{
				title: groupTitle,
				memberCount,
			},
		);

		console.log("AI chat response:", {
			groupId,
			userId,
			length: reply.length,
			preview: reply.substring(0, 50),
		});

		await sendAiReply(bot, groupId, replyToMessageId, reply);
	} catch (error) {
		console.error("AI chat background error:", error);
		await enqueueTelegramSend(async () => {
			await bot.api.sendMessage(
				groupId,
				"涅普？！出了点状况，主角光环暂时失效了……再试一次吧！",
				{ reply_parameters: { message_id: replyToMessageId } },
			);
		});
	} finally {
		stopTyping();
	}
}

async function sendAiReply(
	bot: Bot,
	groupId: number,
	replyToMessageId: number,
	reply: string,
): Promise<void> {
	const formattedReply = formatGeneratedMarkdownV2(reply);
	const markdownResult = await enqueueTelegramSend(async () => {
		await bot.api.sendMessage(groupId, formattedReply, {
			parse_mode: "MarkdownV2",
			reply_parameters: { message_id: replyToMessageId },
		});
	});

	if (markdownResult === "sent") return;
	if (markdownResult === "dropped") {
		console.warn("AI chat reply dropped because send queue is full", {
			groupId,
			replyToMessageId,
		});
		return;
	}

	console.error("AI chat MarkdownV2 send failed, falling back to plain text", {
		groupId,
		replyToMessageId,
		formattedPreview: formattedReply.substring(0, 200),
	});

	const plainResult = await enqueueTelegramSend(async () => {
		await bot.api.sendMessage(groupId, reply, {
			reply_parameters: { message_id: replyToMessageId },
		});
	});
	if (plainResult === "dropped") {
		console.warn(
			"AI chat plain text fallback dropped because send queue is full",
			{
				groupId,
				replyToMessageId,
			},
		);
	}
}
