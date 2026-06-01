import type { Bot } from "grammy";
import {
	createActiveVote,
	getActiveVoteForTarget,
	getGroupConfig,
	getLastVoteByInitiator,
	setVotekickEnabled,
	updateVoteMessageId,
} from "../../shared/db/queries";
import {
	requireAdmin,
	requireGroup,
	requireNonBot,
	requireReplyTarget,
} from "../../shared/utils/command-guards";
import { getNickname } from "../../shared/utils/nickname";
import { replyOptions } from "../../shared/utils/reply";
import { currentTimestamp } from "../../shared/utils/time";
import { buildVoteText } from "./vote";

const VOTE_DURATION = 300;
const INITIATOR_COOLDOWN = 60;

function generateVoteId(): string {
	return crypto.randomUUID();
}

export function registerVotekickCommands(bot: Bot, db: D1Database): void {
	bot.command("enablevotekick", async (ctx) => {
		const { allowed, groupId } = await requireAdmin(db, ctx);
		if (!allowed || !groupId) return;

		const config = await getGroupConfig(db, groupId);
		if (config?.votekick_enabled) {
			await ctx.reply("投票踢人已经处于启用状态。", replyOptions(ctx));
			return;
		}

		await setVotekickEnabled(db, groupId, true);
		await ctx.reply(
			"✅ 投票踢人已启用。使用 /kick（回复目标消息）发起投票。",
			replyOptions(ctx),
		);
	});

	bot.command("disablevotekick", async (ctx) => {
		const { allowed, groupId } = await requireAdmin(db, ctx);
		if (!allowed || !groupId) return;

		const config = await getGroupConfig(db, groupId);
		if (!config?.votekick_enabled) {
			await ctx.reply("投票踢人已经处于禁用状态。", replyOptions(ctx));
			return;
		}

		await setVotekickEnabled(db, groupId, false);
		await ctx.reply("✅ 投票踢人已禁用。", replyOptions(ctx));
	});

	bot.command("kick", async (ctx) => {
		const { allowed: groupAllowed, groupId } = await requireGroup(ctx);
		if (!groupAllowed || !groupId) return;

		const config = await getGroupConfig(db, groupId);
		if (!config?.votekick_enabled) {
			await ctx.reply(
				"投票踢人未启用，请让管理员使用 /enablevotekick 启用。",
				replyOptions(ctx),
			);
			return;
		}

		const from = ctx.from;
		if (!from) return;

		const { allowed: replyAllowed, target } = requireReplyTarget(ctx);
		if (!replyAllowed || !target) return;

		const targetId = target.id;
		const targetName = getNickname(target);

		if (targetId === from.id) {
			await ctx.reply("❌ 不能对自己发起投票。", replyOptions(ctx));
			return;
		}

		if (!(await requireNonBot(ctx, target))) return;

		try {
			const member = await ctx.api.getChatMember(groupId, targetId);
			if (member.status === "administrator" || member.status === "creator") {
				await ctx.reply("❌ 不能对管理员发起投票。", replyOptions(ctx));
				return;
			}
		} catch {}

		const existing = await getActiveVoteForTarget(db, groupId, targetId);
		if (existing) {
			await ctx.reply("❌ 该用户已有进行中的投票。", replyOptions(ctx));
			return;
		}

		const now = currentTimestamp();
		const lastVote = await getLastVoteByInitiator(db, groupId, from.id);
		if (lastVote) {
			const elapsed = now - lastVote.created_at;
			if (elapsed < INITIATOR_COOLDOWN) {
				const remaining = INITIATOR_COOLDOWN - elapsed;
				await ctx.reply(
					`❌ 冷却中，请等待 ${remaining} 秒后再试。`,
					replyOptions(ctx),
				);
				return;
			}
		}

		const voteId = generateVoteId();
		const expiresAt = now + VOTE_DURATION;
		await createActiveVote(
			db,
			voteId,
			groupId,
			targetId,
			from.id,
			now,
			expiresAt,
		);

		const initiatorName = getNickname(from);
		const text = buildVoteText(targetName, initiatorName, 0, 0, expiresAt);

		const sent = await ctx.reply(text, {
			reply_markup: {
				inline_keyboard: [
					[
						{ text: "赞成 (0)", callback_data: `vk:${voteId}:1` },
						{ text: "反对 (0)", callback_data: `vk:${voteId}:0` },
					],
				],
			},
		});

		await updateVoteMessageId(db, voteId, sent.message_id);
	});
}
