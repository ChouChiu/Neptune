package model

// Config holds application configuration from environment variables.
type Config struct {
	BotToken            string
	BotUsername         string
	HermesAPIURL        string
	HermesAPIKey        string
	ReuseCaptcha        bool
	GitHubWebhookSecret string
	ReleaseChannelID    int64
}

// GroupConfig represents a group's configuration.
type GroupConfig struct {
	GroupID          int64  `json:"group_id"`
	WelcomeEnabled   int    `json:"welcome_enabled"`
	WelcomeMessage   string `json:"welcome_message"`
	VerifyButtonText string `json:"verify_button_text"`
	VerifyTimeout    int    `json:"verify_timeout"`
	VotekickEnabled  int    `json:"votekick_enabled"`
	Rule             string `json:"rule"`
}

// KeywordRule represents a keyword/regex trigger rule.
type KeywordRule struct {
	ID           int64  `json:"id"`
	GroupID      int64  `json:"group_id"`
	Pattern      string `json:"pattern"`
	IsRegex      int    `json:"is_regex"`
	ReplyContent string `json:"reply_content"`
	ReplyType    string `json:"reply_type"`
}

// AdminConnection represents an admin-group binding.
type AdminConnection struct {
	UserID  int64 `json:"user_id"`
	GroupID int64 `json:"group_id"`
}

// AdminCurrentGroup tracks which group an admin is currently managing.
type AdminCurrentGroup struct {
	UserID  int64 `json:"user_id"`
	GroupID int64 `json:"group_id"`
}

// PendingVerification represents a pending captcha verification.
type PendingVerification struct {
	UserID           int64  `json:"user_id"`
	GroupID          int64  `json:"group_id"`
	CaptchaText      string `json:"captcha_text"`
	ExpiresAt        int64  `json:"expires_at"`
	WelcomeMessageID *int64 `json:"welcome_message_id"`
	Attempts         int    `json:"attempts"`
	RuleAckDone      int    `json:"rule_ack_done"`
}

// ActiveVote represents an active votekick session.
type ActiveVote struct {
	VoteID      string `json:"vote_id"`
	GroupID     int64  `json:"group_id"`
	TargetID    int64  `json:"target_id"`
	InitiatorID int64  `json:"initiator_id"`
	MessageID   *int64 `json:"message_id"`
	CreatedAt   int64  `json:"created_at"`
	ExpiresAt   int64  `json:"expires_at"`
}

// VoteRecord represents a single vote in a votekick session.
type VoteRecord struct {
	VoteID  string `json:"vote_id"`
	VoterID int64  `json:"voter_id"`
	Choice  int    `json:"choice"`
}

// AiContextMessage represents a message in AI chat context.
type AiContextMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	UserID    *int64 `json:"userId,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// AiChatUsage tracks daily AI chat usage per user per group.
type AiChatUsage struct {
	UserID  int64  `json:"user_id"`
	GroupID int64  `json:"group_id"`
	Date    string `json:"date"`
	Count   int    `json:"count"`
}

// Warning represents an admin warning issued to a user.
type Warning struct {
	ID        int64  `json:"id"`
	GroupID   int64  `json:"group_id"`
	UserID    int64  `json:"user_id"`
	AdminID   int64  `json:"admin_id"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"created_at"`
}

// Report represents a user report.
type Report struct {
	ID                 int64  `json:"id"`
	GroupID            int64  `json:"group_id"`
	ReporterID         int64  `json:"reporter_id"`
	ReportedUserID     int64  `json:"reported_user_id"`
	ReportedMessageID  *int64 `json:"reported_message_id"`
	ReportedMessageText string `json:"reported_message_text"`
	Content            string `json:"content"`
	Status             string `json:"status"`
	ReviewedBy         *int64 `json:"reviewed_by"`
	ReviewedAt         *int64 `json:"reviewed_at"`
	CreatedAt          int64  `json:"created_at"`
}

// KVEntry represents a key-value entry (for AI context storage).
type KVEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expires_at"`
}

// Lock represents a distributed lock entry.
type Lock struct {
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expires_at"`
}
