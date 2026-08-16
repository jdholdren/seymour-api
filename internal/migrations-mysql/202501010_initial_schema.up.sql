-- Initial schema for single-tenant Seymour application (MySQL port)
-- This creates the complete database schema from scratch
--
-- Translated from internal/migrations/202501010_initial_schema.up.sql:
--   - TEXT PRIMARY KEY / TEXT UNIQUE columns become VARCHAR, since MySQL/InnoDB
--     can't build a PK or UNIQUE index over an unbounded TEXT column.
--   - The two SQLite-only partial indexes on timeline_entries.status are
--     dropped; idx_timeline_entries_status and idx_timeline_entries_user_status
--     already cover the same query patterns.

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

-- Timeline entries table: curated feed entries for a user's timeline
CREATE TABLE timeline_entries (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	feed_entry_id VARCHAR(191) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	status TEXT NOT NULL,
	feed_id VARCHAR(64) NOT NULL
);

-- Indexes for subscriptions
CREATE UNIQUE INDEX idx_subscriptions_user_feed ON subscriptions(user_id, feed_id);

-- Indexes for timeline entries
CREATE INDEX idx_timeline_entries_status ON timeline_entries(status(191));
CREATE INDEX idx_timeline_entries_user_status ON timeline_entries(user_id, status(191));
