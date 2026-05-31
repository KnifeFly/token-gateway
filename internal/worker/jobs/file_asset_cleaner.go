package jobs

import (
	"context"
	"log/slog"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

type expiredFileCleaner interface {
	CleanupExpiredFiles(ctx context.Context, now time.Time, limit int) (tasksvc.FileCleanupResult, error)
}

// FileAssetCleaner removes expired transient input asset metadata.
type FileAssetCleaner struct {
	service  expiredFileCleaner
	metrics  *tasksvc.Metrics
	now      func() time.Time
	interval time.Duration
	timeout  time.Duration
	limit    int
}

// NewFileAssetCleaner returns a worker job for expired transient file metadata.
func NewFileAssetCleaner(service expiredFileCleaner, metrics *tasksvc.Metrics, interval time.Duration, limit int) *FileAssetCleaner {
	if interval <= 0 {
		interval = time.Hour
	}
	if limit <= 0 {
		limit = 100
	}
	return &FileAssetCleaner{
		service:  service,
		metrics:  metrics,
		now:      time.Now,
		interval: interval,
		timeout:  30 * time.Second,
		limit:    limit,
	}
}

// Name returns the stable worker job name.
func (j *FileAssetCleaner) Name() string {
	return "file_asset_cleaner"
}

// Interval returns how often the job should run.
func (j *FileAssetCleaner) Interval() time.Duration {
	return j.interval
}

// Timeout returns the maximum runtime for one job execution.
func (j *FileAssetCleaner) Timeout() time.Duration {
	return j.timeout
}

// MaxConcurrency returns the maximum parallel executions for this job.
func (j *FileAssetCleaner) MaxConcurrency() int {
	return 1
}

// Run removes expired transient file metadata in bounded batches.
func (j *FileAssetCleaner) Run(ctx context.Context) error {
	if j == nil || j.service == nil {
		return nil
	}
	now := time.Now().UTC()
	if j.now != nil {
		now = j.now().UTC()
	}
	nextRun := now.Add(j.interval)
	result, err := j.service.CleanupExpiredFiles(ctx, now, j.limit)
	if err != nil {
		if j.metrics != nil {
			j.metrics.RecordFileCleanup("error", 0, 0, nextRun)
		}
		return err
	}
	if j.metrics != nil {
		j.metrics.RecordFileCleanup("success", result.Deleted, result.MaxAge, nextRun)
	}
	slog.Info("expired transient file metadata cleanup complete", "deleted", result.Deleted, "max_age_seconds", result.MaxAge.Seconds(), "next_run", nextRun)
	return nil
}
