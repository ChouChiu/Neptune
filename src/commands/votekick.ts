import type { Bot } from "grammy";
import {
	createActiveVote,
	getActiveVoteForTarget,
	getGroupConfig,
	getLastVoteByInitiator,
	setVotekickEnabled,
	updateVoteMessageId,
} from "../db/queries";
import { getNickname } from "../utils/nickname";
import { checkAdminPermission } from "../utils/permissions";
import { replyOptions } from "../utils/reply";
import { buildVoteText } from "../utils/vote";

const VOTE_DURATION = 300;
const INITIATOR_COOLDOWN = 60;

function generateVoteId(): string {
	return crypto.randomUUID();
}

export function registerVotekickCommands(bot: Bot, db: D1Database): void {
	bot.command("enablevotekick", async (ctx) => {
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

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
		const { allowed, groupId } = await checkAdminPermission(db, ctx);
		if (!allowed || !groupId) {
			await ctx.reply("只有管理员才能使用此命令。", replyOptions(ctx));
			return;
		}

		const config = await getGroupConfig(db, groupId);
		if (!config?.votekick_enabled) {
			await ctx.reply("投票踢人已经处于禁用状态。", replyOptions(ctx));
			return;
		}

		await setVotekickEnabled(db, groupId, false);
		await ctx.reply("✅ 投票踢人已禁用。", replyOptions(ctx));
	});

	bot.command("kick", async (ctx) => {
		if (ctx.chat.type === "private") {
			await ctx.reply("此命令只能在群组中使用。", replyOptions(ctx));
			return;
		}

		const groupId = ctx.chat.id;
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

		const replyMsg = ctx.message?.reply_to_message;
		if (!replyMsg?.from) {
			await ctx.reply("请回复目标用户的消息来发起投票。", replyOptions(ctx));
			return;
		}

		const targetId = replyMsg.from.id;
		const targetName = getNickname(replyMsg.from);

		if (targetId === from.id) {
			await ctx.reply("❌ 不能对自己发起投票。", replyOptions(ctx));
			return;
		}

		if (replyMsg.from.is_bot) {
			await ctx.reply("❌ 不能对机器人发起投票。", replyOptions(ctx));
			return;
		}

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

		const now = Math.floor(Date.now() / 1000);
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
