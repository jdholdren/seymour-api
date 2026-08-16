package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

// validatePostFilterReq turns a request body into the [seymour.Filter] it
// describes, or an error if the type is missing, unrecognized, or not yet
// supported.
func validatePostFilterReq(req apiv1.PostFilterReq) (seymour.Filter, error) {
	switch seymour.FilterType(req.Type) {
	case seymour.FilterTypeAllowList:
		if len(req.Keywords) == 0 {
			return nil, seymour.E("keywords is required", http.StatusBadRequest)
		}
		return seymour.AllowListConfig{Keywords: req.Keywords}, nil
	case seymour.FilterTypeDisallowList:
		if len(req.Keywords) == 0 {
			return nil, seymour.E("keywords is required", http.StatusBadRequest)
		}
		return seymour.DisallowListConfig{Keywords: req.Keywords}, nil
	case seymour.FilterTypeWebhook:
		return nil, seymour.E("webhook filters are not yet supported", http.StatusBadRequest)
	default:
		return nil, seymour.E(fmt.Sprintf("invalid filter type: %s", req.Type), http.StatusBadRequest)
	}
}

func apiFilter(f seymour.Filter) apiv1.FilterResp {
	resp := apiv1.FilterResp{Type: string(f.Type())}

	switch f := f.(type) {
	case seymour.AllowListConfig:
		resp.ID = f.ID
		resp.Keywords = f.Keywords
	case seymour.DisallowListConfig:
		resp.ID = f.ID
		resp.Keywords = f.Keywords
	case seymour.WebhookConfig:
		resp.ID = f.ID
	}

	return resp
}

func (s Server) postFilters(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
		body   apiv1.PostFilterReq
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return seymour.E(err, http.StatusBadRequest)
	}

	filter, err := validatePostFilterReq(body)
	if err != nil {
		return err
	}

	id, err := s.timeline.CreateFilter(ctx, userID, filter)
	if err != nil {
		return err
	}

	resp := apiFilter(filter)
	resp.ID = id

	return writeJSON(w, http.StatusCreated, resp)
}

func (s Server) getFilters(w http.ResponseWriter, r *http.Request) error {
	var (
		ctx    = r.Context()
		userID = mux.Vars(r)["userID"]
	)
	if userID != ctxUserID(ctx) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	filters, err := s.timeline.UserFilters(ctx, userID)
	if err != nil {
		return err
	}

	resp := apiv1.FilterListResp{Filters: []apiv1.FilterResp{}}
	for _, f := range filters {
		resp.Filters = append(resp.Filters, apiFilter(f))
	}

	return writeJSON(w, http.StatusOK, resp)
}
