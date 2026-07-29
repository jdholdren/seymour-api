package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/run"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/sdk/client"

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

	ClaudeAPIKey    string `env:"CLAUDE_API_KEY"`
	ClaudeAPKeyFile string `env:"CLAUDE_API_KEY_FILE"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Parse the config
	var cfg config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("error parsing config: %s", err)
	}
	if cfg.ClaudeAPKeyFile != "" {
		// If the key file is specified use that to try and populate the key
		key, err := os.ReadFile(cfg.ClaudeAPKeyFile)
		if err != nil {
			log.Fatalf("error loading claude api key file: %s", err)
		}

		cfg.ClaudeAPIKey = string(key)
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

	// Create and start the server
	server := api.NewServer(cfg.Port, cfg.Cors, repo, temporalCli, cfg.ClaudeAPIKey != "")

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
