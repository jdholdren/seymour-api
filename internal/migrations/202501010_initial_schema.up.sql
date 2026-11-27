-- Initial schema for single-tenant Seymour application (MySQL port)
-- This creates the complete database schema from scratch
--
-- Translated from internal/migrations/202501010_initial_schema.up.sql and
-- internal/migrations/202608020_add_users.up.sql (folded into one migration
-- here since this migration set is temporary, tracking the sqlite schema
-- as it stood at the start of the MySQL port):
--   - TEXT PRIMARY KEY / TEXT UNIQUE columns become VARCHAR, since MySQL/InnoDB
--     can't build a PK or UNIQUE index over an unbounded TEXT column.

-- Feeds table: stores RSS feed information
CREATE TABLE feeds (
	id VARCHAR(191) PRIMARY KEY,
	-- VARCHAR(767) keeps the unique index within InnoDB's key-length limit;
	-- pathologically long URLs are a known gap, revisit with a hashed index
	-- once the real repo implementation lands.
	url VARCHAR(767) NOT NULL UNIQUE,
	title TEXT,
	description TEXT,
	last_synced_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Feed entries table: stores individual RSS feed items
CREATE TABLE feed_entries (
	id VARCHAR(191) PRIMARY KEY,
	feed_id VARCHAR(191) NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	guid VARCHAR(767) NOT NULL UNIQUE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	publish_time DATETIME NULL,
	link VARCHAR(256) NOT NULL
);

-- Subscriptions table: tracks which feeds a user is subscribed to
CREATE TABLE subscriptions (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	feed_id VARCHAR(191) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Users table: accounts that can sign in, decoupled from any specific IDP
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

-- Filters are part of the current baseline schema; keeping their table here
-- lets fresh databases bootstrap it without retaining a one-off migration.
CREATE TABLE filters (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	type VARCHAR(64) NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	config JSON NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_filters_user_type UNIQUE (user_id, type),
	CONSTRAINT chk_filters_type CHECK (type IN ('list', 'ai')),
	CONSTRAINT chk_filters_enabled CHECK (enabled IN (0, 1)),
	CONSTRAINT chk_filters_config CHECK (JSON_TYPE(config) = 'OBJECT')
);

-- Indexes for subscriptions
CREATE UNIQUE INDEX idx_subscriptions_user_feed ON subscriptions(user_id, feed_id);

-- Indexes for user logins
CREATE UNIQUE INDEX idx_user_logins_idp_idpid ON user_logins(idp, idp_id);
