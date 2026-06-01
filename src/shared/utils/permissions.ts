import type { CommandContext, Context } from "grammy";
import { getAdminGroupId, isAdminConnected } from "../db/queries";

/**
 * 检查用户是否有管理员权限
 * - 群组中：检查是否是 Telegram 管理员或创建者
 * - 私聊中：检查是否是群组的绑定管理员
 */
export async function checkAdminPermission(
	db: D1Database,
	ctx: CommandContext<Context>,
): Promise<{ allowed: boolean; groupId?: number }> {
	const userId = ctx.from?.id;
	if (!userId) return { allowed: false };

	// 私聊中：检查是否绑定了群组
	if (ctx.chat.type === "private") {
		const groupId = await getAdminGroupId(db, userId);
		if (!groupId) return { allowed: false };
		return { allowed: true, groupId };
	}

	// 群组中：检查是否是 Telegram 管理员
	try {
		const chatMember = await ctx.api.getChatMember(ctx.chat.id, userId);
		const isAdmin = ["administrator", "creator"].includes(chatMember.status);
		if (!isAdmin) return { allowed: false };
	} catch {
		return { allowed: false };
	}

	return { allowed: true, groupId: ctx.chat.id };
}

/**
 * 检查用户是否是群组的绑定管理员（用于私聊场景）
 */
export async function checkGroupAdmin(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<boolean> {
	return await isAdminConnected(db, userId, groupId);
}
