package jobs

import (
	"context"
	"time"

	"github.com/KnifeFly/token-gateway/internal/billing"
)

// FailedSettlementReplayer replays repairable settlement failures.
type FailedSettlementReplayer struct {
	service  *billing.FailedSettlementService
	interval time.Duration
	timeout  time.Duration
	limit    int
}

// NewFailedSettlementReplayer returns a worker job for settlement repair.
func NewFailedSettlementReplayer(service *billing.FailedSettlementService, interval time.Duration, limit int) *FailedSettlementReplayer {
	if interval <= 0 {
		interval = time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	return &FailedSettlementReplayer{
		service:  service,
		interval: interval,
		timeout:  30 * time.Second,
		limit:    limit,
	}
}

// Name returns the stable worker job name.
func (j *FailedSettlementReplayer) Name() string {
	return "failed_settlement_replayer"
}

// Interval returns how often the job should run.
func (j *FailedSettlementReplayer) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *FailedSettlementReplayer) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *FailedSettlementReplayer) MaxConcurrency() int {
	return 1
}

// Run replays pending settlement repair records.
func (j *FailedSettlementReplayer) Run(ctx context.Context) error {
	if j == nil || j.service == nil {
		return nil
	}
	_, err := j.service.ReplayPending(ctx, j.limit)
	return err
}
