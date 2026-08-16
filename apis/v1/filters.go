package v1

// PostFilterReq is the body of a POST /api/users/{userID}/filters request.
type PostFilterReq struct {
	Type     string   `json:"type"`
	Keywords []string `json:"keywords,omitempty"`
}

// FilterResp describes a single user-defined filter.
type FilterResp struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Keywords []string `json:"keywords,omitempty"`
}

// FilterListResp is the response of GET /api/users/{userID}/filters.
type FilterListResp struct {
	Filters []FilterResp `json:"filters"`
}
