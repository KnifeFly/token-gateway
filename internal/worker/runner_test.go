package worker

import (
	"context"
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
