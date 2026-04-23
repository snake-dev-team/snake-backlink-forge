// Command migrate runs goose database migrations for the Snake Backlink Forge API.
// Usage: migrate <up|down|status|redo>
// DATABASE_URL must be set in the environment (or .env file loaded by the caller).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/config"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/db"
	"github.com/kekuta/snake-backlink-forge/services/api/internal/db/migrator"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Best-effort .env load; silently ignored if file absent (CI/prod use real env vars).
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <up|down|status|redo>")
	}
	cmd := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("open db pool: %w", err)
	}
	defer pool.Close()

	switch cmd {
	case "up":
		return migrator.Up(ctx, pool)
	case "down":
		return migrator.Down(ctx, pool)
	case "status":
		return migrator.Status(ctx, pool)
	case "redo":
		if err := migrator.Down(ctx, pool); err != nil {
			return fmt.Errorf("redo down: %w", err)
		}
		return migrator.Up(ctx, pool)
	default:
		return fmt.Errorf("unknown command %q — valid: up, down, status, redo", cmd)
	}
}
