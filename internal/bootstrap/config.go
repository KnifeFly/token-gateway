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
	Worker      WorkerConfig    `yaml:"worker"`
}

type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type HTTPConfig struct {
	Addr              string   `yaml:"addr"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	ReadTimeout       Duration `yaml:"read_timeout"`
	WriteTimeout      Duration `yaml:"write_timeout"`
	IdleTimeout       Duration `yaml:"idle_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int      `yaml:"max_header_bytes"`
}

type DatabaseConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Driver          string   `yaml:"driver"`
	DSN             string   `yaml:"dsn"`
	MaxOpenConns    int      `yaml:"max_open_conns"`
	MaxIdleConns    int      `yaml:"max_idle_conns"`
	ConnMaxLifetime Duration `yaml:"conn_max_lifetime"`
	MigrationsDir   string   `yaml:"migrations_dir"`
}

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

type TelemetryConfig struct {
	LogLevel       string        `yaml:"log_level"`
	LogFormat      string        `yaml:"log_format"`
	MetricsEnabled bool          `yaml:"metrics_enabled"`
	Tracing        TracingConfig `yaml:"tracing"`
}

type TracingConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Exporter string `yaml:"exporter"`
}

type GatewayConfig struct {
	Body        BodyConfig         `yaml:"body"`
	Snapshot    SnapshotConfig     `yaml:"snapshot"`
	Protocol    ProtocolConfig     `yaml:"protocol"`
	Idempotency IdempotencyConfig  `yaml:"idempotency"`
	Seed        SeedSnapshotConfig `yaml:"seed_snapshot"`
}

type BodyConfig struct {
	MaxBytes int64 `yaml:"max_bytes"`
}

type SnapshotConfig struct {
	SoftTTL Duration `yaml:"soft_ttl"`
	HardTTL Duration `yaml:"hard_ttl"`
}

type ProtocolConfig struct {
	DefaultMode string `yaml:"default_mode"`
}

type IdempotencyConfig struct {
	TTL Duration `yaml:"ttl"`
}

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

type WorkerConfig struct {
	Enabled bool `yaml:"enabled"`
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
			Snapshot: SnapshotConfig{
				SoftTTL: Duration{30 * time.Second},
				HardTTL: Duration{2 * time.Minute},
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
}
