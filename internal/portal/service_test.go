package portal

import (
	"context"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestCreateAPIKeyRefreshesSnapshot(t *testing.T) {
	ctx := context.Background()
	adminService := admin.NewService(admin.NewMemoryRepository(), admin.NewCredentialCodec("secret"), nil)
	refresher := &countingRefresher{}
	service := NewService(adminService, reporting.NewService(reporting.NewMemoryRepository()), tasksvc.NewMemoryRepository(), WithSnapshotRefresher(refresher))

	response, err := service.CreateAPIKey(ctx, Principal{
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		APIKeyID:      "key_current",
		AllowedModels: []string{"gpt-4o-mini"},
	}, APIKeyCreateRequest{Name: "child key"})
	if err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	if response.PlaintextKey == "" || response.APIKey.ID == "" {
		t.Fatalf("response = %#v", response)
	}
	if refresher.count != 1 {
		t.Fatalf("refresh count = %d, want 1", refresher.count)
	}
}

func TestDisableAPIKeyRefreshesSnapshot(t *testing.T) {
	ctx := context.Background()
	adminService := admin.NewService(admin.NewMemoryRepository(), admin.NewCredentialCodec("secret"), nil)
	current, err := adminService.CreateAPIKey(ctx, admin.APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		Name:         "current",
		PlaintextKey: "tg_current",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey(current) error = %v", err)
	}
	child, err := adminService.CreateAPIKey(ctx, admin.APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		Name:         "child",
		PlaintextKey: "tg_child",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey(child) error = %v", err)
	}
	refresher := &countingRefresher{}
	service := NewService(adminService, reporting.NewService(reporting.NewMemoryRepository()), tasksvc.NewMemoryRepository(), WithSnapshotRefresher(refresher))

	disabled, err := service.DisableAPIKey(ctx, Principal{
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		APIKeyID:  current.ID,
	}, child.ID)
	if err != nil {
		t.Fatalf("DisableAPIKey() error = %v", err)
	}
	if disabled.Enabled {
		t.Fatalf("disabled key = %#v", disabled)
	}
	if refresher.count != 1 {
		t.Fatalf("refresh count = %d, want 1", refresher.count)
	}
}

type countingRefresher struct {
	count int
}

func (r *countingRefresher) RefreshSnapshot(context.Context) error {
	r.count++
	return nil
}
