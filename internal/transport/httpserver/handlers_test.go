package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestHealthz(t *testing.T) {
	handler := NewHandler(func(context.Context) []DependencyStatus { return nil }, prometheus.NewRegistry(), nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if res.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}
}

func TestReadyzUnavailable(t *testing.T) {
	handler := NewHandler(func(context.Context) []DependencyStatus {
		return []DependencyStatus{{Name: "database", Status: "unavailable", Error: "down"}}
	}, prometheus.NewRegistry(), nil)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	handler := NewHandler(func(context.Context) []DependencyStatus { return nil }, registry, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
}
