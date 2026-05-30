package snapshot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestBuilderBuildsValidatedSnapshotWithoutPlaintextCredential(t *testing.T) {
	ctx := context.Background()
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("test-secret"), nil)
	key, err := service.CreateAPIKey(ctx, admin.APIKey{
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		Name:          "test",
		PlaintextKey:  "tg-test",
		AllowedModels: []string{"gpt-4o-mini"},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if key.KeyHash == "" || key.PlaintextKey == "" {
		t.Fatalf("key = %#v", key)
	}
	if _, err := service.UpsertModel(ctx, admin.ModelConfig{
		PublicModel: "gpt-4o-mini",
		Aliases:     []string{"gpt-4o-mini-alias"},
		DisplayName: "GPT 4o Mini",
		Protocol:    string(engine.ProtocolNativeOpenAI),
		Capability:  "chat",
		Schema:      json.RawMessage(`{"type":"object","required":["model"]}`),
		Enabled:     true,
	}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	channel, err := service.UpsertChannel(ctx, admin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "https://provider.example",
		APIKey:       "provider-secret",
		Enabled:      true,
		Models:       []admin.ChannelModel{{PublicModel: "gpt-4o-mini", UpstreamModel: "gpt-4o-mini"}},
	})
	if err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if channel.APIKey != "" || channel.EncryptedAPIKey == "" {
		t.Fatalf("channel credential leaked or missing: %#v", channel)
	}
	if _, err := service.UpsertRoute(ctx, admin.RoutePolicyConfig{
		PublicModel: "gpt-4o-mini",
		Candidates:  []admin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}},
	}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
	if _, err := service.UpsertPrice(ctx, admin.PriceRuleConfig{
		PublicModel:           "gpt-4o-mini",
		Currency:              "USD",
		InputMicrosPerToken:   10,
		OutputMicrosPerToken:  20,
		EstimatedOutputTokens: 64,
		Enabled:               true,
	}); err != nil {
		t.Fatalf("UpsertPrice() error = %v", err)
	}
	if _, err := service.UpsertLimit(ctx, admin.LimitRuleConfig{
		TenantID:            "tenant_1",
		ProjectID:           "project_1",
		APIKeyID:            key.ID,
		PublicModel:         "gpt-4o-mini",
		ProviderType:        "openai_compatible",
		ChannelID:           "channel_1",
		RPM:                 60,
		QPS:                 1,
		TPM:                 1000,
		Concurrency:         5,
		DailyBudgetMicros:   100000,
		CostPerMinuteMicros: 1000,
		Enabled:             true,
	}); err != nil {
		t.Fatalf("UpsertLimit() error = %v", err)
	}
	if _, err := service.UpsertPluginBinding(ctx, admin.PluginBindingConfig{
		Name:          "prompt_guard",
		Phase:         "pre_prompt",
		Model:         "gpt-4o-mini",
		Priority:      10,
		FailurePolicy: "fail_closed",
		Config:        json.RawMessage(`{"deny_terms":["blocked"]}`),
	}); err != nil {
		t.Fatalf("UpsertPluginBinding() error = %v", err)
	}

	runtime, err := NewBuilder(repo).Build(ctx)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if runtime.Channels[0].APIKey != "" || runtime.Channels[0].EncryptedAPIKey == "" {
		t.Fatalf("runtime channel credential = %#v", runtime.Channels[0])
	}
	if runtime.PriceRules[0].InputMicrosPerToken != 10 {
		t.Fatalf("price = %#v", runtime.PriceRules)
	}
	if len(runtime.Models[0].Aliases) != 1 || runtime.Models[0].DisplayName != "GPT 4o Mini" || len(runtime.Models[0].ProviderMappings) != 1 {
		t.Fatalf("model catalog = %#v", runtime.Models[0])
	}
	if len(runtime.LimitRules) != 1 || runtime.LimitRules[0].APIKeyID != key.ID || runtime.LimitRules[0].RPM != 60 {
		t.Fatalf("limit rules = %#v", runtime.LimitRules)
	}
	if len(runtime.PluginBindings) != 1 || runtime.PluginBindings[0].Name != "prompt_guard" {
		t.Fatalf("plugin bindings = %#v", runtime.PluginBindings)
	}
}

func TestValidateRejectsBadRoute(t *testing.T) {
	err := Validate(RuntimeSnapshot{
		Version: "bad",
		Models: []ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}},
		RoutePolicies: []RoutePolicyRuntime{{
			ID:          "route_1",
			PublicModel: "gpt-4o-mini",
			Candidates:  []RouteCandidateRuntime{{ChannelID: "missing", Priority: 1, Weight: 100}},
		}},
	})
	if err == nil {
		t.Fatal("Validate() succeeded, want error")
	}
}

func TestPublisherPublishesAndRollsBack(t *testing.T) {
	ctx := context.Background()
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("test-secret"), nil)
	seedSnapshotConfig(t, ctx, service, "gpt-4o-mini")
	publisher := NewPublisher(repo, nil)
	first, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, admin.ModelConfig{
		PublicModel: "gpt-4.1-mini",
		Protocol:    string(engine.ProtocolNativeOpenAI),
		Capability:  "chat",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("UpsertModel(second) error = %v", err)
	}
	second, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(second) error = %v", err)
	}
	if second.Version == first.Version {
		t.Fatal("snapshot version did not advance")
	}
	rolledBack, err := publisher.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rolledBack.Version != first.Version {
		t.Fatalf("rollback version = %q, want %q", rolledBack.Version, first.Version)
	}
}

func seedSnapshotConfig(t *testing.T, ctx context.Context, service *admin.Service, model string) {
	t.Helper()
	if _, err := service.CreateAPIKey(ctx, admin.APIKey{TenantID: "tenant", ProjectID: "project", PlaintextKey: "tg-test"}); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, admin.ModelConfig{PublicModel: model, Protocol: string(engine.ProtocolNativeOpenAI), Capability: "chat", Enabled: true}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, admin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models:       []admin.ChannelModel{{PublicModel: model, UpstreamModel: model}},
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if _, err := service.UpsertRoute(ctx, admin.RoutePolicyConfig{PublicModel: model, Candidates: []admin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}}}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
}
