-- User-defined filters: rules a user configures to help curate their
-- timeline during judgement. See docs/20260816_user_defined_filters.md.

-- Root table: one row per filter, polymorphic on `type`.
CREATE TABLE user_filters (
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,
	type VARCHAR(64) NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_filters_user_id ON user_filters(user_id);

-- Keywords backing allow_list/disallow_list filters, one row per keyword.
CREATE TABLE filter_keywords (
	id VARCHAR(191) PRIMARY KEY,
	filter_id VARCHAR(191) NOT NULL,
	keyword VARCHAR(255) NOT NULL
);

CREATE INDEX idx_filter_keywords_filter_id ON filter_keywords(filter_id);

-- Config backing webhook filters. Not yet reachable via the API; the
-- planned feature will add more tables here (e.g. attempt logging).
CREATE TABLE filter_webhooks (
	filter_id VARCHAR(191) PRIMARY KEY,
	host VARCHAR(255) NOT NULL
);
