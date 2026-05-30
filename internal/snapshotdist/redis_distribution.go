package snapshotdist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	goredis "github.com/redis/go-redis/v9"
)

// RedisDistribution stores and announces the active runtime snapshot.
type RedisDistribution struct {
	client *goredis.Client
	prefix string
}

// NewRedisDistribution returns a Redis-backed snapshot distribution store.
func NewRedisDistribution(client *goredis.Client, prefix string) *RedisDistribution {
	if prefix == "" {
		prefix = "token-gateway"
	}
	return &RedisDistribution{client: client, prefix: prefix}
}

// PublishActiveRuntimeSnapshot writes the durable active snapshot key and sends a pubsub event.
func (s *RedisDistribution) PublishActiveRuntimeSnapshot(ctx context.Context, runtime cpsnapshot.RuntimeSnapshot) error {
	if s == nil || s.client == nil {
		return nil
	}
	envelope := snapshotEnvelope{
		Version:     runtime.Version,
		Checksum:    runtime.Checksum,
		CreatedAt:   runtime.CreatedAt,
		PublishedAt: time.Now().UTC(),
		Snapshot:    runtime,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	event, err := json.Marshal(snapshotEvent{
		Version:     runtime.Version,
		Checksum:    runtime.Checksum,
		PublishedAt: envelope.PublishedAt,
	})
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.activeKey(), payload, 0)
	pipe.Publish(ctx, s.eventChannel(), event)
	_, err = pipe.Exec(ctx)
	return err
}

// ActiveRuntimeSnapshot reads the Redis active snapshot key.
func (s *RedisDistribution) ActiveRuntimeSnapshot(ctx context.Context) (*cpsnapshot.RuntimeSnapshot, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	payload, err := s.client.Get(ctx, s.activeKey()).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, false, err
	}
	runtime := envelope.Snapshot
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = envelope.CreatedAt
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = envelope.PublishedAt
	}
	if runtime.Version == "" {
		runtime.Version = envelope.Version
	}
	if runtime.Checksum == "" {
		runtime.Checksum = envelope.Checksum
	}
	if envelope.Version != "" && runtime.Version != envelope.Version {
		return nil, false, fmt.Errorf("redis snapshot version mismatch: envelope=%s payload=%s", envelope.Version, runtime.Version)
	}
	if envelope.Checksum != "" && runtime.Checksum != "" && envelope.Checksum != runtime.Checksum {
		return nil, false, fmt.Errorf("redis snapshot checksum mismatch for %s", runtime.Version)
	}
	if err := cpsnapshot.Validate(runtime); err != nil {
		return nil, false, err
	}
	return &runtime, true, nil
}

// SnapshotEvents subscribes to active snapshot publication events.
func (s *RedisDistribution) SnapshotEvents(ctx context.Context) (<-chan struct{}, func() error, error) {
	if s == nil || s.client == nil {
		return nil, nil, nil
	}
	pubsub := s.client.Subscribe(ctx, s.eventChannel())
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}
	events := make(chan struct{}, 1)
	go func() {
		defer close(events)
		for message := range pubsub.Channel() {
			if strings.TrimSpace(message.Payload) == "" {
				continue
			}
			select {
			case events <- struct{}{}:
			default:
			}
		}
	}()
	return events, pubsub.Close, nil
}

func (s *RedisDistribution) activeKey() string {
	return s.prefix + ":snapshot:active"
}

func (s *RedisDistribution) eventChannel() string {
	return s.prefix + ":snapshot:events"
}

type snapshotEnvelope struct {
	Version     string                     `json:"version"`
	Checksum    string                     `json:"checksum"`
	CreatedAt   time.Time                  `json:"created_at"`
	PublishedAt time.Time                  `json:"published_at"`
	Snapshot    cpsnapshot.RuntimeSnapshot `json:"snapshot"`
}

type snapshotEvent struct {
	Version     string    `json:"version"`
	Checksum    string    `json:"checksum"`
	PublishedAt time.Time `json:"published_at"`
}
