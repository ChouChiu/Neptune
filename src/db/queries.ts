import type {
	ActiveVote,
	GroupConfig,
	KeywordRule,
	PendingVerification,
} from "../types";

export async function initGroup(
	db: D1Database,
	groupId: number,
): Promise<void> {
	await db
		.prepare("INSERT OR IGNORE INTO groups (group_id) VALUES (?)")
		.bind(groupId)
		.run();
}

export async function getGroupConfig(
	db: D1Database,
	groupId: number,
): Promise<GroupConfig | null> {
	const result = await db
		.prepare("SELECT * FROM groups WHERE group_id = ?")
		.bind(groupId)
		.first<GroupConfig>();
	return result ?? null;
}

export async function updateWelcomeMessage(
	db: D1Database,
	groupId: number,
	message: string,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET welcome_message = ? WHERE group_id = ?")
		.bind(message, groupId)
		.run();
}

export async function setWelcomeEnabled(
	db: D1Database,
	groupId: number,
	enabled: boolean,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET welcome_enabled = ? WHERE group_id = ?")
		.bind(enabled ? 1 : 0, groupId)
		.run();
}

export async function updateVerifyButtonText(
	db: D1Database,
	groupId: number,
	text: string,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET verify_button_text = ? WHERE group_id = ?")
		.bind(text, groupId)
		.run();
}

export async function updateVerifyTimeout(
	db: D1Database,
	groupId: number,
	timeout: number,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET verify_timeout = ? WHERE group_id = ?")
		.bind(timeout, groupId)
		.run();
}

export async function updateGroupRule(
	db: D1Database,
	groupId: number,
	rule: string,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET rule = ? WHERE group_id = ?")
		.bind(rule, groupId)
		.run();
}

export async function getAdminGroupId(
	db: D1Database,
	userId: number,
): Promise<number | null> {
	// 先查当前选中的群组
	const current = await db
		.prepare("SELECT group_id FROM admin_current_group WHERE user_id = ?")
		.bind(userId)
		.first<{ group_id: number }>();
	if (current) return current.group_id;

	// 没有则返回第一个关联的群组
	const result = await db
		.prepare("SELECT group_id FROM admin_connections WHERE user_id = ? LIMIT 1")
		.bind(userId)
		.first<{ group_id: number }>();
	return result?.group_id ?? null;
}

export async function connectAdmin(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<void> {
	await db
		.prepare(
			"INSERT OR IGNORE INTO admin_connections (user_id, group_id) VALUES (?, ?)",
		)
		.bind(userId, groupId)
		.run();
	// 同时设置为当前群组
	await db
		.prepare(
			"INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)",
		)
		.bind(userId, groupId)
		.run();
}

export async function getAdminGroups(
	db: D1Database,
	userId: number,
): Promise<number[]> {
	const result = await db
		.prepare("SELECT group_id FROM admin_connections WHERE user_id = ?")
		.bind(userId)
		.all<{ group_id: number }>();
	return result.results.map((r) => r.group_id);
}

export async function isAdminConnected(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<boolean> {
	const result = await db
		.prepare(
			"SELECT 1 FROM admin_connections WHERE user_id = ? AND group_id = ?",
		)
		.bind(userId, groupId)
		.first();
	return result !== null;
}

export async function setCurrentGroup(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<void> {
	await db
		.prepare(
			"INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)",
		)
		.bind(userId, groupId)
		.run();
}

export async function addKeyword(
	db: D1Database,
	groupId: number,
	pattern: string,
	isRegex: boolean,
	replyContent: string,
	replyType: string = "text",
): Promise<void> {
	await db
		.prepare(
			"INSERT INTO keywords (group_id, pattern, is_regex, reply_content, reply_type) VALUES (?, ?, ?, ?, ?)",
		)
		.bind(groupId, pattern, isRegex ? 1 : 0, replyContent, replyType)
		.run();
}

export async function getKeywords(
	db: D1Database,
	groupId: number,
): Promise<KeywordRule[]> {
	const result = await db
		.prepare("SELECT * FROM keywords WHERE group_id = ?")
		.bind(groupId)
		.all<KeywordRule>();
	return result.results;
}

export async function removeKeyword(
	db: D1Database,
	groupId: number,
	pattern: string,
): Promise<boolean> {
	const result = await db
		.prepare("DELETE FROM keywords WHERE group_id = ? AND pattern = ?")
		.bind(groupId, pattern)
		.run();
	return result.meta.changes > 0;
}

export async function addPendingVerification(
	db: D1Database,
	userId: number,
	groupId: number,
	captchaText: string,
	expiresAt: number,
	welcomeMessageId?: number,
): Promise<void> {
	await db
		.prepare(
			"INSERT OR REPLACE INTO pending_verifications (user_id, group_id, captcha_text, expires_at, welcome_message_id) VALUES (?, ?, ?, ?, ?)",
		)
		.bind(userId, groupId, captchaText, expiresAt, welcomeMessageId ?? null)
		.run();
}

export async function getPendingVerification(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<PendingVerification | null> {
	const result = await db
		.prepare(
			"SELECT * FROM pending_verifications WHERE user_id = ? AND group_id = ?",
		)
		.bind(userId, groupId)
		.first<PendingVerification>();
	return result ?? null;
}

export async function removePendingVerification(
	db: D1Database,
	userId: number,
	groupId: number,
): Promise<void> {
	await db
		.prepare(
			"DELETE FROM pending_verifications WHERE user_id = ? AND group_id = ?",
		)
		.bind(userId, groupId)
		.run();
}

export async function cleanExpiredVerifications(db: D1Database): Promise<void> {
	const now = Math.floor(Date.now() / 1000);
	await db
		.prepare("DELETE FROM pending_verifications WHERE expires_at < ?")
		.bind(now)
		.run();
}

// ── Vote Kick ──────────────────────────────────────────────────────

export async function setVotekickEnabled(
	db: D1Database,
	groupId: number,
	enabled: boolean,
): Promise<void> {
	await db
		.prepare("UPDATE groups SET votekick_enabled = ? WHERE group_id = ?")
		.bind(enabled ? 1 : 0, groupId)
		.run();
}

export async function createActiveVote(
	db: D1Database,
	voteId: string,
	groupId: number,
	targetId: number,
	initiatorId: number,
	createdAt: number,
	expiresAt: number,
): Promise<void> {
	await db
		.prepare(
			"INSERT INTO active_votes (vote_id, group_id, target_id, initiator_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		)
		.bind(voteId, groupId, targetId, initiatorId, createdAt, expiresAt)
		.run();
}

export async function getActiveVote(
	db: D1Database,
	voteId: string,
): Promise<ActiveVote | null> {
	const result = await db
		.prepare("SELECT * FROM active_votes WHERE vote_id = ?")
		.bind(voteId)
		.first<ActiveVote>();
	return result ?? null;
}

export async function getActiveVoteForTarget(
	db: D1Database,
	groupId: number,
	targetId: number,
): Promise<ActiveVote | null> {
	const now = Math.floor(Date.now() / 1000);
	const result = await db
		.prepare(
			"SELECT * FROM active_votes WHERE group_id = ? AND target_id = ? AND expires_at > ?",
		)
		.bind(groupId, targetId, now)
		.first<ActiveVote>();
	return result ?? null;
}

export async function getLastVoteByInitiator(
	db: D1Database,
	groupId: number,
	initiatorId: number,
): Promise<ActiveVote | null> {
	const result = await db
		.prepare(
			"SELECT * FROM active_votes WHERE group_id = ? AND initiator_id = ? ORDER BY created_at DESC LIMIT 1",
		)
		.bind(groupId, initiatorId)
		.first<ActiveVote>();
	return result ?? null;
}

export async function updateVoteMessageId(
	db: D1Database,
	voteId: string,
	messageId: number,
): Promise<void> {
	await db
		.prepare("UPDATE active_votes SET message_id = ? WHERE vote_id = ?")
		.bind(messageId, voteId)
		.run();
}

export async function deleteActiveVote(
	db: D1Database,
	voteId: string,
): Promise<void> {
	await db
		.prepare("DELETE FROM active_votes WHERE vote_id = ?")
		.bind(voteId)
		.run();
	await db
		.prepare("DELETE FROM vote_records WHERE vote_id = ?")
		.bind(voteId)
		.run();
}

export async function addVoteRecord(
	db: D1Database,
	voteId: string,
	voterId: number,
	choice: number,
): Promise<boolean> {
	try {
		await db
			.prepare(
				"INSERT INTO vote_records (vote_id, voter_id, choice) VALUES (?, ?, ?)",
			)
			.bind(voteId, voterId, choice)
			.run();
		return true;
	} catch (error) {
		const msg = error instanceof Error ? error.message : String(error);
		if (msg.includes("UNIQUE") || msg.includes("unique")) {
			return false;
		}
		throw error;
	}
}

export async function getVoteCounts(
	db: D1Database,
	voteId: string,
): Promise<{ yes: number; no: number }> {
	const rows = await db
		.prepare(
			"SELECT choice, COUNT(*) as cnt FROM vote_records WHERE vote_id = ? GROUP BY choice",
		)
		.bind(voteId)
		.all<{ choice: number; cnt: number }>();
	let yes = 0;
	let no = 0;
	for (const row of rows.results) {
		if (row.choice === 1) yes = row.cnt;
		else no = row.cnt;
	}
	return { yes, no };
}

export async function cleanExpiredVotes(db: D1Database): Promise<void> {
	const now = Math.floor(Date.now() / 1000);
	await db
		.prepare(
			"DELETE FROM vote_records WHERE vote_id IN (SELECT vote_id FROM active_votes WHERE expires_at < ?)",
		)
		.bind(now)
		.run();
	await db
		.prepare("DELETE FROM active_votes WHERE expires_at < ?")
		.bind(now)
		.run();
}

export async function getExpiredVotes(db: D1Database): Promise<ActiveVote[]> {
	const now = Math.floor(Date.now() / 1000);
	const result = await db
		.prepare(
			"SELECT * FROM active_votes WHERE expires_at < ? AND message_id IS NOT NULL",
		)
		.bind(now)
		.all<ActiveVote>();
	return result.results;
}
