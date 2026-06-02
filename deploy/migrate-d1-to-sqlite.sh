#!/usr/bin/env bash
set -euo pipefail

# migrate-d1-to-sqlite.sh — Export Cloudflare D1 data and import into local SQLite
#
# Prerequisites:
#   - wrangler CLI installed and authenticated
#   - sqlite3 CLI available
#   - wrangler.toml configured with D1 database_id
#
# Usage:
#   ./deploy/migrate-d1-to-sqlite.sh [output_dir]

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${1:-$PROJECT_DIR/data}"
DB_FILE="$OUTPUT_DIR/neptune.db"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[migrate]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }

# Check prerequisites
command -v wrangler >/dev/null 2>&1 || { err "wrangler not found. Install with: npm install -g wrangler"; exit 1; }
command -v sqlite3 >/dev/null 2>&1 || { err "sqlite3 not found. Install with: apt install sqlite3"; exit 1; }

mkdir -p "$OUTPUT_DIR"

# Tables to export (order matters for foreign keys)
TABLES=(
  "groups"
  "keywords"
  "admin_connections"
  "admin_current_group"
  "pending_verifications"
  "active_votes"
  "vote_records"
  "warnings"
  "reports"
  "locks"
  "ai_chat_usage"
  "kv"
)

log "Exporting D1 database..."

# Export each table
for table in "${TABLES[@]}"; do
  EXPORT_FILE="$OUTPUT_DIR/${table}.sql"
  log "Exporting table: $table"

  # wrangler d1 export outputs SQL INSERT statements
  if wrangler d1 export neptune --remote --table="$table" --output="$EXPORT_FILE" 2>/dev/null; then
    if [ -f "$EXPORT_FILE" ] && [ -s "$EXPORT_FILE" ]; then
      log "  Exported $table ($(wc -l < "$EXPORT_FILE") lines)"
    else
      warn "  Table $table is empty or export failed"
      rm -f "$EXPORT_FILE"
    fi
  else
    warn "  Failed to export $table (table may not exist)"
    rm -f "$EXPORT_FILE"
  fi
done

# Create fresh SQLite database with schema
log "Creating SQLite database: $DB_FILE"
rm -f "$DB_FILE"

# Apply base schema
sqlite3 "$DB_FILE" <<'SQL'
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
SQL

log "Schema applied to $DB_FILE"

# Import data from exported SQL files
IMPORTED=0
SKIPPED=0
for table in "${TABLES[@]}"; do
  EXPORT_FILE="$OUTPUT_DIR/${table}.sql"
  if [ -f "$EXPORT_FILE" ] && [ -s "$EXPORT_FILE" ]; then
    log "Importing $table..."
    sqlite3 "$DB_FILE" < "$EXPORT_FILE" 2>/dev/null && {
      log "  Imported $table"
      IMPORTED=$((IMPORTED + 1))
    } || {
      warn "  Failed to import $table (may have SQL format issues)"
      SKIPPED=$((SKIPPED + 1))
    }
  else
    SKIPPED=$((SKIPPED + 1))
  fi
done

# Clean up temp SQL files
log "Cleaning up temporary files..."
for table in "${TABLES[@]}"; do
  rm -f "$OUTPUT_DIR/${table}.sql"
done

# Verify
TABLE_COUNT=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
ROW_COUNTS=""
for table in "${TABLES[@]}"; do
  count=$(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM $table" 2>/dev/null || echo "0")
  if [ "$count" -gt 0 ]; then
    ROW_COUNTS="$ROW_COUNTS  $table: $count rows\n"
  fi
done

log "Migration complete!"
log "Database: $DB_FILE"
log "Tables: $TABLE_COUNT"
if [ -n "$ROW_COUNTS" ]; then
  log "Row counts:"
  echo -e "$ROW_COUNTS"
fi
log "Imported: $IMPORTED, Skipped: $SKIPPED"
