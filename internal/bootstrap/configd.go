package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/internal/snapshotdist"
	"github.com/KnifeFly/token-gateway/internal/transport/configdhttp"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
)

// ConfigdApp wires the config-plane snapshot publishing process.
type ConfigdApp struct {
	server    *httpserver.Server
	db        *dbinfra.Client
	redis     *redisinfra.Client
	telemetry *telemetry.Provider
	logger    *slog.Logger
}

// NewConfigdApp builds the independent configd application.
func NewConfigdApp(ctx context.Context, cfg Config) (*ConfigdApp, error) {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	tel, err := telemetry.New(ctx, telemetry.Config{
		ServiceName:     cfg.Service.Name + "-configd",
		ServiceVersion:  cfg.Service.Version,
		MetricsEnabled:  cfg.Telemetry.MetricsEnabled,
		TracingEnabled:  cfg.Telemetry.Tracing.Enabled,
		TracingExporter: cfg.Telemetry.Tracing.Exporter,
	})
	if err != nil {
		return nil, err
	}
	database, err := dbinfra.New(ctx, dbConfig(cfg), logger)
	if err != nil {
		return nil, err
	}
	redisClient, err := redisinfra.New(ctx, redisConfig(cfg), logger)
	if err != nil {
		return nil, err
	}

	repo := configadmin.Repository(configadmin.NewMemoryRepository())
	if cfg.Database.Enabled && database.DB() != nil {
		repo = configadmin.NewMySQLRepository(database.DB())
	}
	publisherOpts := []cpsnapshot.PublisherOption{}
	if redisClient.Raw() != nil {
		publisherOpts = append(publisherOpts, cpsnapshot.WithDistributor(
			snapshotdist.NewRedisDistribution(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix),
		))
	}
	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo), publisherOpts...)
	if cfg.Configd.PublishOnStart {
		if _, err := publisher.Publish(ctx); err != nil {
			logger.Warn("initial snapshot publish failed", "error", err)
		}
	}

	readiness := func(ctx context.Context) []httpserver.DependencyStatus {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return []httpserver.DependencyStatus{
			dependencyFromDB(database.Ping(ctx)),
			dependencyFromRedis(redisClient.Ping(ctx)),
		}
	}
	handler := httpserver.NewHandlerWithRoutes(
		readiness,
		tel.Registry,
		logger,
		[]httpserver.RouteRegistrar{configdhttp.NewHandler(publisher, cfg.Control.AdminToken, logger)},
	)
	server := httpserver.New(configdServerConfig(cfg), handler, logger)
	return &ConfigdApp{server: server, db: database, redis: redisClient, telemetry: tel, logger: logger}, nil
}

// Run starts configd until ctx is canceled.
func (a *ConfigdApp) Run(ctx context.Context) error {
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

// Close releases configd dependencies.
func (a *ConfigdApp) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return errors.Join(
		a.db.Close(),
		a.redis.Close(),
		a.telemetry.Shutdown(ctx),
	)
}

// Logger returns the process logger.
func (a *ConfigdApp) Logger() *slog.Logger {
	return a.logger
}

func configdServerConfig(cfg Config) httpserver.Config {
	return httpserver.Config{
		Addr:              cfg.Configd.Addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   cfg.Configd.ShutdownTimeout.Duration,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}
