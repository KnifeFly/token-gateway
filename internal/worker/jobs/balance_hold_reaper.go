package jobs

import (
	"context"
	"time"
)

type expiredHoldReleaser interface {
	ReleaseExpiredHolds(ctx context.Context, now time.Time, limit int) (int, error)
}

// BalanceHoldReaper releases expired active balance holds.
type BalanceHoldReaper struct {
	service  expiredHoldReleaser
	now      func() time.Time
	interval time.Duration
	timeout  time.Duration
	limit    int
}

// NewBalanceHoldReaper returns a worker job for stale balance reservations.
func NewBalanceHoldReaper(service expiredHoldReleaser, interval time.Duration, limit int) *BalanceHoldReaper {
	if interval <= 0 {
		interval = time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	return &BalanceHoldReaper{
		service:  service,
		now:      time.Now,
		interval: interval,
		timeout:  30 * time.Second,
		limit:    limit,
	}
}

// Name returns the stable worker job name.
func (j *BalanceHoldReaper) Name() string {
	return "balance_hold_reaper"
}

// Interval returns how often the job should run.
func (j *BalanceHoldReaper) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *BalanceHoldReaper) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *BalanceHoldReaper) MaxConcurrency() int {
	return 1
}

// Run releases expired holds in bounded batches.
func (j *BalanceHoldReaper) Run(ctx context.Context) error {
	if j == nil || j.service == nil {
		return nil
	}
	now := time.Now().UTC()
	if j.now != nil {
		now = j.now().UTC()
	}
	_, err := j.service.ReleaseExpiredHolds(ctx, now, j.limit)
	return err
}
