CREATE INDEX IF NOT EXISTS sessions_expiry ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS enrollments_expiry ON agent_enrollments(expires_at);
CREATE INDEX IF NOT EXISTS attempts_window ON login_attempts(window_start);
CREATE INDEX IF NOT EXISTS servers_seen ON servers(revoked,last_seen_at);
CREATE INDEX IF NOT EXISTS servers_created ON servers(created_at);
CREATE INDEX IF NOT EXISTS events_order ON ssh_events(occurred_at DESC,id DESC);
