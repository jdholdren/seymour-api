CREATE TABLE filters (
	-- Identity and ownership are independent of enabled state and configuration.
	id VARCHAR(191) PRIMARY KEY,
	user_id VARCHAR(191) NOT NULL,

	-- The type selects the shape of the JSON configuration.
	type VARCHAR(64) NOT NULL,
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	config JSON NOT NULL,

	-- Updates advance updated_at explicitly while preserving created_at.
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

	-- Disabled filters still occupy their user's slot for this type.
	CONSTRAINT uq_filters_user_type UNIQUE (user_id, type),

	-- Validate storage-level shapes; application validation checks config contents.
	CONSTRAINT chk_filters_type CHECK (type IN ('list', 'ai')),
	CONSTRAINT chk_filters_enabled CHECK (enabled IN (0, 1)),
	CONSTRAINT chk_filters_config CHECK (JSON_TYPE(config) = 'OBJECT')
);
