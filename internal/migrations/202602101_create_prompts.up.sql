CREATE TABLE prompts (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_prompts_active ON prompts(active) WHERE active = 1;
