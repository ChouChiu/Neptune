import type { Bot } from "grammy";
import {
	addVoteRecord,
	deleteActiveVote,
	getActiveVote,
	getVoteCounts,
} from "../../shared/db/queries";
import { getNickname } from "../../shared/utils/nickname";
import { buildVoteText, VOTE_THRESHOLD } from "./vote";

export function registerVotekickHandler(bot: Bot, db: D1Database): void {
	bot.callbackQuery(/^vk:(.+):([01])$/, async (ctx) => {
		const match = ctx.match;
		if (!match?.[1] || match[2] === undefined) return;

		const voteId = match[1];
		const choice = Number.parseInt(match[2], 10);
		const voterId = ctx.from.id;

		const vote = await getActiveVote(db, voteId);
		if (!vote) {
			await ctx.answerCallbackQuery({ text: "投票已结束。" });
			return;
		}

		const now = Math.floor(Date.now() / 1000);
		if (now >= vote.expires_at) {
			if (vote.message_id) {
				try {
					await ctx.api.deleteMessage(vote.group_id, vote.message_id);
				} catch {}
			}
			await deleteActiveVote(db, voteId);
			await ctx.answerCallbackQuery({ text: "投票已过期。" });
			return;
		}

		if (voterId === vote.target_id) {
			await ctx.answerCallbackQuery({ text: "你不能参与关于自己的投票。" });
			return;
		}

		const added = await addVoteRecord(db, voteId, voterId, choice);
		if (!added) {
			await ctx.answerCallbackQuery({ text: "你已经投过票了。" });
			return;
		}

		const counts = await getVoteCounts(db, voteId);

		let targetName = `用户 ${vote.target_id}`;
		let initiatorName = `用户 ${vote.initiator_id}`;
		try {
			const chatId = vote.group_id;
			const targetMember = await ctx.api.getChatMember(chatId, vote.target_id);
			targetName = getNickname(targetMember.user);
			const initiatorMember = await ctx.api.getChatMember(
				chatId,
				vote.initiator_id,
			);
			initiatorName = getNickname(initiatorMember.user);
		} catch {}

		const text = buildVoteText(
			targetName,
			initiatorName,
			counts.yes,
			counts.no,
			vote.expires_at,
		);

		if (vote.message_id) {
			try {
				await ctx.api.editMessageText(vote.group_id, vote.message_id, text, {
					reply_markup: {
						inline_keyboard: [
							[
								{
									text: `赞成 (${counts.yes})`,
									callback_data: `vk:${voteId}:1`,
								},
								{
									text: `反对 (${counts.no})`,
									callback_data: `vk:${voteId}:0`,
								},
							],
						],
					},
				});
			} catch {}
		}

		if (counts.yes >= VOTE_THRESHOLD) {
			if (vote.message_id) {
				try {
					await ctx.api.deleteMessage(vote.group_id, vote.message_id);
				} catch {}
			}
			try {
				await ctx.api.banChatMember(vote.group_id, vote.target_id);
				await ctx.api.sendMessage(
					vote.group_id,
					`✅ 投票通过，已将 ${targetName} 移出群组。`,
				);
			} catch {
				await ctx.api.sendMessage(
					vote.group_id,
					`⚠️ 投票通过，但无法移出 ${targetName}（权限不足）。`,
				);
			}
			await deleteActiveVote(db, voteId);
			await ctx.answerCallbackQuery({ text: "投票通过！" });
			return;
		}

		const choiceText = choice === 1 ? "赞成" : "反对";
		await ctx.answerCallbackQuery({ text: `已投${choiceText}。` });
	});
}
