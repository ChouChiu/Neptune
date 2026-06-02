package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection with helper methods.
type DB struct {
	*sql.DB
}

// New creates a new SQLite database connection with recommended settings.
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	// Open with WAL mode for better concurrency
	connStr := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Set connection pool settings (SQLite is single-connection anyway)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	slog.Info("Database connected", "path", dbPath)
	return &DB{db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}
