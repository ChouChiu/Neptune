package db

import "github.com/ChouChiu/neptune/internal/model"

const keywordColumns = "id, group_id, pattern, is_regex, reply_content, reply_type"

// AddKeyword adds a keyword rule to a group.
func (d *DB) AddKeyword(groupID int64, pattern string, isRegex bool, replyContent, replyType string) error {
	if err := d.InitGroup(groupID); err != nil {
		return err
	}
	if replyType == "" {
		replyType = "text"
	}
	_, err := d.Exec(
		"INSERT INTO keywords (group_id, pattern, is_regex, reply_content, reply_type) VALUES (?, ?, ?, ?, ?)",
		groupID, pattern, boolToInt(isRegex), replyContent, replyType,
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
