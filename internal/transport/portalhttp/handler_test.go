package portalhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	portalservice "github.com/KnifeFly/token-gateway/internal/app/portal/service"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/auth"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	dpsnapshot "github.com/KnifeFly/token-gateway/internal/dataplane/snapshot"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestPortalModelsCreditsAndUsage(t *testing.T) {
	handler, _ := testPortalHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/portal/models", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("models status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"gpt-public"`) || strings.Contains(rec.Body.String(), `"id":"image-public"`) {
		t.Fatalf("models body = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/models/gpt-public/schema", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"input"`) {
		t.Fatalf("schema status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/models/image-public/schema", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("forbidden model schema status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/credits?currency=CNY", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"remaining_credits":5`) || strings.Contains(rec.Body.String(), "provider_cost") {
		t.Fatalf("credits status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/usage?limit=10", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "provider_cost") || strings.Contains(rec.Body.String(), "failed_settlements") {
		t.Fatalf("usage status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPortalAPIKeyBoundaries(t *testing.T) {
	handler, _ := testPortalHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/portal/api-keys", strings.NewReader(`{"name":"bad","allowed_models":["image-public"]}`))
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expanded key status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/portal/api-keys", strings.NewReader(`{"name":"derived","allowed_models":["gpt-public"]}`))
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"plaintext_key"`) || strings.Contains(rec.Body.String(), "key_hash") {
		t.Fatalf("create key status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		APIKey struct {
			ID string `json:"id"`
		} `json:"api_key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created key: %v", err)
	}
	if created.APIKey.ID == "" {
		t.Fatalf("created key missing id: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/api-keys", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "plaintext_key") || strings.Contains(rec.Body.String(), "key_hash") {
		t.Fatalf("list keys status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/portal/api-keys/key_current/disable", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("self disable status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/portal/api-keys/key_other_project/disable", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross project disable status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/portal/api-keys/"+created.APIKey.ID+"/disable", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("disable derived status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPortalTaskBoundaries(t *testing.T) {
	handler, ids := testPortalHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/portal/tasks", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list tasks status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), ids.ownTaskID) || strings.Contains(rec.Body.String(), ids.otherTaskID) || strings.Contains(rec.Body.String(), "secret-token") {
		t.Fatalf("list tasks body = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/tasks/"+ids.ownTaskID, nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ids.ownTaskID) || strings.Contains(rec.Body.String(), "secret-token") {
		t.Fatalf("get task status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/portal/tasks/"+ids.otherTaskID, nil)
	req.Header.Set("Authorization", "Bearer test-key")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross task status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type portalTestIDs struct {
	ownTaskID   string
	otherTaskID string
}

func testPortalHandler(t *testing.T) (http.Handler, portalTestIDs) {
	t.Helper()
	ctx := context.Background()
	adminRepo := configadmin.NewMemoryRepository()
	adminService := configadmin.NewService(adminRepo, nil, nil)
	if _, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		ID:            "key_current",
		TenantID:      "tenant_1",
		ProjectID:     "project_1",
		Name:          "primary",
		PlaintextKey:  "test-key",
		AllowedModels: []string{"gpt-public"},
	}); err != nil {
		t.Fatalf("CreateAPIKey(current) error = %v", err)
	}
	if _, err := adminService.CreateAPIKey(ctx, configadmin.APIKey{
		ID:            "key_other_project",
		TenantID:      "tenant_1",
		ProjectID:     "project_2",
		Name:          "other",
		PlaintextKey:  "other-key",
		AllowedModels: []string{"gpt-public"},
	}); err != nil {
		t.Fatalf("CreateAPIKey(other) error = %v", err)
	}

	reportRepo := reporting.NewMemoryRepository()
	reportService := reporting.NewService(reportRepo)
	if _, err := reportService.CreateManualAdjustment(ctx, reporting.ManualAdjustmentRequest{
		IdempotencyKey: "seed",
		TenantID:       "tenant_1",
		ProjectID:      "project_1",
		Currency:       "CNY",
		AmountMicros:   5_000_000,
		Reason:         "seed",
		OperatorID:     "test",
	}); err != nil {
		t.Fatalf("CreateManualAdjustment() error = %v", err)
	}

	taskRepo := tasksvc.NewMemoryRepository()
	taskService := tasksvc.NewService(taskRepo, 0)
	ownTask, _, err := taskService.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:  "tenant_1",
		ProjectID: "project_1",
		APIKeyID:  "key_current",
		RequestID: "req_1",
		Endpoint:  "/v1/images/generations",
		Kind:      tasksvc.KindImageGeneration,
		MediaType: "image",
		Model:     "gpt-public",
		Input:     []byte(`{"model":"gpt-public","prompt":"hi"}`),
		Metadata:  map[string]string{"workflow": "wf_1", "api_secret": "secret-token"},
	})
	if err != nil {
		t.Fatalf("CreateMediaTask(own) error = %v", err)
	}
	otherTask, _, err := taskService.CreateMediaTask(ctx, tasksvc.CreateTaskRequest{
		TenantID:  "tenant_2",
		ProjectID: "project_2",
		APIKeyID:  "other",
		RequestID: "req_2",
		Endpoint:  "/v1/images/generations",
		Kind:      tasksvc.KindImageGeneration,
		MediaType: "image",
		Model:     "gpt-public",
		Input:     []byte(`{"model":"gpt-public","prompt":"hi"}`),
	})
	if err != nil {
		t.Fatalf("CreateMediaTask(other) error = %v", err)
	}

	indexed, err := dpsnapshot.Build(cpsnapshot.RuntimeSnapshot{
		Version:   "v1",
		CreatedAt: time.Now().UTC(),
		APIKeys: []cpsnapshot.APIKeyRuntime{{
			ID:            "key_current",
			TenantID:      "tenant_1",
			ProjectID:     "project_1",
			KeyHash:       auth.HashAPIKey("test-key"),
			AllowedModels: []string{"gpt-public"},
			Enabled:       true,
		}},
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-public",
			DisplayName: "GPT Public",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "text",
			Schema:      json.RawMessage(`{"type":"object","required":["model","input"]}`),
			Enabled:     true,
		}, {
			PublicModel: "image-public",
			DisplayName: "Image Public",
			Protocol:    string(engine.ProtocolUnified),
			Capability:  "image",
			Schema:      json.RawMessage(`{"type":"object"}`),
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	registrar := NewHandler(
		dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)),
		auth.NewSnapshotAuthenticator(),
		portalservice.New(dpsnapshot.NewProvider(dpsnapshot.NewStore(indexed)), auth.NewSnapshotAuthenticator(), adminService, reportService, taskRepo, nil),
		nil,
	)
	mux := http.NewServeMux()
	registrar.Register(mux)
	return mux, portalTestIDs{ownTaskID: ownTask.ID, otherTaskID: otherTask.ID}
}
