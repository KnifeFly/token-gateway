package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Runner schedules repeatable jobs with shared leases and process shutdown.
type Runner struct {
	jobs     []Job
	leases   LeaseStore
	leaseTTL time.Duration
	logger   *slog.Logger
	metrics  *Metrics
}

// Config controls runner behavior.
type Config struct {
	LeaseTTL time.Duration
}

// NewRunner returns a background job runner.
func NewRunner(jobs []Job, leases LeaseStore, logger *slog.Logger, metrics *Metrics, cfg Config) *Runner {
	if leases == nil {
		leases = NoopLeaseStore{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = defaultLeaseTTL
	}
	return &Runner{
		jobs:     append([]Job(nil), jobs...),
		leases:   leases,
		leaseTTL: cfg.LeaseTTL,
		logger:   logger,
		metrics:  metrics,
	}
}

// Run starts all configured job loops until ctx is canceled.
func (r *Runner) Run(ctx context.Context) error {
	if r == nil {
		<-ctx.Done()
		return nil
	}
	var wg sync.WaitGroup
	for _, job := range r.jobs {
		if job == nil {
			continue
		}
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			r.runJob(ctx, job)
		}(job)
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (r *Runner) runJob(ctx context.Context, job Job) {
	interval := job.Interval()
	if interval <= 0 {
		interval = time.Minute
	}
	backoff := interval
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			err := r.runOnce(ctx, job)
			delay := interval
			if err != nil {
				r.logger.Warn("worker job failed", "job", job.Name(), "error", err)
				delay = backoff
				backoff *= 2
				if backoff > time.Minute {
					backoff = time.Minute
				}
			} else {
				backoff = interval
			}
			timer.Reset(delay)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context, job Job) (err error) {
	lease, ok, err := r.leases.Acquire(ctx, job.Name(), r.leaseTTL)
	if err != nil {
		return err
	}
	if !ok {
		r.metrics.skip(job.Name())
		return nil
	}
	defer func() {
		if releaseErr := lease.Release(context.Background()); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	timeout := job.Timeout()
	if timeout <= 0 {
		timeout = intervalOrDefault(job.Interval())
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	outcome := "success"
	r.metrics.begin(job.Name())
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("worker job panic: %v", recovered)
			outcome = "panic"
		} else if err != nil {
			outcome = "error"
		}
		r.metrics.finish(job.Name(), outcome, time.Since(started).Seconds())
	}()
	return job.Run(runCtx)
}

func intervalOrDefault(interval time.Duration) time.Duration {
	if interval <= 0 {
		return time.Minute
	}
	return interval
}
