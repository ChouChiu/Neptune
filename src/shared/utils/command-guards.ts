import type { Context } from "grammy";
import { checkAdminPermission } from "./permissions";
import { replyOptions } from "./reply";

export async function requireGroup(
	ctx: Context,
): Promise<{ allowed: boolean; groupId?: number }> {
	if (ctx.chat?.type === "private") {
		await ctx.reply("此命令只能在群组中使用。", replyOptions(ctx));
		return { allowed: false };
	}
	const groupId = ctx.chat?.id;
	if (!groupId) return { allowed: false };
	return { allowed: true, groupId };
}

export function requireReplyTarget(ctx: Context): {
	allowed: boolean;
	target?: {
		id: number;
		is_bot: boolean;
		first_name: string;
		last_name?: string;
	};
} {
	const replyMsg = ctx.message?.reply_to_message;
	if (!replyMsg?.from) {
		ctx.reply("请回复目标用户的消息。", replyOptions(ctx));
		return { allowed: false };
	}
	return { allowed: true, target: replyMsg.from };
}

export async function requireNonBot(
	ctx: Context,
	target: { id: number; is_bot: boolean },
): Promise<boolean> {
	if (target.is_bot) {
		await ctx.reply("涅普不能对机器人执行此操作哦～", replyOptions(ctx));
		return false;
	}
	return true;
}

export async function requireAdmin(
	db: D1Database,
	ctx: Context,
): Promise<{ allowed: boolean; groupId?: number }> {
	const { allowed, groupId } = await checkAdminPermission(db, ctx as any);
	if (!allowed || !groupId) {
		await ctx.reply("你没有权限执行此操作。", replyOptions(ctx));
		return { allowed: false };
	}
	return { allowed: true, groupId };
}
