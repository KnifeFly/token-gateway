package task

import (
	"context"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/tokenusage"
)

// Repository persists M4 task, file, idempotency, and callback state.
type Repository interface {
	GetTask(ctx context.Context, taskID string) (*Task, bool, error)
	GetTaskByIdempotency(ctx context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*Task, *IdempotencyRecord, bool, error)
	CreateTask(ctx context.Context, task Task, idempotency *IdempotencyRecord) (*Task, error)
	UpdateTaskDispatch(ctx context.Context, taskID, providerType, channelID, providerTaskID string, status Status) (*Task, error)
	UpdateTaskStatus(ctx context.Context, update TaskStatusUpdate) (*Task, error)
	ListProviderTasks(ctx context.Context, limit int) ([]Task, error)
	ListTasks(ctx context.Context, filter TaskListFilter) ([]Task, error)

	GetFileByIdempotency(ctx context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*FileAsset, *IdempotencyRecord, bool, error)
	CreateFile(ctx context.Context, file FileAsset, idempotency *IdempotencyRecord) (*FileAsset, error)
	FileQuota(ctx context.Context, tenantID, projectID string, maxFiles int, maxBytes int64) (FileQuota, error)

	EnqueueCallback(ctx context.Context, event CallbackEvent) error
	ListDueCallbacks(ctx context.Context, limit int, now time.Time) ([]CallbackEvent, error)
	MarkCallbackDelivered(ctx context.Context, id string) error
	MarkCallbackFailed(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error
	MarkCallbackDeadLetter(ctx context.Context, id string, lastError string) error
}

// TaskListFilter scopes customer task list queries.
type TaskListFilter struct {
	TenantID  string
	ProjectID string
	Status    Status
	Cursor    string
	Limit     int
}

// TaskStatusUpdate contains mutable task completion fields.
type TaskStatusUpdate struct {
	TaskID       string
	Status       Status
	Progress     int
	Result       []byte
	Usage        tokenusage.Actual
	ErrorCode    string
	ErrorMessage string
	Metadata     map[string]string
	CompletedAt  *time.Time
}
