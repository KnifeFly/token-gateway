package service

import (
	"context"
	"testing"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestCreateAPIKeyRefreshesSnapshot(t *testing.T) {
	ctx := context.Background()
	adminService := configadmin.NewService(configadmin.NewMemoryRepository(), configadmin.NewCredentialCodec("secret"), nil)
	refresher := &countingRefresher{}
	service := New(nil, nil, adminService, reporting.NewService(reporting.NewMemoryRepository()), tasksvc.NewMemoryRepository(), nil, WithSnapshotRefresher(refresher))

	response, err := service.CreateAPIKey(ctx, portalapp.Principal{
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		APIKeyID:      "key_current",
		AllowedModels: []string{"gpt-4o-mini"},
	}, portalapp.APIKeyCreateRequest{Name: "child key"})
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
	adminService := configadmin.NewService(configadmin.NewMemoryRepository(), configadmin.NewCredentialCodec("secret"), nil)
	current, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		Name:         "current",
		PlaintextKey: "tg_current",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey(current) error = %v", err)
	}
	child, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		Name:         "child",
		PlaintextKey: "tg_child",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey(child) error = %v", err)
	}
	refresher := &countingRefresher{}
	service := New(nil, nil, adminService, reporting.NewService(reporting.NewMemoryRepository()), tasksvc.NewMemoryRepository(), nil, WithSnapshotRefresher(refresher))

	disabled, err := service.DisableAPIKey(ctx, portalapp.Principal{
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
