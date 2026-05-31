import type { Bot } from "grammy";
import {
	addPendingVerification,
	cleanExpiredVerifications,
	getGroupConfig,
} from "../../shared/db/queries";
import { getBotUsername } from "../../shared/utils/botInfo";
import { escapeMarkdown } from "../../shared/utils/markdown";
import { getNickname } from "../../shared/utils/nickname";
import { replacePlaceholders } from "../../shared/utils/placeholders";
import { restrictUser } from "../verify/handlers";

export function registerWelcomeHandlers(bot: Bot, db: D1Database): void {
	bot.on("message:new_chat_members", async (ctx) => {
		const newMembers = ctx.message.new_chat_members;
		if (!newMembers) return;

		const groupId = ctx.chat.id;

		const config = await getGroupConfig(db, groupId);
		if (!config?.welcome_enabled) return;

		await cleanExpiredVerifications(db);

		try {
			await ctx.api.deleteMessage(groupId, ctx.message.message_id);
		} catch (error) {
			console.error("Failed to delete join message:", error);
		}

		for (const newMember of newMembers) {
			if (newMember.is_bot) continue;

			const userId = newMember.id;
			const nickname = getNickname(newMember);

			const welcomeText = replacePlaceholders(config.welcome_message, {
				nickname: escapeMarkdown(nickname),
				userid: userId,
				groupname: ctx.chat.title ? escapeMarkdown(ctx.chat.title) : undefined,
			});

			const botUsername = await getBotUsername(ctx.api);
			const verifyUrl = `https://t.me/${botUsername}?start=verify${groupId}_${userId}`;

			const welcomeMsg = await ctx.reply(welcomeText, {
				parse_mode: "MarkdownV2",
				reply_markup: {
					inline_keyboard: [
						[
							{
								text: config.verify_button_text,
								url: verifyUrl,
							},
						],
					],
				},
			});

			const timeout = config.verify_timeout;
			const expiresAt = Math.floor(Date.now() / 1000) + timeout + 300;
			await addPendingVerification(
				db,
				userId,
				groupId,
				"",
				expiresAt,
				welcomeMsg.message_id,
			);

			try {
				await restrictUser(ctx.api, groupId, userId);
			} catch (error) {
				console.error(
					"Failed to restrict user:",
					error instanceof Error ? error.message : error,
				);
			}
		}
	});
}
