-- Users table: accounts that can sign in, decoupled from any specific IDP
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	preferred_name TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- User logins table: ties a user to an identity provider (e.g. github)
CREATE TABLE user_logins (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	idp TEXT NOT NULL,
	idp_id TEXT NOT NULL,
	last_login DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_user_logins_idp_idpid ON user_logins(idp, idp_id);
