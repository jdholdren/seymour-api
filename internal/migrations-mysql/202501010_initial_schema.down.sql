-- Initial schema teardown for single-tenant Seymour application (MySQL port)
-- This completely removes all tables and indexes

-- Drop indexes first
DROP INDEX idx_timeline_entries_user_status ON timeline_entries;
DROP INDEX idx_timeline_entries_status ON timeline_entries;
DROP INDEX idx_subscriptions_user_feed ON subscriptions;

-- Drop tables
DROP TABLE IF EXISTS timeline_entries;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS feed_entries;
DROP TABLE IF EXISTS feeds;
