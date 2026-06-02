package db

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// baseSchema contains the core tables.
const baseSchema = `
CREATE TABLE IF NOT EXISTS groups (
  group_id INTEGER PRIMARY KEY,
  welcome_enabled INTEGER DEFAULT 0,
  welcome_message TEXT DEFAULT '欢迎 {nickname} 加入群组！',
  verify_button_text TEXT DEFAULT '开始认证',
  verify_timeout INTEGER DEFAULT 300,
  votekick_enabled INTEGER DEFAULT 0,
  rule TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS keywords (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL,
  pattern TEXT NOT NULL,
  is_regex INTEGER DEFAULT 0,
  reply_content TEXT NOT NULL,
  reply_type TEXT DEFAULT 'text',
  FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE TABLE IF NOT EXISTS admin_connections (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  PRIMARY KEY (user_id, group_id),
  FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE TABLE IF NOT EXISTS admin_current_group (
  user_id INTEGER PRIMARY KEY,
  group_id INTEGER NOT NULL,
  FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE TABLE IF NOT EXISTS pending_verifications (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  captcha_text TEXT NOT NULL,
  expires_at INTEGER NOT NULL,
  welcome_message_id INTEGER,
  attempts INTEGER DEFAULT 0,
  rule_ack_done INTEGER DEFAULT 0,
  PRIMARY KEY (user_id, group_id)
);

CREATE TABLE IF NOT EXISTS active_votes (
  vote_id TEXT PRIMARY KEY,
  group_id INTEGER NOT NULL,
  target_id INTEGER NOT NULL,
  initiator_id INTEGER NOT NULL,
  message_id INTEGER,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS vote_records (
  vote_id TEXT NOT NULL,
  voter_id INTEGER NOT NULL,
  choice INTEGER NOT NULL,
  PRIMARY KEY (vote_id, voter_id)
);

CREATE TABLE IF NOT EXISTS warnings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL,
  user_id INTEGER NOT NULL,
  admin_id INTEGER NOT NULL,
  reason TEXT DEFAULT '',
  created_at INTEGER NOT NULL,
  FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE TABLE IF NOT EXISTS reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL,
  reporter_id INTEGER NOT NULL,
  reported_user_id INTEGER NOT NULL,
  reported_message_id INTEGER,
  reported_message_text TEXT DEFAULT '',
  content TEXT NOT NULL,
  status TEXT DEFAULT 'pending',
  reviewed_by INTEGER,
  reviewed_at INTEGER,
  created_at INTEGER NOT NULL,
  FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE INDEX IF NOT EXISTS idx_warnings_group_user ON warnings(group_id, user_id);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);

CREATE TABLE IF NOT EXISTS locks (
  name TEXT PRIMARY KEY,
  expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS ai_chat_usage (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  count INTEGER DEFAULT 1,
  PRIMARY KEY (user_id, group_id, date)
);

CREATE TABLE IF NOT EXISTS kv (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kv_expires ON kv(expires_at);
`

// migrationsDir is the path to the migrations directory.
const migrationsDir = "migrations"

// ApplySchema creates the base schema if tables don't exist.
func (d *DB) ApplySchema() error {
	slog.Info("Applying base schema...")
	_, err := d.Exec(baseSchema)
	if err != nil {
		return fmt.Errorf("apply base schema: %w", err)
	}
	slog.Info("Base schema applied successfully")
	return nil
}

// ApplyMigrations reads and applies SQL migration files from the migrations directory.
func (d *DB) ApplyMigrations() error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("No migrations directory found, skipping")
			return nil
		}
		return fmt.Errorf("read migrations directory: %w", err)
	}

	// Sort migration files by name
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// Create migrations tracking table
	_, err = d.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Apply each migration
	for _, file := range files {
		version := strings.TrimSuffix(file, ".sql")

		// Check if already applied
		var count int
		err := d.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if count > 0 {
			slog.Debug("Migration already applied", "version", version)
			continue
		}

		// Read and apply migration
		content, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		slog.Info("Applying migration", "version", version)
		_, err = d.Exec(string(content))
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}

		// Record migration
		_, err = d.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, strftime('%s', 'now'))", version)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		slog.Info("Migration applied", "version", version)
	}

	return nil
}

// InitDatabase applies schema and migrations.
func (d *DB) InitDatabase() error {
	if err := d.ApplySchema(); err != nil {
		return err
	}
	return d.ApplyMigrations()
}
