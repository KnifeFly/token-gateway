package limit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	streamdp "github.com/KnifeFly/token-gateway/internal/dataplane/stream"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisEnforcerIntegrationCoversP1LimitTypes(t *testing.T) {
	addr := os.Getenv("TOKEN_GATEWAY_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TOKEN_GATEWAY_REDIS_ADDR to run Redis integration limits")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	prefix := "token-gateway-test:" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		keys, _ := client.Keys(ctx, prefix+"*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
	})

	enforcer := NewRedisEnforcer(client, Config{Enabled: true, KeyPrefix: prefix, DenyCacheTTL: time.Millisecond, LeaseTTL: time.Second})
	state := &engine.RequestState{
		RequestID:             "req_1",
		TenantID:              "tenant_1",
		ProjectID:             "project_1",
		APIKeyID:              "key_1",
		RequestedModel:        "gpt-4o-mini",
		EstimatedChargeMicros: 10,
		EstimatedUsage:        tokenusage.Estimate{InputTokens: 4, OutputTokens: 4},
		Snapshot: staticLimitSnapshot{rules: []engine.LimitRuleView{{
			ID:                  "rule_1",
			Scope:               engine.LimitScope{TenantID: "tenant_1", ProjectID: "project_1", APIKeyID: "key_1", PublicModel: "gpt-4o-mini"},
			RPM:                 1,
			QPS:                 1,
			TPM:                 8,
			Concurrency:         1,
			DailyBudgetMicros:   10,
			CostPerMinuteMicros: 10,
			Enabled:             true,
		}}},
	}

	release, err := enforcer.Acquire(ctx, state)
	if err != nil {
		t.Fatalf("Acquire(first) error = %v", err)
	}
	if release == nil {
		t.Fatal("release is nil")
	}
	state.RequestID = "req_2"
	if _, err := enforcer.Acquire(ctx, state); err == nil {
		t.Fatal("Acquire(second) succeeded, want rate limit")
	}
	if err := release.Release(ctx); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
}

func TestRedisStreamLeaseReleasedOnAccountingStreamClose(t *testing.T) {
	addr := os.Getenv("TOKEN_GATEWAY_REDIS_ADDR")
	if addr == "" {
		t.Skip("set TOKEN_GATEWAY_REDIS_ADDR to run Redis integration limits")
	}
	ctx := context.Background()
	client := goredis.NewClient(&goredis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	prefix := "token-gateway-stream-test:" + time.Now().Format("150405.000000000")
	t.Cleanup(func() {
		keys, _ := client.Keys(ctx, prefix+"*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
	})
	enforcer := NewRedisEnforcer(client, Config{Enabled: true, KeyPrefix: prefix, DenyCacheTTL: time.Millisecond, LeaseTTL: time.Minute})
	state := &engine.RequestState{
		RequestID:      "req_stream",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		RequestedModel: "gpt-4o-mini",
		Snapshot: staticLimitSnapshot{rules: []engine.LimitRuleView{{
			ID:          "rule_stream",
			Scope:       engine.LimitScope{TenantID: "tenant_1", ProjectID: "project_1", APIKeyID: "key_1", PublicModel: "gpt-4o-mini"},
			Concurrency: 1,
			Enabled:     true,
		}}},
	}
	release, err := enforcer.Acquire(ctx, state)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	state.AddLimitRelease(release)
	_, _, leases := enforcer.operationsFor(state, state.Snapshot.LookupLimits(engine.LimitScope{}), time.Now().UTC())
	if len(leases) != 1 {
		t.Fatalf("leases = %d, want 1", len(leases))
	}
	response, err := streamdp.NewFinalizer(redisStreamSettlement{}, nil).Wrap(ctx, state, &engine.ProviderResult{
		Response: &engine.GatewayResponse{
			Stream: &relay.StaticStream{
				Chunks: [][]byte{[]byte("data: hello\n\n")},
				Actual: tokenusage.Actual{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			},
		},
	})
	if err != nil {
		t.Fatalf("Wrap() error = %v", err)
	}
	if err := client.ZScore(ctx, leases[0].key, state.RequestID).Err(); err != nil {
		t.Fatalf("stream lease missing before close: %v", err)
	}
	if err := response.Stream.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := client.ZScore(ctx, leases[0].key, state.RequestID).Err(); err != goredis.Nil {
		t.Fatalf("stream lease after close error = %v, want redis.Nil", err)
	}
}

type staticLimitSnapshot struct {
	rules []engine.LimitRuleView
}

func (s staticLimitSnapshot) Ref() engine.SnapshotRef        { return engine.SnapshotRef{Version: "test"} }
func (s staticLimitSnapshot) ListModels() []engine.ModelView { return nil }
func (s staticLimitSnapshot) LookupAPIKeyHash(string) (engine.APIKeyView, bool) {
	return engine.APIKeyView{}, false
}
func (s staticLimitSnapshot) LookupModel(string) (engine.ModelView, bool) {
	return engine.ModelView{}, false
}
func (s staticLimitSnapshot) LookupRoute(string) (engine.RoutePolicyView, bool) {
	return engine.RoutePolicyView{}, false
}
func (s staticLimitSnapshot) LookupChannel(string) (engine.ChannelView, bool) {
	return engine.ChannelView{}, false
}
func (s staticLimitSnapshot) LookupPrice(string) (engine.PriceRuleView, bool) {
	return engine.PriceRuleView{}, false
}
func (s staticLimitSnapshot) LookupLimit(string) (engine.LimitRuleView, bool) {
	return engine.LimitRuleView{}, false
}
func (s staticLimitSnapshot) LookupLimits(scope engine.LimitScope) []engine.LimitRuleView {
	return s.rules
}
func (s staticLimitSnapshot) LookupPluginBindings(string) []engine.PluginBindingView { return nil }
func (s staticLimitSnapshot) IsAPIKeyRevoked(string) bool                            { return false }

type redisStreamSettlement struct{}

func (redisStreamSettlement) Settle(context.Context, *engine.RequestState) error { return nil }

func (redisStreamSettlement) RecordFailed(context.Context, *engine.RequestState, error) error {
	return nil
}
