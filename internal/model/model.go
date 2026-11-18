package model

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const (
	defaultDBPath     = "data/neptune.db"
	defaultListenAddr = ":8080"
)

// Config holds application configuration from environment variables.
type Config struct {
	BotToken            string
	BotUsername         string
	GitHubWebhookSecret string
	ReleaseChannelID    int64
	DBPath              string
	ListenAddr          string
}

// LoadConfigFromEnv loads and validates application configuration from environment variables.
func LoadConfigFromEnv() (*Config, error) {
	botToken := strings.TrimSpace(os.Getenv("BOT_TOKEN"))
	if botToken == "" {
		return nil, errors.New("BOT_TOKEN environment variable is required")
	}

	cfg := &Config{
		BotToken:            botToken,
		BotUsername:         strings.TrimPrefix(strings.TrimSpace(os.Getenv("BOT_USERNAME")), "@"),
		GitHubWebhookSecret: strings.TrimSpace(os.Getenv("GITHUB_WEBHOOK_SECRET")),
		DBPath:              strings.TrimSpace(os.Getenv("DB_PATH")),
		ListenAddr:          strings.TrimSpace(os.Getenv("LISTEN_ADDR")),
	}

	if cfg.DBPath == "" {
		cfg.DBPath = defaultDBPath
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}

	if id := strings.TrimSpace(os.Getenv("RELEASE_CHANNEL_ID")); id != "" {
		parsed, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			slog.Warn("Invalid RELEASE_CHANNEL_ID", "value", id, "error", err)
		} else {
			cfg.ReleaseChannelID = parsed
		}
	}

	return cfg, nil
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

// PendingVerification represents a pending captcha verification.
type PendingVerification struct {
	UserID            int64  `json:"user_id"`
	GroupID           int64  `json:"group_id"`
	CaptchaText       string `json:"captcha_text"`
	ExpiresAt         int64  `json:"expires_at"`
	WelcomeMessageID  *int64 `json:"welcome_message_id"`
	Attempts          int    `json:"attempts"`
	RuleAckDone       int    `json:"rule_ack_done"`
	PromptedOnMessage int    `json:"prompted_on_message"`
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
	ID                  int64  `json:"id"`
	GroupID             int64  `json:"group_id"`
	ReporterID          int64  `json:"reporter_id"`
	ReportedUserID      int64  `json:"reported_user_id"`
	ReportedMessageID   *int64 `json:"reported_message_id"`
	ReportedMessageText string `json:"reported_message_text"`
	Content             string `json:"content"`
	Status              string `json:"status"`
	ReviewedBy          *int64 `json:"reviewed_by"`
	ReviewedAt          *int64 `json:"reviewed_at"`
	CreatedAt           int64  `json:"created_at"`
}
