package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerRunOnceUsesLease(t *testing.T) {
	leases := NewMemoryLeaseStore()
	held, ok, err := leases.Acquire(context.Background(), "job", time.Minute)
	if err != nil || !ok {
		t.Fatalf("Acquire() = %v, %v", ok, err)
	}
	defer held.Release(context.Background())
	var runs int32
	runner := NewRunner([]Job{}, leases, nil, nil, Config{LeaseTTL: time.Minute})

	if err := runner.runOnce(context.Background(), testJob{runs: &runs}); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if got := atomic.LoadInt32(&runs); got != 0 {
		t.Fatalf("runs = %d, want 0", got)
	}
}

func TestRunnerRunOnceRecoversPanic(t *testing.T) {
	runner := NewRunner([]Job{}, NewMemoryLeaseStore(), nil, nil, Config{LeaseTTL: time.Minute})

	if err := runner.runOnce(context.Background(), panicJob{}); err == nil {
		t.Fatal("expected panic error")
	}
}

func TestRunnerRenewsLeaseWhileJobRuns(t *testing.T) {
	renewed := make(chan struct{})
	lease := &renewCountingLease{renewed: renewed}
	runner := NewRunner([]Job{}, renewLeaseStore{lease: lease}, nil, nil, Config{
		LeaseTTL:          time.Minute,
		HeartbeatInterval: 5 * time.Millisecond,
	})
	job := functionJob{
		name:        "heartbeat",
		interval:    time.Minute,
		timeout:     time.Second,
		concurrency: 1,
		run: func(ctx context.Context) error {
			select {
			case <-renewed:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				t.Fatal("heartbeat did not renew lease")
				return nil
			}
		},
	}

	if err := runner.runOnce(context.Background(), job); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if got := atomic.LoadInt32(&lease.count); got == 0 {
		t.Fatal("lease was not renewed")
	}
}

func TestRunnerRunStartsMaxConcurrencySlots(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	job := functionJob{
		name:        "concurrent",
		interval:    time.Minute,
		timeout:     time.Second,
		concurrency: 3,
		run: func(ctx context.Context) error {
			started <- struct{}{}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	runner := NewRunner([]Job{job}, NoopLeaseStore{}, nil, nil, Config{
		LeaseTTL:          time.Minute,
		HeartbeatInterval: time.Minute,
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			cancel()
			close(release)
			t.Fatalf("started slots = %d, want 3", i)
		}
	}
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

type testJob struct {
	runs *int32
}

func (j testJob) Name() string              { return "job" }
func (j testJob) Interval() time.Duration   { return time.Minute }
func (j testJob) Timeout() time.Duration    { return time.Second }
func (j testJob) MaxConcurrency() int       { return 1 }
func (j testJob) Run(context.Context) error { atomic.AddInt32(j.runs, 1); return nil }
func (panicJob) Name() string               { return "panic" }
func (panicJob) Interval() time.Duration    { return time.Minute }
func (panicJob) Timeout() time.Duration     { return time.Second }
func (panicJob) MaxConcurrency() int        { return 1 }
func (panicJob) Run(context.Context) error  { panic("boom") }

type panicJob struct{}

type renewLeaseStore struct {
	lease Lease
}

func (s renewLeaseStore) Acquire(context.Context, string, time.Duration) (Lease, bool, error) {
	return s.lease, true, nil
}

type renewCountingLease struct {
	once    sync.Once
	renewed chan struct{}
	count   int32
}

func (l *renewCountingLease) Renew(context.Context, time.Duration) error {
	atomic.AddInt32(&l.count, 1)
	l.once.Do(func() { close(l.renewed) })
	return nil
}

func (l *renewCountingLease) Release(context.Context) error {
	return nil
}

type functionJob struct {
	name        string
	interval    time.Duration
	timeout     time.Duration
	concurrency int
	run         func(context.Context) error
}

func (j functionJob) Name() string            { return j.name }
func (j functionJob) Interval() time.Duration { return j.interval }
func (j functionJob) Timeout() time.Duration  { return j.timeout }
func (j functionJob) MaxConcurrency() int     { return j.concurrency }
func (j functionJob) Run(ctx context.Context) error {
	if j.run == nil {
		return nil
	}
	return j.run(ctx)
}
