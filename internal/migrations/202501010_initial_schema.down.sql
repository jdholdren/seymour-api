-- Initial schema teardown for single-tenant Seymour application
-- This completely removes all tables and indexes

-- Drop indexes first
DROP INDEX IF EXISTS idx_timeline_entries_status_requires_judgement;
DROP INDEX IF EXISTS idx_timeline_entries_status_approved;
DROP INDEX IF EXISTS idx_timeline_entries_status;
DROP INDEX IF EXISTS idx_subscriptions_feed_id;

-- Drop tables
DROP TABLE IF EXISTS timeline_entries;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS feed_entries;
DROP TABLE IF EXISTS feeds;
