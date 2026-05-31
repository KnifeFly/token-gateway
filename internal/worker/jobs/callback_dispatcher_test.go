package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestCallbackDispatcherSignsDeliveredPayload(t *testing.T) {
	ctx := context.Background()
	repo := tasksvc.NewMemoryRepository()
	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	event := tasksvc.CallbackEvent{
		ID:          "cb_1",
		TaskID:      "task_1",
		TenantID:    "tenant_1",
		ProjectID:   "project_1",
		URL:         server.URL,
		Payload:     []byte(`{"id":"task_1"}`),
		Status:      tasksvc.CallbackStatusPending,
		NextRetryAt: time.Now().Add(-time.Second),
	}
	if err := repo.EnqueueCallback(ctx, event); err != nil {
		t.Fatalf("EnqueueCallback() error = %v", err)
	}
	dispatcher := NewCallbackDispatcherWithMetrics(
		repo,
		server.Client(),
		nil,
		time.Second,
		10,
		WithCallbackSigningSecret("secret"),
		WithCallbackMaxRetries(2),
	)
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	header := <-received
	if header.Get("X-Gateway-Task-ID") != "task_1" || header.Get("X-Gateway-Callback-ID") != "cb_1" {
		t.Fatalf("headers = %#v", header)
	}
	if header.Get("X-Gateway-Callback-Timestamp") == "" || !strings.HasPrefix(header.Get("X-Gateway-Callback-Signature"), "sha256=") {
		t.Fatalf("signature headers = %#v", header)
	}
	due, err := repo.ListDueCallbacks(ctx, 10, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListDueCallbacks() error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due callbacks = %#v", due)
	}
}

func TestCallbackDispatcherDeadLettersAtRetryCeiling(t *testing.T) {
	ctx := context.Background()
	repo := tasksvc.NewMemoryRepository()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	if err := repo.EnqueueCallback(ctx, tasksvc.CallbackEvent{
		ID:          "cb_1",
		TaskID:      "task_1",
		TenantID:    "tenant_1",
		ProjectID:   "project_1",
		URL:         server.URL,
		Payload:     []byte(`{"id":"task_1"}`),
		Status:      tasksvc.CallbackStatusPending,
		RetryCount:  1,
		NextRetryAt: time.Now().Add(-time.Second),
	}); err != nil {
		t.Fatalf("EnqueueCallback() error = %v", err)
	}
	dispatcher := NewCallbackDispatcherWithMetrics(repo, server.Client(), nil, time.Second, 10, WithCallbackMaxRetries(2))
	if err := dispatcher.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	due, err := repo.ListDueCallbacks(ctx, 10, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("ListDueCallbacks() error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("dead-lettered callback is still due: %#v", due)
	}
}
