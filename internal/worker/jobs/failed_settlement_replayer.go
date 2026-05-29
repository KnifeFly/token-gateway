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

func (j *FailedSettlementReplayer) Name() string {
	return "failed_settlement_replayer"
}

func (j *FailedSettlementReplayer) Interval() time.Duration {
	return j.interval
}

func (j *FailedSettlementReplayer) Timeout() time.Duration {
	return j.timeout
}

func (j *FailedSettlementReplayer) MaxConcurrency() int {
	return 1
}

func (j *FailedSettlementReplayer) Run(ctx context.Context) error {
	if j == nil || j.service == nil {
		return nil
	}
	_, err := j.service.ReplayPending(ctx, j.limit)
	return err
}
