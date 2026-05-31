import type { Context } from "grammy";
import { removePendingVerification } from "../../shared/db/queries";
import { replyOptions } from "../../shared/utils/reply";

const MAX_ATTEMPTS = 5;

export async function handleCaptchaReply(
	ctx: Context,
	db: D1Database,
): Promise<boolean> {
	if (ctx.chat?.type !== "private" || !ctx.message?.text || !ctx.from)
		return false;

	const text = ctx.message.text;
	if (text.startsWith("/")) return false;

	const userId = ctx.from.id;

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

	for (const verification of verifications.results) {
		const groupId = verification.group_id;

		if (verification.attempts >= MAX_ATTEMPTS) {
			await removePendingVerification(db, userId, groupId);
			await ctx.reply("验证失败次数过多，请重新加入群组。", replyOptions(ctx));
			continue;
		}

		if (text.toUpperCase() === verification.captcha_text.toUpperCase()) {
			if (verification.welcome_message_id) {
				try {
					await ctx.api.deleteMessage(groupId, verification.welcome_message_id);
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
		return true;
	}

	return false;
}
