-- Initial schema for single-tenant Seymour application
-- This creates the complete database schema from scratch

-- Feeds table: stores RSS feed information
CREATE TABLE feeds (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL UNIQUE,
	title TEXT,
	description TEXT,
	last_synced_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Feed entries table: stores individual RSS feed items
CREATE TABLE feed_entries (
	id TEXT PRIMARY KEY,
	feed_id TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	guid TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	publish_time DATETIME NULL,
	link VARCHAR(256) NOT NULL
);

-- Subscriptions table: tracks which feeds a user is subscribed to
CREATE TABLE subscriptions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	feed_id TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Timeline entries table: curated feed entries for a user's timeline
CREATE TABLE timeline_entries (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	feed_entry_id TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	status TEXT NOT NULL,
	feed_id VARCHAR(64) NOT NULL
);

-- Indexes for subscriptions
CREATE UNIQUE INDEX idx_subscriptions_user_feed ON subscriptions(user_id, feed_id);

-- Indexes for timeline entries
CREATE INDEX idx_timeline_entries_status ON timeline_entries(status);
CREATE INDEX idx_timeline_entries_user_status ON timeline_entries(user_id, status);
CREATE INDEX idx_timeline_entries_status_approved ON timeline_entries(status) WHERE status = 'approved';
CREATE INDEX idx_timeline_entries_status_requires_judgement ON timeline_entries(status) WHERE status = 'requires_judgement';
