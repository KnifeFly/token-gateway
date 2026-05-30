package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultLeaseTTL = 30 * time.Second

// LeaseStore coordinates worker jobs across process replicas.
type LeaseStore interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (Lease, bool, error)
}

// Lease is a held execution lease.
type Lease interface {
	Release(ctx context.Context) error
}

// NoopLeaseStore grants every lease. It is only appropriate when no shared
// dependency is configured.
type NoopLeaseStore struct{}

// Acquire grants a no-op lease.
func (NoopLeaseStore) Acquire(context.Context, string, time.Duration) (Lease, bool, error) {
	return noopLease{}, true, nil
}

type noopLease struct{}

func (noopLease) Release(context.Context) error { return nil }

// MemoryLeaseStore is a deterministic lease store for tests.
type MemoryLeaseStore struct {
	mu     sync.Mutex
	leases map[string]time.Time
	now    func() time.Time
}

// NewMemoryLeaseStore returns an in-memory lease store.
func NewMemoryLeaseStore() *MemoryLeaseStore {
	return &MemoryLeaseStore{leases: make(map[string]time.Time), now: time.Now}
}

// Acquire grants the lease when no unexpired local holder exists.
func (s *MemoryLeaseStore) Acquire(_ context.Context, name string, ttl time.Duration) (Lease, bool, error) {
	if ttl <= 0 {
		ttl = defaultLeaseTTL
	}
	if s == nil {
		return noopLease{}, true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if expiresAt, ok := s.leases[name]; ok && expiresAt.After(now) {
		return nil, false, nil
	}
	s.leases[name] = now.Add(ttl)
	return memoryLease{store: s, name: name}, true, nil
}

type memoryLease struct {
	store *MemoryLeaseStore
	name  string
}

func (l memoryLease) Release(context.Context) error {
	if l.store == nil {
		return nil
	}
	l.store.mu.Lock()
	defer l.store.mu.Unlock()
	delete(l.store.leases, l.name)
	return nil
}

// RedisLeaseStore stores execution leases in Redis with compare-and-delete
// release semantics.
type RedisLeaseStore struct {
	client *goredis.Client
	prefix string
	owner  string
}

// NewRedisLeaseStore returns a Redis-backed lease store.
func NewRedisLeaseStore(client *goredis.Client, prefix string) *RedisLeaseStore {
	if prefix == "" {
		prefix = "token-gateway"
	}
	return &RedisLeaseStore{client: client, prefix: prefix, owner: randomOwner()}
}

// Acquire grants the lease when Redis SET NX succeeds.
func (s *RedisLeaseStore) Acquire(ctx context.Context, name string, ttl time.Duration) (Lease, bool, error) {
	if s == nil || s.client == nil {
		return noopLease{}, true, nil
	}
	if ttl <= 0 {
		ttl = defaultLeaseTTL
	}
	key := s.key(name)
	ok, err := s.client.SetNX(ctx, key, s.owner, ttl).Result()
	if err != nil || !ok {
		return nil, ok, err
	}
	return redisLease{client: s.client, key: key, owner: s.owner}, true, nil
}

func (s *RedisLeaseStore) key(name string) string {
	return s.prefix + ":worker:lease:" + name
}

type redisLease struct {
	client *goredis.Client
	key    string
	owner  string
}

func (l redisLease) Release(ctx context.Context) error {
	if l.client == nil {
		return nil
	}
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`
	return l.client.Eval(ctx, script, []string{l.key}, l.owner).Err()
}

func randomOwner() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
