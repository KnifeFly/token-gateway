package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/dataplane/admission"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/classifier"
	"github.com/KnifeFly/token-gateway/internal/dataplane/dispatch"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/limit"
	"github.com/KnifeFly/token-gateway/internal/dataplane/observe"
	"github.com/KnifeFly/token-gateway/internal/dataplane/parser"
	"github.com/KnifeFly/token-gateway/internal/dataplane/router"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/internal/provider"
	"github.com/KnifeFly/token-gateway/internal/provider/openai"
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
	gatewayEngine, err := newGatewayEngine(ctx, cfg, tel, logger, database, redisClient)
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
	handler := httpserver.NewHandler(readiness, tel.Registry, logger, gatewayEngine)
	server := httpserver.New(httpServerConfig(cfg), handler, logger)
	return &GatewayApp{
		server:    server,
		db:        database,
		redis:     redisClient,
		telemetry: tel,
		logger:    logger,
	}, nil
}

func newGatewayEngine(ctx context.Context, cfg Config, tel *telemetry.Provider, logger *slog.Logger, database *dbinfra.Client, redisClient *redisinfra.Client) (*engine.GatewayEngine, error) {
	indexed, err := buildSeedSnapshot(cfg)
	if err != nil {
		return nil, err
	}
	observeRecorder, err := observe.NewRecorder(tel.Registry, logger)
	if err != nil {
		return nil, err
	}
	registry := provider.NewRegistry()
	if err := registry.Register("openai_compatible", openai.NewAdapter(nil)); err != nil {
		return nil, err
	}
	admissionController := engine.AdmissionController(engine.NoopAdmission{})
	limitEnforcer := engine.LimitEnforcer(engine.NoopLimitEnforcer{})
	settlementService := engine.SettlementService(engine.NoopSettlement{})
	var attemptRecorder dispatch.AttemptRecorder
	if cfg.Gateway.Billing.Enabled {
		repo := billing.NewMySQLRepository(database.DB())
		if err := ensureLocalSeedBalance(ctx, cfg, repo); err != nil {
			return nil, err
		}
		billingMetrics, err := billing.NewMetrics(tel.Registry)
		if err != nil {
			return nil, err
		}
		price := pricing.TokenPrice{
			Currency:             cfg.Gateway.Billing.Currency,
			InputMicrosPerToken:  cfg.Gateway.Billing.InputMicrosPerToken,
			OutputMicrosPerToken: cfg.Gateway.Billing.OutputMicrosPerToken,
		}
		admissionController = admission.NewController(
			billing.NewBalanceService(repo),
			admission.NewPriceEstimator(price, cfg.Gateway.Billing.EstimatedOutputTokens),
			cfg.Gateway.Billing.HoldTTL.Duration,
		)
		attemptRecorder = billing.NewAttemptWriter(repo)
		settlementService = billing.NewSettlementService(repo, billing.NewSettlementPlanner(price), billingMetrics)
	}
	if cfg.Gateway.Limits.Enabled {
		limitEnforcer = limit.NewRedisEnforcer(redisClient.Raw(), limit.Config{
			Enabled:     cfg.Gateway.Limits.Enabled,
			QPS:         cfg.Gateway.Limits.QPS,
			TPM:         cfg.Gateway.Limits.TPM,
			Concurrency: cfg.Gateway.Limits.Concurrency,
			Window:      cfg.Gateway.Limits.Window.Duration,
			LeaseTTL:    cfg.Gateway.Limits.LeaseTTL.Duration,
			KeyPrefix:   cfg.Gateway.Limits.KeyPrefix,
		})
	}
	return engine.New(
		engine.WithSnapshot(dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed))),
		engine.WithClassifier(classifier.NewDefault()),
		engine.WithParser(parser.NewOpenAIChatParser(cfg.Gateway.Body.MaxBytes)),
		engine.WithAuthenticator(auth.NewSnapshotAuthenticator()),
		engine.WithRoutePlanner(router.NewRoutePlanner(nil)),
		engine.WithAdmission(admissionController),
		engine.WithLimitEnforcer(limitEnforcer),
		engine.WithDispatcher(dispatch.New(registry, observeRecorder, attemptRecorder, logger)),
		engine.WithSettlement(settlementService),
		engine.WithObserveRecorder(observeRecorder),
	)
}

func ensureLocalSeedBalance(ctx context.Context, cfg Config, repo billing.Repository) error {
	if !cfg.Gateway.Seed.Enabled || cfg.Gateway.Billing.LocalSeedBalanceMicros <= 0 {
		return nil
	}
	return repo.EnsureBalanceAccount(ctx, billing.BalanceAccount{
		ID:              "acct_local_seed",
		TenantID:        cfg.Gateway.Seed.TenantID,
		ProjectID:       cfg.Gateway.Seed.ProjectID,
		Currency:        cfg.Gateway.Billing.Currency,
		OpeningMicros:   cfg.Gateway.Billing.LocalSeedBalanceMicros,
		AvailableMicros: cfg.Gateway.Billing.LocalSeedBalanceMicros,
	})
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
