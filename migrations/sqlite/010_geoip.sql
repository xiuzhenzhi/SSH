CREATE TABLE IF NOT EXISTS geoip_settings (id integer PRIMARY KEY, config_ciphertext text NOT NULL);
CREATE TABLE IF NOT EXISTS geoip_database (id integer PRIMARY KEY, revision varchar(36) NOT NULL, content blob NOT NULL, updated_at timestamp NOT NULL);
CREATE TABLE IF NOT EXISTS server_network (server_id varchar(36) PRIMARY KEY REFERENCES servers(id), public_ip varchar(45) NOT NULL);
