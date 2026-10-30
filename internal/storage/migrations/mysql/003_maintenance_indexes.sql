CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE INDEX enrollments_expiry ON agent_enrollments(expires_at);
CREATE INDEX attempts_window ON login_attempts(window_start);
CREATE INDEX servers_seen ON servers(revoked,last_seen_at);
CREATE INDEX servers_created ON servers(created_at);
