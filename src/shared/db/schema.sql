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
