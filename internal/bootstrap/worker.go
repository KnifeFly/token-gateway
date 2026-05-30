package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
	dbinfra "github.com/KnifeFly/token-gateway/internal/infra/db"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
	redisinfra "github.com/KnifeFly/token-gateway/internal/infra/redis"
	"github.com/KnifeFly/token-gateway/internal/infra/telemetry"
	"github.com/KnifeFly/token-gateway/internal/provider/replicate"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/internal/transport/httpserver"
	"github.com/KnifeFly/token-gateway/internal/worker"
	"github.com/KnifeFly/token-gateway/internal/worker/jobs"
)

// WorkerApp wires the P0 worker process.
type WorkerApp struct {
	server          *httpserver.Server
	runner          *worker.Runner
	db              *dbinfra.Client
	redis           *redisinfra.Client
	telemetry       *telemetry.Provider
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

// NewWorkerApp builds the worker process.
func NewWorkerApp(ctx context.Context, cfg Config) (*WorkerApp, error) {
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	tel, err := telemetry.New(ctx, telemetry.Config{
		ServiceName:     cfg.Service.Name + "-worker",
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
	taskRepo := tasksvc.Repository(tasksvc.NewMemoryRepository())
	adminRepo := admin.Repository(admin.NewMemoryRepository())
	billingRepo := billing.Repository(billing.NewMemoryRepository())
	if cfg.Database.Enabled && database.DB() != nil {
		taskRepo = tasksvc.NewMySQLRepository(database.DB())
		adminRepo = admin.NewMySQLRepository(database.DB())
		billingRepo = billing.NewMySQLRepository(database.DB())
	}
	taskMetrics, err := tasksvc.NewMetrics(tel.Registry)
	if err != nil {
		return nil, err
	}
	billingMetrics, err := billing.NewMetrics(tel.Registry)
	if err != nil {
		return nil, err
	}
	workerMetrics, err := worker.NewMetrics(tel.Registry)
	if err != nil {
		return nil, err
	}
	taskService := tasksvc.NewServiceWithMetrics(taskRepo, cfg.Gateway.Idempotency.TTL.Duration, taskMetrics)
	dispatcher := tasksvc.NewHTTPProviderTaskDispatcher(
		&http.Client{Timeout: cfg.Worker.JobTimeout.Duration},
		providerCredentialResolver{codec: admin.NewCredentialCodec(cfg.Control.CredentialKey)},
		adminChannelResolver{repo: adminRepo},
	)
	dispatcher.RegisterAdapter("replicate", replicate.NewTaskAdapter(
		&http.Client{Timeout: cfg.Worker.JobTimeout.Duration},
		providerCredentialResolver{codec: admin.NewCredentialCodec(cfg.Control.CredentialKey)},
	))
	price := pricing.TokenPrice{
		Currency:             cfg.Gateway.Billing.Currency,
		InputMicrosPerToken:  cfg.Gateway.Billing.InputMicrosPerToken,
		OutputMicrosPerToken: cfg.Gateway.Billing.OutputMicrosPerToken,
	}
	taskSettlement := tasksvc.Settlement(tasksvc.NoopSettlement{})
	if cfg.Gateway.Billing.Enabled {
		taskSettlement = tasksvc.NewBillingSettlement(billingRepo, price)
	}
	jobList := []worker.Job{
		jobs.NewProviderTaskPoller(taskRepo, dispatcher, taskService, taskSettlement, cfg.Worker.ProviderTaskPollInterval.Duration, cfg.Worker.BatchSize),
		jobs.NewFailedSettlementReplayer(billing.NewFailedSettlementServiceWithMetrics(billingRepo, billingMetrics), cfg.Worker.FailedSettlementInterval.Duration, cfg.Worker.BatchSize),
		jobs.NewBalanceHoldReaper(billing.NewBalanceService(billingRepo), cfg.Worker.HoldReaperInterval.Duration, cfg.Worker.BatchSize),
		jobs.NewReconciliationJob(billing.NewReconciliationService(billingRepo), cfg.Worker.ReconciliationInterval.Duration),
		jobs.NewCallbackDispatcherWithMetrics(taskRepo, &http.Client{Timeout: cfg.Worker.JobTimeout.Duration}, taskMetrics, cfg.Worker.CallbackInterval.Duration, cfg.Worker.BatchSize),
	}
	leaseStore := worker.LeaseStore(worker.NewRedisLeaseStore(redisClient.Raw(), cfg.Gateway.Limits.KeyPrefix))
	runner := worker.NewRunner(jobList, leaseStore, logger, workerMetrics, worker.Config{LeaseTTL: cfg.Worker.LeaseTTL.Duration})
	readiness := func(ctx context.Context) []httpserver.DependencyStatus {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		return []httpserver.DependencyStatus{
			dependencyFromDB(database.Ping(ctx)),
			dependencyFromRedis(redisClient.Ping(ctx)),
		}
	}
	handler := httpserver.NewHandler(readiness, tel.Registry, logger)
	server := httpserver.New(workerServerConfig(cfg), handler, logger)
	return &WorkerApp{
		server:          server,
		runner:          runner,
		db:              database,
		redis:           redisClient,
		telemetry:       tel,
		logger:          logger,
		shutdownTimeout: cfg.Worker.ShutdownTimeout.Duration,
	}, nil
}

// Run starts the worker runner and metrics server until ctx is canceled.
func (a *WorkerApp) Run(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() { errCh <- a.server.Start() }()
	go func() { errCh <- a.runner.Run(runCtx) }()
	select {
	case <-ctx.Done():
		cancel()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()
		shutdownErr := a.server.Shutdown(shutdownCtx)
		closeErr := a.Close(context.Background())
		return errors.Join(shutdownErr, closeErr)
	case err := <-errCh:
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer shutdownCancel()
		shutdownErr := a.server.Shutdown(shutdownCtx)
		closeErr := a.Close(context.Background())
		return errors.Join(err, shutdownErr, closeErr)
	}
}

// Close releases process dependencies.
func (a *WorkerApp) Close(ctx context.Context) error {
	if a == nil {
		return nil
	}
	return errors.Join(a.db.Close(), a.redis.Close(), a.telemetry.Shutdown(ctx))
}

// Logger returns the process logger.
func (a *WorkerApp) Logger() *slog.Logger {
	return a.logger
}

func workerServerConfig(cfg Config) httpserver.Config {
	return httpserver.Config{
		Addr:              cfg.Worker.Addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   cfg.Worker.ShutdownTimeout.Duration,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}

type adminChannelResolver struct {
	repo admin.Repository
}

func (r adminChannelResolver) ResolveProviderChannel(ctx context.Context, channelID string) (engine.ChannelView, bool, error) {
	if r.repo == nil {
		return engine.ChannelView{}, false, nil
	}
	runtime, ok, err := cpsnapshot.ActiveRuntimeSnapshot(ctx, r.repo)
	if err != nil || !ok {
		return engine.ChannelView{}, false, err
	}
	for _, channel := range runtime.Channels {
		if channel.ID != channelID {
			continue
		}
		models := make(map[string]string, len(channel.Models))
		for _, model := range channel.Models {
			models[model.PublicModel] = model.UpstreamModel
		}
		return engine.ChannelView{
			ID:              channel.ID,
			ProviderType:    channel.ProviderType,
			BaseURL:         channel.BaseURL,
			APIKey:          channel.APIKey,
			CredentialRef:   channel.CredentialRef,
			EncryptedAPIKey: channel.EncryptedAPIKey,
			Enabled:         channel.Enabled,
			Timeout:         channel.Timeout,
			Models:          models,
		}, true, nil
	}
	return engine.ChannelView{}, false, nil
}
