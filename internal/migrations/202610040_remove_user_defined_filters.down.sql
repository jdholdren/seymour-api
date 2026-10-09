-- Rollback restores the schema only; data removed by the up migration cannot be restored.
CREATE TABLE user_filters (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	type VARCHAR(64) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_filters_user_id ON user_filters(user_id);

CREATE TABLE filter_keywords (
	id VARCHAR(191) PRIMARY KEY,
	filter_id VARCHAR(191) NOT NULL,
	keyword VARCHAR(255) NOT NULL
);

CREATE INDEX idx_filter_keywords_filter_id ON filter_keywords(filter_id);
