package worker

import (
	"context"
	"time"
)

// Job is a repeatable background unit run by the worker process.
type Job interface {
	Name() string
	Interval() time.Duration
	Timeout() time.Duration
	MaxConcurrency() int
	Run(ctx context.Context) error
}
