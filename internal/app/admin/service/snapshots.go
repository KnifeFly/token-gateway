package service

import (
	"context"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// SnapshotDiagnostics returns safe active and rollback snapshot state.
func (s *Service) SnapshotDiagnostics(ctx context.Context, actor adminapp.Actor) (adminapp.SnapshotSummary, error) {
	if err := s.Authorize(actor, "read", "snapshot"); err != nil {
		return adminapp.SnapshotSummary{}, err
	}
	if s.snapshots == nil {
		return adminapp.SnapshotSummary{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
	}
	diagnostics, err := s.snapshots.Diagnostics(ctx)
	if err != nil {
		return adminapp.SnapshotSummary{}, err
	}
	return adminapp.SnapshotSummary{Active: diagnostics.Active, Previous: diagnostics.Previous}, nil
}

// ValidateSnapshot validates the current config graph enough for browser preflight.
func (s *Service) ValidateSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (map[string]any, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "validate", map[string]string{"operation": "validate"}, func() (map[string]any, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return nil, err
		}
		for _, channel := range cfg.Channels {
			if channel.APIKey != "" {
				return nil, apperr.InvalidArgument("snapshot must not contain plaintext provider credentials")
			}
		}
		return map[string]any{
			"valid":        true,
			"api_keys":     len(cfg.APIKeys),
			"models":       len(cfg.Models),
			"channels":     len(cfg.Channels),
			"routes":       len(cfg.Routes),
			"pricing":      len(cfg.Prices),
			"limits":       len(cfg.Limits),
			"generated_at": s.now(),
		}, nil
	})
}

// PublishSnapshot publishes a runtime snapshot through the snapshot owner.
func (s *Service) PublishSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (adminapp.SnapshotOperationResult, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "active", map[string]string{"operation": "publish"}, func() (adminapp.SnapshotOperationResult, error) {
		if s.snapshots == nil {
			return adminapp.SnapshotOperationResult{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
		}
		runtime, err := s.snapshots.Publish(ctx)
		if err != nil {
			return adminapp.SnapshotOperationResult{}, err
		}
		return safeSnapshotResult(runtime), nil
	})
}

// RollbackSnapshot rolls back to the previous runtime snapshot through the snapshot owner.
func (s *Service) RollbackSnapshot(ctx context.Context, actor adminapp.Actor, opts adminapp.MutationOptions) (adminapp.SnapshotOperationResult, error) {
	return mutate(ctx, s, actor, opts, "publish", "snapshot", "rollback", map[string]string{"operation": "rollback"}, func() (adminapp.SnapshotOperationResult, error) {
		if s.snapshots == nil {
			return adminapp.SnapshotOperationResult{}, apperr.ConfigUnavailable("snapshot manager is unavailable")
		}
		runtime, err := s.snapshots.Rollback(ctx)
		if err != nil {
			return adminapp.SnapshotOperationResult{}, err
		}
		return safeSnapshotResult(runtime), nil
	})
}

func safeSnapshotResult(runtime *cpsnapshot.RuntimeSnapshot) adminapp.SnapshotOperationResult {
	if runtime == nil {
		return adminapp.SnapshotOperationResult{}
	}
	return adminapp.SnapshotOperationResult{
		Version:       runtime.Version,
		Checksum:      runtime.Checksum,
		SchemaVersion: runtime.SchemaVersion,
		CreatedAt:     runtime.CreatedAt,
	}
}
