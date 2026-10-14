package v1

import "time"

// PostFilterReq is the body of a POST /api/users/{userID}/filters request.
type PostFilterReq struct {
	Type       string            `json:"type" enum:"ai,list"`
	Enabled    *bool             `json:"enabled" required:"true"`
	AIConfig   *FilterAIConfig   `json:"ai_config,omitempty"`
	ListConfig *FilterListConfig `json:"list_config,omitempty"`
}

// PutFilterReq is the full replacement body of a PUT /api/users/{userID}/filters/{filterID} request.
type PutFilterReq struct {
	Enabled    *bool             `json:"enabled" required:"true"`
	AIConfig   *FilterAIConfig   `json:"ai_config,omitempty"`
	ListConfig *FilterListConfig `json:"list_config,omitempty"`
}

// FilterAIConfig configures an AI filter.
type FilterAIConfig struct {
	Prompt string `json:"prompt"`
}

// FilterListConfig configures an allowlist/disallowlist filter.
type FilterListConfig struct {
	Allowlist    []string `json:"allowlist"`
	Disallowlist []string `json:"disallowlist"`
}

// FilterResp describes a user's filter.
type FilterResp struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`
	Type       string            `json:"type"`
	Enabled    bool              `json:"enabled"`
	AIConfig   *FilterAIConfig   `json:"ai_config,omitempty"`
	ListConfig *FilterListConfig `json:"list_config,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// FilterListResp is the response of GET /api/users/{userID}/filters.
type FilterListResp struct {
	Filters []FilterResp `json:"filters"`
}
