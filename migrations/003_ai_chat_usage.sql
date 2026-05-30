CREATE TABLE IF NOT EXISTS ai_chat_usage (
  user_id INTEGER NOT NULL,
  group_id INTEGER NOT NULL,
  date TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, group_id, date)
);
