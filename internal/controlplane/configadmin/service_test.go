package configadmin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/domain/pricing"
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

func TestUpsertModelNormalizesP11CatalogFields(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	model, err := service.UpsertModel(ctx, ModelConfig{
		PublicModel:     "tts-1",
		Protocol:        "unified",
		Capability:      "audio speech",
		Tags:            []string{" audio ", "audio"},
		Modalities:      []string{"text", "audio"},
		Capabilities:    []string{"speech"},
		ContextWindow:   8192,
		MaxOutputTokens: 2048,
	})
	if err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if model.Category != string(pricing.CategoryAudioSpeech) || model.Status != "active" {
		t.Fatalf("model = %#v", model)
	}
	if len(model.Tags) != 1 || model.Metadata == nil {
		t.Fatalf("catalog fields = %#v", model)
	}
}

func TestUpsertPriceNormalizesComponentsAndLegacyFields(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	price, err := service.UpsertPrice(ctx, PriceRuleConfig{
		PublicModel: "img",
		Category:    string(pricing.CategoryImage),
		Currency:    "usd",
		Components: []pricing.Component{
			{Unit: pricing.UnitRequest, MicrosPerUnit: 100},
			{Unit: pricing.UnitInputToken, MicrosPerUnit: 2},
			{Unit: pricing.UnitOutputToken, MicrosPerUnit: 5},
		},
	})
	if err != nil {
		t.Fatalf("UpsertPrice() error = %v", err)
	}
	if price.Currency != "USD" || price.InputMicrosPerToken != 2 || price.OutputMicrosPerToken != 5 {
		t.Fatalf("price = %#v", price)
	}
}

func TestUpsertPriceRejectsInvalidCategoryUnit(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)

	_, err := service.UpsertPrice(ctx, PriceRuleConfig{
		PublicModel: "embed",
		Category:    string(pricing.CategoryEmbedding),
		Currency:    "USD",
		Components:  []pricing.Component{{Unit: pricing.UnitVideoSecond, MicrosPerUnit: 1}},
	})
	if err == nil {
		t.Fatal("UpsertPrice() succeeded, want invalid unit error")
	}
}

func TestPreviewChannelModelSyncDoesNotWriteAndFlagsCatalogGaps(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	service := NewService(repo, NewCredentialCodec("secret"), nil)
	if _, err := service.UpsertModel(ctx, ModelConfig{PublicModel: "gpt-4o-mini", Protocol: "native_openai", Capability: "chat"}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if _, err := service.UpsertPrice(ctx, PriceRuleConfig{PublicModel: "gpt-4o-mini", Currency: "USD", InputMicrosPerToken: 1}); err != nil {
		t.Fatalf("UpsertPrice() error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "https://provider.example",
		Models: []ChannelModel{{
			PublicModel:   "gpt-4o-mini",
			UpstreamModel: "gpt-4o-mini",
		}},
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}

	preview, err := service.PreviewChannelModelSync(ctx, ChannelModelSyncPreviewRequest{
		ChannelID: "channel_1",
		UpstreamModels: []ChannelModel{
			{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini-2026"},
			{PublicModel: "new-model", UpstreamModel: "new-model"},
		},
	})
	if err != nil {
		t.Fatalf("PreviewChannelModelSync() error = %v", err)
	}
	if len(preview.Changed) != 1 || len(preview.Added) != 1 || len(preview.Warnings) != 2 {
		t.Fatalf("preview = %#v", preview)
	}
	cfg, err := repo.LoadSnapshotConfig(ctx)
	if err != nil {
		t.Fatalf("LoadSnapshotConfig() error = %v", err)
	}
	if cfg.Channels[0].Models[0].UpstreamModel != "gpt-4o-mini" {
		t.Fatalf("preview wrote channel config: %#v", cfg.Channels[0].Models)
	}
}
