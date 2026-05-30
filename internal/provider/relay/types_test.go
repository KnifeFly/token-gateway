package relay

import (
	"context"
	"net/http"
	"testing"
)

func TestErrorFromStatusUsesStableProviderClasses(t *testing.T) {
	tests := []struct {
		status    int
		wantCode  string
		retryable bool
	}{
		{status: http.StatusBadRequest, wantCode: "provider_request_invalid"},
		{status: http.StatusUnauthorized, wantCode: "provider_auth_failed"},
		{status: http.StatusForbidden, wantCode: "provider_auth_failed"},
		{status: http.StatusNotFound, wantCode: "provider_not_found"},
		{status: http.StatusTooManyRequests, wantCode: "provider_rate_limited", retryable: true},
		{status: http.StatusServiceUnavailable, wantCode: "provider_unavailable", retryable: true},
	}
	for _, tt := range tests {
		t.Run(tt.wantCode, func(t *testing.T) {
			err := ErrorFromStatus(tt.status, []byte(`{"error":{"code":"upstream_code","message":"secret prompt"}}`))
			if err.Code != tt.wantCode || err.Retryable != tt.retryable {
				t.Fatalf("error = %#v", err)
			}
			if err.ProviderCode != "upstream_code" {
				t.Fatalf("provider code = %q", err.ProviderCode)
			}
			if got := err.Error(); got == "" || got == "secret prompt" {
				t.Fatalf("unsafe message = %q", got)
			}
		})
	}
}

func TestErrorFromRequestFailureMapsTimeout(t *testing.T) {
	err := ErrorFromRequestFailure(context.DeadlineExceeded)
	if err.Code != "provider_timeout" || !err.Retryable {
		t.Fatalf("error = %#v", err)
	}
}
