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

func TestTrustedProxyClientIPParsing(t *testing.T) {
	tests := []struct {
		name       string
		trusted    []string
		remoteAddr string
		xff        string
		xRealIP    string
		want       string
	}{
		{
			name:       "direct ignores spoofed forwarded header",
			remoteAddr: "203.0.113.7:443",
			xff:        "198.51.100.9",
			want:       "203.0.113.7",
		},
		{
			name:       "trusted proxy uses forwarded for",
			trusted:    []string{"10.0.0.0/8"},
			remoteAddr: "10.1.2.3:443",
			xff:        "198.51.100.9, 10.1.2.3",
			want:       "198.51.100.9",
		},
		{
			name:       "untrusted proxy ignores forwarded for",
			trusted:    []string{"10.0.0.0/8"},
			remoteAddr: "203.0.113.7:443",
			xff:        "198.51.100.9",
			want:       "203.0.113.7",
		},
		{
			name:       "trusted ipv6 proxy uses real ip",
			trusted:    []string{"2001:db8::/32"},
			remoteAddr: "[2001:db8::1]:443",
			xRealIP:    "2001:db8:abcd::2",
			want:       "2001:db8:abcd::2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &capturingGateway{}
			handler := NewHandlerWithRoutesConfig(
				func(context.Context) []DependencyStatus { return nil },
				prometheus.NewRegistry(),
				nil,
				HandlerConfig{TrustedProxyCIDRs: tt.trusted},
				nil,
				gateway,
			)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", io.NopCloser(nil))
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status = %d", res.Code)
			}
			if gateway.remoteAddr != tt.want {
				t.Fatalf("remote addr = %q, want %q", gateway.remoteAddr, tt.want)
			}
		})
	}
}

type fakeGateway struct{}
type capturingGateway struct {
	remoteAddr string
}

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

func (g *capturingGateway) Handle(_ context.Context, request engine.IncomingRequest) (*engine.GatewayResponse, error) {
	g.remoteAddr = request.RemoteAddr
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
