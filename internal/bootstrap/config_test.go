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
}

func TestLoadConfigYAMLAndEnv(t *testing.T) {
	t.Setenv("TOKEN_GATEWAY_HTTP_ADDR", ":19090")
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
