package db

import (
	"database/sql"
	"errors"

	"github.com/ChouChiu/neptune/internal/model"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const activeVoteColumns = "vote_id, group_id, target_id, initiator_id, message_id, created_at, expires_at"

// CreateActiveVote creates a new active vote after cleaning up expired ones.
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

// GetActiveVote returns an active vote by ID, or nil if not found.
func (d *DB) GetActiveVote(voteID string) (*model.ActiveVote, error) {
	return d.scanActiveVote("SELECT "+activeVoteColumns+" FROM active_votes WHERE vote_id = ?", voteID)
}

// GetActiveVoteForTarget returns an active (non-expired) vote for a target in a group.
func (d *DB) GetActiveVoteForTarget(groupID, targetID int64) (*model.ActiveVote, error) {
	now := currentTimestamp()
	return d.scanActiveVote(
		"SELECT "+activeVoteColumns+" FROM active_votes WHERE group_id = ? AND target_id = ? AND expires_at > ?",
		groupID, targetID, now,
	)
}

// GetLastVoteByInitiator returns the last vote initiated by a user in a group.
func (d *DB) GetLastVoteByInitiator(groupID, initiatorID int64) (*model.ActiveVote, error) {
	return d.scanActiveVote(
		"SELECT "+activeVoteColumns+" FROM active_votes WHERE group_id = ? AND initiator_id = ? ORDER BY created_at DESC LIMIT 1",
		groupID, initiatorID,
	)
}

func (d *DB) scanActiveVote(query string, args ...any) (*model.ActiveVote, error) {
	var v model.ActiveVote
	err := d.QueryRow(query, args...).Scan(
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

// UpdateVoteMessageID updates the message ID for a vote.
func (d *DB) UpdateVoteMessageID(voteID string, messageID int64) error {
	_, err := d.Exec("UPDATE active_votes SET message_id = ? WHERE vote_id = ?", messageID, voteID)
	return err
}

// DeleteActiveVote deletes a vote and its records atomically.
func (d *DB) DeleteActiveVote(voteID string) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM vote_records WHERE vote_id = ?", voteID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM active_votes WHERE vote_id = ?", voteID); err != nil {
		return err
	}
	return tx.Commit()
}

// AddVoteRecord adds a vote record. Returns false if the user has already voted.
func (d *DB) AddVoteRecord(voteID string, voterID int64, choice int) (bool, error) {
	_, err := d.Exec(
		"INSERT INTO vote_records (vote_id, voter_id, choice) VALUES (?, ?, ?)",
		voteID, voterID, choice,
	)
	if err != nil {
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) &&
			(sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY || sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE) {
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

// CleanExpiredVotes removes expired votes and their records atomically.
func (d *DB) CleanExpiredVotes() error {
	now := currentTimestamp()
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM vote_records WHERE vote_id IN (SELECT vote_id FROM active_votes WHERE expires_at < ?)", now); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM active_votes WHERE expires_at < ?", now); err != nil {
		return err
	}
	return tx.Commit()
}
