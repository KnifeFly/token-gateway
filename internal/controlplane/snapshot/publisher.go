package snapshot

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
)

// Publisher validates and publishes runtime snapshots.
type Publisher struct {
	repo    ConfigRepository
	builder *Builder
}

// NewPublisher returns a snapshot publisher.
func NewPublisher(repo ConfigRepository, builder *Builder) *Publisher {
	if builder == nil {
		builder = NewBuilder(repo)
	}
	return &Publisher{repo: repo, builder: builder}
}

// ActiveProvider adapts a ConfigRepository to the data-plane watcher interface.
type ActiveProvider struct {
	repo ConfigRepository
}

// NewActiveProvider returns an active snapshot provider.
func NewActiveProvider(repo ConfigRepository) *ActiveProvider {
	return &ActiveProvider{repo: repo}
}

// ActiveRuntimeSnapshot loads the active runtime snapshot payload.
func (p *ActiveProvider) ActiveRuntimeSnapshot(ctx context.Context) (*RuntimeSnapshot, bool, error) {
	return ActiveRuntimeSnapshot(ctx, p.repo)
}

// Publish builds, validates, stores, and activates a snapshot.
func (p *Publisher) Publish(ctx context.Context) (*RuntimeSnapshot, error) {
	runtime, err := p.builder.Build(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(runtime)
	if err != nil {
		return nil, err
	}
	record := admin.SnapshotRecord{
		Version:   runtime.Version,
		Checksum:  runtime.Checksum,
		Status:    admin.SnapshotStatusInactive,
		Payload:   payload,
		CreatedAt: runtime.CreatedAt,
	}
	if _, err := p.repo.SaveSnapshot(ctx, record); err != nil {
		return nil, err
	}
	if _, err := p.repo.ActivateSnapshot(ctx, runtime.Version); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Rollback activates the previous active snapshot.
func (p *Publisher) Rollback(ctx context.Context) (*RuntimeSnapshot, error) {
	previous, ok, err := p.repo.PreviousSnapshot(ctx)
	if err != nil || !ok {
		return nil, err
	}
	if _, err := p.repo.ActivateSnapshot(ctx, previous.Version); err != nil {
		return nil, err
	}
	var runtime RuntimeSnapshot
	if err := json.Unmarshal(previous.Payload, &runtime); err != nil {
		return nil, err
	}
	return &runtime, nil
}

// ActiveRuntimeSnapshot loads the active runtime snapshot payload.
func ActiveRuntimeSnapshot(ctx context.Context, repo ConfigRepository) (*RuntimeSnapshot, bool, error) {
	record, ok, err := repo.ActiveSnapshot(ctx)
	if err != nil || !ok {
		return nil, false, err
	}
	var runtime RuntimeSnapshot
	if err := json.Unmarshal(record.Payload, &runtime); err != nil {
		return nil, false, err
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = record.CreatedAt
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = time.Now().UTC()
	}
	return &runtime, true, nil
}
