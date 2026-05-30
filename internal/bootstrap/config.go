package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-friendly time.Duration wrapper.
type Duration struct {
	time.Duration
}

// UnmarshalYAML supports both duration strings and raw nanosecond integers.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!int" {
			n, err := strconv.ParseInt(value.Value, 10, 64)
			if err != nil {
				return err
			}
			d.Duration = time.Duration(n)
			return nil
		}
		parsed, err := time.ParseDuration(value.Value)
		if err != nil {
			return err
		}
		d.Duration = parsed
		return nil
	default:
		return fmt.Errorf("invalid duration node kind %d", value.Kind)
	}
}

// Config contains process configuration.
type Config struct {
	Environment string          `yaml:"environment"`
	Service     ServiceConfig   `yaml:"service"`
	HTTP        HTTPConfig      `yaml:"http"`
	Database    DatabaseConfig  `yaml:"database"`
	Redis       RedisConfig     `yaml:"redis"`
	Telemetry   TelemetryConfig `yaml:"telemetry"`
	Gateway     GatewayConfig   `yaml:"gateway"`
	Control     ControlConfig   `yaml:"control"`
	Worker      WorkerConfig    `yaml:"worker"`
	Configd     ConfigdConfig   `yaml:"configd"`
}

// ServiceConfig identifies the running service in logs and telemetry.
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// HTTPConfig controls public gateway HTTP server timeouts and limits.
type HTTPConfig struct {
	Addr              string   `yaml:"addr"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	ReadTimeout       Duration `yaml:"read_timeout"`
	WriteTimeout      Duration `yaml:"write_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int      `yaml:"max_header_bytes"`
}

// DatabaseConfig controls optional MySQL connectivity and migrations.
type DatabaseConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Driver          string   `yaml:"driver"`
	DSN             string   `yaml:"dsn"`
	MaxOpenConns    int      `yaml:"max_open_conns"`
	MaxIdleConns    int      `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"`
	MigrationsDir   string   `yaml:"migrations_dir"`
}

// RedisConfig controls optional Redis connectivity.
type RedisConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Addr         string   `yaml:"addr"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	DB           int      `yaml:"db"`
	TLS          bool     `yaml:"tls"`
	DialTimeout  Duration `yaml:"dial_timeout"`
	ReadTimeout  Duration `yaml:"read_timeout"`
	WriteTimeout Duration `yaml:"write_timeout"`
}

// TelemetryConfig controls logging, metrics, and tracing.
type TelemetryConfig struct {
	LogLevel       string        `yaml:"log_level"`
	LogFormat      string        `yaml:"log_format"`
	MetricsEnabled bool          `yaml:"metrics_enabled"`
	Tracing        TracingConfig `yaml:"tracing"`
}

// TracingConfig controls OpenTelemetry tracing export.
type TracingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Exporter string `yaml:"exporter"`
}

// GatewayConfig groups data-plane behavior toggles.
type GatewayConfig struct {
	Body        BodyConfig         `yaml:"body"`
	Protocol    ProtocolConfig     `yaml:"protocol"`
	Idempotency IdempotencyConfig  `yaml:"idempotency"`
	Seed        SeedSnapshotConfig `yaml:"seed_snapshot"`
	Billing     BillingConfig      `yaml:"billing"`
	Limits      LimitsConfig       `yaml:"limits"`
}

// BodyConfig controls request body limits.
type BodyConfig struct {
	MaxBytes int64 `yaml:"max_bytes"`
}

// ProtocolConfig controls fallback protocol classification.
type ProtocolConfig struct {
	DefaultMode string `yaml:"default_mode"`
}

// IdempotencyConfig controls async task and file idempotency retention.
type IdempotencyConfig struct {
	TTL Duration `yaml:"ttl"`
}

// SeedSnapshotConfig describes the local bootstrap snapshot.
type SeedSnapshotConfig struct {
	Enabled         bool     `yaml:"enabled"`
	APIKey          string   `yaml:"api_key"`
	APIKeyID        string   `yaml:"api_key_id"`
	TenantID        string   `yaml:"tenant_id"`
	ProjectID       string   `yaml:"project_id"`
	Model           string   `yaml:"model"`
	UpstreamModel   string   `yaml:"upstream_model"`
	ProviderType    string   `yaml:"provider_type"`
	ChannelID       string   `yaml:"channel_id"`
	ProviderBaseURL string   `yaml:"provider_base_url"`
	ProviderAPIKey  string   `yaml:"provider_api_key"`
	RouteStrategy   string   `yaml:"route_strategy"`
	RoutePriority   int      `yaml:"route_priority"`
	RouteWeight     int      `yaml:"route_weight"`
	ChannelTimeout  Duration `yaml:"channel_timeout"`
}

// BillingConfig controls local billing and balance reservation behavior.
type BillingConfig struct {
	Enabled                bool     `yaml:"enabled"`
	Currency               string   `yaml:"currency"`
	InputMicrosPerToken    int64    `yaml:"input_micros_per_token"`
	OutputMicrosPerToken   int64    `yaml:"output_micros_per_token"`
	EstimatedOutputTokens  int64    `yaml:"estimated_output_tokens"`
	HoldTTL                Duration `yaml:"hold_ttl"`
	LocalSeedBalanceMicros int64    `yaml:"local_seed_balance_micros"`
}

// LimitsConfig controls Redis-backed admission limits.
type LimitsConfig struct {
	Enabled             bool     `yaml:"enabled"`
	RPM                 int64    `yaml:"rpm"`
	QPS                 int64    `yaml:"qps"`
	TPM                 int64    `yaml:"tpm"`
	Concurrency         int64    `yaml:"concurrency"`
	DailyBudgetMicros   int64    `yaml:"daily_budget_micros"`
	CostPerMinuteMicros int64    `yaml:"cost_per_minute_micros"`
	Window              Duration `yaml:"window"`
	LeaseTTL            Duration `yaml:"lease_ttl"`
	DenyCacheTTL        Duration `yaml:"deny_cache_ttl"`
	KeyPrefix           string   `yaml:"key_prefix"`
}

// ControlConfig controls the admin control API process.
type ControlConfig struct {
	Addr                 string   `yaml:"addr"`
	AdminToken           string   `yaml:"admin_token"`
	CredentialKey        string   `yaml:"credential_key"`
	SnapshotPollInterval Duration `yaml:"snapshot_poll_interval"`
	RevocationTTL        Duration `yaml:"revocation_ttl"`
}

// WorkerConfig controls background worker scheduling.
type WorkerConfig struct {
	Enabled                  bool     `yaml:"enabled"`
	Addr                     string   `yaml:"addr"`
	ShutdownTimeout          Duration `yaml:"shutdown_timeout"`
	LeaseTTL                 Duration `yaml:"lease_ttl"`
	JobTimeout               Duration `yaml:"job_timeout"`
	ProviderTaskPollInterval Duration `yaml:"provider_task_poll_interval"`
	FailedSettlementInterval Duration `yaml:"failed_settlement_interval"`
	HoldReaperInterval       Duration `yaml:"hold_reaper_interval"`
	ReconciliationInterval   Duration `yaml:"reconciliation_interval"`
	CallbackInterval         Duration `yaml:"callback_interval"`
	BatchSize                int      `yaml:"batch_size"`
}

// ConfigdConfig controls the snapshot publishing service.
type ConfigdConfig struct {
	Addr            string   `yaml:"addr"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`
	PublishOnStart  bool     `yaml:"publish_on_start"`
}

// DefaultConfig returns local-safe defaults. External dependencies are disabled
// so unit tests never need Docker services.
func DefaultConfig() Config {
	return Config{
		Environment: "local",
		Service: ServiceConfig{
			Name:    "token-gateway",
			Version: "dev",
		},
		HTTP: HTTPConfig{
			Addr:              ":9501",
			ReadHeaderTimeout: Duration{5 * time.Second},
			ReadTimeout:       Duration{30 * time.Second},
			WriteTimeout:      Duration{30 * time.Second},
			IdleTimeout:       Duration{60 * time.Second},
			ShutdownTimeout:   Duration{10 * time.Second},
			MaxHeaderBytes:    1 << 20,
		},
		Database: DatabaseConfig{
			Enabled:         false,
			Driver:          "mysql",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: Duration{30 * time.Minute},
			MigrationsDir:   "migrations/mysql",
		},
		Redis: RedisConfig{
			Enabled:      false,
			Addr:         "127.0.0.1:6379",
			DialTimeout:  Duration{5 * time.Second},
			ReadTimeout:  Duration{3 * time.Second},
			WriteTimeout: Duration{3 * time.Second},
		},
		Telemetry: TelemetryConfig{
			LogLevel:       "info",
			LogFormat:      "text",
			MetricsEnabled: true,
			Tracing: TracingConfig{
				Enabled:  false,
				Exporter: "noop",
			},
		},
		Gateway: GatewayConfig{
			Body: BodyConfig{
				MaxBytes: 4 << 20,
			},
			Protocol: ProtocolConfig{
				DefaultMode: "auto",
			},
			Idempotency: IdempotencyConfig{
				TTL: Duration{24 * time.Hour},
			},
			Seed: SeedSnapshotConfig{
				Enabled:        false,
				APIKeyID:       "key_local",
				TenantID:       "tenant_local",
				ProjectID:      "project_local",
				Model:          "gpt-4o-mini",
				UpstreamModel:  "gpt-4o-mini",
				ProviderType:   "openai_compatible",
				ChannelID:      "channel_mock_openai",
				RouteStrategy:  "priority",
				RoutePriority:  1,
				RouteWeight:    100,
				ChannelTimeout: Duration{30 * time.Second},
			},
			Billing: BillingConfig{
				Enabled:               false,
				Currency:              "USD",
				InputMicrosPerToken:   1,
				OutputMicrosPerToken:  2,
				EstimatedOutputTokens: 256,
				HoldTTL:               Duration{10 * time.Minute},
			},
			Limits: LimitsConfig{
				Enabled:      false,
				RPM:          3600,
				QPS:          60,
				TPM:          60000,
				Concurrency:  100,
				Window:       Duration{time.Second},
				LeaseTTL:     Duration{30 * time.Second},
				DenyCacheTTL: Duration{time.Second},
				KeyPrefix:    "token-gateway",
			},
		},
		Control: ControlConfig{
			Addr:                 ":9502",
			AdminToken:           "local-admin-token",
			CredentialKey:        "local-control-plane-credential-key",
			SnapshotPollInterval: Duration{5 * time.Second},
			RevocationTTL:        Duration{24 * time.Hour},
		},
		Worker: WorkerConfig{
			Addr:                     ":9503",
			ShutdownTimeout:          Duration{10 * time.Second},
			LeaseTTL:                 Duration{30 * time.Second},
			JobTimeout:               Duration{30 * time.Second},
			ProviderTaskPollInterval: Duration{5 * time.Second},
			FailedSettlementInterval: Duration{time.Minute},
			HoldReaperInterval:       Duration{time.Minute},
			ReconciliationInterval:   Duration{15 * time.Minute},
			CallbackInterval:         Duration{5 * time.Second},
			BatchSize:                100,
		},
		Configd: ConfigdConfig{
			Addr:            ":9504",
			ShutdownTimeout: Duration{10 * time.Second},
			PublishOnStart:  false,
		},
	}
}

// LoadConfig loads config from disk and applies environment overrides.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		content, err := os.ReadFile(path)
		if err != nil {
			return Config{}, err
		}
		if err := yaml.Unmarshal(content, &cfg); err != nil {
			return Config{}, err
		}
	}
	applyEnv(&cfg)
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Normalize fills derived defaults.
func (c *Config) Normalize() {
	c.Environment = strings.TrimSpace(c.Environment)
	c.Service.Name = strings.TrimSpace(c.Service.Name)
	c.Service.Version = strings.TrimSpace(c.Service.Version)
	c.Database.Driver = strings.ToLower(strings.TrimSpace(c.Database.Driver))
	c.Telemetry.LogLevel = strings.ToLower(strings.TrimSpace(c.Telemetry.LogLevel))
	c.Telemetry.LogFormat = strings.ToLower(strings.TrimSpace(c.Telemetry.LogFormat))
	c.Telemetry.Tracing.Exporter = strings.ToLower(strings.TrimSpace(c.Telemetry.Tracing.Exporter))
	c.Control.AdminToken = strings.TrimSpace(c.Control.AdminToken)

	if c.Environment == "" {
		c.Environment = "local"
	}
	if c.Service.Name == "" {
		c.Service.Name = "token-gateway"
	}
	if c.Service.Version == "" {
		c.Service.Version = "dev"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "mysql"
	}
	if c.Database.MigrationsDir == "" {
		c.Database.MigrationsDir = "migrations/mysql"
	}
	if c.Telemetry.LogLevel == "" {
		c.Telemetry.LogLevel = "info"
	}
	if c.Telemetry.LogFormat == "" {
		c.Telemetry.LogFormat = "text"
	}
	if c.Telemetry.Tracing.Exporter == "" {
		c.Telemetry.Tracing.Exporter = "noop"
	}
	c.Gateway.Seed.ProviderType = strings.TrimSpace(c.Gateway.Seed.ProviderType)
	c.Gateway.Seed.ChannelID = strings.TrimSpace(c.Gateway.Seed.ChannelID)
	c.Gateway.Seed.Model = strings.TrimSpace(c.Gateway.Seed.Model)
	c.Gateway.Seed.UpstreamModel = strings.TrimSpace(c.Gateway.Seed.UpstreamModel)
	c.Gateway.Seed.RouteStrategy = strings.TrimSpace(c.Gateway.Seed.RouteStrategy)
	if c.Gateway.Seed.APIKeyID == "" {
		c.Gateway.Seed.APIKeyID = "key_local"
	}
	if c.Gateway.Seed.TenantID == "" {
		c.Gateway.Seed.TenantID = "tenant_local"
	}
	if c.Gateway.Seed.ProjectID == "" {
		c.Gateway.Seed.ProjectID = "project_local"
	}
	if c.Gateway.Seed.UpstreamModel == "" {
		c.Gateway.Seed.UpstreamModel = c.Gateway.Seed.Model
	}
	if c.Gateway.Seed.ProviderType == "" {
		c.Gateway.Seed.ProviderType = "openai_compatible"
	}
	if c.Gateway.Seed.ChannelID == "" {
		c.Gateway.Seed.ChannelID = "channel_mock_openai"
	}
	if c.Gateway.Seed.RouteStrategy == "" {
		c.Gateway.Seed.RouteStrategy = "priority"
	}
	if c.Gateway.Seed.RouteWeight <= 0 {
		c.Gateway.Seed.RouteWeight = 100
	}
	if c.Gateway.Seed.ChannelTimeout.Duration <= 0 {
		c.Gateway.Seed.ChannelTimeout = Duration{30 * time.Second}
	}
	c.Gateway.Billing.Currency = strings.ToUpper(strings.TrimSpace(c.Gateway.Billing.Currency))
	if c.Gateway.Billing.Currency == "" {
		c.Gateway.Billing.Currency = "USD"
	}
	if c.Gateway.Billing.HoldTTL.Duration <= 0 {
		c.Gateway.Billing.HoldTTL = Duration{10 * time.Minute}
	}
	if c.Gateway.Billing.EstimatedOutputTokens <= 0 {
		c.Gateway.Billing.EstimatedOutputTokens = 256
	}
	if c.Gateway.Limits.Window.Duration <= 0 {
		c.Gateway.Limits.Window = Duration{time.Second}
	}
	if c.Gateway.Limits.LeaseTTL.Duration <= 0 {
		c.Gateway.Limits.LeaseTTL = Duration{30 * time.Second}
	}
	if c.Gateway.Limits.KeyPrefix == "" {
		c.Gateway.Limits.KeyPrefix = "token-gateway"
	}
	if c.Control.Addr == "" {
		c.Control.Addr = ":9502"
	}
	if c.Control.CredentialKey == "" {
		c.Control.CredentialKey = "local-control-plane-credential-key"
	}
	if c.Control.SnapshotPollInterval.Duration <= 0 {
		c.Control.SnapshotPollInterval = Duration{5 * time.Second}
	}
	if c.Control.RevocationTTL.Duration <= 0 {
		c.Control.RevocationTTL = Duration{24 * time.Hour}
	}
	if c.Worker.Addr == "" {
		c.Worker.Addr = ":9503"
	}
	if c.Worker.ShutdownTimeout.Duration <= 0 {
		c.Worker.ShutdownTimeout = Duration{10 * time.Second}
	}
	if c.Worker.LeaseTTL.Duration <= 0 {
		c.Worker.LeaseTTL = Duration{30 * time.Second}
	}
	if c.Worker.JobTimeout.Duration <= 0 {
		c.Worker.JobTimeout = Duration{30 * time.Second}
	}
	if c.Worker.ProviderTaskPollInterval.Duration <= 0 {
		c.Worker.ProviderTaskPollInterval = Duration{5 * time.Second}
	}
	if c.Worker.FailedSettlementInterval.Duration <= 0 {
		c.Worker.FailedSettlementInterval = Duration{time.Minute}
	}
	if c.Worker.HoldReaperInterval.Duration <= 0 {
		c.Worker.HoldReaperInterval = Duration{time.Minute}
	}
	if c.Worker.ReconciliationInterval.Duration <= 0 {
		c.Worker.ReconciliationInterval = Duration{15 * time.Minute}
	}
	if c.Worker.CallbackInterval.Duration <= 0 {
		c.Worker.CallbackInterval = Duration{5 * time.Second}
	}
	if c.Worker.BatchSize <= 0 {
		c.Worker.BatchSize = 100
	}
	if c.Configd.Addr == "" {
		c.Configd.Addr = ":9504"
	}
	if c.Configd.ShutdownTimeout.Duration <= 0 {
		c.Configd.ShutdownTimeout = Duration{10 * time.Second}
	}
}

// Validate checks the minimum viable M0 configuration.
func (c Config) Validate() error {
	var errs []error
	if c.Service.Name == "" {
		errs = append(errs, errors.New("service.name is required"))
	}
	if c.HTTP.Addr == "" {
		errs = append(errs, errors.New("http.addr is required"))
	}
	if c.HTTP.MaxHeaderBytes <= 0 {
		errs = append(errs, errors.New("http.max_header_bytes must be positive"))
	}
	if c.Database.Enabled {
		if c.Database.Driver != "mysql" {
			errs = append(errs, fmt.Errorf("unsupported database driver %q", c.Database.Driver))
		}
		if c.Database.DSN == "" {
			errs = append(errs, errors.New("database.dsn is required when database is enabled"))
		}
	}
	if c.Redis.Enabled && c.Redis.Addr == "" {
		errs = append(errs, errors.New("redis.addr is required when redis is enabled"))
	}
	if c.Control.AdminToken == "" {
		errs = append(errs, errors.New("control.admin_token is required"))
	}
	if c.Telemetry.Tracing.Enabled {
		switch c.Telemetry.Tracing.Exporter {
		case "noop", "stdout":
		default:
			errs = append(errs, fmt.Errorf("unsupported tracing exporter %q", c.Telemetry.Tracing.Exporter))
		}
	}
	if c.Gateway.Seed.Enabled {
		if c.Gateway.Seed.APIKey == "" {
			errs = append(errs, errors.New("gateway.seed_snapshot.api_key is required when seed snapshot is enabled"))
		}
		if c.Gateway.Seed.Model == "" {
			errs = append(errs, errors.New("gateway.seed_snapshot.model is required when seed snapshot is enabled"))
		}
		if c.Gateway.Seed.ProviderBaseURL == "" {
			errs = append(errs, errors.New("gateway.seed_snapshot.provider_base_url is required when seed snapshot is enabled"))
		}
	}
	if c.Gateway.Billing.Enabled && !c.Database.Enabled {
		errs = append(errs, errors.New("database must be enabled when gateway.billing is enabled"))
	}
	if c.Gateway.Billing.Enabled && c.Gateway.Billing.Currency == "" {
		errs = append(errs, errors.New("gateway.billing.currency is required when billing is enabled"))
	}
	if c.Gateway.Limits.Enabled && !c.Redis.Enabled {
		errs = append(errs, errors.New("redis must be enabled when gateway.limits is enabled"))
	}
	if c.Worker.Enabled && !c.Database.Enabled {
		errs = append(errs, errors.New("database must be enabled when worker is enabled"))
	}
	if c.Worker.Enabled && !c.Redis.Enabled {
		errs = append(errs, errors.New("redis must be enabled when worker is enabled"))
	}
	return errors.Join(errs...)
}

func applyEnv(cfg *Config) {
	setString := func(key string, dst *string) {
		if value, ok := os.LookupEnv(key); ok {
			*dst = value
		}
	}
	setBool := func(key string, dst *bool) {
		if value, ok := os.LookupEnv(key); ok {
			parsed, err := strconv.ParseBool(value)
			if err == nil {
				*dst = parsed
			}
		}
	}
	setInt := func(key string, dst *int) {
		if value, ok := os.LookupEnv(key); ok {
			parsed, err := strconv.Atoi(value)
			if err == nil {
				*dst = parsed
			}
		}
	}

	setString("TOKEN_GATEWAY_ENVIRONMENT", &cfg.Environment)
	setString("TOKEN_GATEWAY_SERVICE_NAME", &cfg.Service.Name)
	setString("TOKEN_GATEWAY_SERVICE_VERSION", &cfg.Service.Version)
	setString("TOKEN_GATEWAY_HTTP_ADDR", &cfg.HTTP.Addr)
	setBool("TOKEN_GATEWAY_DATABASE_ENABLED", &cfg.Database.Enabled)
	setString("TOKEN_GATEWAY_DATABASE_DRIVER", &cfg.Database.Driver)
	setString("TOKEN_GATEWAY_DATABASE_DSN", &cfg.Database.DSN)
	setString("TOKEN_GATEWAY_DATABASE_MIGRATIONS_DIR", &cfg.Database.MigrationsDir)
	setBool("TOKEN_GATEWAY_REDIS_ENABLED", &cfg.Redis.Enabled)
	setString("TOKEN_GATEWAY_REDIS_ADDR", &cfg.Redis.Addr)
	setString("TOKEN_GATEWAY_REDIS_USERNAME", &cfg.Redis.Username)
	setString("TOKEN_GATEWAY_REDIS_PASSWORD", &cfg.Redis.Password)
	setInt("TOKEN_GATEWAY_REDIS_DB", &cfg.Redis.DB)
	setBool("TOKEN_GATEWAY_REDIS_TLS", &cfg.Redis.TLS)
	setString("TOKEN_GATEWAY_LOG_LEVEL", &cfg.Telemetry.LogLevel)
	setString("TOKEN_GATEWAY_LOG_FORMAT", &cfg.Telemetry.LogFormat)
	setBool("TOKEN_GATEWAY_METRICS_ENABLED", &cfg.Telemetry.MetricsEnabled)
	setBool("TOKEN_GATEWAY_TRACING_ENABLED", &cfg.Telemetry.Tracing.Enabled)
	setString("TOKEN_GATEWAY_TRACING_EXPORTER", &cfg.Telemetry.Tracing.Exporter)
	setBool("TOKEN_GATEWAY_SEED_SNAPSHOT_ENABLED", &cfg.Gateway.Seed.Enabled)
	setString("TOKEN_GATEWAY_SEED_API_KEY", &cfg.Gateway.Seed.APIKey)
	setString("TOKEN_GATEWAY_SEED_MODEL", &cfg.Gateway.Seed.Model)
	setString("TOKEN_GATEWAY_SEED_UPSTREAM_MODEL", &cfg.Gateway.Seed.UpstreamModel)
	setString("TOKEN_GATEWAY_SEED_PROVIDER_TYPE", &cfg.Gateway.Seed.ProviderType)
	setString("TOKEN_GATEWAY_SEED_PROVIDER_BASE_URL", &cfg.Gateway.Seed.ProviderBaseURL)
	setString("TOKEN_GATEWAY_SEED_PROVIDER_API_KEY", &cfg.Gateway.Seed.ProviderAPIKey)
	setBool("TOKEN_GATEWAY_BILLING_ENABLED", &cfg.Gateway.Billing.Enabled)
	setString("TOKEN_GATEWAY_BILLING_CURRENCY", &cfg.Gateway.Billing.Currency)
	setBool("TOKEN_GATEWAY_LIMITS_ENABLED", &cfg.Gateway.Limits.Enabled)
	setString("TOKEN_GATEWAY_CONTROL_ADDR", &cfg.Control.Addr)
	setString("TOKEN_GATEWAY_CONTROL_ADMIN_TOKEN", &cfg.Control.AdminToken)
	setString("TOKEN_GATEWAY_CONTROL_CREDENTIAL_KEY", &cfg.Control.CredentialKey)
	setBool("TOKEN_GATEWAY_WORKER_ENABLED", &cfg.Worker.Enabled)
	setString("TOKEN_GATEWAY_WORKER_ADDR", &cfg.Worker.Addr)
}
