package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/run"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/sdk/client"
	_ "golang.org/x/crypto/x509roots/fallback"
	_ "modernc.org/sqlite"

	"github.com/jdholdren/seymour/internal/logger"
	seyqlite "github.com/jdholdren/seymour/internal/sqlite"
	seyworker "github.com/jdholdren/seymour/internal/worker"
)

type config struct {
	Database         string `env:"DATABASE, required"`
	TemporalHostPort string `env:"TEMPORAL_HOST_PORT, required"`

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
	dbx, err := sqlx.Open("sqlite", cfg.Database)
	if err != nil {
		log.Fatalf("error opening database: %s", err)
	}
	defer func() { _ = dbx.Close() }()

	repo := seyqlite.New(dbx)

	// Retry until temporal is ready
	var temporalCli client.Client
	if err := retry.Fibonacci(ctx, 1*time.Second, func(ctx context.Context) error {
		c, err := client.Dial(client.Options{
			HostPort: cfg.TemporalHostPort,
			Logger:   slog.Default(),
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
	if err := seyworker.EnsureDefaultNamespace(ctx, temporalCli.WorkflowService()); err != nil {
		log.Fatalln("error ensuring default namespace:", err)
	}

	// Create claude client
	claudeClient := anthropic.NewClient(
		option.WithAPIKey(cfg.ClaudeAPIKey),
	)

	// Create the worker
	w, err := seyworker.NewWorker(ctx, repo, temporalCli, &claudeClient)
	if err != nil {
		log.Fatalf("Failed to create worker: %v", err)
	}

	// Set up run group
	var g run.Group

	// Add temporal worker
	g.Add(func() error {
		log.Println("Worker starting...")
		if err := w.Start(); err != nil {
			return err
		}
		// Block forever until shutdown
		select {}
	}, func(error) {
		log.Println("Shutting down worker...")
		w.Stop()
	})

	// Add signal handler
	g.Add(run.SignalHandler(ctx, os.Interrupt))

	// Run all services
	if err := g.Run(); err != nil {
		log.Printf("Service group error: %v", err)
	}
	log.Println("Worker stopped")
}
