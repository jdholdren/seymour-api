package api

import (
	"net/http"
	"time"

	seyerrs "github.com/jdholdren/seymour/internal/errors"
)

type PromptResp struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

func (s Server) getPrompt(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	prompt, err := s.repo.ActivePrompt(ctx)
	if err != nil {
		return err
	}

	if prompt == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	return writeJSON(w, http.StatusOK, PromptResp{
		ID:        prompt.ID,
		Content:   prompt.Content,
		Active:    prompt.Active,
		CreatedAt: prompt.CreatedAt.Time,
	})
}

type SetPromptReq struct {
	Prompt string `json:"prompt"`
}

func (req SetPromptReq) Validate() error {
	if req.Prompt == "" {
		return seyerrs.E("content is required", http.StatusBadRequest)
	}
	return nil
}

func (s Server) setPrompt(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	body, err := decodeValid[SetPromptReq](r.Body)
	if err != nil {
		return seyerrs.E(err, http.StatusBadRequest)
	}

	prompt, err := s.repo.SetPrompt(ctx, body.Prompt)
	if err != nil {
		return err
	}

	return writeJSON(w, http.StatusOK, PromptResp{
		ID:        prompt.ID,
		Content:   prompt.Content,
		Active:    prompt.Active,
		CreatedAt: prompt.CreatedAt.Time,
	})
}
