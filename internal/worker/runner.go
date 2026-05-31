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
	jobs              []Job
	leases            LeaseStore
	leaseTTL          time.Duration
	heartbeatInterval time.Duration
	logger            *slog.Logger
	metrics           *Metrics
}

// Config controls runner behavior.
type Config struct {
	LeaseTTL          time.Duration
	HeartbeatInterval time.Duration
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
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = cfg.LeaseTTL / 3
	}
	return &Runner{
		jobs:              append([]Job(nil), jobs...),
		leases:            leases,
		leaseTTL:          cfg.LeaseTTL,
		heartbeatInterval: cfg.HeartbeatInterval,
		logger:            logger,
		metrics:           metrics,
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
		concurrency := maxConcurrency(job)
		for slot := 0; slot < concurrency; slot++ {
			wg.Add(1)
			go func(job Job, slot int, concurrency int) {
				defer wg.Done()
				r.runJob(ctx, job, slot, concurrency)
			}(job, slot, concurrency)
		}
	}
	<-ctx.Done()
	wg.Wait()
	return nil
}

func (r *Runner) runJob(ctx context.Context, job Job, slot int, concurrency int) {
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
			err := r.runOnceWithLease(ctx, job, leaseName(job, slot, concurrency))
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
	return r.runOnceWithLease(ctx, job, job.Name())
}

func (r *Runner) runOnceWithLease(ctx context.Context, job Job, leaseName string) (err error) {
	lease, ok, err := r.leases.Acquire(ctx, leaseName, r.leaseTTL)
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
	stopHeartbeat := r.startHeartbeat(runCtx, cancel, job.Name(), lease)
	defer stopHeartbeat()

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

func (r *Runner) startHeartbeat(ctx context.Context, cancel context.CancelFunc, job string, lease Lease) func() {
	if lease == nil || r.heartbeatInterval <= 0 {
		return func() {}
	}
	heartbeatCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTimer(r.heartbeatInterval)
		defer timer.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-timer.C:
				if err := lease.Renew(heartbeatCtx, r.leaseTTL); err != nil {
					r.metrics.heartbeat(job, "error")
					r.logger.Warn("worker lease heartbeat failed", "job", job, "error", err)
					cancel()
					return
				}
				r.metrics.heartbeat(job, "success")
				timer.Reset(r.heartbeatInterval)
			}
		}
	}()
	return func() {
		stop()
		<-done
	}
}

func intervalOrDefault(interval time.Duration) time.Duration {
	if interval <= 0 {
		return time.Minute
	}
	return interval
}

func maxConcurrency(job Job) int {
	concurrency := job.MaxConcurrency()
	if concurrency <= 0 {
		return 1
	}
	return concurrency
}

func leaseName(job Job, slot int, concurrency int) string {
	if concurrency <= 1 {
		return job.Name()
	}
	return fmt.Sprintf("%s:%d", job.Name(), slot+1)
}
