package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// service.go owns async task idempotency, lifecycle transitions, callbacks, and cancellation.

// Service owns task creation, idempotency, status updates, and cancel state.
type Service struct {
	repo    Repository
	ttl     time.Duration
	metrics *Metrics
	now     func() time.Time
}

// NewService returns a task service backed by repo.
func NewService(repo Repository, ttl time.Duration) *Service {
	return NewServiceWithMetrics(repo, ttl, nil)
}

// NewServiceWithMetrics returns a task service backed by repo and metrics.
func NewServiceWithMetrics(repo Repository, ttl time.Duration, metrics *Metrics) *Service {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{repo: repo, ttl: ttl, metrics: metrics, now: func() time.Time { return time.Now().UTC() }}
}

// CreateTaskRequest contains durable inputs for a new async media task.
type CreateTaskRequest struct {
	TenantID       string
	ProjectID      string
	APIKeyID       string
	RequestID      string
	Endpoint       string
	IdempotencyKey string
	Kind           Kind
	MediaType      string
	Model          string
	Input          []byte
	CallbackURL    string
	Metadata       map[string]string
	BalanceHoldID  string
	PriceSnapshot  PriceSnapshot
}

// IdempotencyCheck contains lookup inputs for an async write request.
type IdempotencyCheck struct {
	TenantID       string
	APIKeyID       string
	Endpoint       string
	IdempotencyKey string
	Body           []byte
}

// FindIdempotentTask returns the existing task for a matching idempotency key.
func (s *Service) FindIdempotentTask(ctx context.Context, check IdempotencyCheck) (*Task, bool, error) {
	if s == nil || s.repo == nil || check.IdempotencyKey == "" {
		return nil, false, nil
	}
	task, record, ok, err := s.repo.GetTaskByIdempotency(ctx, check.TenantID, check.APIKeyID, check.Endpoint, check.IdempotencyKey, s.now())
	if err != nil || !ok {
		return nil, false, err
	}
	if err := checkIdempotencyHash(record, requestHash(check.Body)); err != nil {
		return nil, false, err
	}
	return task, true, nil
}

// CreateMediaTask creates an internal task and optional idempotency binding.
func (s *Service) CreateMediaTask(ctx context.Context, request CreateTaskRequest) (*Task, bool, error) {
	if s == nil || s.repo == nil {
		return nil, false, apperr.ConfigUnavailable("task repository is unavailable")
	}
	if request.Model == "" {
		return nil, false, apperr.InvalidArgument("model is required")
	}
	hash := requestHash(request.Input)
	if request.IdempotencyKey != "" {
		existing, hit, err := s.FindIdempotentTask(ctx, IdempotencyCheck{
			TenantID:       request.TenantID,
			APIKeyID:       request.APIKeyID,
			Endpoint:       request.Endpoint,
			IdempotencyKey: request.IdempotencyKey,
			Body:           request.Input,
		})
		if err != nil || hit {
			return existing, hit, err
		}
	}
	now := s.now()
	idem := newIdempotencyRecord(request.TenantID, request.APIKeyID, request.Endpoint, request.IdempotencyKey, hash, ResourceTask, s.ttl, now)
	task, err := s.repo.CreateTask(ctx, Task{
		ID:             newID("task"),
		TenantID:       request.TenantID,
		ProjectID:      request.ProjectID,
		APIKeyID:       request.APIKeyID,
		RequestID:      request.RequestID,
		IdempotencyKey: request.IdempotencyKey,
		RequestHash:    hash,
		Kind:           request.Kind,
		MediaType:      request.MediaType,
		Model:          request.Model,
		Status:         StatusQueued,
		Progress:       0,
		Input:          append([]byte(nil), request.Input...),
		CallbackURL:    request.CallbackURL,
		Metadata:       cloneMetadata(request.Metadata),
		BalanceHoldID:  request.BalanceHoldID,
		PriceSnapshot:  request.PriceSnapshot,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, idem)
	if err == nil {
		s.metrics.RecordTransition(request.Kind, "", StatusQueued)
	}
	return task, false, err
}

// GetTask returns a tenant-scoped task.
func (s *Service) GetTask(ctx context.Context, tenantID, projectID, taskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("task repository is unavailable")
	}
	task, ok, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !ok || task.TenantID != tenantID || task.ProjectID != projectID {
		return nil, apperr.NotFound("task not found")
	}
	return task, nil
}

// MarkDispatched records the upstream provider task id.
func (s *Service) MarkDispatched(ctx context.Context, taskID, providerType, channelID, providerTaskID string) (*Task, error) {
	if s == nil || s.repo == nil {
		return nil, apperr.ConfigUnavailable("task repository is unavailable")
	}
	task, err := s.repo.UpdateTaskDispatch(ctx, taskID, providerType, channelID, providerTaskID, StatusRunning)
	if err == nil {
		s.metrics.RecordTransition(task.Kind, StatusQueued, task.Status)
	}
	return task, err
}

// MarkFailed marks a task failed after local or provider failure.
func (s *Service) MarkFailed(ctx context.Context, taskID, code, message string) (*Task, error) {
	now := s.now()
	task, err := s.repo.UpdateTaskStatus(ctx, TaskStatusUpdate{
		TaskID:       taskID,
		Status:       StatusFailed,
		Progress:     100,
		ErrorCode:    code,
		ErrorMessage: message,
		CompletedAt:  &now,
	})
	if err == nil {
		s.metrics.RecordTransition(task.Kind, "", task.Status)
	}
	return task, err
}

// CancelTask marks a task canceled when it has not already reached a terminal state.
func (s *Service) CancelTask(ctx context.Context, tenantID, projectID, taskID string) (*Task, error) {
	task, err := s.GetTask(ctx, tenantID, projectID, taskID)
	if err != nil {
		return nil, err
	}
	if IsTerminal(task.Status) {
		return task, nil
	}
	now := s.now()
	updated, err := s.repo.UpdateTaskStatus(ctx, TaskStatusUpdate{
		TaskID:      taskID,
		Status:      StatusCanceled,
		Progress:    task.Progress,
		CompletedAt: &now,
	})
	if err == nil {
		s.metrics.RecordTransition(updated.Kind, task.Status, updated.Status)
	}
	return updated, err
}

// CompleteTask marks a task terminal and enqueues callback when configured.
func (s *Service) CompleteTask(ctx context.Context, task Task, result ProviderTaskResult) (*Task, error) {
	now := s.now()
	result = NormalizeProviderTaskResult(result)
	progress := result.Progress
	if progress <= 0 && IsTerminal(result.Status) {
		progress = 100
	}
	metadata := mergeProviderMetadata(task.Metadata, result.ProviderMetadata)
	updated, err := s.repo.UpdateTaskStatus(ctx, TaskStatusUpdate{
		TaskID:       task.ID,
		Status:       result.Status,
		Progress:     progress,
		Result:       result.Result,
		Usage:        result.Usage,
		ErrorCode:    result.ErrorCode,
		ErrorMessage: result.ErrorMessage,
		Metadata:     metadata,
		CompletedAt:  &now,
	})
	if err != nil {
		return nil, err
	}
	s.metrics.RecordTransition(updated.Kind, task.Status, updated.Status)
	if updated.CallbackURL != "" && IsTerminal(updated.Status) {
		payload, marshalErr := json.Marshal(TaskObject(updated))
		if marshalErr == nil {
			_ = s.repo.EnqueueCallback(ctx, CallbackEvent{
				ID:          newID("cb"),
				TaskID:      updated.ID,
				TenantID:    updated.TenantID,
				ProjectID:   updated.ProjectID,
				URL:         updated.CallbackURL,
				Payload:     payload,
				Status:      CallbackStatusPending,
				NextRetryAt: now,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
		}
	}
	return updated, nil
}

// IsTerminal reports whether a task no longer advances.
func IsTerminal(status Status) bool {
	switch status {
	case StatusSucceeded, StatusFailed, StatusCanceled, StatusExpired:
		return true
	default:
		return false
	}
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func mergeProviderMetadata(base map[string]string, provider map[string]string) map[string]string {
	merged := cloneMetadata(base)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range cleanMetadata(provider) {
		merged["provider."+key] = value
	}
	return merged
}
