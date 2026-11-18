package db

import (
	"database/sql"
	"errors"

	"github.com/ChouChiu/neptune/internal/model"
)

const pendingVerificationColumns = "user_id, group_id, captcha_text, expires_at, welcome_message_id, attempts, rule_ack_done, prompted_on_message"

// AddPendingVerification inserts or resets a pending verification (used on member join).
func (d *DB) AddPendingVerification(userID, groupID int64, captchaText string, expiresAt int64, welcomeMessageID *int64, ruleAckDone bool) error {
	_, err := d.Exec(
		`INSERT INTO pending_verifications (user_id, group_id, captcha_text, expires_at, welcome_message_id, attempts, rule_ack_done, prompted_on_message)
		VALUES (?, ?, ?, ?, ?, 0, ?, 0)
		ON CONFLICT(user_id, group_id) DO UPDATE SET
			captcha_text = excluded.captcha_text,
			expires_at = excluded.expires_at,
			welcome_message_id = excluded.welcome_message_id,
			attempts = 0,
			rule_ack_done = excluded.rule_ack_done,
			prompted_on_message = 0`,
		userID, groupID, captchaText, expiresAt, welcomeMessageID, boolToInt(ruleAckDone),
	)
	return err
}

// MarkFirstMessagePrompted atomically marks an unprompted pending verification as prompted on first group message.
// Returns true if this call transitioned prompted_on_message from 0 to 1.
func (d *DB) MarkFirstMessagePrompted(userID, groupID, expiresAt int64) (bool, error) {
	res, err := d.Exec(
		`UPDATE pending_verifications
		SET prompted_on_message = 1,
			captcha_text = '',
			expires_at = ?,
			attempts = 0,
			rule_ack_done = 0
		WHERE user_id = ? AND group_id = ? AND prompted_on_message = 0`,
		expiresAt, userID, groupID,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// UpdatePendingWelcomeMessageID updates the welcome_message_id for a pending verification.
func (d *DB) UpdatePendingWelcomeMessageID(userID, groupID int64, welcomeMessageID *int64) error {
	_, err := d.Exec(
		"UPDATE pending_verifications SET welcome_message_id = ? WHERE user_id = ? AND group_id = ?",
		welcomeMessageID, userID, groupID,
	)
	return err
}

// SetPendingCaptcha sets or refreshes captcha details while preserving existing attempt counts.
func (d *DB) SetPendingCaptcha(userID, groupID int64, captchaText string, expiresAt int64, welcomeMessageID *int64, ruleAckDone bool) error {
	_, err := d.Exec(
		`INSERT INTO pending_verifications (user_id, group_id, captcha_text, expires_at, welcome_message_id, attempts, rule_ack_done)
		VALUES (?, ?, ?, ?, ?, 0, ?)
		ON CONFLICT(user_id, group_id) DO UPDATE SET
			captcha_text = excluded.captcha_text,
			expires_at = excluded.expires_at,
			welcome_message_id = COALESCE(excluded.welcome_message_id, pending_verifications.welcome_message_id),
			rule_ack_done = excluded.rule_ack_done`,
		userID, groupID, captchaText, expiresAt, welcomeMessageID, boolToInt(ruleAckDone),
	)
	return err
}

// SetRuleAckDone marks rule acknowledgment as done for a verification.
func (d *DB) SetRuleAckDone(userID, groupID int64) error {
	_, err := d.Exec("UPDATE pending_verifications SET rule_ack_done = 1 WHERE user_id = ? AND group_id = ?", userID, groupID)
	return err
}

// GetPendingVerification returns a pending verification, or nil if not found.
func (d *DB) GetPendingVerification(userID, groupID int64) (*model.PendingVerification, error) {
	var pv model.PendingVerification
	err := d.QueryRow(
		"SELECT "+pendingVerificationColumns+" FROM pending_verifications WHERE user_id = ? AND group_id = ?", userID, groupID,
	).Scan(&pv.UserID, &pv.GroupID, &pv.CaptchaText, &pv.ExpiresAt, &pv.WelcomeMessageID, &pv.Attempts, &pv.RuleAckDone, &pv.PromptedOnMessage)
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

// GetPendingVerificationsByUser returns active (non-expired, captcha-issued) verifications for a user.
func (d *DB) GetPendingVerificationsByUser(userID int64) ([]model.PendingVerification, error) {
	now := currentTimestamp()
	rows, err := d.Query(
		"SELECT "+pendingVerificationColumns+" FROM pending_verifications WHERE user_id = ? AND expires_at > ? AND captcha_text != ''",
		userID, now,
	)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var verifications []model.PendingVerification
	for rows.Next() {
		var pv model.PendingVerification
		if err := rows.Scan(&pv.UserID, &pv.GroupID, &pv.CaptchaText, &pv.ExpiresAt, &pv.WelcomeMessageID, &pv.Attempts, &pv.RuleAckDone, &pv.PromptedOnMessage); err != nil {
			return nil, err
		}
		verifications = append(verifications, pv)
	}
	return verifications, rows.Err()
}

// CleanExpiredVerifications removes expired verifications that have already been prompted and restricted.
func (d *DB) CleanExpiredVerifications() error {
	now := currentTimestamp()
	_, err := d.Exec("DELETE FROM pending_verifications WHERE prompted_on_message = 1 AND expires_at < ?", now)
	return err
}

// IncrementVerificationAttempts increments attempts for all active captcha verifications of a user.
func (d *DB) IncrementVerificationAttempts(userID int64) error {
	now := currentTimestamp()
	_, err := d.Exec(
		"UPDATE pending_verifications SET attempts = attempts + 1 WHERE user_id = ? AND expires_at > ? AND captcha_text != ''",
		userID, now,
	)
	return err
}

// AcquireLock attempts to acquire a lock with TTL in seconds.
func (d *DB) AcquireLock(name string, ttlSeconds int64) (bool, error) {
	now := currentTimestamp()
	expiresAt := now + ttlSeconds

	res, err := d.Exec(
		`INSERT INTO locks (name, expires_at) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET expires_at = excluded.expires_at
		WHERE locks.expires_at <= ?`,
		name, expiresAt, now,
	)
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ReleaseLock releases a lock.
func (d *DB) ReleaseLock(name string) error {
	_, err := d.Exec("DELETE FROM locks WHERE name = ?", name)
	return err
}
