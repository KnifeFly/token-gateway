package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
)

func TestBalanceHoldReaperReleasesExpiredHolds(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	repo := billing.NewMemoryRepository()
	if err := repo.EnsureBalanceAccount(ctx, billing.BalanceAccount{
		ID:              "acct_1",
		TenantID:        "tenant_1",
		ProjectID:       "project_1",
		Currency:        "USD",
		AvailableMicros: 1000,
	}); err != nil {
		t.Fatalf("EnsureBalanceAccount() error = %v", err)
	}
	if _, err := repo.CreateHold(ctx, billing.HoldRequest{
		RequestID:    "req_expired",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 100,
		ExpiresAt:    now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("CreateHold(expired) error = %v", err)
	}
	if _, err := repo.CreateHold(ctx, billing.HoldRequest{
		RequestID:    "req_active",
		TenantID:     "tenant_1",
		ProjectID:    "project_1",
		APIKeyID:     "key_1",
		Currency:     "USD",
		AmountMicros: 100,
		ExpiresAt:    now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("CreateHold(active) error = %v", err)
	}
	job := NewBalanceHoldReaper(billing.NewBalanceService(repo), time.Second, 10)
	job.now = func() time.Time { return now }
	if err := job.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	expired, _, err := repo.GetHoldByRequestID(ctx, "req_expired")
	if err != nil {
		t.Fatalf("GetHoldByRequestID(expired) error = %v", err)
	}
	if expired.Status != billing.HoldStatusReleased {
		t.Fatalf("expired hold status = %q, want %q", expired.Status, billing.HoldStatusReleased)
	}
	active, _, err := repo.GetHoldByRequestID(ctx, "req_active")
	if err != nil {
		t.Fatalf("GetHoldByRequestID(active) error = %v", err)
	}
	if active.Status != billing.HoldStatusActive {
		t.Fatalf("active hold status = %q, want %q", active.Status, billing.HoldStatusActive)
	}
}

func TestBalanceHoldReaperReturnsServiceError(t *testing.T) {
	errBoom := errors.New("boom")
	job := NewBalanceHoldReaper(failingHoldReleaser{err: errBoom}, time.Second, 1)
	if err := job.Run(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("Run() error = %v, want %v", err, errBoom)
	}
}

type failingHoldReleaser struct {
	err error
}

func (s failingHoldReleaser) ReleaseExpiredHolds(context.Context, time.Time, int) (int, error) {
	return 0, s.err
}
