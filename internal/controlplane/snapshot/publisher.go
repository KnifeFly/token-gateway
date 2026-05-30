package snapshot

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Publisher validates and publishes runtime snapshots.
type Publisher struct {
	repo    ConfigRepository
	builder *Builder
}

// SnapshotRecordDiagnostics describes one persisted runtime snapshot without payload details.
type SnapshotRecordDiagnostics struct {
	Version    string     `json:"version,omitempty"`
	Checksum   string     `json:"checksum,omitempty"`
	Status     string     `json:"status,omitempty"`
	CreatedAt  time.Time  `json:"created_at,omitempty"`
	ActiveAt   *time.Time `json:"active_at,omitempty"`
	AgeSeconds float64    `json:"age_seconds,omitempty"`
	Valid      bool       `json:"valid"`
	Error      string     `json:"error,omitempty"`
}

// Diagnostics summarizes the active and rollback snapshot state.
type Diagnostics struct {
	Active   *SnapshotRecordDiagnostics `json:"active,omitempty"`
	Previous *SnapshotRecordDiagnostics `json:"previous,omitempty"`
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
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.NotFound("previous snapshot not found")
	}
	runtime, err := runtimeFromRecord(*previous)
	if err != nil {
		return nil, err
	}
	if _, err := p.repo.ActivateSnapshot(ctx, previous.Version); err != nil {
		return nil, err
	}
	return runtime, nil
}

// Diagnostics reports active and rollback snapshot health for configd.
func (p *Publisher) Diagnostics(ctx context.Context) (*Diagnostics, error) {
	if p == nil || p.repo == nil {
		return nil, apperr.ConfigUnavailable("snapshot repository is unavailable")
	}
	diagnostics := &Diagnostics{}
	if active, ok, err := p.repo.ActiveSnapshot(ctx); err != nil {
		return nil, err
	} else if ok {
		diagnostics.Active = recordDiagnostics(*active)
	}
	if previous, ok, err := p.repo.PreviousSnapshot(ctx); err != nil {
		return nil, err
	} else if ok {
		diagnostics.Previous = recordDiagnostics(*previous)
	}
	return diagnostics, nil
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
	if err := Validate(runtime); err != nil {
		return nil, false, err
	}
	return &runtime, true, nil
}

func runtimeFromRecord(record admin.SnapshotRecord) (*RuntimeSnapshot, error) {
	var runtime RuntimeSnapshot
	if err := json.Unmarshal(record.Payload, &runtime); err != nil {
		return nil, err
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = record.CreatedAt
	}
	if runtime.CreatedAt.IsZero() {
		runtime.CreatedAt = time.Now().UTC()
	}
	if err := Validate(runtime); err != nil {
		return nil, err
	}
	return &runtime, nil
}

func recordDiagnostics(record admin.SnapshotRecord) *SnapshotRecordDiagnostics {
	diag := &SnapshotRecordDiagnostics{
		Version:   record.Version,
		Checksum:  record.Checksum,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		ActiveAt:  record.ActiveAt,
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() && record.ActiveAt != nil {
		createdAt = *record.ActiveAt
	}
	if !createdAt.IsZero() {
		diag.AgeSeconds = time.Since(createdAt).Seconds()
	}
	if _, err := runtimeFromRecord(record); err != nil {
		diag.Valid = false
		diag.Error = err.Error()
		return diag
	}
	diag.Valid = true
	return diag
}
