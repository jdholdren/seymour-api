package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/gorilla/mux"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

const (
	maxTimelinePromptBytes     = 16 * 1024
	maxTimelinePromptBodyBytes = 128 * 1024
)

func (s Server) getTimelinePrompt(w http.ResponseWriter, r *http.Request) error {
	userID := mux.Vars(r)["userID"]
	if userID != ctxUserID(r.Context()) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	user, err := s.users.User(r.Context(), userID)
	if err != nil {
		return err
	}
	prompt := ""
	if user.TimelinePrompt != nil {
		prompt = *user.TimelinePrompt
	}
	return writeJSON(w, http.StatusOK, apiv1.TimelinePromptResp{Prompt: prompt})
}

func (s Server) putTimelinePrompt(w http.ResponseWriter, r *http.Request) error {
	userID := mux.Vars(r)["userID"]
	if userID != ctxUserID(r.Context()) {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTimelinePromptBodyBytes)
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return seymour.E("invalid request body", http.StatusBadRequest)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return seymour.E("invalid request body", http.StatusBadRequest)
	}
	raw, ok := fields["prompt"]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return seymour.E("prompt is required and must be a string", http.StatusBadRequest)
	}
	var req apiv1.PutTimelinePromptReq
	if err := json.Unmarshal(raw, &req.Prompt); err != nil {
		return seymour.E("prompt must be a string", http.StatusBadRequest)
	}
	if !utf8.ValidString(req.Prompt) || len([]byte(req.Prompt)) > maxTimelinePromptBytes {
		return seymour.E("prompt exceeds 16 KiB UTF-8 byte limit", http.StatusBadRequest)
	}
	if err := s.users.SetTimelinePrompt(r.Context(), userID, req.Prompt); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, apiv1.TimelinePromptResp(req))
}
