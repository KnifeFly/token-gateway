package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
)

type reconciliationFinder interface {
	FindIssues(ctx context.Context) ([]billing.ReconciliationIssue, error)
}

// ReconciliationJob periodically detects ledger and balance mismatches.
type ReconciliationJob struct {
	service  reconciliationFinder
	interval time.Duration
	timeout  time.Duration
}

// NewReconciliationJob returns a worker job for scheduled billing reconciliation.
func NewReconciliationJob(service reconciliationFinder, interval time.Duration) *ReconciliationJob {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &ReconciliationJob{
		service:  service,
		interval: interval,
		timeout:  2 * time.Minute,
	}
}

// Name returns the stable worker job name.
func (j *ReconciliationJob) Name() string {
	return "reconciliation"
}

// Interval returns how often the job should run.
func (j *ReconciliationJob) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *ReconciliationJob) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *ReconciliationJob) MaxConcurrency() int {
	return 1
}

// Run checks for balance/ledger drift and surfaces mismatches as job failures.
func (j *ReconciliationJob) Run(ctx context.Context) error {
	if j == nil || j.service == nil {
		return nil
	}
	issues, err := j.service.FindIssues(ctx)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return fmt.Errorf("billing reconciliation found %d issue(s)", len(issues))
	}
	return nil
}
