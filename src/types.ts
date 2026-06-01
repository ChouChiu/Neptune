export interface Env {
	BOT_TOKEN: string;
	BOT_USERNAME: string;
	MIMO_API_KEY: string;
	REUSE_CAPTCHA?: string;
	GITHUB_WEBHOOK_SECRET: string;
	RELEASE_CHANNEL_ID?: string;
	db: D1Database;
	captcha: R2Bucket;
	aiContext: KVNamespace;
}

export interface GroupConfig {
	group_id: number;
	welcome_enabled: number;
	welcome_message: string;
	verify_button_text: string;
	verify_timeout: number;
	votekick_enabled: number;
	rule: string;
}

export interface KeywordRule {
	id: number;
	group_id: number;
	pattern: string;
	is_regex: number;
	reply_content: string;
	reply_type: "text" | "markdown";
}

export interface AdminConnection {
	user_id: number;
	group_id: number;
}

export interface AdminCurrentGroup {
	user_id: number;
	group_id: number;
}

export interface PendingVerification {
	user_id: number;
	group_id: number;
	captcha_text: string;
	expires_at: number;
	welcome_message_id: number | null;
	attempts: number;
	rule_ack_done: number;
}

export interface ActiveVote {
	vote_id: string;
	group_id: number;
	target_id: number;
	initiator_id: number;
	message_id: number | null;
	created_at: number;
	expires_at: number;
}

export interface VoteRecord {
	vote_id: string;
	voter_id: number;
	choice: number;
}

export interface AiContextMessage {
	role: "user" | "assistant";
	content: string;
	userId?: number;
	timestamp: number;
}

export interface AiChatUsage {
	user_id: number;
	group_id: number;
	date: string;
	count: number;
}

export interface Warning {
	id: number;
	group_id: number;
	user_id: number;
	admin_id: number;
	reason: string;
	created_at: number;
}

export interface Report {
	id: number;
	group_id: number;
	reporter_id: number;
	reported_user_id: number;
	reported_message_id: number | null;
	reported_message_text: string;
	content: string;
	status: "pending" | "approved" | "dismissed";
	reviewed_by: number | null;
	reviewed_at: number | null;
	created_at: number;
}
