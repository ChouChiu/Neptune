CREATE INDEX IF NOT EXISTS idx_pending_verifications_expires ON pending_verifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_active_votes_expires ON active_votes(expires_at);
