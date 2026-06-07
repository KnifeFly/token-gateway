package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/transport/controlhttp"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
)

// ControlAPIApp wires the M5 control-plane API process.
type ControlAPIApp struct {
	server *httpserver.Server
	db     *dbinfra.Client
	redis  *redisinfra.Client
	logger *slog.Logger
}

// NewControlAPIApp builds the control API application.
func NewControlAPIApp(ctx context.Context, cfg Config) (*ControlAPIApp, error) {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	database, err := dbinfra.New(ctx, dbConfig(cfg), logger)
	if err != nil {
		return nil, err
	}
	redisClient, err := redisinfra.New(ctx, redisConfig(cfg), logger)
	if err != nil {
		return nil, err
	}
	repo := configadmin.Repository(configadmin.NewMemoryRepository())
	reportRepo := reporting.Repository(reporting.NewMemoryRepository())
	if cfg.Database.Enabled && database.DB() != nil {
		repo = configadmin.NewMySQLRepository(database.DB())
		reportRepo = reporting.NewMySQLRepository(database.DB())
	}
	revocations := redisinfra.NewRevocationStore(redisClient.Raw(), cfg.Control.RevocationTTL.Duration)
	adminService := configadmin.NewService(
		repo,
		configadmin.NewCredentialCodec(cfg.Control.CredentialKey),
		revocations,
		configadmin.WithAPIKeyHasher(auth.NewAPIKeyHasher(cfg.Gateway.Auth.APIKeyHashSecret)),
	)
	reportingService := reporting.NewService(reportRepo)
	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	emergencyDisableStore := redisinfra.NewEmergencyDisableStore(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix)
	handler := controlhttp.NewHandlerWithEmergency(adminService, publisher, cfg.Control.AdminToken, logger, emergencyDisableStore, reportingService)
	server := httpserver.New(controlServerConfig(cfg), handler, logger)
	return &ControlAPIApp{server: server, db: database, redis: redisClient, logger: logger}, nil
}

// Run starts the control API server until ctx is canceled.
func (a *ControlAPIApp) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- a.server.Start()
	}()
	select {
	case <-ctx.Done():
		shutdownErr := a.server.Shutdown(context.Background())
		closeErr := a.Close(context.Background())
		return errors.Join(shutdownErr, closeErr)
	case err := <-errCh:
		closeErr := a.Close(context.Background())
		return errors.Join(err, closeErr)
	}
}

// Close releases process dependencies.
func (a *ControlAPIApp) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return errors.Join(a.db.Close(), a.redis.Close())
}

// Logger returns the process logger.
func (a *ControlAPIApp) Logger() *slog.Logger {
	return a.logger
}

func controlServerConfig(cfg Config) httpserver.Config {
	return httpserver.Config{
		Addr:              cfg.Control.Addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}
