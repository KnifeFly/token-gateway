package egressguard

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestGuardRejectsUnsafeURLTargets(t *testing.T) {
	guard, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tests := []string{
		"file:///tmp/input.png",
		"http://127.0.0.1/callback",
		"http://10.0.0.2/callback",
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := guard.ValidateURL(context.Background(), raw); err == nil {
				t.Fatal("ValidateURL() error = nil, want reject")
			}
		})
	}
}

func TestGuardAllowsPublicAddressAndExplicitCIDR(t *testing.T) {
	guard, err := New(Config{AllowedCIDRs: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := guard.ValidateURL(context.Background(), "https://1.1.1.1/callback"); err != nil {
		t.Fatalf("ValidateURL(public) error = %v", err)
	}
	if err := guard.ValidateURL(context.Background(), "https://10.0.0.2/callback"); err != nil {
		t.Fatalf("ValidateURL(allowed private) error = %v", err)
	}
}

func TestGuardRejectsDNSRebindingCandidate(t *testing.T) {
	guard, err := New(Config{Resolver: fakeResolver{ips: []string{"8.8.8.8", "127.0.0.1"}}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = guard.ValidateURL(context.Background(), "https://hooks.example/callback")
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("ValidateURL() error = %v, want unsafe resolved address", err)
	}
}

type fakeResolver struct {
	ips []string
}

func (r fakeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	out := make([]net.IPAddr, 0, len(r.ips))
	for _, value := range r.ips {
		out = append(out, net.IPAddr{IP: net.ParseIP(value)})
	}
	return out, nil
}
