package v1

// FilterType enumerates the different filter "features" a user can
// configure to help curate their timeline during judgement.
type FilterType string

// Filter is the wire representation of a user-defined filter. Only one of
// AllowListConfig/DisallowListConfig is populated, matching Type.
type Filter struct {
	ID                 string              `json:"id"`
	Type               FilterType          `json:"type"`
	AllowListConfig    *AllowListConfig    `json:"allow_list_config,omitempty"`
	DisallowListConfig *DisallowListConfig `json:"disallow_list_config,omitempty"`
}

// AllowListConfig requires an entry's title or description to match one of
// Keywords in order to be approved.
type AllowListConfig struct {
	Keywords []string `json:"keywords"`
}

// DisallowListConfig rejects an entry outright if its title or description
// matches any of Keywords.
type DisallowListConfig struct {
	Keywords []string `json:"keywords"`
}

// UserFiltersResp is the response for GET /users/{userID}/filters.
type UserFiltersResp struct {
	Filters []Filter `json:"filters"`
}
