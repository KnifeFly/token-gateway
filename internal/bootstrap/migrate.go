package bootstrap

import (
	"context"
	"fmt"
	"os"

	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
)

// RunMigration runs an up or down migration command.
func RunMigration(ctx context.Context, cfg Config, direction string) error {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	database, err := dbinfra.New(ctx, dbConfig(cfg), logger)
	if err != nil {
		return err
	}
	defer database.Close()

	switch direction {
	case "up":
		return database.MigrateUp(ctx)
	case "down":
		return database.MigrateDown(ctx)
	default:
		return fmt.Errorf("unsupported migration direction %q", direction)
	}
}
