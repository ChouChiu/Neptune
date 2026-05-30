-- Migration: Add votekick support
-- Run: wrangler d1 execute neptune --remote --file=migrations/0002_votekick.sql

ALTER TABLE groups ADD COLUMN votekick_enabled INTEGER DEFAULT 0;

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
