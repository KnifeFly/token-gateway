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

	buckets, counters, leases := enforcer.operationsFor(state, rules, now)
	if len(buckets) != 4 {
		t.Fatalf("bucket operations = %d, want 4", len(buckets))
	}
	if len(counters) != 1 {
		t.Fatalf("counter operations = %d, want 1", len(counters))
	}
	if len(leases) != 1 {
		t.Fatalf("lease operations = %d, want 1", len(leases))
	}
	if buckets[2].cost != 30 {
		t.Fatalf("tpm cost = %d", buckets[2].cost)
	}
	if counters[0].cost != 42 || buckets[3].cost != 42 {
		t.Fatalf("budget costs = %d/%d", counters[0].cost, buckets[3].cost)
	}
}

func TestRequestScopeSeparatesProviderCandidateScope(t *testing.T) {
	state := &engine.RequestState{
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		APIKeyID:       "key_1",
		RequestedModel: "gpt-4o-mini",
		RoutePlan: &engine.RoutePlan{Candidates: []engine.ProviderCandidate{{
			ProviderType: "openai_compatible",
			ChannelID:    "channel_1",
		}}},
	}

	scope := requestScope(state)
	if scope.ProviderType != "" || scope.ChannelID != "" {
		t.Fatalf("request scope = %#v", scope)
	}
	candidateScope := requestScopeForCandidate(state, state.RoutePlan.Candidates[0])
	if candidateScope.ProviderType != "openai_compatible" || candidateScope.ChannelID != "channel_1" {
		t.Fatalf("candidate scope = %#v", candidateScope)
	}
}

func TestProviderChannelRulesFiltersGlobalRules(t *testing.T) {
	rules := providerChannelRules([]engine.LimitRuleView{
		{ID: "global", Scope: engine.LimitScope{TenantID: "tenant_1"}},
		{ID: "provider", Scope: engine.LimitScope{TenantID: "tenant_1", ProviderType: "openai_compatible"}},
		{ID: "channel", Scope: engine.LimitScope{TenantID: "tenant_1", ChannelID: "channel_1"}},
	})
	if len(rules) != 2 || rules[0].ID != "provider" || rules[1].ID != "channel" {
		t.Fatalf("rules = %#v", rules)
	}
}

func TestAcquireUsesLocalDenyCacheBeforeRedis(t *testing.T) {
	client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"})
	t.Cleanup(func() { _ = client.Close() })
	enforcer := NewRedisEnforcer(client, Config{Enabled: true, QPS: 1, DenyCacheTTL: time.Second})
	state := &engine.RequestState{RequestID: "req_1", TenantID: "tenant_1", ProjectID: "project_1"}
	rules := enforcer.rulesFor(state)
	buckets, _, _ := enforcer.operationsFor(state, rules, time.Now().UTC())
	if len(buckets) == 0 {
		t.Fatal("missing bucket operation")
	}
	enforcer.deny.Set(buckets[0].cacheKey, "cached deny", time.Now().UTC().Add(time.Second))

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
