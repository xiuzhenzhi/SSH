CREATE TABLE IF NOT EXISTS agent_install_settings (token_hash varchar(64) PRIMARY KEY, report_interval_seconds integer NOT NULL, expires_at timestamp NOT NULL);
CREATE INDEX IF NOT EXISTS agent_install_expiry ON agent_install_settings(expires_at);
