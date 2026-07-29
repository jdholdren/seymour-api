package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.temporal.io/sdk/client"

	seyerrs "github.com/jdholdren/seymour/internal/errors"
	"github.com/jdholdren/seymour/internal/seymour"
)

func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return fmt.Errorf("error encoding json response: %s", err)
	}

	return nil
}

// validator is a surface that can validate itself and return an error
// if something is wrong.
type validator interface {
	Validate() error
}

// decodeValid decodes a request and then validates it.
func decodeValid[V validator](r io.Reader) (V, error) {
	var v V
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		return v, fmt.Errorf("error decoding request: %w", err)
	}
	if err := v.Validate(); err != nil {
		return v, fmt.Errorf("error validating request: %w", err)
	}

	return v, nil
}

// handlerFuncE is a modified type of [http.HandlerFunc] that returns an error.
type handlerFuncE func(w http.ResponseWriter, r *http.Request) error

func (f handlerFuncE) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := f(w, r)
	if err == nil {
		slog.Info("no error coming back")
		return
	}

	// Either it's already a structured error, or coerce it to one
	sErr := &seyerrs.Error{}
	if !errors.As(err, &sErr) {
		slog.Error("non seyerr", "err", err)
		sErr = seyerrs.E(http.StatusInternalServerError, "internal server error")
	}

	if err := writeJSON(w, sErr.Status, sErr); err != nil {
		slog.Error("error writing response", "error", err)
	}
}

// errRouter is a newtype around a mux router that allows attaching handlers that return errors.
type errRouter struct {
	*mux.Router
}

func (r errRouter) HandleFuncE(path string, f handlerFuncE) *mux.Route {
	return r.Handle(path, f)
}

// Server is an instance of the aggregation server and handles requests
// to search feeds or add new ones for ingestion.
type Server struct {
	*http.Server

	fetchClient    *http.Client
	entryRespCache *lru.Cache[string, FeedEntryResp]

	repo         seymour.Repository
	tempCli      client.Client
	hasPromptKey bool
}

func NewServer(port int, corsHeader string, repo seymour.Repository, temporalCli client.Client, hasPromptKey bool) *Server {
	var (
		r        = errRouter{Router: mux.NewRouter()}
		cache, _ = lru.New[string, FeedEntryResp](1024)
	)

	srvr := Server{
		fetchClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		entryRespCache: cache,
		repo:           repo,
		tempCli:        temporalCli,
		hasPromptKey:   hasPromptKey,
		Server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			Handler: handlers.CORS(
				handlers.AllowedOrigins([]string{corsHeader}),
				handlers.AllowCredentials(),
				handlers.AllowedMethods([]string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions}),
				handlers.AllowedHeaders([]string{"content-type"}),
			)(r),
		},
	}

	r.Use(accessLogMiddleware) // Log everything
	r.HandleFuncE("/api/viewer", srvr.handleViewer).Methods(http.MethodGet)

	// Prompt management
	r.HandleFuncE("/api/prompt", srvr.getPrompt).Methods(http.MethodGet)
	r.HandleFuncE("/api/prompt", srvr.setPrompt).Methods(http.MethodPut)

	// Subscription management
	r.HandleFuncE("/api/subscriptions", srvr.postSusbcriptions).Methods(http.MethodPost)
	r.HandleFuncE("/api/subscriptions", srvr.getSusbcriptions).Methods(http.MethodGet)

	// Timeline view
	r.HandleFuncE("/api/timeline", srvr.getTimeline).Methods(http.MethodGet)

	// Reader view
	r.HandleFuncE("/api/feed-entries/{feedEntryID}", srvr.getFeedEntry).Methods(http.MethodGet)

	return &srvr
}
