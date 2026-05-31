package admin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
)

func TestUpsertModelPreservesExplicitDisabledFromJSON(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	var model ModelConfig
	if err := json.Unmarshal([]byte(`{
		"public_model":"gpt-4o-mini",
		"protocol":"native_openai",
		"capability":"chat",
		"enabled":false
	}`), &model); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, model); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}

	cfg, err := repo.LoadSnapshotConfig(ctx)
	if err != nil {
		t.Fatalf("LoadSnapshotConfig() error = %v", err)
	}
	if len(cfg.Models) != 1 || cfg.Models[0].Enabled {
		t.Fatalf("models = %#v", cfg.Models)
	}
}

func TestUpsertDefaultsEnabledWhenFieldOmitted(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	if _, err := service.UpsertTenant(ctx, Tenant{Name: "tenant"}); err != nil {
		t.Fatalf("UpsertTenant() error = %v", err)
	}
	cfg, err := repo.LoadSnapshotConfig(ctx)
	if err != nil {
		t.Fatalf("LoadSnapshotConfig() error = %v", err)
	}
	if len(cfg.APIKeys) != 0 {
		t.Fatalf("api keys = %#v", cfg.APIKeys)
	}

	tenantsRepo := repo
	tenantsRepo.mu.RLock()
	defer tenantsRepo.mu.RUnlock()
	for _, tenant := range tenantsRepo.tenants {
		if !tenant.Enabled {
			t.Fatalf("tenant default enabled = false: %#v", tenant)
		}
		return
	}
	t.Fatal("missing tenant")
}

func TestUpsertLimitPreservesProgrammaticExplicitDisabled(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	limit, err := service.UpsertLimit(ctx, LimitRuleConfig{
		TenantID:   "tenant_1",
		RPM:        1,
		Enabled:    false,
		EnabledSet: true,
	})
	if err != nil {
		t.Fatalf("UpsertLimit() error = %v", err)
	}
	if limit.Enabled {
		t.Fatalf("limit enabled = true, want false")
	}
}

func TestCreateAPIKeyUsesConfiguredHMACHasher(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil, WithAPIKeyHasher(auth.NewAPIKeyHasher("server-secret")))

	key, err := service.CreateAPIKey(ctx, APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		PlaintextKey: "sk-local",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if !strings.HasPrefix(key.KeyHash, "hmac-sha256:") {
		t.Fatalf("key hash = %q", key.KeyHash)
	}
	if key.KeyHash != auth.HashAPIKeyHMAC("sk-local", "server-secret") {
		t.Fatalf("key hash = %q", key.KeyHash)
	}
}
