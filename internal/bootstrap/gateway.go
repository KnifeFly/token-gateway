package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/admission"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/classifier"
	"github.com/KnifeFly/token-gateway/internal/dataplane/dispatch"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/dataplane/limit"
	"github.com/KnifeFly/token-gateway/internal/dataplane/observe"
	"github.com/KnifeFly/token-gateway/internal/dataplane/parser"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin/builtin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/policy"
	"github.com/KnifeFly/token-gateway/internal/dataplane/realtime"
	"github.com/KnifeFly/token-gateway/internal/dataplane/router"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/stream"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/internal/portal"
	"github.com/KnifeFly/token-gateway/internal/provider"
	"github.com/KnifeFly/token-gateway/internal/provider/claude"
	"github.com/KnifeFly/token-gateway/internal/provider/gemini"
	"github.com/KnifeFly/token-gateway/internal/provider/openai"
	"github.com/KnifeFly/token-gateway/internal/provider/replicate"
	"github.com/KnifeFly/token-gateway/internal/snapshotdist"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
	"github.com/KnifeFly/token-gateway/internal/transport/portalhttp"
	"github.com/KnifeFly/token-gateway/internal/transport/publichttp"
	"github.com/KnifeFly/token-gateway/internal/transport/realtimehttp"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
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
	gatewayRuntime, err := newGatewayRuntime(ctx, cfg, tel, logger, database, redisClient)
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
	realtimeHandler, err := realtimehttp.NewHandler(
		gatewayRuntime.snapshotProvider,
		gatewayRuntime.authenticator,
		realtime.DisabledEngine{},
		gatewayRuntime.observeRecorder,
		tel.Registry,
		logger,
	)
	if err != nil {
		return nil, err
	}
	reportRepo := reporting.Repository(reporting.NewMemoryRepository())
	if cfg.Database.Enabled && database.DB() != nil {
		reportRepo = reporting.NewMySQLRepository(database.DB())
	}
	publicHandler := publichttp.NewHandler(gatewayRuntime.snapshotProvider, gatewayRuntime.authenticator, reporting.NewService(reportRepo), logger)
	portalHandler := portalhttp.NewHandler(
		gatewayRuntime.snapshotProvider,
		gatewayRuntime.authenticator,
		portal.NewService(gatewayRuntime.adminService, reporting.NewService(reportRepo), gatewayRuntime.taskRepo, gatewayRuntime.portalOptions()...),
		logger,
	)
	handler := httpserver.NewHandlerWithRoutesConfig(
		readiness,
		tel.Registry,
		logger,
		httpserver.HandlerConfig{TrustedProxyCIDRs: cfg.HTTP.TrustedProxyCIDRs},
		[]httpserver.RouteRegistrar{realtimeHandler, publicHandler, portalHandler},
		gatewayRuntime.engine,
	)
	server := httpserver.New(httpServerConfig(cfg), handler, logger)
	return &GatewayApp{
		server:    server,
		db:        database,
		redis:     redisClient,
		telemetry: tel,
		logger:    logger,
	}, nil
}

type gatewayRuntime struct {
	engine            *engine.GatewayEngine
	snapshotProvider  engine.SnapshotProvider
	authenticator     engine.Authenticator
	observeRecorder   engine.ObserveRecorder
	adminService      *admin.Service
	taskRepo          tasksvc.Repository
	failedSettlements *billing.FailedSettlementService
	portalSnapshots   portal.SnapshotRefresher
	snapshotManager   *runtimeSnapshotRefresher
}

func newGatewayRuntime(ctx context.Context, cfg Config, tel *telemetry.Provider, logger *slog.Logger, database *dbinfra.Client, redisClient *redisinfra.Client) (*gatewayRuntime, error) {
	indexed, err := buildSeedSnapshot(cfg)
	if err != nil {
		return nil, err
	}
	snapshotStore := dpsnapshot.NewStore(indexed)
	observeRecorder, err := observe.NewRecorder(tel.Registry, logger)
	if err != nil {
		return nil, err
	}
	snapshotMetrics, err := dpsnapshot.NewMetrics(tel.Registry)
	if err != nil {
		return nil, err
	}
	if snapshotMetrics != nil {
		snapshotMetrics.Observe(indexed.Ref())
	}
	adminRepo := admin.Repository(admin.NewMemoryRepository())
	usingDBAdmin := cfg.Database.Enabled && database != nil && database.DB() != nil
	var snapshotDistributor cpsnapshot.RuntimeSnapshotDistributor
	if usingDBAdmin {
		adminRepo = admin.NewMySQLRepository(database.DB())
		activeProvider := dpsnapshot.ActiveRuntimeProvider(cpsnapshot.NewActiveProvider(adminRepo))
		var watcherOpts []dpsnapshot.WatcherOption
		if redisClient.Raw() != nil {
			redisSnapshots := snapshotdist.NewRedisDistribution(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix)
			snapshotDistributor = redisSnapshots
			activeProvider = dpsnapshot.NewFallbackActiveProvider(redisSnapshots, activeProvider)
			watcherOpts = append(watcherOpts, dpsnapshot.WithEventSource(redisSnapshots))
		}
		if active, ok, err := activeProvider.ActiveRuntimeSnapshot(ctx); err == nil && ok {
			if activeIndexed, buildErr := dpsnapshot.Build(*active); buildErr == nil {
				_ = snapshotStore.Replace(activeIndexed)
			} else {
				logger.Warn("active runtime snapshot rejected", "error", buildErr)
			}
		} else if err != nil {
			logger.Warn("active runtime snapshot unavailable", "error", err)
		}
		watcher := dpsnapshot.NewWatcher(activeProvider, snapshotStore, snapshotMetrics, cfg.Control.SnapshotPollInterval.Duration, logger, watcherOpts...)
		go watcher.Start(ctx)
	}
	egressGuard, err := newEgressGuard(cfg.Gateway.Egress)
	if err != nil {
		return nil, err
	}
	registry := provider.NewRegistry()
	if err := registry.Register("openai_compatible", openai.NewAdapter(outboundHTTPClient(0, egressGuard)).WithEgressGuard(egressGuard)); err != nil {
		return nil, err
	}
	if err := registry.Register("claude", claude.NewAdapter(outboundHTTPClient(0, egressGuard)).WithEgressGuard(egressGuard)); err != nil {
		return nil, err
	}
	if err := registry.Register("gemini", gemini.NewAdapter(outboundHTTPClient(0, egressGuard)).WithEgressGuard(egressGuard)); err != nil {
		return nil, err
	}

	admissionController := engine.AdmissionController(engine.NoopAdmission{})
	limitEnforcer := engine.LimitEnforcer(engine.NoopLimitEnforcer{})
	var attemptLimiter dispatch.AttemptLimiter
	settlementService := engine.SettlementService(engine.NoopSettlement{})
	taskSettlement := tasksvc.Settlement(tasksvc.NoopSettlement{})
	taskRepo := tasksvc.Repository(tasksvc.NewMemoryRepository())
	defaultPrice := pricing.TokenPrice{
		Currency:             cfg.Gateway.Billing.Currency,
		InputMicrosPerToken:  cfg.Gateway.Billing.InputMicrosPerToken,
		OutputMicrosPerToken: cfg.Gateway.Billing.OutputMicrosPerToken,
	}
	if cfg.Database.Enabled && database != nil && database.DB() != nil {
		taskRepo = tasksvc.NewMySQLRepository(database.DB())
	}
	taskMetrics, err := tasksvc.NewMetrics(tel.Registry)
	if err != nil {
		return nil, err
	}
	taskService := tasksvc.NewServiceWithMetrics(taskRepo, cfg.Gateway.Idempotency.TTL.Duration, taskMetrics)
	credentialResolver := providerCredentialResolver{codec: admin.NewCredentialCodec(cfg.Control.CredentialKey)}
	taskDispatcher := tasksvc.NewHTTPProviderTaskDispatcher(outboundHTTPClient(0, egressGuard), credentialResolver, snapshotChannelResolver{store: snapshotStore}).WithEgressGuard(egressGuard)
	taskDispatcher.RegisterAdapter("replicate", replicate.NewTaskAdapter(outboundHTTPClient(0, egressGuard), credentialResolver).WithEgressGuard(egressGuard))
	fileBridge := tasksvc.NewFileBridge(tasksvc.NewFileService(taskRepo, cfg.Gateway.Idempotency.TTL.Duration, tasksvc.WithFileEgressGuard(egressGuard)))
	var attemptRecorder dispatch.AttemptRecorder
	var failedSettlementService *billing.FailedSettlementService
	if cfg.Gateway.Billing.Enabled {
		repo := billing.NewMySQLRepository(database.DB())
		if err := ensureLocalSeedBalance(ctx, cfg, repo); err != nil {
			return nil, err
		}
		billingMetrics, err := billing.NewMetrics(tel.Registry)
		if err != nil {
			return nil, err
		}
		admissionController = admission.NewController(
			billing.NewBalanceService(repo),
			admission.NewPriceEstimator(defaultPrice, cfg.Gateway.Billing.EstimatedOutputTokens),
			cfg.Gateway.Billing.HoldTTL.Duration,
		)
		attemptRecorder = billing.NewAttemptWriter(repo)
		settlementService = billing.NewSettlementService(repo, billing.NewSettlementPlanner(defaultPrice), billingMetrics)
		taskSettlement = tasksvc.NewBillingSettlement(repo, defaultPrice)
		failedSettlementService = billing.NewFailedSettlementService(repo)
	}
	taskBridge := tasksvc.NewBridge(taskService, taskDispatcher, taskSettlement).
		WithDefaultPrice(defaultPrice).
		WithAttemptRecorder(attemptRecorder)
	if cfg.Gateway.Limits.Enabled {
		redisLimiter := limit.NewRedisEnforcer(redisClient.Raw(), limit.Config{
			Enabled:             cfg.Gateway.Limits.Enabled,
			RPM:                 cfg.Gateway.Limits.RPM,
			QPS:                 cfg.Gateway.Limits.QPS,
			TPM:                 cfg.Gateway.Limits.TPM,
			Concurrency:         cfg.Gateway.Limits.Concurrency,
			DailyBudgetMicros:   cfg.Gateway.Limits.DailyBudgetMicros,
			CostPerMinuteMicros: cfg.Gateway.Limits.CostPerMinuteMicros,
			Window:              cfg.Gateway.Limits.Window.Duration,
			LeaseTTL:            cfg.Gateway.Limits.LeaseTTL.Duration,
			DenyCacheTTL:        cfg.Gateway.Limits.DenyCacheTTL.Duration,
			KeyPrefix:           cfg.Gateway.Limits.KeyPrefix,
		})
		limitEnforcer = redisLimiter
		attemptLimiter = redisLimiter
	}
	streamFinalizer := stream.NewFinalizer(settlementService, observeRecorder)
	pluginManager := plugin.NewManager(builtin.Registry())

	revocationStore := redisinfra.NewRevocationStore(redisClient.Raw(), cfg.Control.RevocationTTL.Duration)
	apiKeyHasher := auth.NewAPIKeyHasher(cfg.Gateway.Auth.APIKeyHashSecret)
	adminService := admin.NewService(adminRepo, admin.NewCredentialCodec(cfg.Control.CredentialKey), revocationStore, admin.WithAPIKeyHasher(apiKeyHasher))
	if !usingDBAdmin {
		if err := seedLocalPortalAPIKey(ctx, cfg, adminService); err != nil {
			return nil, err
		}
	}
	var portalSnapshots portal.SnapshotRefresher
	var snapshotManager *runtimeSnapshotRefresher
	if usingDBAdmin {
		var publisherOpts []cpsnapshot.PublisherOption
		if snapshotDistributor != nil {
			publisherOpts = append(publisherOpts, cpsnapshot.WithDistributor(snapshotDistributor))
		}
		snapshotManager = &runtimeSnapshotRefresher{
			publisher: cpsnapshot.NewPublisher(adminRepo, cpsnapshot.NewBuilder(adminRepo), publisherOpts...),
			store:     snapshotStore,
		}
		portalSnapshots = snapshotManager
	}
	emergencyDisableStore := redisinfra.NewEmergencyDisableStore(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix)
	circuitBreaker := router.NewCircuitBreaker(router.DefaultCircuitConfig())
	snapshotProvider := dpsnapshot.NewProvider(snapshotStore, dpsnapshot.WithMetrics(snapshotMetrics))
	authenticator := auth.NewSnapshotAuthenticatorWithOptions(revocationStore, auth.WithAPIKeyHasher(apiKeyHasher))
	routePlanner := router.NewRoutePlanner(nil, emergencyDisableStore).WithSignals(
		router.NewCompositeSignalProvider(
			router.NewRedisSignalProvider(redisinfra.NewRouteSignalStore(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix)),
			circuitBreaker,
		),
	)
	dispatcher := dispatch.NewWithCredentials(registry, observeRecorder, attemptRecorder, credentialResolver, logger, emergencyDisableStore).WithReliability(circuitBreaker)
	if attemptLimiter != nil {
		dispatcher = dispatcher.WithAttemptLimiter(attemptLimiter)
	}
	gatewayEngine, err := engine.New(
		engine.WithSnapshot(snapshotProvider),
		engine.WithClassifier(classifier.NewDefault()),
		engine.WithParser(parser.NewOpenAIChatParser(cfg.Gateway.Body.MaxBytes)),
		engine.WithAuthenticator(authenticator),
		engine.WithPolicyEvaluator(policy.NewEvaluator()),
		engine.WithRoutePlanner(routePlanner),
		engine.WithAdmission(admissionController),
		engine.WithLimitEnforcer(limitEnforcer),
		engine.WithDispatcher(dispatcher),
		engine.WithSettlement(settlementService),
		engine.WithStreamFinalizer(streamFinalizer),
		engine.WithTaskBridge(taskBridge),
		engine.WithFileService(fileBridge),
		engine.WithPluginManager(pluginManager),
		engine.WithObserveRecorder(observeRecorder),
	)
	if err != nil {
		return nil, err
	}
	return &gatewayRuntime{
		engine:            gatewayEngine,
		snapshotProvider:  snapshotProvider,
		authenticator:     authenticator,
		observeRecorder:   observeRecorder,
		adminService:      adminService,
		taskRepo:          taskRepo,
		failedSettlements: failedSettlementService,
		portalSnapshots:   portalSnapshots,
		snapshotManager:   snapshotManager,
	}, nil
}

func (r *gatewayRuntime) portalOptions() []portal.ServiceOption {
	if r == nil || r.portalSnapshots == nil {
		return nil
	}
	return []portal.ServiceOption{portal.WithSnapshotRefresher(r.portalSnapshots)}
}

type runtimeSnapshotRefresher struct {
	publisher *cpsnapshot.Publisher
	store     *dpsnapshot.Store
}

func (r runtimeSnapshotRefresher) RefreshSnapshot(ctx context.Context) error {
	_, err := r.Publish(ctx)
	return err
}

func (r runtimeSnapshotRefresher) Publish(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error) {
	if r.publisher == nil || r.store == nil {
		return nil, apperr.ConfigUnavailable("snapshot publisher is unavailable")
	}
	runtime, err := r.publisher.Publish(ctx)
	if err != nil {
		return nil, err
	}
	indexed, err := dpsnapshot.Build(*runtime)
	if err != nil {
		return nil, err
	}
	if err := r.store.Replace(indexed); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r runtimeSnapshotRefresher) Rollback(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, error) {
	if r.publisher == nil || r.store == nil {
		return nil, apperr.ConfigUnavailable("snapshot publisher is unavailable")
	}
	runtime, err := r.publisher.Rollback(ctx)
	if err != nil {
		return nil, err
	}
	indexed, err := dpsnapshot.Build(*runtime)
	if err != nil {
		return nil, err
	}
	if err := r.store.Replace(indexed); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (r runtimeSnapshotRefresher) Diagnostics(ctx context.Context) (*cpsnapshot.Diagnostics, error) {
	if r.publisher == nil {
		return nil, apperr.ConfigUnavailable("snapshot publisher is unavailable")
	}
	return r.publisher.Diagnostics(ctx)
}

func newGatewayEngine(ctx context.Context, cfg Config, tel *telemetry.Provider, logger *slog.Logger, database *dbinfra.Client, redisClient *redisinfra.Client) (*engine.GatewayEngine, error) {
	runtime, err := newGatewayRuntime(ctx, cfg, tel, logger, database, redisClient)
	if err != nil {
		return nil, err
	}
	return runtime.engine, nil
}

type providerCredentialResolver struct {
	codec *admin.CredentialCodec
}

func (r providerCredentialResolver) ResolveProviderAPIKey(_ context.Context, channel engine.ChannelView) (string, error) {
	if channel.APIKey != "" || channel.EncryptedAPIKey == "" {
		return channel.APIKey, nil
	}
	return r.codec.Decrypt(channel.EncryptedAPIKey)
}

type snapshotChannelResolver struct {
	store *dpsnapshot.Store
}

func (r snapshotChannelResolver) ResolveProviderChannel(_ context.Context, channelID string) (engine.ChannelView, bool, error) {
	if r.store == nil {
		return engine.ChannelView{}, false, nil
	}
	current, err := r.store.Current()
	if err != nil {
		return engine.ChannelView{}, false, err
	}
	channel, ok := current.LookupChannel(channelID)
	return channel, ok, nil
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

func seedLocalPortalAPIKey(ctx context.Context, cfg Config, service *admin.Service) error {
	if !cfg.Gateway.Seed.Enabled || service == nil {
		return nil
	}
	_, err := service.CreateAPIKey(ctx, admin.APIKey{
		ID:            cfg.Gateway.Seed.APIKeyID,
		TenantID:      cfg.Gateway.Seed.TenantID,
		ProjectID:     cfg.Gateway.Seed.ProjectID,
		Name:          "local seed key",
		PlaintextKey:  cfg.Gateway.Seed.APIKey,
		AllowedModels: []string{"*"},
	})
	return err
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
