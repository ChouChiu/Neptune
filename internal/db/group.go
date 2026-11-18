package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/ChouChiu/neptune/internal/model"
)

const groupColumns = "group_id, welcome_enabled, welcome_message, verify_button_text, verify_timeout, votekick_enabled, rule"

// ErrAdminNotConnected is returned when attempting to switch to a group the admin has not connected.
var ErrAdminNotConnected = errors.New("admin is not connected to group")

// InitGroup creates a group record if it doesn't exist.
func (d *DB) InitGroup(groupID int64) error {
	_, err := d.Exec("INSERT OR IGNORE INTO groups (group_id) VALUES (?)", groupID)
	return err
}

// GetGroupConfig returns the configuration for a group, or nil if not found.
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

func (d *DB) updateGroupColumn(groupID int64, column string, value any) error {
	if err := d.InitGroup(groupID); err != nil {
		return err
	}
	_, err := d.Exec("UPDATE groups SET "+column+" = ? WHERE group_id = ?", value, groupID)
	return err
}

// UpdateWelcomeMessage updates the welcome message for a group.
func (d *DB) UpdateWelcomeMessage(groupID int64, message string) error {
	return d.updateGroupColumn(groupID, "welcome_message", message)
}

// SetWelcomeEnabled enables or disables welcome messages for a group.
func (d *DB) SetWelcomeEnabled(groupID int64, enabled bool) error {
	return d.updateGroupColumn(groupID, "welcome_enabled", boolToInt(enabled))
}

// UpdateVerifyButtonText updates the verify button text for a group.
func (d *DB) UpdateVerifyButtonText(groupID int64, text string) error {
	return d.updateGroupColumn(groupID, "verify_button_text", text)
}

// UpdateVerifyTimeout updates the verify timeout for a group.
func (d *DB) UpdateVerifyTimeout(groupID int64, timeout int) error {
	return d.updateGroupColumn(groupID, "verify_timeout", timeout)
}

// UpdateGroupRule updates the group rule.
func (d *DB) UpdateGroupRule(groupID int64, rule string) error {
	return d.updateGroupColumn(groupID, "rule", rule)
}

// SetVotekickEnabled enables or disables votekick for a group.
func (d *DB) SetVotekickEnabled(groupID int64, enabled bool) error {
	return d.updateGroupColumn(groupID, "votekick_enabled", boolToInt(enabled))
}

// GetAdminGroupID returns the currently selected connected group for an admin.
func (d *DB) GetAdminGroupID(userID int64) (*int64, error) {
	var groupID int64
	err := d.QueryRow(
		`SELECT cg.group_id
		FROM admin_current_group cg
		JOIN admin_connections ac ON cg.user_id = ac.user_id AND cg.group_id = ac.group_id
		WHERE cg.user_id = ?`,
		userID,
	).Scan(&groupID)
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
	if err := d.InitGroup(groupID); err != nil {
		return fmt.Errorf("init group before connect: %w", err)
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("INSERT OR IGNORE INTO admin_connections (user_id, group_id) VALUES (?, ?)", userID, groupID); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)", userID, groupID); err != nil {
		return err
	}
	return tx.Commit()
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

// SetCurrentGroup sets the current group for an admin if the admin is connected to it.
func (d *DB) SetCurrentGroup(userID, groupID int64) error {
	connected, err := d.IsAdminConnected(userID, groupID)
	if err != nil {
		return err
	}
	if !connected {
		return fmt.Errorf("%w: user=%d group=%d", ErrAdminNotConnected, userID, groupID)
	}
	_, err = d.Exec("INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)", userID, groupID)
	return err
}
