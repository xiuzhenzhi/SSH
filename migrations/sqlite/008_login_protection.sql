CREATE TABLE IF NOT EXISTS login_protection (id integer PRIMARY KEY, config_ciphertext text NOT NULL);
CREATE TABLE IF NOT EXISTS login_guards (source_hash varchar(64) PRIMARY KEY, source_ip varchar(45) NOT NULL, blocked_until timestamp, updated_at timestamp NOT NULL);
CREATE TABLE IF NOT EXISTS login_failures (id varchar(36) PRIMARY KEY, source_hash varchar(64) NOT NULL, occurred_at timestamp NOT NULL);
CREATE INDEX IF NOT EXISTS login_failure_source ON login_failures(source_hash,occurred_at);
CREATE INDEX IF NOT EXISTS login_failure_time ON login_failures(occurred_at);
CREATE TABLE IF NOT EXISTS captcha_used (token_hash varchar(64) PRIMARY KEY, expires_at timestamp NOT NULL);
CREATE INDEX IF NOT EXISTS login_guard_expiry ON login_guards(blocked_until);
CREATE INDEX IF NOT EXISTS login_guard_age ON login_guards(updated_at);
CREATE INDEX IF NOT EXISTS captcha_expiry ON captcha_used(expires_at);
