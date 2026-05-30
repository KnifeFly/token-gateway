package limit

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	goredis "github.com/redis/go-redis/v9"
)

// Config controls Redis-backed token bucket, budget, and concurrency limits.
type Config struct {
	Enabled             bool
	RPM                 int64
	QPS                 int64
	TPM                 int64
	Concurrency         int64
	DailyBudgetMicros   int64
	CostPerMinuteMicros int64
	Window              time.Duration
	LeaseTTL            time.Duration
	DenyCacheTTL        time.Duration
	KeyPrefix           string
}

// RedisEnforcer uses Redis as the multi-replica source of truth.
type RedisEnforcer struct {
	client *goredis.Client
	cfg    Config
	deny   *DenyCache
}

// DenyCache stores short-lived negative limit decisions.
type DenyCache struct {
	mu      sync.Mutex
	entries map[string]denyEntry
}

type denyEntry struct {
	reason    string
	expiresAt time.Time
}

type limitOp struct {
	key      string
	cacheKey string
	cost     int64
	limit    int64
	ttl      time.Duration
	reason   string
}

type limitResult struct {
	allowed  bool
	reason   string
	cacheKey string
	limit    int64
	resetAt  time.Time
}

// NewDenyCache returns an empty local negative-decision cache.
func NewDenyCache() *DenyCache {
	return &DenyCache{entries: map[string]denyEntry{}}
}

// Get reports whether key is denied at now.
func (c *DenyCache) Get(key string, now time.Time) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if !entry.expiresAt.After(now) {
		delete(c.entries, key)
		return "", false
	}
	return entry.reason, true
}

// Set records a deny decision until expiresAt.
func (c *DenyCache) Set(key, reason string, expiresAt time.Time) {
	if c == nil || key == "" || expiresAt.IsZero() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = denyEntry{reason: reason, expiresAt: expiresAt}
}

func NewRedisEnforcer(client *goredis.Client, cfg Config) *RedisEnforcer {
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 30 * time.Second
	}
	if cfg.DenyCacheTTL <= 0 {
		cfg.DenyCacheTTL = time.Second
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "token-gateway"
	}
	return &RedisEnforcer{client: client, cfg: cfg, deny: NewDenyCache()}
}

func (e *RedisEnforcer) Acquire(ctx context.Context, state *engine.RequestState) (engine.LimitRelease, error) {
	if e == nil || !e.cfg.Enabled || e.client == nil {
		return noopRelease{}, nil
	}
	now := time.Now().UTC()
	rules := e.rulesFor(state)
	if len(rules) == 0 {
		return noopRelease{}, nil
	}
	buckets, counters, leases := e.operationsFor(state, rules, now)
	if len(buckets) == 0 && len(counters) == 0 && len(leases) == 0 {
		return noopRelease{}, nil
	}
	if reason, ok := e.cachedDeny(now, buckets, counters, leases); ok {
		return nil, apperr.RateLimited(reason, apperr.WithTemporary())
	}
	result, err := e.runScript(ctx, state.RequestID, buckets, counters, leases, now)
	if err != nil {
		return nil, err
	}
	if !result.allowed {
		e.cacheDeny(result, now)
		return nil, apperr.RateLimited(result.reason, apperr.WithTemporary())
	}
	if len(leases) == 0 {
		return noopRelease{}, nil
	}
	keys := make([]string, 0, len(leases))
	for _, lease := range leases {
		keys = append(keys, lease.key)
	}
	return &redisRelease{client: e.client, keys: keys, member: state.RequestID}, nil
}

func (e *RedisEnforcer) rulesFor(state *engine.RequestState) []engine.LimitRuleView {
	scope := requestScope(state)
	if state != nil && state.Snapshot != nil {
		rules := state.Snapshot.LookupLimits(scope)
		if len(rules) > 0 {
			return rules
		}
	}
	if e.cfg.RPM <= 0 && e.cfg.QPS <= 0 && e.cfg.TPM <= 0 && e.cfg.Concurrency <= 0 && e.cfg.DailyBudgetMicros <= 0 && e.cfg.CostPerMinuteMicros <= 0 {
		return nil
	}
	return []engine.LimitRuleView{{
		ID: "config",
		Scope: engine.LimitScope{
			TenantID:  scope.TenantID,
			ProjectID: scope.ProjectID,
		},
		RPM:                 e.cfg.RPM,
		QPS:                 e.cfg.QPS,
		TPM:                 e.cfg.TPM,
		Concurrency:         e.cfg.Concurrency,
		DailyBudgetMicros:   e.cfg.DailyBudgetMicros,
		CostPerMinuteMicros: e.cfg.CostPerMinuteMicros,
		Enabled:             true,
	}}
}

func (e *RedisEnforcer) operationsFor(state *engine.RequestState, rules []engine.LimitRuleView, now time.Time) ([]limitOp, []limitOp, []limitOp) {
	var buckets []limitOp
	var counters []limitOp
	var leases []limitOp
	tokens := estimatedTokens(state)
	costMicros := estimatedCostMicros(state)
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		scope := e.ruleScope(rule)
		if rule.QPS > 0 {
			buckets = append(buckets, e.bucketOp(scope, "qps", 1, rule.QPS, time.Second))
		}
		if rule.RPM > 0 {
			buckets = append(buckets, e.bucketOp(scope, "rpm", 1, rule.RPM, time.Minute))
		}
		if rule.TPM > 0 {
			buckets = append(buckets, e.bucketOp(scope, "tpm", tokens, rule.TPM, time.Minute))
		}
		if rule.DailyBudgetMicros > 0 {
			counters = append(counters, e.counterOp(scope, "daily_budget", costMicros, rule.DailyBudgetMicros, untilNextUTC(now)))
		}
		if rule.CostPerMinuteMicros > 0 {
			buckets = append(buckets, e.bucketOp(scope, "cost_per_minute", costMicros, rule.CostPerMinuteMicros, time.Minute))
		}
		if rule.Concurrency > 0 {
			leases = append(leases, e.counterOp(scope, "concurrency", 1, rule.Concurrency, e.cfg.LeaseTTL))
		}
	}
	return buckets, counters, leases
}

func (e *RedisEnforcer) bucketOp(scope, kind string, cost, limit int64, ttl time.Duration) limitOp {
	key := strings.Join([]string{e.cfg.KeyPrefix, "bucket", scope, kind}, ":")
	return limitOp{
		key:      key,
		cacheKey: key,
		cost:     cost,
		limit:    limit,
		ttl:      ttl,
		reason:   kind + " limit exceeded",
	}
}

func (e *RedisEnforcer) counterOp(scope, kind string, cost, limit int64, ttl time.Duration) limitOp {
	key := strings.Join([]string{e.cfg.KeyPrefix, "limit", scope, kind}, ":")
	return limitOp{
		key:      key,
		cacheKey: key,
		cost:     cost,
		limit:    limit,
		ttl:      ttl,
		reason:   kind + " limit exceeded",
	}
}

func (e *RedisEnforcer) ruleScope(rule engine.LimitRuleView) string {
	parts := []string{
		nonEmpty(rule.ID, "rule"),
		nonEmpty(rule.Scope.TenantID, "*"),
		nonEmpty(rule.Scope.ProjectID, "*"),
		nonEmpty(rule.Scope.APIKeyID, "*"),
		nonEmpty(rule.Scope.PublicModel, "*"),
		nonEmpty(rule.Scope.ProviderType, "*"),
		nonEmpty(rule.Scope.ChannelID, "*"),
	}
	return strings.Join(parts, ":")
}

func (e *RedisEnforcer) runScript(ctx context.Context, requestID string, buckets, counters, leases []limitOp, now time.Time) (limitResult, error) {
	args := []any{
		now.UnixMilli(),
		requestID,
		len(buckets),
	}
	for _, op := range buckets {
		args = append(args, op.key, op.cost, op.limit, op.ttl.Milliseconds(), op.reason, op.cacheKey)
	}
	args = append(args, len(counters))
	for _, op := range counters {
		args = append(args, op.key, op.cost, op.limit, op.ttl.Milliseconds(), op.reason, op.cacheKey)
	}
	args = append(args, len(leases))
	for _, op := range leases {
		args = append(args, op.key, op.limit, op.ttl.Milliseconds(), op.reason, op.cacheKey)
	}
	raw, err := limitScript.Run(ctx, e.client, nil, args...).Result()
	if err != nil {
		return limitResult{}, err
	}
	values, ok := raw.([]any)
	if !ok {
		return limitResult{}, fmt.Errorf("unexpected limit script result %T", raw)
	}
	return parseLimitResult(values, now), nil
}

func (e *RedisEnforcer) cachedDeny(now time.Time, groups ...[]limitOp) (string, bool) {
	for _, group := range groups {
		for _, op := range group {
			if reason, ok := e.deny.Get(op.cacheKey, now); ok {
				return reason, true
			}
		}
	}
	return "", false
}

func (e *RedisEnforcer) cacheDeny(result limitResult, now time.Time) {
	expiresAt := result.resetAt
	maxExpiresAt := now.Add(e.cfg.DenyCacheTTL)
	if expiresAt.IsZero() || expiresAt.After(maxExpiresAt) {
		expiresAt = maxExpiresAt
	}
	e.deny.Set(result.cacheKey, result.reason, expiresAt)
}

type redisRelease struct {
	client *goredis.Client
	keys   []string
	member string
}

func (r *redisRelease) Release(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	for _, key := range r.keys {
		if err := r.client.ZRem(ctx, key, r.member).Err(); err != nil {
			return err
		}
	}
	return nil
}

type noopRelease struct{}

func (noopRelease) Release(context.Context) error { return nil }

func requestScope(state *engine.RequestState) engine.LimitScope {
	if state == nil {
		return engine.LimitScope{}
	}
	scope := engine.LimitScope{
		TenantID:    state.TenantID,
		ProjectID:   state.ProjectID,
		APIKeyID:    state.APIKeyID,
		PublicModel: state.RequestedModel,
	}
	if state.ResolvedModel.PublicModel != "" {
		scope.PublicModel = state.ResolvedModel.PublicModel
	}
	if state.RoutePlan != nil && len(state.RoutePlan.Candidates) > 0 {
		scope.ProviderType = state.RoutePlan.Candidates[0].ProviderType
		scope.ChannelID = state.RoutePlan.Candidates[0].ChannelID
	}
	return scope
}

func estimatedTokens(state *engine.RequestState) int64 {
	if state == nil {
		return 1
	}
	tokens := state.EstimatedUsage.InputTokens + state.EstimatedUsage.OutputTokens
	if tokens <= 0 {
		return 1
	}
	return tokens
}

func estimatedCostMicros(state *engine.RequestState) int64 {
	if state == nil || state.EstimatedChargeMicros <= 0 {
		return 1
	}
	return state.EstimatedChargeMicros
}

func untilNextUTC(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return next.Sub(now)
}

func parseLimitResult(values []any, now time.Time) limitResult {
	if len(values) == 0 {
		return limitResult{allowed: true}
	}
	allowed := int64Value(values[0]) == 1
	if allowed {
		return limitResult{allowed: true}
	}
	result := limitResult{
		allowed:  false,
		reason:   "rate limit exceeded",
		cacheKey: "",
		resetAt:  now.Add(time.Second),
	}
	if len(values) > 1 {
		result.reason = stringValue(values[1], result.reason)
	}
	if len(values) > 2 {
		result.cacheKey = stringValue(values[2], "")
	}
	if len(values) > 3 {
		result.limit = int64Value(values[3])
	}
	if len(values) > 5 {
		resetMillis := int64Value(values[5])
		if resetMillis > 0 {
			result.resetAt = time.UnixMilli(resetMillis).UTC()
		}
	}
	return result
}

func int64Value(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	default:
		return 0
	}
}

func stringValue(value any, fallback string) string {
	switch v := value.(type) {
	case string:
		if v != "" {
			return v
		}
	case []byte:
		if len(v) > 0 {
			return string(v)
		}
	}
	return fallback
}

func nonEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

var limitScript = goredis.NewScript(`
local now = tonumber(ARGV[1])
local member = ARGV[2]
local bucket_count = tonumber(ARGV[3])
local pos = 4
local buckets = {}

for i = 1, bucket_count do
  local key = ARGV[pos]
  local cost = tonumber(ARGV[pos + 1])
  local limit = tonumber(ARGV[pos + 2])
  local ttl = tonumber(ARGV[pos + 3])
  local reason = ARGV[pos + 4]
  local cache_key = ARGV[pos + 5]
  pos = pos + 6
  if cost > limit then
    return {0, reason, cache_key, tostring(limit), "0", tostring(now + ttl)}
  end
  local values = redis.call("HMGET", key, "tokens", "updated_at")
  local tokens = tonumber(values[1])
  local updated_at = tonumber(values[2])
  if tokens == nil then
    tokens = limit
    updated_at = now
  end
  if updated_at == nil then
    updated_at = now
  end
  local elapsed = now - updated_at
  if elapsed < 0 then
    elapsed = 0
  end
  if elapsed > 0 then
    local refill = (elapsed * limit) / ttl
    tokens = math.min(limit, tokens + refill)
  end
  if tokens + 0.000001 < cost then
    local needed = cost - tokens
    local reset_at = now + math.ceil((needed * ttl) / limit)
    return {0, reason, cache_key, tostring(limit), tostring(math.floor(tokens)), tostring(reset_at)}
  end
  table.insert(buckets, {key, tokens - cost, now, math.max(ttl * 2, ttl + 1000)})
end

local counter_count = tonumber(ARGV[pos])
pos = pos + 1
local counters = {}
for i = 1, counter_count do
  local key = ARGV[pos]
  local cost = tonumber(ARGV[pos + 1])
  local limit = tonumber(ARGV[pos + 2])
  local ttl = tonumber(ARGV[pos + 3])
  local reason = ARGV[pos + 4]
  local cache_key = ARGV[pos + 5]
  pos = pos + 6
  local current = tonumber(redis.call("GET", key) or "0")
  if current + cost > limit then
    local pttl = redis.call("PTTL", key)
    if pttl < 0 then
      pttl = ttl
    end
    return {0, reason, cache_key, tostring(limit), tostring(math.max(limit - current, 0)), tostring(now + pttl)}
  end
  table.insert(counters, {key, cost, ttl})
end

local lease_count = tonumber(ARGV[pos])
pos = pos + 1
local leases = {}
for i = 1, lease_count do
  local key = ARGV[pos]
  local limit = tonumber(ARGV[pos + 1])
  local ttl = tonumber(ARGV[pos + 2])
  local reason = ARGV[pos + 3]
  local cache_key = ARGV[pos + 4]
  pos = pos + 5
  redis.call("ZREMRANGEBYSCORE", key, "0", now - ttl)
  local count = redis.call("ZCARD", key)
  if count >= limit then
    return {0, reason, cache_key, tostring(limit), "0", tostring(now + ttl)}
  end
  table.insert(leases, {key, ttl})
end

for _, op in ipairs(buckets) do
  redis.call("HSET", op[1], "tokens", tostring(op[2]), "updated_at", tostring(op[3]))
  redis.call("PEXPIRE", op[1], op[4])
end

for _, op in ipairs(counters) do
  local value = redis.call("INCRBY", op[1], op[2])
  if value == op[2] then
    redis.call("PEXPIRE", op[1], op[3])
  end
end

for _, op in ipairs(leases) do
  redis.call("ZADD", op[1], now, member)
  redis.call("PEXPIRE", op[1], op[2])
end

return {1, "", "", "0", "0", "0"}
`)
