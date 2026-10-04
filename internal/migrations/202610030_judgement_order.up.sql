CREATE INDEX idx_timeline_entries_pending_order ON timeline_entries(status(191), created_at, id);
