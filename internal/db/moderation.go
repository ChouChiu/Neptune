package db

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/ChouChiu/neptune/internal/model"
)

const (
	warningColumns         = "id, group_id, user_id, admin_id, reason, created_at"
	reportColumnsWithAlias = "r.id, r.group_id, r.reporter_id, r.reported_user_id, r.reported_message_id, r.reported_message_text, r.content, r.status, r.reviewed_by, r.reviewed_at, r.created_at"
)

// AddWarning adds a warning for a user.
func (d *DB) AddWarning(groupID, userID, adminID int64, reason string) error {
	if err := d.InitGroup(groupID); err != nil {
		return err
	}
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
	query := "SELECT " + warningColumns + " FROM warnings WHERE group_id = ?"
	args := []any{groupID}
	if userID != nil {
		query += " AND user_id = ?"
		args = append(args, *userID)
	}
	query += " ORDER BY created_at DESC"
	return d.queryWarnings(query, args...)
}

// GetAllWarnings returns all warnings, optionally filtered by admin's connected groups.
func (d *DB) GetAllWarnings(userID *int64) ([]model.Warning, error) {
	query := "SELECT w.id, w.group_id, w.user_id, w.admin_id, w.reason, w.created_at FROM warnings w"
	var args []any
	if userID != nil {
		query += " JOIN admin_connections ac ON w.group_id = ac.group_id WHERE ac.user_id = ?"
		args = append(args, *userID)
	}
	query += " ORDER BY w.created_at DESC"
	return d.queryWarnings(query, args...)
}

func (d *DB) queryWarnings(query string, args ...any) ([]model.Warning, error) {
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

// AddReport adds a new report.
func (d *DB) AddReport(groupID, reporterID, reportedUserID int64, reportedMessageID *int64, reportedMessageText, content string) error {
	if err := d.InitGroup(groupID); err != nil {
		return err
	}
	now := currentTimestamp()
	_, err := d.Exec(
		`INSERT INTO reports (group_id, reporter_id, reported_user_id, reported_message_id, reported_message_text, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		groupID, reporterID, reportedUserID, reportedMessageID, reportedMessageText, content, now,
	)
	return err
}

// GetReports returns reports, optionally filtered by status and admin's connected groups.
func (d *DB) GetReports(status *string, userID *int64) ([]model.Report, error) {
	query := "SELECT " + reportColumnsWithAlias + " FROM reports r"
	var conditions []string
	var args []any

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
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY r.created_at DESC"

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer closeRows(rows)

	var reports []model.Report
	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

// GetReport returns a single report by ID, optionally scoped to admin's connected groups.
func (d *DB) GetReport(reportID int64, userID *int64) (*model.Report, error) {
	query := "SELECT " + reportColumnsWithAlias + " FROM reports r"
	conditions := []string{"r.id = ?"}
	args := []any{reportID}

	if userID != nil {
		query += " JOIN admin_connections ac ON r.group_id = ac.group_id"
		conditions = append(conditions, "ac.user_id = ?")
		args = append(args, *userID)
	}

	query += " WHERE " + strings.Join(conditions, " AND ")

	r, err := scanReport(d.QueryRow(query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func scanReport(scanner interface{ Scan(dest ...any) error }) (model.Report, error) {
	var r model.Report
	err := scanner.Scan(
		&r.ID, &r.GroupID, &r.ReporterID, &r.ReportedUserID,
		&r.ReportedMessageID, &r.ReportedMessageText, &r.Content,
		&r.Status, &r.ReviewedBy, &r.ReviewedAt, &r.CreatedAt,
	)
	return r, err
}

// ResolvePendingReport atomically transitions a pending report to a resolved status.
// Returns false if the report was not in pending state or did not exist.
func (d *DB) ResolvePendingReport(reportID int64, status string, reviewedBy int64) (bool, error) {
	now := currentTimestamp()
	res, err := d.Exec(
		"UPDATE reports SET status = ?, reviewed_by = ?, reviewed_at = ? WHERE id = ? AND status = 'pending'",
		status, reviewedBy, now, reportID,
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
