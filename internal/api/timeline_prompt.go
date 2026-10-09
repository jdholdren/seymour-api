package api

import (
	"encoding/json"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/gorilla/mux"

	apiv1 "github.com/jdholdren/seymour/apis/v1"
	"github.com/jdholdren/seymour/internal/seymour"
)

const maxTimelinePromptCharacters = 1000

func (s Server) getTimelinePrompt(w http.ResponseWriter, r *http.Request) error {
	userID := mux.Vars(r)["userID"]
	contextUserID := ctxUserID(r.Context())
	if contextUserID == "" {
		return seymour.E("unauthorized", http.StatusUnauthorized)
	}
	if userID != contextUserID {
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
	contextUserID := ctxUserID(r.Context())
	if contextUserID == "" {
		return seymour.E("unauthorized", http.StatusUnauthorized)
	}
	if userID != contextUserID {
		return seymour.E("forbidden", http.StatusForbidden)
	}

	var req apiv1.PutTimelinePromptReq
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		return seymour.E("invalid request body", http.StatusBadRequest)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return seymour.E("invalid request body", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(req.Prompt) > maxTimelinePromptCharacters {
		return seymour.E("prompt exceeds 1000 character limit", http.StatusBadRequest)
	}

	if err := s.users.UpdateUser(r.Context(), userID, seymour.UpdateUserArgs{TimelinePrompt: &req.Prompt}); err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, apiv1.TimelinePromptResp(req))
}
