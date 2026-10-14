package seymour

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// FilterType identifies the configuration used by a filter.
type FilterType string

const (
	FilterTypeAI   FilterType = "ai"
	FilterTypeList FilterType = "list"
)

// AIConfig configures an AI-backed filter.
type AIConfig struct {
	Prompt string `json:"prompt"`
}

// ListConfig configures an allowlist/denylist filter.
type ListConfig struct {
	Allowlist    []string `json:"allowlist"`
	Disallowlist []string `json:"disallowlist"`
}

// Filter represents a user's configured timeline filter.
type Filter struct {
	ID         string      `db:"id"`
	UserID     string      `db:"user_id"`
	Enabled    bool        `db:"enabled"`
	Type       FilterType  `db:"type"`
	AIConfig   *AIConfig   `db:"-"`
	ListConfig *ListConfig `db:"-"`
	CreatedAt  time.Time   `db:"created_at"`
	UpdatedAt  time.Time   `db:"updated_at"`
}

// ValidateFilter checks that the filter has exactly the configuration required by its type.
func ValidateFilter(filter Filter) error {
	valid := false
	switch filter.Type {
	case FilterTypeAI:
		valid = filter.AIConfig != nil && filter.ListConfig == nil
		if valid && strings.TrimSpace(filter.AIConfig.Prompt) == "" {
			return E("AI prompt must not be blank", http.StatusBadRequest)
		}
	case FilterTypeList:
		valid = filter.ListConfig != nil && filter.AIConfig == nil
		if valid {
			for _, list := range [][]string{filter.ListConfig.Allowlist, filter.ListConfig.Disallowlist} {
				for _, value := range list {
					if strings.TrimSpace(value) == "" {
						return E("list entries must not be blank", http.StatusBadRequest)
					}
				}
			}
		}
	}
	if !valid {
		return E("filter type and configuration do not match", http.StatusBadRequest)
	}
	return nil
}

// CreateFilterArgs contains the values required to create a filter.
type CreateFilterArgs struct {
	Type       FilterType
	Enabled    bool
	AIConfig   *AIConfig
	ListConfig *ListConfig
}

// UpdateFilterArgs replaces a filter's enabled state and configuration.
type UpdateFilterArgs struct {
	Enabled    bool
	AIConfig   *AIConfig
	ListConfig *ListConfig
}

// FilterService provides user-scoped filter operations.
type FilterService interface {
	Filters(ctx context.Context, userID string) ([]Filter, error)
	Filter(ctx context.Context, userID, id string) (Filter, error)
	CreateFilter(ctx context.Context, userID string, args CreateFilterArgs) (Filter, error)
	UpdateFilter(ctx context.Context, userID, id string, args UpdateFilterArgs) (Filter, error)
	DeleteFilter(ctx context.Context, userID, id string) error
}
