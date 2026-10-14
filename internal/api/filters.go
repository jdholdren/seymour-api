package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

// filterUser verifies the path owner before any user-scoped service operation.
func (s Server) filterUser(r *http.Request) (string, error) {
	userID := ctxUserID(r.Context())
	if mux.Vars(r)["userID"] != userID {
		return "", seymour.E("forbidden", http.StatusForbidden)
	}
	return userID, nil
}

// decodeFilterBody enforces a single, non-null JSON object with no unknown fields.
func decodeFilterBody(r *http.Request, dst any) error {
	// Buffer the request so syntax and the second-value check share one strict decoder.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return seymour.E("invalid JSON body", http.StatusBadRequest)
	}

	// Filter requests must be objects; in particular, JSON null must not decode as an empty struct.
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return seymour.E("request body must be a JSON object", http.StatusBadRequest)
	}

	// Reject unknown fields and any content after the one request value.
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return seymour.E("invalid JSON body", http.StatusBadRequest)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return seymour.E("request body must contain one JSON value", http.StatusBadRequest)
	}
	return nil
}

// apiFilter converts domain configuration into its one-variant wire representation.
func apiFilter(filter seymour.Filter) apiv1.FilterResp {
	// Copy identity, enabled state, and timestamps without exposing domain types.
	resp := apiv1.FilterResp{
		ID: filter.ID, UserID: filter.UserID, Type: string(filter.Type), Enabled: filter.Enabled,
		CreatedAt: filter.CreatedAt, UpdatedAt: filter.UpdatedAt,
	}

	// Include only the configured variant; list arrays are always JSON arrays.
	if filter.AIConfig != nil {
		resp.AIConfig = &apiv1.FilterAIConfig{Prompt: filter.AIConfig.Prompt}
	}
	if filter.ListConfig != nil {
		allowlist := filter.ListConfig.Allowlist
		if allowlist == nil {
			allowlist = []string{}
		}
		disallowlist := filter.ListConfig.Disallowlist
		if disallowlist == nil {
			disallowlist = []string{}
		}
		resp.ListConfig = &apiv1.FilterListConfig{Allowlist: allowlist, Disallowlist: disallowlist}
	}
	return resp
}

// filterConfigs translates the tagged API variant into the domain's oneOf config.
func filterConfigs(ai *apiv1.FilterAIConfig, list *apiv1.FilterListConfig) (*seymour.AIConfig, *seymour.ListConfig) {
	// Preserve both variants when provided so ValidateFilter can reject a oneOf violation.
	var aiConfig *seymour.AIConfig
	if ai != nil {
		aiConfig = &seymour.AIConfig{Prompt: ai.Prompt}
	}
	var listConfig *seymour.ListConfig
	if list != nil {
		listConfig = &seymour.ListConfig{Allowlist: list.Allowlist, Disallowlist: list.Disallowlist}
	}
	return aiConfig, listConfig
}

func (s Server) getFilters(w http.ResponseWriter, r *http.Request) error {
	// Keep collection access scoped to the authenticated owner in both the path and service call.
	userID, err := s.filterUser(r)
	if err != nil {
		return err
	}

	// Convert every owned domain filter into the stable list response shape.
	filters, err := s.filters.Filters(r.Context(), userID)
	if err != nil {
		return err
	}
	responses := make([]apiv1.FilterResp, 0, len(filters))
	for _, filter := range filters {
		responses = append(responses, apiFilter(filter))
	}
	return writeJSON(w, http.StatusOK, apiv1.FilterListResp{Filters: responses})
}

func (s Server) postFilter(w http.ResponseWriter, r *http.Request) error {
	// Reject cross-user paths before parsing or persisting user configuration.
	userID, err := s.filterUser(r)
	if err != nil {
		return err
	}

	// Decode the complete creation request and require explicit enabled state.
	var req apiv1.PostFilterReq
	if err := decodeFilterBody(r, &req); err != nil {
		return err
	}
	if req.Enabled == nil {
		return seymour.E("enabled is required", http.StatusBadRequest)
	}

	// Validate type/config pairing before the service is allowed to persist it.
	aiConfig, listConfig := filterConfigs(req.AIConfig, req.ListConfig)
	input := seymour.Filter{Type: seymour.FilterType(req.Type), Enabled: *req.Enabled, AIConfig: aiConfig, ListConfig: listConfig}
	if err := seymour.ValidateFilter(input); err != nil {
		return err
	}

	// Persist within the authenticated owner's scope and return the created resource.
	filter, err := s.filters.CreateFilter(r.Context(), userID, seymour.CreateFilterArgs{
		Type: input.Type, Enabled: input.Enabled, AIConfig: aiConfig, ListConfig: listConfig,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, apiFilter(filter))
}

func (s Server) getFilter(w http.ResponseWriter, r *http.Request) error {
	// Ensure the URL names the authenticated owner before looking up a filter.
	userID, err := s.filterUser(r)
	if err != nil {
		return err
	}

	// Fetch only through the user-scoped service so foreign IDs remain indistinguishable from missing IDs.
	filter, err := s.filters.Filter(r.Context(), userID, mux.Vars(r)["filterID"])
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, apiFilter(filter))
}

func (s Server) putFilter(w http.ResponseWriter, r *http.Request) error {
	// Restrict the operation to the authenticated owner before processing replacement data.
	userID, err := s.filterUser(r)
	if err != nil {
		return err
	}

	// Full replacement requires enabled explicitly; type remains fixed by the existing resource.
	var req apiv1.PutFilterReq
	if err := decodeFilterBody(r, &req); err != nil {
		return err
	}
	if req.Enabled == nil {
		return seymour.E("enabled is required", http.StatusBadRequest)
	}

	// Load the current type so updates cannot change the immutable filter variant.
	filterID := mux.Vars(r)["filterID"]
	current, err := s.filters.Filter(r.Context(), userID, filterID)
	if err != nil {
		return err
	}

	// Treat provided config as a full replacement and validate before persistence.
	aiConfig, listConfig := filterConfigs(req.AIConfig, req.ListConfig)
	if err := seymour.ValidateFilter(seymour.Filter{Type: current.Type, Enabled: *req.Enabled, AIConfig: aiConfig, ListConfig: listConfig}); err != nil {
		return err
	}

	// Apply the replacement through the owner-scoped service and return the updated resource.
	filter, err := s.filters.UpdateFilter(r.Context(), userID, filterID, seymour.UpdateFilterArgs{
		Enabled: *req.Enabled, AIConfig: aiConfig, ListConfig: listConfig,
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, apiFilter(filter))
}

func (s Server) deleteFilter(w http.ResponseWriter, r *http.Request) error {
	// The owner-scoped delete returns not-found for foreign and missing filter IDs alike.
	userID, err := s.filterUser(r)
	if err != nil {
		return err
	}

	// Delete only through the user-scoped service; success intentionally has no response body.
	if err := s.filters.DeleteFilter(r.Context(), userID, mux.Vars(r)["filterID"]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
