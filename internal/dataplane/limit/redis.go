package limit

import (
	"context"
	"fmt"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
	goredis "github.com/redis/go-redis/v9"
)

// Config controls Redis-backed fixed-window token and concurrency limits.
type Config struct {
	Enabled     bool
	QPS         int64
	TPM         int64
	Concurrency int64
	Window      time.Duration
	LeaseTTL    time.Duration
	KeyPrefix   string
}

// RedisEnforcer uses Redis as the multi-replica source of truth.
type RedisEnforcer struct {
	client *goredis.Client
	cfg    Config
}

func NewRedisEnforcer(client *goredis.Client, cfg Config) *RedisEnforcer {
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 30 * time.Second
	}
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "token-gateway"
	}
	return &RedisEnforcer{client: client, cfg: cfg}
}

func (e *RedisEnforcer) Acquire(ctx context.Context, state *engine.RequestState) (engine.LimitRelease, error) {
	if e == nil || !e.cfg.Enabled || e.client == nil {
		return noopRelease{}, nil
	}
	scope := fmt.Sprintf("%s:%s:%s", e.cfg.KeyPrefix, state.TenantID, state.ProjectID)
	if e.cfg.QPS > 0 {
		if err := e.consumeFixedWindow(ctx, scope+":qps", 1, e.cfg.QPS, time.Second); err != nil {
			return nil, err
		}
	}
	if e.cfg.TPM > 0 {
		tokens := state.EstimatedUsage.InputTokens + state.EstimatedUsage.OutputTokens
		if tokens <= 0 {
			tokens = 1
		}
		if err := e.consumeFixedWindow(ctx, scope+":tpm", tokens, e.cfg.TPM, time.Minute); err != nil {
			return nil, err
		}
	}
	if e.cfg.Concurrency <= 0 {
		return noopRelease{}, nil
	}
	key := scope + ":concurrency"
	now := time.Now().UnixMilli()
	expireBefore := now - e.cfg.LeaseTTL.Milliseconds()
	member := state.RequestID
	acquired, err := concurrencyScript.Run(ctx, e.client, []string{key}, expireBefore, e.cfg.Concurrency, now, member, int64(e.cfg.LeaseTTL/time.Millisecond)).Int()
	if err != nil {
		return nil, err
	}
	if acquired != 1 {
		return nil, apperr.RateLimited("concurrency limit exceeded", apperr.WithTemporary())
	}
	return &redisRelease{client: e.client, key: key, member: member}, nil
}

func (e *RedisEnforcer) consumeFixedWindow(ctx context.Context, key string, cost int64, limit int64, window time.Duration) error {
	value, err := e.client.IncrBy(ctx, key, cost).Result()
	if err != nil {
		return err
	}
	if value == cost {
		_ = e.client.Expire(ctx, key, window).Err()
	}
	if value > limit {
		return apperr.RateLimited("rate limit exceeded", apperr.WithTemporary())
	}
	return nil
}

type redisRelease struct {
	client *goredis.Client
	key    string
	member string
}

func (r *redisRelease) Release(ctx context.Context) error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.ZRem(ctx, r.key, r.member).Err()
}

type noopRelease struct{}

func (noopRelease) Release(context.Context) error { return nil }

var concurrencyScript = goredis.NewScript(`
redis.call("ZREMRANGEBYSCORE", KEYS[1], "0", ARGV[1])
local count = redis.call("ZCARD", KEYS[1])
if count >= tonumber(ARGV[2]) then
  return 0
end
redis.call("ZADD", KEYS[1], ARGV[3], ARGV[4])
redis.call("PEXPIRE", KEYS[1], ARGV[5])
return 1
`)
