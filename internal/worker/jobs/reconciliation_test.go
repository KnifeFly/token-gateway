package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
)

func TestReconciliationJobSucceedsWithNoIssues(t *testing.T) {
	job := NewReconciliationJob(fakeReconciliationService{}, time.Second)
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestReconciliationJobFailsWhenIssuesExist(t *testing.T) {
	job := NewReconciliationJob(fakeReconciliationService{
		issues: []billing.ReconciliationIssue{{AccountID: "acct_1", Message: "mismatch"}},
	}, time.Second)
	err := job.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 issue") {
		t.Fatalf("Run() error = %v, want issue count", err)
	}
}

func TestReconciliationJobReturnsServiceError(t *testing.T) {
	errBoom := errors.New("boom")
	job := NewReconciliationJob(fakeReconciliationService{err: errBoom}, time.Second)
	if err := job.Run(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("Run() error = %v, want %v", err, errBoom)
	}
}

type fakeReconciliationService struct {
	issues []billing.ReconciliationIssue
	err    error
}

func (s fakeReconciliationService) FindIssues(context.Context) ([]billing.ReconciliationIssue, error) {
	return s.issues, s.err
}
