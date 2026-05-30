package limit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
	goredis "github.com/redis/go-redis/v9"
)

func TestDenyCacheExpires(t *testing.T) {
	cache := NewDenyCache()
	now := time.Unix(100, 0)
	cache.Set("key", "blocked", now.Add(time.Second))
	if reason, ok := cache.Get("key", now); !ok || reason != "blocked" {
		t.Fatalf("cache hit = %v reason = %q", ok, reason)
	}
	if _, ok := cache.Get("key", now.Add(2*time.Second)); ok {
		t.Fatal("cache hit after expiry")
	}
}

func TestOperationsIncludeAllP1LimitTypes(t *testing.T) {
	enforcer := NewRedisEnforcer(nil, Config{Enabled: true, KeyPrefix: "test"})
	now := time.Unix(100, 0).UTC()
	state := &engine.RequestState{
		TenantID:              "tenant_1",
		ProjectID:             "project_1",
		APIKeyID:              "key_1",
		RequestedModel:        "gpt-4o-mini",
		EstimatedChargeMicros: 42,
		EstimatedUsage:        tokenusage.Estimate{InputTokens: 10, OutputTokens: 20},
		RoutePlan: &engine.RoutePlan{Candidates: []engine.ProviderCandidate{{
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
		}}},
	}
	rules := []engine.LimitRuleView{{
		ID: "rule_1",
		Scope: engine.LimitScope{
			TenantID:     "tenant_1",
			ProjectID:    "project_1",
			APIKeyID:     "key_1",
			PublicModel:  "gpt-4o-mini",
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
		},
		RPM:                 60,
		QPS:                 1,
		TPM:                 1000,
		Concurrency:         5,
		DailyBudgetMicros:   100000,
		CostPerMinuteMicros: 1000,
		Enabled:             true,
	}}

	fixed, leases := enforcer.operationsFor(state, rules, now)
	if len(fixed) != 5 {
		t.Fatalf("fixed operations = %d, want 5", len(fixed))
	}
	if len(leases) != 1 {
		t.Fatalf("lease operations = %d, want 1", len(leases))
	}
	if fixed[2].cost != 30 {
		t.Fatalf("tpm cost = %d", fixed[2].cost)
	}
	if fixed[3].cost != 42 || fixed[4].cost != 42 {
		t.Fatalf("budget costs = %d/%d", fixed[3].cost, fixed[4].cost)
	}
}

func TestAcquireUsesLocalDenyCacheBeforeRedis(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	enforcer := NewRedisEnforcer(client, Config{Enabled: true, QPS: 1, DenyCacheTTL: time.Second})
	state := &engine.RequestState{RequestID: "req_1", TenantID: "tenant_1", ProjectID: "project_1"}
	rules := enforcer.rulesFor(state)
	fixed, _ := enforcer.operationsFor(state, rules, time.Now().UTC())
	if len(fixed) == 0 {
		t.Fatal("missing fixed operation")
	}
	enforcer.deny.Set(fixed[0].cacheKey, "cached deny", time.Now().UTC().Add(time.Second))

	if _, err := enforcer.Acquire(context.Background(), state); err == nil || !strings.Contains(err.Error(), "cached deny") {
		t.Fatalf("Acquire() error = %v", err)
	}
}

func TestParseLimitResult(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	result := parseLimitResult([]any{int64(0), "qps limit exceeded", "cache-key", "1", "0", "101000"}, now)
	if result.allowed || result.reason != "qps limit exceeded" || result.cacheKey != "cache-key" {
		t.Fatalf("result = %#v", result)
	}
	if !result.resetAt.Equal(time.UnixMilli(101000).UTC()) {
		t.Fatalf("resetAt = %s", result.resetAt)
	}
}
