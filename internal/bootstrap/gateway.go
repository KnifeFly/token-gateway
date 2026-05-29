package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
)

// GatewayApp wires M0 dependencies for the data-plane process.
type GatewayApp struct {
	server    *httpserver.Server
	db        *dbinfra.Client
	redis     *redisinfra.Client
	telemetry *telemetry.Provider
	logger    *slog.Logger
}

// NewGatewayApp builds the gateway application.
func NewGatewayApp(ctx context.Context, cfg Config) (*GatewayApp, error) {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	tel, err := telemetry.New(ctx, telemetry.Config{
		ServiceName:     cfg.Service.Name,
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

	readiness := func(ctx context.Context) []httpserver.DependencyStatus {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return []httpserver.DependencyStatus{
			dependencyFromDB(database.Ping(ctx)),
			dependencyFromRedis(redisClient.Ping(ctx)),
		}
	}
	handler := httpserver.NewHandler(readiness, tel.Registry, logger)
	server := httpserver.New(httpServerConfig(cfg), handler, logger)
	return &GatewayApp{
		server:    server,
		db:        database,
		redis:     redisClient,
		telemetry: tel,
		logger:    logger,
	}, nil
}

// Run starts the HTTP server until ctx is canceled.
func (a *GatewayApp) Run(ctx context.Context) error {
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
func (a *GatewayApp) Close(ctx context.Context) error {
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
func (a *GatewayApp) Logger() *slog.Logger {
	return a.logger
}

func httpServerConfig(cfg Config) httpserver.Config {
	return httpserver.Config{
		Addr:              cfg.HTTP.Addr,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout.Duration,
		ReadTimeout:       cfg.HTTP.ReadTimeout.Duration,
		WriteTimeout:      cfg.HTTP.WriteTimeout.Duration,
		IdleTimeout:       cfg.HTTP.IdleTimeout.Duration,
		ShutdownTimeout:   cfg.HTTP.ShutdownTimeout.Duration,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}

func dbConfig(cfg Config) dbinfra.Config {
	return dbinfra.Config{
		Enabled:         cfg.Database.Enabled,
		Driver:          cfg.Database.Driver,
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime.Duration,
		MigrationsDir:   cfg.Database.MigrationsDir,
	}
}

func redisConfig(cfg Config) redisinfra.Config {
	return redisinfra.Config{
		Enabled:      cfg.Redis.Enabled,
		Addr:         cfg.Redis.Addr,
		Username:     cfg.Redis.Username,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		TLS:          cfg.Redis.TLS,
		DialTimeout:  cfg.Redis.DialTimeout.Duration,
		ReadTimeout:  cfg.Redis.ReadTimeout.Duration,
		WriteTimeout: cfg.Redis.WriteTimeout.Duration,
	}
}

func dependencyFromDB(status dbinfra.Status) httpserver.DependencyStatus {
	return httpserver.DependencyStatus{Name: status.Name, Status: status.Status, Error: status.Error}
}

func dependencyFromRedis(status redisinfra.Status) httpserver.DependencyStatus {
	return httpserver.DependencyStatus{Name: status.Name, Status: status.Status, Error: status.Error}
}
