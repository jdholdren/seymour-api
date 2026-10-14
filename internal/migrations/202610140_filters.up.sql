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
