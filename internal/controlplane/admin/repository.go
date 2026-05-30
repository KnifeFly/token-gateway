package admin

import (
	"context"
	"time"
)

const (
	// SnapshotStatusActive marks the runtime snapshot currently served by data-plane.
	SnapshotStatusActive = "active"
	// SnapshotStatusInactive marks a retained runtime snapshot that is not current.
	SnapshotStatusInactive = "inactive"
	// SnapshotStatusFailed marks a snapshot build or publish failure.
	SnapshotStatusFailed = "failed"
)

// Repository persists control-plane configuration.
type Repository interface {
	UpsertTenant(ctx context.Context, tenant Tenant) (*Tenant, error)
	UpsertProject(ctx context.Context, project Project) (*Project, error)
	CreateAPIKey(ctx context.Context, key APIKey) (*APIKey, error)
	ListAPIKeys(ctx context.Context, tenantID, projectID string) ([]APIKey, error)
	DisableAPIKey(ctx context.Context, keyID string, revokedAt *time.Time) (*APIKey, error)
	UpsertModel(ctx context.Context, model ModelConfig) (*ModelConfig, error)
	UpsertChannel(ctx context.Context, channel ChannelConfig) (*ChannelConfig, error)
	UpsertRoute(ctx context.Context, route RoutePolicyConfig) (*RoutePolicyConfig, error)
	UpsertPrice(ctx context.Context, price PriceRuleConfig) (*PriceRuleConfig, error)
	UpsertLimit(ctx context.Context, limit LimitRuleConfig) (*LimitRuleConfig, error)
	UpsertPluginBinding(ctx context.Context, binding PluginBindingConfig) (*PluginBindingConfig, error)
	UpsertModelMarketplace(ctx context.Context, config ModelMarketplaceConfig) (*ModelMarketplaceConfig, error)
	ListVisibleModels(ctx context.Context, tenantID, projectID string) ([]VisibleModel, error)
	LoadSnapshotConfig(ctx context.Context) (*SnapshotConfig, error)
	SaveSnapshot(ctx context.Context, record SnapshotRecord) (*SnapshotRecord, error)
	ActiveSnapshot(ctx context.Context) (*SnapshotRecord, bool, error)
	PreviousSnapshot(ctx context.Context) (*SnapshotRecord, bool, error)
	ActivateSnapshot(ctx context.Context, version string) (*SnapshotRecord, error)
}
