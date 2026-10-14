package seymour

import (
	"errors"
	"net/http"
	"testing"
)

func TestValidateFilter(t *testing.T) {
	// Cover both supported configurations and invalid oneOf combinations without applying filters.
	tests := []struct {
		name   string
		filter Filter
		valid  bool
	}{
		{name: "AI config", filter: Filter{Type: FilterTypeAI, Enabled: true, AIConfig: &AIConfig{Prompt: " keep useful entries "}}, valid: true},
		{name: "list config", filter: Filter{Type: FilterTypeList, ListConfig: &ListConfig{Allowlist: []string{"news"}}}, valid: true},
		{name: "empty lists", filter: Filter{Type: FilterTypeList, ListConfig: &ListConfig{}}, valid: true},
		{name: "disabled config is validated", filter: Filter{Type: FilterTypeAI, AIConfig: &AIConfig{Prompt: " "}}},
		{name: "unknown type", filter: Filter{Type: "other", AIConfig: &AIConfig{Prompt: "prompt"}}},
		{name: "missing config", filter: Filter{Type: FilterTypeAI}},
		{name: "missing list config", filter: Filter{Type: FilterTypeList}},
		{name: "AI type with list config", filter: Filter{Type: FilterTypeAI, ListConfig: &ListConfig{}}},
		{name: "list type with AI config", filter: Filter{Type: FilterTypeList, AIConfig: &AIConfig{Prompt: "prompt"}}},
		{name: "extra config", filter: Filter{Type: FilterTypeAI, AIConfig: &AIConfig{Prompt: "prompt"}, ListConfig: &ListConfig{}}},
		{name: "extra AI config", filter: Filter{Type: FilterTypeList, AIConfig: &AIConfig{Prompt: "prompt"}, ListConfig: &ListConfig{}}},
		{name: "blank allowlist entry", filter: Filter{Type: FilterTypeList, ListConfig: &ListConfig{Allowlist: []string{"  "}}}},
		{name: "blank list entry", filter: Filter{Type: FilterTypeList, ListConfig: &ListConfig{Disallowlist: []string{"  "}}}},
	}

	// Each case checks that type/config validation accepts only valid configurations, regardless of enabled state.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate the supplied filter without changing its configuration.
			err := ValidateFilter(tt.filter)

			// Valid configurations return nil; invalid configurations return a bad-request domain error.
			if tt.valid && err != nil {
				t.Fatalf("ValidateFilter() error = %v, want nil", err)
			}
			if !tt.valid {
				var domainErr *Error
				if !errors.As(err, &domainErr) {
					t.Fatalf("ValidateFilter() error = %v, want *Error", err)
				}
				if domainErr.Status != http.StatusBadRequest {
					t.Errorf("ValidateFilter() status = %d, want %d", domainErr.Status, http.StatusBadRequest)
				}
			}
		})
	}
}
