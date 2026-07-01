package db

import (
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/ChouChiu/neptune/internal/model"
	"modernc.org/sqlite"
)

const (
	groupColumns               = "group_id, welcome_enabled, welcome_message, verify_button_text, verify_timeout, votekick_enabled, rule"
	keywordColumns             = "id, group_id, pattern, is_regex, reply_content, reply_type"
	pendingVerificationColumns = "user_id, group_id, captcha_text, expires_at, welcome_message_id, attempts, rule_ack_done"
	activeVoteColumns          = "vote_id, group_id, target_id, initiator_id, message_id, created_at, expires_at"
	warningColumns             = "id, group_id, user_id, admin_id, reason, created_at"
	reportColumns              = "id, group_id, reporter_id, reported_user_id, reported_message_id, reported_message_text, content, status, reviewed_by, reviewed_at, created_at"
	reportColumnsWithAlias     = "r.id, r.group_id, r.reporter_id, r.reported_user_id, r.reported_message_id, r.reported_message_text, r.content, r.status, r.reviewed_by, r.reviewed_at, r.created_at"
)

// currentTimestamp returns the current Unix timestamp in seconds.
func currentTimestamp() int64 {
	return time.Now().Unix()
}

// ── Group Config ──────────────────────────────────────────────────

// InitGroup creates a group record if it doesn't exist.
func (d *DB) InitGroup(groupID int64) error {
	_, err := d.Exec("INSERT OR IGNORE INTO groups (group_id) VALUES (?)", groupID)
	return err
}

// GetGroupConfig returns the configuration for a group.
func (d *DB) GetGroupConfig(groupID int64) (*model.GroupConfig, error) {
	var cfg model.GroupConfig
	err := d.QueryRow("SELECT "+groupColumns+" FROM groups WHERE group_id = ?", groupID).Scan(
		&cfg.GroupID, &cfg.WelcomeEnabled, &cfg.WelcomeMessage,
		&cfg.VerifyButtonText, &cfg.VerifyTimeout, &cfg.VotekickEnabled, &cfg.Rule,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateWelcomeMessage updates the welcome message for a group.
func (d *DB) UpdateWelcomeMessage(groupID int64, message string) error {
	_, err := d.Exec("UPDATE groups SET welcome_message = ? WHERE group_id = ?", message, groupID)
	return err
}

// SetWelcomeEnabled enables or disables welcome messages for a group.
func (d *DB) SetWelcomeEnabled(groupID int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := d.Exec("UPDATE groups SET welcome_enabled = ? WHERE group_id = ?", val, groupID)
	return err
}

// UpdateVerifyButtonText updates the verify button text for a group.
func (d *DB) UpdateVerifyButtonText(groupID int64, text string) error {
	_, err := d.Exec("UPDATE groups SET verify_button_text = ? WHERE group_id = ?", text, groupID)
	return err
}

// UpdateVerifyTimeout updates the verify timeout for a group.
func (d *DB) UpdateVerifyTimeout(groupID int64, timeout int) error {
	_, err := d.Exec("UPDATE groups SET verify_timeout = ? WHERE group_id = ?", timeout, groupID)
	return err
}

// UpdateGroupRule updates the group rule.
func (d *DB) UpdateGroupRule(groupID int64, rule string) error {
	_, err := d.Exec("UPDATE groups SET rule = ? WHERE group_id = ?", rule, groupID)
	return err
}

// SetVotekickEnabled enables or disables votekick for a group.
func (d *DB) SetVotekickEnabled(groupID int64, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := d.Exec("UPDATE groups SET votekick_enabled = ? WHERE group_id = ?", val, groupID)
	return err
}

// ── Admin Connections ─────────────────────────────────────────────

// GetAdminGroupID returns the currently selected group for an admin.
func (d *DB) GetAdminGroupID(userID int64) (*int64, error) {
	// First check current group
	var groupID int64
	err := d.QueryRow("SELECT group_id FROM admin_current_group WHERE user_id = ?", userID).Scan(&groupID)
	if err == nil {
		return &groupID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Fall back to first connected group
	err = d.QueryRow("SELECT group_id FROM admin_connections WHERE user_id = ? LIMIT 1", userID).Scan(&groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &groupID, nil
}

// ConnectAdmin binds an admin to a group and sets it as current.
func (d *DB) ConnectAdmin(userID, groupID int64) error {
	_, err := d.Exec("INSERT OR IGNORE INTO admin_connections (user_id, group_id) VALUES (?, ?)", userID, groupID)
	if err != nil {
		return err
	}
	_, err = d.Exec("INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)", userID, groupID)
	return err
}

// GetAdminGroups returns all groups an admin is connected to.
func (d *DB) GetAdminGroups(userID int64) ([]int64, error) {
	rows, err := d.Query("SELECT group_id FROM admin_connections WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var groups []int64
	for rows.Next() {
		var gid int64
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		groups = append(groups, gid)
	}
	return groups, rows.Err()
}

// IsAdminConnected checks if an admin is connected to a group.
func (d *DB) IsAdminConnected(userID, groupID int64) (bool, error) {
	var exists int
	err := d.QueryRow("SELECT 1 FROM admin_connections WHERE user_id = ? AND group_id = ?", userID, groupID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// SetCurrentGroup sets the current group for an admin.
func (d *DB) SetCurrentGroup(userID, groupID int64) error {
	_, err := d.Exec("INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)", userID, groupID)
	return err
}

// ── Keywords ──────────────────────────────────────────────────────

// AddKeyword adds a keyword rule to a group.
func (d *DB) AddKeyword(groupID int64, pattern string, isRegex bool, replyContent, replyType string) error {
	isRegexInt := 0
	if isRegex {
		isRegexInt = 1
	}
	if replyType == "" {
		replyType = "text"
	}
	_, err := d.Exec(
		"INSERT INTO keywords (group_id, pattern, is_regex, reply_content, reply_type) VALUES (?, ?, ?, ?, ?)",
		groupID, pattern, isRegexInt, replyContent, replyType,
	)
	return err
}

// GetKeywords returns all keyword rules for a group.
func (d *DB) GetKeywords(groupID int64) ([]model.KeywordRule, error) {
	rows, err := d.Query("SELECT "+keywordColumns+" FROM keywords WHERE group_id = ?", groupID)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var rules []model.KeywordRule
	for rows.Next() {
		var r model.KeywordRule
		if err := rows.Scan(&r.ID, &r.GroupID, &r.Pattern, &r.IsRegex, &r.ReplyContent, &r.ReplyType); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// RemoveKeyword removes a keyword rule from a group.
func (d *DB) RemoveKeyword(groupID int64, pattern string) (bool, error) {
	result, err := d.Exec("DELETE FROM keywords WHERE group_id = ? AND pattern = ?", groupID, pattern)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ── Pending Verifications ─────────────────────────────────────────

// AddPendingVerification adds or updates a pending verification.
func (d *DB) AddPendingVerification(userID, groupID int64, captchaText string, expiresAt int64, welcomeMessageID *int64, ruleAckDone bool) error {
	ruleAckDoneInt := 0
	if ruleAckDone {
		ruleAckDoneInt = 1
	}
	_, err := d.Exec(
		`INSERT INTO pending_verifications (user_id, group_id, captcha_text, expires_at, welcome_message_id, attempts, rule_ack_done)
		VALUES (?, ?, ?, ?, ?, 0, ?)
		ON CONFLICT(user_id, group_id) DO UPDATE SET
			captcha_text = excluded.captcha_text,
			expires_at = excluded.expires_at,
			welcome_message_id = excluded.welcome_message_id,
			attempts = 0,
			rule_ack_done = excluded.rule_ack_done`,
		userID, groupID, captchaText, expiresAt, welcomeMessageID, ruleAckDoneInt,
	)
	return err
}

// SetRuleAckDone marks rule acknowledgment as done for a verification.
func (d *DB) SetRuleAckDone(userID, groupID int64) error {
	_, err := d.Exec("UPDATE pending_verifications SET rule_ack_done = 1 WHERE user_id = ? AND group_id = ?", userID, groupID)
	return err
}

// GetPendingVerification returns a pending verification.
func (d *DB) GetPendingVerification(userID, groupID int64) (*model.PendingVerification, error) {
	var pv model.PendingVerification
	err := d.QueryRow(
		"SELECT "+pendingVerificationColumns+" FROM pending_verifications WHERE user_id = ? AND group_id = ?", userID, groupID,
	).Scan(&pv.UserID, &pv.GroupID, &pv.CaptchaText, &pv.ExpiresAt, &pv.WelcomeMessageID, &pv.Attempts, &pv.RuleAckDone)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pv, nil
}

// RemovePendingVerification removes a pending verification.
func (d *DB) RemovePendingVerification(userID, groupID int64) error {
	_, err := d.Exec("DELETE FROM pending_verifications WHERE user_id = ? AND group_id = ?", userID, groupID)
	return err
}

// GetPendingVerificationsByUser returns all pending (non-expired) verifications for a user.
func (d *DB) GetPendingVerificationsByUser(userID int64) ([]model.PendingVerification, error) {
	now := currentTimestamp()
	rows, err := d.Query(
		"SELECT "+pendingVerificationColumns+" FROM pending_verifications WHERE user_id = ? AND expires_at > ?", userID, now,
	)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var verifications []model.PendingVerification
	for rows.Next() {
		var pv model.PendingVerification
		if err := rows.Scan(&pv.UserID, &pv.GroupID, &pv.CaptchaText, &pv.ExpiresAt, &pv.WelcomeMessageID, &pv.Attempts, &pv.RuleAckDone); err != nil {
			return nil, err
		}
		verifications = append(verifications, pv)
	}
	return verifications, rows.Err()
}

// CleanExpiredVerifications removes expired verifications.
func (d *DB) CleanExpiredVerifications() error {
	now := currentTimestamp()
	_, err := d.Exec("DELETE FROM pending_verifications WHERE expires_at < ?", now)
	return err
}

// IncrementVerificationAttempts increments attempts for all pending verifications of a user.
func (d *DB) IncrementVerificationAttempts(userID int64) error {
	now := currentTimestamp()
	_, err := d.Exec(
		"UPDATE pending_verifications SET attempts = attempts + 1 WHERE user_id = ? AND expires_at > ?",
		userID, now,
	)
	return err
}

// ── Vote Kick ─────────────────────────────────────────────────────

// CreateActiveVote creates a new active vote.
func (d *DB) CreateActiveVote(voteID string, groupID, targetID, initiatorID, createdAt, expiresAt int64) error {
	if err := d.CleanExpiredVotes(); err != nil {
		return err
	}
	_, err := d.Exec(
		"INSERT INTO active_votes (vote_id, group_id, target_id, initiator_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		voteID, groupID, targetID, initiatorID, createdAt, expiresAt,
	)
	return err
}

// GetActiveVote returns an active vote by ID.
func (d *DB) GetActiveVote(voteID string) (*model.ActiveVote, error) {
	var v model.ActiveVote
	err := d.QueryRow("SELECT "+activeVoteColumns+" FROM active_votes WHERE vote_id = ?", voteID).Scan(
		&v.VoteID, &v.GroupID, &v.TargetID, &v.InitiatorID, &v.MessageID, &v.CreatedAt, &v.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetActiveVoteForTarget returns an active vote for a target in a group.
func (d *DB) GetActiveVoteForTarget(groupID, targetID int64) (*model.ActiveVote, error) {
	now := currentTimestamp()
	var v model.ActiveVote
	err := d.QueryRow(
		"SELECT "+activeVoteColumns+" FROM active_votes WHERE group_id = ? AND target_id = ? AND expires_at > ?",
		groupID, targetID, now,
	).Scan(&v.VoteID, &v.GroupID, &v.TargetID, &v.InitiatorID, &v.MessageID, &v.CreatedAt, &v.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// GetLastVoteByInitiator returns the last vote initiated by a user in a group.
func (d *DB) GetLastVoteByInitiator(groupID, initiatorID int64) (*model.ActiveVote, error) {
	var v model.ActiveVote
	err := d.QueryRow(
		"SELECT "+activeVoteColumns+" FROM active_votes WHERE group_id = ? AND initiator_id = ? ORDER BY created_at DESC LIMIT 1",
		groupID, initiatorID,
	).Scan(&v.VoteID, &v.GroupID, &v.TargetID, &v.InitiatorID, &v.MessageID, &v.CreatedAt, &v.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// UpdateVoteMessageID updates the message ID for a vote.
func (d *DB) UpdateVoteMessageID(voteID string, messageID int64) error {
	_, err := d.Exec("UPDATE active_votes SET message_id = ? WHERE vote_id = ?", messageID, voteID)
	return err
}

// DeleteActiveVote deletes a vote and its records.
func (d *DB) DeleteActiveVote(voteID string) error {
	_, err := d.Exec("DELETE FROM active_votes WHERE vote_id = ?", voteID)
	if err != nil {
		return err
	}
	_, err = d.Exec("DELETE FROM vote_records WHERE vote_id = ?", voteID)
	return err
}

// AddVoteRecord adds a vote record. Returns false if already voted.
func (d *DB) AddVoteRecord(voteID string, voterID int64, choice int) (bool, error) {
	_, err := d.Exec(
		"INSERT INTO vote_records (vote_id, voter_id, choice) VALUES (?, ?, ?)",
		voteID, voterID, choice,
	)
	if err != nil {
		// Check for UNIQUE constraint violation
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == 2067 { // SQLITE_CONSTRAINT_UNIQUE
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetVoteCounts returns the yes/no counts for a vote.
func (d *DB) GetVoteCounts(voteID string) (yes, no int, err error) {
	rows, err := d.Query(
		"SELECT choice, COUNT(*) as cnt FROM vote_records WHERE vote_id = ? GROUP BY choice", voteID,
	)
	if err != nil {
		return 0, 0, err
	}
	defer closeRows(rows)

	for rows.Next() {
		var choice, cnt int
		if err := rows.Scan(&choice, &cnt); err != nil {
			return 0, 0, err
		}
		if choice == 1 {
			yes = cnt
		} else {
			no = cnt
		}
	}
	return yes, no, rows.Err()
}

// CleanExpiredVotes removes expired votes and their records.
func (d *DB) CleanExpiredVotes() error {
	now := currentTimestamp()
	_, err := d.Exec(
		"DELETE FROM vote_records WHERE vote_id IN (SELECT vote_id FROM active_votes WHERE expires_at < ?)", now,
	)
	if err != nil {
		return err
	}
	_, err = d.Exec("DELETE FROM active_votes WHERE expires_at < ?", now)
	return err
}

// GetExpiredVotes returns expired votes that have a message ID.
func (d *DB) GetExpiredVotes() ([]model.ActiveVote, error) {
	now := currentTimestamp()
	rows, err := d.Query(
		"SELECT "+activeVoteColumns+" FROM active_votes WHERE expires_at < ? AND message_id IS NOT NULL", now,
	)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var votes []model.ActiveVote
	for rows.Next() {
		var v model.ActiveVote
		if err := rows.Scan(&v.VoteID, &v.GroupID, &v.TargetID, &v.InitiatorID, &v.MessageID, &v.CreatedAt, &v.ExpiresAt); err != nil {
			return nil, err
		}
		votes = append(votes, v)
	}
	return votes, rows.Err()
}

// ── Locks ─────────────────────────────────────────────────────────

// AcquireLock attempts to acquire a lock with TTL.
func (d *DB) AcquireLock(name string, ttlSeconds int64) (bool, error) {
	now := currentTimestamp()
	expiresAt := now + ttlSeconds

	_, err := d.Exec(
		`INSERT INTO locks (name, expires_at) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET expires_at = excluded.expires_at
		WHERE locks.expires_at < ?`,
		name, expiresAt, now,
	)
	if err != nil {
		return false, err
	}

	// Check if we got the lock
	var actualExpiresAt int64
	err = d.QueryRow("SELECT expires_at FROM locks WHERE name = ?", name).Scan(&actualExpiresAt)
	if err != nil {
		return false, err
	}
	return actualExpiresAt == expiresAt, nil
}

// ReleaseLock releases a lock.
func (d *DB) ReleaseLock(name string) error {
	_, err := d.Exec("DELETE FROM locks WHERE name = ?", name)
	return err
}

// ── Warnings ──────────────────────────────────────────────────────

// AddWarning adds a warning for a user.
func (d *DB) AddWarning(groupID, userID, adminID int64, reason string) error {
	now := currentTimestamp()
	_, err := d.Exec(
		"INSERT INTO warnings (group_id, user_id, admin_id, reason, created_at) VALUES (?, ?, ?, ?, ?)",
		groupID, userID, adminID, reason, now,
	)
	return err
}

// GetWarningCount returns the number of warnings for a user in a group.
func (d *DB) GetWarningCount(groupID, userID int64) (int, error) {
	var count int
	err := d.QueryRow(
		"SELECT COUNT(*) FROM warnings WHERE group_id = ? AND user_id = ?", groupID, userID,
	).Scan(&count)
	return count, err
}

// GetWarnings returns warnings for a group, optionally filtered by user.
func (d *DB) GetWarnings(groupID int64, userID *int64) ([]model.Warning, error) {
	var rows *sql.Rows
	var err error

	if userID != nil {
		rows, err = d.Query(
			"SELECT "+warningColumns+" FROM warnings WHERE group_id = ? AND user_id = ? ORDER BY created_at DESC",
			groupID, *userID,
		)
	} else {
		rows, err = d.Query(
			"SELECT "+warningColumns+" FROM warnings WHERE group_id = ? ORDER BY created_at DESC", groupID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var warnings []model.Warning
	for rows.Next() {
		var w model.Warning
		if err := rows.Scan(&w.ID, &w.GroupID, &w.UserID, &w.AdminID, &w.Reason, &w.CreatedAt); err != nil {
			return nil, err
		}
		warnings = append(warnings, w)
	}
	return warnings, rows.Err()
}

// GetAllWarnings returns all warnings, optionally filtered by admin's groups.
func (d *DB) GetAllWarnings(userID *int64) ([]model.Warning, error) {
	query := "SELECT w.id, w.group_id, w.user_id, w.admin_id, w.reason, w.created_at FROM warnings w"
	var args []interface{}

	if userID != nil {
		query += " JOIN admin_connections ac ON w.group_id = ac.group_id WHERE ac.user_id = ?"
		args = append(args, *userID)
	}
	query += " ORDER BY w.created_at DESC"

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var warnings []model.Warning
	for rows.Next() {
		var w model.Warning
		if err := rows.Scan(&w.ID, &w.GroupID, &w.UserID, &w.AdminID, &w.Reason, &w.CreatedAt); err != nil {
			return nil, err
		}
		warnings = append(warnings, w)
	}
	return warnings, rows.Err()
}

// ── Reports ───────────────────────────────────────────────────────

// AddReport adds a new report.
func (d *DB) AddReport(groupID, reporterID, reportedUserID int64, reportedMessageID *int64, reportedMessageText, content string) error {
	now := currentTimestamp()
	_, err := d.Exec(
		`INSERT INTO reports (group_id, reporter_id, reported_user_id, reported_message_id, reported_message_text, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		groupID, reporterID, reportedUserID, reportedMessageID, reportedMessageText, content, now,
	)
	return err
}

// GetReports returns reports, optionally filtered by status and admin's groups.
func (d *DB) GetReports(status *string, userID *int64) ([]model.Report, error) {
	query := "SELECT " + reportColumnsWithAlias + " FROM reports r"
	var conditions []string
	var args []interface{}

	if userID != nil {
		query += " JOIN admin_connections ac ON r.group_id = ac.group_id"
		conditions = append(conditions, "ac.user_id = ?")
		args = append(args, *userID)
	}
	if status != nil {
		conditions = append(conditions, "r.status = ?")
		args = append(args, *status)
	}

	if len(conditions) > 0 {
		query += " WHERE " + joinStrings(conditions, " AND ")
	}
	query += " ORDER BY r.created_at DESC"

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var reports []model.Report
	for rows.Next() {
		var r model.Report
		if err := rows.Scan(
			&r.ID, &r.GroupID, &r.ReporterID, &r.ReportedUserID,
			&r.ReportedMessageID, &r.ReportedMessageText, &r.Content,
			&r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// GetReport returns a single report by ID.
func (d *DB) GetReport(reportID int64, userID *int64) (*model.Report, error) {
	query := "SELECT " + reportColumnsWithAlias + " FROM reports r"
	var args []interface{}
	conditions := []string{"r.id = ?"}
	args = append(args, reportID)

	if userID != nil {
		query += " JOIN admin_connections ac ON r.group_id = ac.group_id"
		conditions = append(conditions, "ac.user_id = ?")
		args = append(args, *userID)
	}

	query += " WHERE " + joinStrings(conditions, " AND ")

	var r model.Report
	err := d.QueryRow(query, args...).Scan(
		&r.ID, &r.GroupID, &r.ReporterID, &r.ReportedUserID,
		&r.ReportedMessageID, &r.ReportedMessageText, &r.Content,
		&r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateReportStatus updates the status of a report.
func (d *DB) UpdateReportStatus(reportID int64, status string, reviewedBy int64) error {
	now := currentTimestamp()
	_, err := d.Exec(
		"UPDATE reports SET status = ?, reviewed_by = ?, reviewed_at = ? WHERE id = ?",
		status, reviewedBy, now, reportID,
	)
	return err
}

// ── KV Store ──────────────────────────────────────────────────────

// KVGet returns the value for a key, or nil if not found or expired.
func (d *DB) KVGet(key string) (*string, error) {
	now := currentTimestamp()
	var value string
	err := d.QueryRow(
		"SELECT value FROM kv WHERE key = ? AND expires_at > ?", key, now,
	).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// KVSet sets a key-value pair with TTL in seconds.
func (d *DB) KVSet(key, value string, ttlSeconds int64) error {
	expiresAt := currentTimestamp() + ttlSeconds
	_, err := d.Exec(
		`INSERT INTO kv (key, value, expires_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, value, expiresAt,
	)
	return err
}

// KVDelete deletes a key-value pair.
func (d *DB) KVDelete(key string) error {
	_, err := d.Exec("DELETE FROM kv WHERE key = ?", key)
	return err
}

// CleanExpiredKV removes expired key-value pairs.
func (d *DB) CleanExpiredKV() error {
	now := currentTimestamp()
	_, err := d.Exec("DELETE FROM kv WHERE expires_at < ?", now)
	return err
}

// ── Helpers ───────────────────────────────────────────────────────

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func closeRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		slog.Warn("Failed to close database rows", "error", err)
	}
}

// ── Legacy Compatibility ──────────────────────────────────────────
// These functions maintain compatibility with the original TypeScript API names.

// InitGroupCompat is a compatibility wrapper.
func (d *DB) InitGroupCompat(groupID int) error {
	return d.InitGroup(int64(groupID))
}

// GetGroupConfigCompat is a compatibility wrapper.
func (d *DB) GetGroupConfigCompat(groupID int) (*model.GroupConfig, error) {
	return d.GetGroupConfig(int64(groupID))
}

// AddPendingVerificationCompat is a compatibility wrapper.
func (d *DB) AddPendingVerificationCompat(userID, groupID int, captchaText string, expiresAt int64, welcomeMessageID *int, ruleAckDone bool) error {
	var wmID *int64
	if welcomeMessageID != nil {
		v := int64(*welcomeMessageID)
		wmID = &v
	}
	return d.AddPendingVerification(int64(userID), int64(groupID), captchaText, expiresAt, wmID, ruleAckDone)
}

// GetPendingVerificationCompat is a compatibility wrapper.
func (d *DB) GetPendingVerificationCompat(userID, groupID int) (*model.PendingVerification, error) {
	return d.GetPendingVerification(int64(userID), int64(groupID))
}

// RemovePendingVerificationCompat is a compatibility wrapper.
func (d *DB) RemovePendingVerificationCompat(userID, groupID int) error {
	return d.RemovePendingVerification(int64(userID), int64(groupID))
}

// IncrementVerificationAttemptsCompat is a compatibility wrapper.
func (d *DB) IncrementVerificationAttemptsCompat(userID int) error {
	return d.IncrementVerificationAttempts(int64(userID))
}

// AddVoteRecordCompat is a compatibility wrapper.
func (d *DB) AddVoteRecordCompat(voteID string, voterID int, choice int) (bool, error) {
	return d.AddVoteRecord(voteID, int64(voterID), choice)
}

// GetVoteCountsCompat is a compatibility wrapper.
func (d *DB) GetVoteCountsCompat(voteID string) (yes, no int, err error) {
	return d.GetVoteCounts(voteID)
}

// CreateActiveVoteCompat is a compatibility wrapper.
func (d *DB) CreateActiveVoteCompat(voteID string, groupID, targetID, initiatorID, createdAt, expiresAt int) error {
	return d.CreateActiveVote(voteID, int64(groupID), int64(targetID), int64(initiatorID), int64(createdAt), int64(expiresAt))
}

// GetActiveVoteCompat is a compatibility wrapper.
func (d *DB) GetActiveVoteCompat(voteID string) (*model.ActiveVote, error) {
	return d.GetActiveVote(voteID)
}

// GetActiveVoteForTargetCompat is a compatibility wrapper.
func (d *DB) GetActiveVoteForTargetCompat(groupID, targetID int) (*model.ActiveVote, error) {
	return d.GetActiveVoteForTarget(int64(groupID), int64(targetID))
}

// DeleteActiveVoteCompat is a compatibility wrapper.
func (d *DB) DeleteActiveVoteCompat(voteID string) error {
	return d.DeleteActiveVote(voteID)
}

// AddWarningCompat is a compatibility wrapper.
func (d *DB) AddWarningCompat(groupID, userID, adminID int, reason string) error {
	return d.AddWarning(int64(groupID), int64(userID), int64(adminID), reason)
}

// GetWarningCountCompat is a compatibility wrapper.
func (d *DB) GetWarningCountCompat(groupID, userID int) (int, error) {
	return d.GetWarningCount(int64(groupID), int64(userID))
}

// GetWarningsCompat is a compatibility wrapper.
func (d *DB) GetWarningsCompat(groupID int, userID *int) ([]model.Warning, error) {
	var uid *int64
	if userID != nil {
		v := int64(*userID)
		uid = &v
	}
	return d.GetWarnings(int64(groupID), uid)
}

// AddReportCompat is a compatibility wrapper.
func (d *DB) AddReportCompat(groupID, reporterID, reportedUserID int, reportedMessageID *int, reportedMessageText, content string) error {
	var rmID *int64
	if reportedMessageID != nil {
		v := int64(*reportedMessageID)
		rmID = &v
	}
	return d.AddReport(int64(groupID), int64(reporterID), int64(reportedUserID), rmID, reportedMessageText, content)
}

// GetReportsCompat is a compatibility wrapper.
func (d *DB) GetReportsCompat(status *string, userID *int) ([]model.Report, error) {
	var uid *int64
	if userID != nil {
		v := int64(*userID)
		uid = &v
	}
	return d.GetReports(status, uid)
}

// GetReportCompat is a compatibility wrapper.
func (d *DB) GetReportCompat(reportID int, userID *int) (*model.Report, error) {
	var uid *int64
	if userID != nil {
		v := int64(*userID)
		uid = &v
	}
	return d.GetReport(int64(reportID), uid)
}

// UpdateReportStatusCompat is a compatibility wrapper.
func (d *DB) UpdateReportStatusCompat(reportID int, status string, reviewedBy int) error {
	return d.UpdateReportStatus(int64(reportID), status, int64(reviewedBy))
}

// ConnectAdminCompat is a compatibility wrapper.
func (d *DB) ConnectAdminCompat(userID, groupID int) error {
	return d.ConnectAdmin(int64(userID), int64(groupID))
}

// GetAdminGroupIdCompat is a compatibility wrapper.
func (d *DB) GetAdminGroupIdCompat(userID int) (*int, error) {
	result, err := d.GetAdminGroupID(int64(userID))
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	v := int(*result)
	return &v, nil
}

// GetAdminGroupsCompat is a compatibility wrapper.
func (d *DB) GetAdminGroupsCompat(userID int) ([]int, error) {
	groups, err := d.GetAdminGroups(int64(userID))
	if err != nil {
		return nil, err
	}
	result := make([]int, len(groups))
	for i, g := range groups {
		result[i] = int(g)
	}
	return result, nil
}

// IsAdminConnectedCompat is a compatibility wrapper.
func (d *DB) IsAdminConnectedCompat(userID, groupID int) (bool, error) {
	return d.IsAdminConnected(int64(userID), int64(groupID))
}

// SetCurrentGroupCompat is a compatibility wrapper.
func (d *DB) SetCurrentGroupCompat(userID, groupID int) error {
	return d.SetCurrentGroup(int64(userID), int64(groupID))
}

// AddKeywordCompat is a compatibility wrapper.
func (d *DB) AddKeywordCompat(groupID int, pattern string, isRegex bool, replyContent, replyType string) error {
	return d.AddKeyword(int64(groupID), pattern, isRegex, replyContent, replyType)
}

// GetKeywordsCompat is a compatibility wrapper.
func (d *DB) GetKeywordsCompat(groupID int) ([]model.KeywordRule, error) {
	return d.GetKeywords(int64(groupID))
}

// RemoveKeywordCompat is a compatibility wrapper.
func (d *DB) RemoveKeywordCompat(groupID int, pattern string) (bool, error) {
	return d.RemoveKeyword(int64(groupID), pattern)
}

// UpdateWelcomeMessageCompat is a compatibility wrapper.
func (d *DB) UpdateWelcomeMessageCompat(groupID int, message string) error {
	return d.UpdateWelcomeMessage(int64(groupID), message)
}

// SetWelcomeEnabledCompat is a compatibility wrapper.
func (d *DB) SetWelcomeEnabledCompat(groupID int, enabled bool) error {
	return d.SetWelcomeEnabled(int64(groupID), enabled)
}

// UpdateVerifyButtonTextCompat is a compatibility wrapper.
func (d *DB) UpdateVerifyButtonTextCompat(groupID int, text string) error {
	return d.UpdateVerifyButtonText(int64(groupID), text)
}

// UpdateVerifyTimeoutCompat is a compatibility wrapper.
func (d *DB) UpdateVerifyTimeoutCompat(groupID int, timeout int) error {
	return d.UpdateVerifyTimeout(int64(groupID), timeout)
}

// UpdateGroupRuleCompat is a compatibility wrapper.
func (d *DB) UpdateGroupRuleCompat(groupID int, rule string) error {
	return d.UpdateGroupRule(int64(groupID), rule)
}

// SetVotekickEnabledCompat is a compatibility wrapper.
func (d *DB) SetVotekickEnabledCompat(groupID int, enabled bool) error {
	return d.SetVotekickEnabled(int64(groupID), enabled)
}

// SetRuleAckDoneCompat is a compatibility wrapper.
func (d *DB) SetRuleAckDoneCompat(userID, groupID int) error {
	return d.SetRuleAckDone(int64(userID), int64(groupID))
}

// GetLastVoteByInitiatorCompat is a compatibility wrapper.
func (d *DB) GetLastVoteByInitiatorCompat(groupID, initiatorID int) (*model.ActiveVote, error) {
	return d.GetLastVoteByInitiator(int64(groupID), int64(initiatorID))
}

// UpdateVoteMessageIDCompat is a compatibility wrapper.
func (d *DB) UpdateVoteMessageIDCompat(voteID string, messageID int) error {
	return d.UpdateVoteMessageID(voteID, int64(messageID))
}

// AcquireLockCompat is a compatibility wrapper.
func (d *DB) AcquireLockCompat(name string, ttlSeconds int) (bool, error) {
	return d.AcquireLock(name, int64(ttlSeconds))
}

// ReleaseLockCompat is a compatibility wrapper.
func (d *DB) ReleaseLockCompat(name string) error {
	return d.ReleaseLock(name)
}

// GetAllWarningsCompat is a compatibility wrapper.
func (d *DB) GetAllWarningsCompat(userID *int) ([]model.Warning, error) {
	var uid *int64
	if userID != nil {
		v := int64(*userID)
		uid = &v
	}
	return d.GetAllWarnings(uid)
}
