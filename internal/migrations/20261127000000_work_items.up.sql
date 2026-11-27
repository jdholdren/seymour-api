-- Work items retain completed history while allowing the same type/key to recur.
CREATE TABLE work_items (
	id VARCHAR(191) PRIMARY KEY,
	type VARCHAR(64) NOT NULL,
	`key` VARCHAR(191) NOT NULL,
	input JSON NULL,
	result JSON NULL,
	leased_at DATETIME NULL,
	leased_until DATETIME NULL,
	fencing_generation BIGINT NOT NULL DEFAULT 1,
	completed_at DATETIME NULL,
	active_key TINYINT GENERATED ALWAYS AS (
		CASE WHEN completed_at IS NULL THEN 1 ELSE NULL END
	) STORED,
	UNIQUE INDEX idx_work_items_active_key (type, `key`, active_key),
	INDEX idx_work_items_available (type, completed_at, leased_until)
);
