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
    content TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    reviewed_by INTEGER,
    reviewed_at INTEGER,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (group_id) REFERENCES groups(group_id)
);

CREATE INDEX IF NOT EXISTS idx_warnings_group_user ON warnings(group_id, user_id);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
