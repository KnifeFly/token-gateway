package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/internal/provider/relay"
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

func TestExtensionRouteRegistered(t *testing.T) {
	handler := NewHandlerWithRoutes(
		func(context.Context) []DependencyStatus { return nil },
		prometheus.NewRegistry(),
		nil,
		[]RouteRegistrar{fakeRoute{}},
	)
	req := httptest.NewRequest(http.MethodGet, "/extension", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("status = %d", res.Code)
	}
}

func TestChatCompletionsRouteDelegatesGateway(t *testing.T) {
	handler := NewHandler(func(context.Context) []DependencyStatus { return nil }, prometheus.NewRegistry(), nil, fakeGateway{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(nil))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if res.Body.String() != `{"id":"ok"}` {
		t.Fatalf("body = %q", res.Body.String())
	}
}

func TestStreamRouteWritesSSE(t *testing.T) {
	handler := NewHandler(func(context.Context) []DependencyStatus { return nil }, prometheus.NewRegistry(), nil, fakeStreamGateway{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(nil))
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	if !strings.Contains(res.Body.String(), "data: hello") {
		t.Fatalf("body = %q", res.Body.String())
	}
}

type fakeGateway struct{}

type fakeRoute struct{}

func (fakeRoute) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /extension", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
}

func (fakeGateway) Handle(context.Context, engine.IncomingRequest) (*engine.GatewayResponse, error) {
	return &engine.GatewayResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"ok"}`),
	}, nil
}

type fakeStreamGateway struct{}

func (fakeStreamGateway) Handle(context.Context, engine.IncomingRequest) (*engine.GatewayResponse, error) {
	return &engine.GatewayResponse{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Stream: &relay.StaticStream{
			Chunks: [][]byte{[]byte("data: hello\n\n")},
		},
	}, nil
}
