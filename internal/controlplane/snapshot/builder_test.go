package snapshot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestBuilderBuildsValidatedSnapshotWithoutPlaintextCredential(t *testing.T) {
	ctx := context.Background()
	repo := configadmin.NewMemoryRepository()
	service := configadmin.NewService(repo, configadmin.NewCredentialCodec("test-secret"), nil)
	key, err := service.CreateAPIKey(ctx, configadmin.APIKey{
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
	if _, err := service.UpsertModel(ctx, configadmin.ModelConfig{
		PublicModel:     "gpt-4o-mini",
		Aliases:         []string{"gpt-4o-mini-alias"},
		DisplayName:     "GPT 4o Mini",
		Protocol:        string(engine.ProtocolNativeOpenAI),
		Capability:      "chat",
		Category:        "chat",
		Tags:            []string{"fast"},
		ProviderFamily:  "openai",
		Modalities:      []string{"text"},
		Capabilities:    []string{"chat"},
		ContextWindow:   128000,
		MaxOutputTokens: 4096,
		Schema:          json.RawMessage(`{"type":"object","required":["model"]}`),
		Enabled:         true,
	}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	channel, err := service.UpsertChannel(ctx, configadmin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "https://provider.example",
		APIKey:       "provider-secret",
		Enabled:      true,
		Models: []configadmin.ChannelModel{{
			PublicModel:         "gpt-4o-mini",
			UpstreamModel:       "gpt-4o-mini",
			SupportedParameters: []string{"temperature"},
			TestStatus:          "passed",
			CostConfigStatus:    "configured",
		}},
	})
	if err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if channel.APIKey != "" || channel.EncryptedAPIKey == "" {
		t.Fatalf("channel credential leaked or missing: %#v", channel)
	}
	if _, err := service.UpsertRoute(ctx, configadmin.RoutePolicyConfig{
		PublicModel: "gpt-4o-mini",
		Candidates:  []configadmin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}},
	}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
	if _, err := service.UpsertPrice(ctx, configadmin.PriceRuleConfig{
		PublicModel:           "gpt-4o-mini",
		Currency:              "USD",
		InputMicrosPerToken:   10,
		OutputMicrosPerToken:  20,
		EstimatedOutputTokens: 64,
		Enabled:               true,
	}); err != nil {
		t.Fatalf("UpsertPrice() error = %v", err)
	}
	if _, err := service.UpsertLimit(ctx, configadmin.LimitRuleConfig{
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
	if _, err := service.UpsertPluginBinding(ctx, configadmin.PluginBindingConfig{
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
	if runtime.Models[0].Category != "chat" || runtime.Models[0].ProviderFamily != "openai" || runtime.Models[0].ContextWindow != 128000 {
		t.Fatalf("p11 model catalog = %#v", runtime.Models[0])
	}
	if runtime.Channels[0].Models[0].CostConfigStatus != "configured" || runtime.Models[0].ProviderMappings[0].TestStatus != "passed" {
		t.Fatalf("channel model metadata = %#v mappings = %#v", runtime.Channels[0].Models[0], runtime.Models[0].ProviderMappings)
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
	repo := configadmin.NewMemoryRepository()
	service := configadmin.NewService(repo, configadmin.NewCredentialCodec("test-secret"), nil)
	seedSnapshotConfig(t, ctx, service, "gpt-4o-mini")
	distributor := &fakeRuntimeDistributor{}
	publisher := NewPublisher(repo, nil, WithDistributor(distributor))
	first, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	if distributor.versions[len(distributor.versions)-1] != first.Version {
		t.Fatalf("distributed first versions = %#v", distributor.versions)
	}
	if _, err := service.UpsertModel(ctx, configadmin.ModelConfig{
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
	if distributor.versions[len(distributor.versions)-1] != first.Version {
		t.Fatalf("distributed rollback versions = %#v", distributor.versions)
	}
	diagnostics, err := publisher.Diagnostics(ctx)
	if err != nil {
		t.Fatalf("Diagnostics() error = %v", err)
	}
	if diagnostics.Active == nil || !diagnostics.Active.Valid || diagnostics.Active.Version != first.Version {
		t.Fatalf("active diagnostics = %#v", diagnostics.Active)
	}
}

type fakeRuntimeDistributor struct {
	versions []string
}

func (d *fakeRuntimeDistributor) PublishActiveRuntimeSnapshot(_ context.Context, runtime RuntimeSnapshot) error {
	d.versions = append(d.versions, runtime.Version)
	return nil
}

func seedSnapshotConfig(t *testing.T, ctx context.Context, service *configadmin.Service, model string) {
	t.Helper()
	if _, err := service.CreateAPIKey(ctx, configadmin.APIKey{TenantID: "tenant", ProjectID: "project", PlaintextKey: "tg-test"}); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if _, err := service.UpsertModel(ctx, configadmin.ModelConfig{PublicModel: model, Protocol: string(engine.ProtocolNativeOpenAI), Capability: "chat", Enabled: true}); err != nil {
		t.Fatalf("UpsertModel() error = %v", err)
	}
	if _, err := service.UpsertChannel(ctx, configadmin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models:       []configadmin.ChannelModel{{PublicModel: model, UpstreamModel: model}},
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	if _, err := service.UpsertRoute(ctx, configadmin.RoutePolicyConfig{PublicModel: model, Candidates: []configadmin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}}}); err != nil {
		t.Fatalf("UpsertRoute() error = %v", err)
	}
}
