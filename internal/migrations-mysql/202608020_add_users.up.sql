-- Users table: accounts that can sign in, decoupled from any specific IDP
-- (MySQL port of internal/migrations/202608020_add_users.up.sql)
CREATE TABLE users (
	id VARCHAR(191) PRIMARY KEY,
	preferred_name TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User logins table: ties a user to an identity provider (e.g. github)
CREATE TABLE user_logins (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	idp VARCHAR(64) NOT NULL,
	idp_id VARCHAR(191) NOT NULL,
	last_login DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_user_logins_idp_idpid ON user_logins(idp, idp_id);
