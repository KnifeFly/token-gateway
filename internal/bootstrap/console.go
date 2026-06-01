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
	"github.com/KnifeFly/token-gateway/internal/transport/consolehttp"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
)

// ConsoleApp wires the Human Console Plane process.
type ConsoleApp struct {
	server    *httpserver.Server
	db        *dbinfra.Client
	redis     *redisinfra.Client
	telemetry *telemetry.Provider
	logger    *slog.Logger
}

// NewConsoleApp builds the browser-facing console application.
func NewConsoleApp(ctx context.Context, cfg Config) (*ConsoleApp, error) {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	tel, err := telemetry.New(ctx, telemetry.Config{
		ServiceName:     cfg.Service.Name + "-console",
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
	routes := consolehttp.NewHandler(consolehttp.Config{
		PortalStaticDir: cfg.Console.PortalStaticDir,
		AdminStaticDir:  cfg.Console.AdminStaticDir,
	}, logger)
	handler := httpserver.NewHandlerWithRoutes(readiness, tel.Registry, logger, []httpserver.RouteRegistrar{routes})
	server := httpserver.New(consoleServerConfig(cfg), handler, logger)

	return &ConsoleApp{
		server:    server,
		db:        database,
		redis:     redisClient,
		telemetry: tel,
		logger:    logger,
	}, nil
}

// Run starts the console server until ctx is canceled.
func (a *ConsoleApp) Run(ctx context.Context) error {
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
func (a *ConsoleApp) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return errors.Join(a.db.Close(), a.redis.Close(), a.telemetry.Shutdown(ctx))
}

// Logger returns the process logger.
func (a *ConsoleApp) Logger() *slog.Logger {
	return a.logger
}

func consoleServerConfig(cfg Config) httpserver.Config {
	return httpserver.Config{
		Addr:              cfg.Console.Addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   cfg.Console.ShutdownTimeout.Duration,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}
