package api

import (
	"net/http"
	"strconv"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
)

// parsePaginationParams parses pagination parameters from an HTTP request.
// Supports offset-based pagination (?offset=20&limit=10).
func parsePaginationParams(r *http.Request, defaultLimit, maxLimit int) (int, int) {
	query := r.URL.Query()

	// Parse limit with validation
	limit, _ := strconv.Atoi(query.Get("limit"))
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}

	// Parse offset with validation
	offset, _ := strconv.Atoi(query.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}

// calculatePaginationMeta builds pagination metadata for responses.
func calculatePaginationMeta(limit, offset, total int) apiv1.PaginationMeta {
	return apiv1.PaginationMeta{
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}
}
