package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
)

func TestFileAssetCleanerRemovesExpiredMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	repo := tasksvc.NewMemoryRepository()
	expiredAt := now.Add(-time.Hour)
	activeExpiresAt := now.Add(time.Hour)
	if _, err := repo.CreateFile(ctx, tasksvc.FileAsset{
		ID:        "file_expired",
		TenantID:  "tenant",
		ProjectID: "project",
		SizeBytes: 10,
		ExpiresAt: &expiredAt,
	}, nil); err != nil {
		t.Fatalf("CreateFile(expired) error = %v", err)
	}
	if _, err := repo.CreateFile(ctx, tasksvc.FileAsset{
		ID:        "file_active",
		TenantID:  "tenant",
		ProjectID: "project",
		SizeBytes: 20,
		ExpiresAt: &activeExpiresAt,
	}, nil); err != nil {
		t.Fatalf("CreateFile(active) error = %v", err)
	}
	job := NewFileAssetCleaner(tasksvc.NewFileService(repo, 0), nil, time.Second, 10)
	job.now = func() time.Time { return now }

	if err := job.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	quota, err := repo.FileQuota(ctx, "tenant", "project", 10, 100)
	if err != nil {
		t.Fatalf("FileQuota() error = %v", err)
	}
	if quota.UsedFiles != 1 || quota.UsedBytes != 20 {
		t.Fatalf("quota = %#v", quota)
	}
}

func TestFileAssetCleanerReturnsServiceError(t *testing.T) {
	errBoom := errors.New("boom")
	job := NewFileAssetCleaner(failingFileCleaner{err: errBoom}, nil, time.Second, 1)
	if err := job.Run(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("Run() error = %v, want %v", err, errBoom)
	}
}

type failingFileCleaner struct {
	err error
}

func (s failingFileCleaner) CleanupExpiredFiles(context.Context, time.Time, int) (tasksvc.FileCleanupResult, error) {
	return tasksvc.FileCleanupResult{}, s.err
}
