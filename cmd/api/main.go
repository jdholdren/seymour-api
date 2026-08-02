package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/run"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/sdk/client"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"net/http"

	_ "modernc.org/sqlite"

	"github.com/jdholdren/seymour/internal/api"
	"github.com/jdholdren/seymour/internal/logger"
	"github.com/jdholdren/seymour/internal/migrations"
	seyqlite "github.com/jdholdren/seymour/internal/sqlite"
	"github.com/jdholdren/seymour/internal/worker"
)

type config struct {
	Database         string `env:"DATABASE, required"`
	TemporalHostPort string `env:"TEMPORAL_HOST_PORT, required"`

	Port int    `env:"PORT, default=4444"`
	Cors string `env:"CORS"`

	FrontendURL string `env:"FRONTEND_URL, required"` // Where the browser is sent after GitHub OAuth completes, e.g. http://localhost:3000

	GithubClientID     string `env:"GITHUB_CLIENT_ID, required"`
	GithubClientSecret string `env:"GITHUB_CLIENT_SECRET, required"`
	GithubRedirectURL  string `env:"GITHUB_REDIRECT_URL, required"`

	SessionHashKey  string `env:"SESSION_HASH_KEY, required"`  // hex-encoded, 32 or 64 bytes
	SessionBlockKey string `env:"SESSION_BLOCK_KEY, required"` // hex-encoded, 16, 24, or 32 bytes
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Parse the config
	var cfg config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("error parsing config: %s", err)
	}

	l := slog.New(logger.NewContextHandler(slog.NewTextHandler(os.Stdout, nil)))
	slog.SetDefault(l)

	// Connect to the sqlite db
	dbx, err := sqlx.Open("sqlite", fmt.Sprintf("%s?_txlock=immediate&_busy_timeout=5000", cfg.Database))
	if err != nil {
		log.Fatalf("error opening database: %s", err)
	}
	defer func() { _ = dbx.Close() }()

	// Run all migrations
	if err := migrations.Run(dbx); err != nil {
		log.Fatalf("error running migrations: %s", err)
	}

	repo := seyqlite.New(dbx)

	// Retry until temporal is ready
	var temporalCli client.Client
	if err := retry.Fibonacci(ctx, 1*time.Second, func(ctx context.Context) error {
		c, err := client.Dial(client.Options{
			HostPort: cfg.TemporalHostPort,
		})
		if err != nil {
			return retry.RetryableError(err)
		}
		temporalCli = c

		return nil
	}); err != nil {
		log.Fatalln("Unable to create Temporal client:", err)
	}

	// Ensure that the default queue is there:
	// Ensure that the default queue is there:
	if err := worker.EnsureDefaultNamespace(ctx, temporalCli.WorkflowService()); err != nil {
		log.Fatalln("error ensuring default namespace:", err)
	}

	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GithubClientID,
		ClientSecret: cfg.GithubClientSecret,
		RedirectURL:  cfg.GithubRedirectURL,
		Scopes:       []string{"read:user"},
		Endpoint:     github.Endpoint,
	}

	sessionHashKey, err := hex.DecodeString(cfg.SessionHashKey)
	if err != nil {
		log.Fatalf("error decoding SESSION_HASH_KEY as hex: %s", err)
	}
	sessionBlockKey, err := hex.DecodeString(cfg.SessionBlockKey)
	if err != nil {
		log.Fatalf("error decoding SESSION_BLOCK_KEY as hex: %s", err)
	}

	frontendURL, err := url.Parse(cfg.FrontendURL)
	if err != nil {
		log.Fatalf("error parsing FRONTEND_URL: %s", err)
	}

	// Create and start the server
	server := api.NewServer(
		cfg.Port,
		cfg.Cors,
		repo,
		repo,
		repo,
		temporalCli,
		oauthCfg,
		sessionHashKey,
		sessionBlockKey,
		frontendURL,
	)

	// Set up run group
	var g run.Group

	// Add HTTP server
	g.Add(func() error {
		log.Printf("Server starting on port %d", cfg.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	}, func(error) {
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	})

	// Add signal handler
	g.Add(run.SignalHandler(ctx, os.Interrupt))

	// Run all services
	if err := g.Run(); err != nil {
		log.Printf("Service group error: %v", err)
	}
	log.Println("Server stopped")
}
