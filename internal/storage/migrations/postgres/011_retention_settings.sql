CREATE TABLE IF NOT EXISTS retention_settings (id integer PRIMARY KEY, event_days integer NOT NULL, alert_days integer NOT NULL, audit_days integer NOT NULL);
