package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.HTTP.Addr != ":9501" {
		t.Fatalf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.Database.Enabled {
		t.Fatal("database should be disabled by default")
	}
	if cfg.Configd.Addr != ":9504" {
		t.Fatalf("Configd.Addr = %q", cfg.Configd.Addr)
	}
	if cfg.Console.Addr != ":9505" {
		t.Fatalf("Console.Addr = %q", cfg.Console.Addr)
	}
	if cfg.Worker.HeartbeatInterval.Duration != 10*time.Second ||
		cfg.Worker.CallbackMaxConcurrency != 4 ||
		cfg.Worker.FileCleanupInterval.Duration != time.Hour {
		t.Fatalf("worker defaults heartbeat = %s callback concurrency = %d file cleanup = %s", cfg.Worker.HeartbeatInterval.Duration, cfg.Worker.CallbackMaxConcurrency, cfg.Worker.FileCleanupInterval.Duration)
	}
}

func TestLoadConfigYAMLAndEnv(t *testing.T) {
	t.Setenv("TOKEN_GATEWAY_HTTP_ADDR", ":19090")
	t.Setenv("TOKEN_GATEWAY_CONSOLE_ADDR", ":19095")
	t.Setenv("TOKEN_GATEWAY_REDIS_ENABLED", "false")

	path := filepath.Join(t.TempDir(), "config.yaml")
	content := []byte(`
environment: test
service:
  name: custom-gateway
http:
  addr: ":8080"
  read_header_timeout: 2s
  max_header_bytes: 4096
redis:
  enabled: true
  addr: "127.0.0.1:6379"
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.HTTP.Addr != ":19090" {
		t.Fatalf("env override failed, addr = %q", cfg.HTTP.Addr)
	}
	if cfg.Console.Addr != ":19095" {
		t.Fatalf("console env override failed, addr = %q", cfg.Console.Addr)
	}
	if cfg.Redis.Enabled {
		t.Fatal("redis env override failed")
	}
	if cfg.HTTP.ReadHeaderTimeout.Duration != 2*time.Second {
		t.Fatalf("duration = %s", cfg.HTTP.ReadHeaderTimeout.Duration)
	}
}

func TestValidateEnabledDatabaseRequiresDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Enabled = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateSeedSnapshotRequiresAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Gateway.Seed.Enabled = true
	cfg.Gateway.Seed.ProviderBaseURL = "mock://openai"
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateBillingRequiresDatabase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Gateway.Billing.Enabled = true
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRequiresControlAdminToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Control.AdminToken = " "
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateProductionRequiresAPIKeyHashSecret(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = "production"
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.Gateway.Auth.APIKeyHashSecret = "prod-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateProductionRequiresEgressGuard(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = "production"
	cfg.Gateway.Auth.APIKeyHashSecret = "prod-secret"
	cfg.Gateway.Egress.Enabled = false
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}

	cfg.Gateway.Egress.Enabled = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateWorkerHeartbeatMustFitLeaseTTL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = "production"
	cfg.Gateway.Auth.APIKeyHashSecret = "prod-secret"
	cfg.Database.Enabled = true
	cfg.Database.DSN = "user:pass@tcp(127.0.0.1:3306)/token_gateway"
	cfg.Redis.Enabled = true
	cfg.Worker.Enabled = true
	cfg.Worker.LeaseTTL = Duration{30 * time.Second}
	cfg.Worker.HeartbeatInterval = Duration{15 * time.Second}
	cfg.Normalize()

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected heartbeat validation error")
	}

	cfg.Worker.HeartbeatInterval = Duration{10 * time.Second}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
