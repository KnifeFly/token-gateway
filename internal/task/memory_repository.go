package task

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

var errTaskNotFound = errors.New("task not found")

// MemoryRepository is an in-process M4 repository for local development and tests.
type MemoryRepository struct {
	mu        sync.RWMutex
	tasks     map[string]Task
	idem      map[string]IdempotencyRecord
	files     map[string]FileAsset
	callbacks map[string]CallbackEvent
}

// NewMemoryRepository returns an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		tasks:     make(map[string]Task),
		idem:      make(map[string]IdempotencyRecord),
		files:     make(map[string]FileAsset),
		callbacks: make(map[string]CallbackEvent),
	}
}

// GetTask returns a task by id.
func (r *MemoryRepository) GetTask(_ context.Context, taskID string) (*Task, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, false, nil
	}
	return cloneTask(task), true, nil
}

// GetTaskByIdempotency returns the task bound to one unexpired idempotency key.
func (r *MemoryRepository) GetTaskByIdempotency(_ context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*Task, *IdempotencyRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.idem[idempotencyScope(tenantID, apiKeyID, endpoint, key)]
	if !ok || !record.ExpiresAt.After(now) || record.ResourceType != ResourceTask {
		return nil, nil, false, nil
	}
	task, ok := r.tasks[record.ResourceID]
	if !ok {
		return nil, nil, false, nil
	}
	return cloneTask(task), cloneIdempotency(record), true, nil
}

// CreateTask creates a task and optional idempotency binding.
func (r *MemoryRepository) CreateTask(_ context.Context, task Task, idempotency *IdempotencyRecord) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if task.ID == "" {
		task.ID = newID("task")
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.Metadata == nil {
		task.Metadata = map[string]string{}
	}
	r.tasks[task.ID] = task
	if idempotency != nil && idempotency.IdempotencyKey != "" {
		record := *idempotency
		if record.ID == "" {
			record.ID = newID("idem")
		}
		record.ResourceID = task.ID
		record.ResourceType = ResourceTask
		record.Status = IdempotencyStatusBound
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		record.UpdatedAt = now
		r.idem[idempotencyScope(record.TenantID, record.APIKeyID, record.Endpoint, record.IdempotencyKey)] = record
	}
	return cloneTask(task), nil
}

// UpdateTaskDispatch records the external provider task id.
func (r *MemoryRepository) UpdateTaskDispatch(_ context.Context, taskID, providerType, channelID, providerTaskID string, status Status) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, errTaskNotFound
	}
	task.ProviderType = providerType
	task.ChannelID = channelID
	task.ProviderTaskID = providerTaskID
	task.Status = status
	if task.Progress < 1 {
		task.Progress = 1
	}
	task.UpdatedAt = time.Now().UTC()
	r.tasks[task.ID] = task
	return cloneTask(task), nil
}

// UpdateTaskStatus updates the current task state and terminal fields.
func (r *MemoryRepository) UpdateTaskStatus(_ context.Context, update TaskStatusUpdate) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[update.TaskID]
	if !ok {
		return nil, errTaskNotFound
	}
	task.Status = update.Status
	task.Progress = update.Progress
	task.Result = append(task.Result[:0], update.Result...)
	task.Usage = update.Usage
	task.ErrorCode = update.ErrorCode
	task.ErrorMessage = update.ErrorMessage
	if update.Metadata != nil {
		task.Metadata = cloneMetadata(update.Metadata)
	}
	task.CompletedAt = update.CompletedAt
	task.UpdatedAt = time.Now().UTC()
	r.tasks[task.ID] = task
	return cloneTask(task), nil
}

// ListProviderTasks returns runnable provider tasks.
func (r *MemoryRepository) ListProviderTasks(_ context.Context, limit int) ([]Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	tasks := make([]Task, 0, limit)
	for _, task := range r.tasks {
		if len(tasks) >= limit {
			break
		}
		if task.ProviderTaskID == "" {
			continue
		}
		if task.Status == StatusQueued || task.Status == StatusRunning {
			tasks = append(tasks, *cloneTask(task))
		}
	}
	return tasks, nil
}

// ListTasks returns tenant/project scoped tasks ordered by newest first.
func (r *MemoryRepository) ListTasks(_ context.Context, filter TaskListFilter) ([]Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	tasks := make([]Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if filter.TenantID != "" && task.TenantID != filter.TenantID {
			continue
		}
		if filter.ProjectID != "" && task.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		tasks = append(tasks, *cloneTask(task))
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt.Equal(tasks[j].CreatedAt) {
			return tasks[i].ID > tasks[j].ID
		}
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	if filter.Cursor != "" {
		start := -1
		for i, task := range tasks {
			if task.ID == filter.Cursor {
				start = i + 1
				break
			}
		}
		if start >= 0 {
			tasks = tasks[start:]
		}
	}
	if len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}
	return tasks, nil
}

// GetFileByIdempotency returns the file bound to one unexpired idempotency key.
func (r *MemoryRepository) GetFileByIdempotency(_ context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*FileAsset, *IdempotencyRecord, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.idem[idempotencyScope(tenantID, apiKeyID, endpoint, key)]
	if !ok || !record.ExpiresAt.After(now) || record.ResourceType != ResourceFile {
		return nil, nil, false, nil
	}
	file, ok := r.files[record.ResourceID]
	if !ok {
		return nil, nil, false, nil
	}
	return cloneFile(file), cloneIdempotency(record), true, nil
}

// CreateFile creates a file asset and optional idempotency binding.
func (r *MemoryRepository) CreateFile(_ context.Context, file FileAsset, idempotency *IdempotencyRecord) (*FileAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if file.ID == "" {
		file.ID = newID("file")
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = now
	}
	r.files[file.ID] = file
	if idempotency != nil && idempotency.IdempotencyKey != "" {
		record := *idempotency
		if record.ID == "" {
			record.ID = newID("idem")
		}
		record.ResourceID = file.ID
		record.ResourceType = ResourceFile
		record.Status = IdempotencyStatusBound
		if record.CreatedAt.IsZero() {
			record.CreatedAt = now
		}
		record.UpdatedAt = now
		r.idem[idempotencyScope(record.TenantID, record.APIKeyID, record.Endpoint, record.IdempotencyKey)] = record
	}
	return cloneFile(file), nil
}

// FileQuota returns current file usage for one project.
func (r *MemoryRepository) FileQuota(_ context.Context, tenantID, projectID string, maxFiles int, maxBytes int64) (FileQuota, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	quota := FileQuota{MaxFiles: maxFiles, MaxBytes: maxBytes}
	now := time.Now().UTC()
	for _, file := range r.files {
		if file.TenantID != tenantID || file.ProjectID != projectID || fileExpired(file, now) {
			continue
		}
		quota.UsedFiles++
		quota.UsedBytes += file.SizeBytes
	}
	if maxFiles > 0 {
		quota.RemainingFiles = maxFiles - quota.UsedFiles
		if quota.RemainingFiles < 0 {
			quota.RemainingFiles = 0
		}
	}
	return quota, nil
}

// CleanupExpiredFiles removes expired transient input asset metadata.
func (r *MemoryRepository) CleanupExpiredFiles(_ context.Context, now time.Time, limit int) (FileCleanupResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	result := FileCleanupResult{}
	deleted := make(map[string]struct{})
	for id, file := range r.files {
		if result.Deleted >= limit {
			break
		}
		if !fileExpired(file, now) {
			continue
		}
		age := now.Sub(*file.ExpiresAt)
		if age > result.MaxAge {
			result.MaxAge = age
		}
		delete(r.files, id)
		deleted[id] = struct{}{}
		result.Deleted++
	}
	if len(deleted) > 0 {
		for scope, record := range r.idem {
			if record.ResourceType != ResourceFile {
				continue
			}
			if _, ok := deleted[record.ResourceID]; ok {
				delete(r.idem, scope)
			}
		}
	}
	return result, nil
}

// EnqueueCallback stores a callback event.
func (r *MemoryRepository) EnqueueCallback(_ context.Context, event CallbackEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	if event.ID == "" {
		event.ID = newID("cb")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}
	event.UpdatedAt = now
	if event.Status == "" {
		event.Status = CallbackStatusPending
	}
	r.callbacks[event.ID] = event
	return nil
}

// ListDueCallbacks returns pending callback events due for delivery.
func (r *MemoryRepository) ListDueCallbacks(_ context.Context, limit int, now time.Time) ([]CallbackEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	events := make([]CallbackEvent, 0, limit)
	for _, event := range r.callbacks {
		if len(events) >= limit {
			break
		}
		if event.Status == CallbackStatusPending && !event.NextRetryAt.After(now) {
			events = append(events, event)
		}
	}
	return events, nil
}

// ClaimDueCallbacks atomically assigns due callback rows to one dispatcher.
func (r *MemoryRepository) ClaimDueCallbacks(_ context.Context, ownerID string, claimTimeout time.Duration, limit int, now time.Time) ([]CallbackEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ownerID == "" {
		ownerID = "memory-callback-owner"
	}
	if claimTimeout <= 0 {
		claimTimeout = 5 * time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	events := make([]CallbackEvent, 0, limit)
	for id, event := range r.callbacks {
		if len(events) >= limit {
			break
		}
		due := (event.Status == CallbackStatusPending || event.Status == CallbackStatusFailed) && !event.NextRetryAt.After(now)
		expiredClaim := event.Status == CallbackStatusProcessing &&
			(event.HeartbeatAt.IsZero() || !event.HeartbeatAt.Add(claimTimeout).After(now))
		if !due && !expiredClaim {
			continue
		}
		event.Status = CallbackStatusProcessing
		event.OwnerID = ownerID
		event.ClaimedAt = now
		event.HeartbeatAt = now
		if event.DeliveryID == "" {
			event.DeliveryID = newID("cbdel")
		}
		event.UpdatedAt = now
		r.callbacks[id] = event
		events = append(events, event)
	}
	return events, nil
}

// MarkCallbackDelivered marks one callback as delivered.
func (r *MemoryRepository) MarkCallbackDelivered(_ context.Context, id string, ownerID string, statusCode int, latency time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.callbacks[id]
	if !ok {
		return nil
	}
	if ownerID != "" && event.OwnerID != "" && event.OwnerID != ownerID {
		return nil
	}
	event.Status = CallbackStatusDelivered
	event.OwnerID = ""
	event.LastStatusCode = statusCode
	event.LastLatencyMS = latency.Milliseconds()
	event.UpdatedAt = time.Now().UTC()
	r.callbacks[id] = event
	return nil
}

// MarkCallbackFailed records callback retry state.
func (r *MemoryRepository) MarkCallbackFailed(_ context.Context, id string, ownerID string, nextRetryAt time.Time, lastError string, statusCode int, latency time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.callbacks[id]
	if !ok {
		return nil
	}
	if ownerID != "" && event.OwnerID != "" && event.OwnerID != ownerID {
		return nil
	}
	event.Status = CallbackStatusPending
	event.OwnerID = ""
	event.RetryCount++
	event.NextRetryAt = nextRetryAt
	event.LastError = lastError
	event.LastStatusCode = statusCode
	event.LastLatencyMS = latency.Milliseconds()
	event.UpdatedAt = time.Now().UTC()
	r.callbacks[id] = event
	return nil
}

// MarkCallbackDeadLetter records a terminal callback delivery failure.
func (r *MemoryRepository) MarkCallbackDeadLetter(_ context.Context, id string, ownerID string, lastError string, statusCode int, latency time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	event, ok := r.callbacks[id]
	if !ok {
		return nil
	}
	if ownerID != "" && event.OwnerID != "" && event.OwnerID != ownerID {
		return nil
	}
	event.Status = CallbackStatusDeadLetter
	event.OwnerID = ""
	event.RetryCount++
	event.LastError = lastError
	event.LastStatusCode = statusCode
	event.LastLatencyMS = latency.Milliseconds()
	event.UpdatedAt = time.Now().UTC()
	r.callbacks[id] = event
	return nil
}

func idempotencyScope(tenantID, apiKeyID, endpoint, key string) string {
	return tenantID + "\x00" + apiKeyID + "\x00" + endpoint + "\x00" + key
}

func cloneTask(task Task) *Task {
	task.Input = append([]byte(nil), task.Input...)
	task.Result = append([]byte(nil), task.Result...)
	if task.Metadata != nil {
		metadata := make(map[string]string, len(task.Metadata))
		for key, value := range task.Metadata {
			metadata[key] = value
		}
		task.Metadata = metadata
	}
	if task.CompletedAt != nil {
		completedAt := *task.CompletedAt
		task.CompletedAt = &completedAt
	}
	return &task
}

func cloneIdempotency(record IdempotencyRecord) *IdempotencyRecord {
	return &record
}

func cloneFile(file FileAsset) *FileAsset {
	if file.ExpiresAt != nil {
		expiresAt := *file.ExpiresAt
		file.ExpiresAt = &expiresAt
	}
	return &file
}

func fileExpired(file FileAsset, now time.Time) bool {
	return file.ExpiresAt != nil && !file.ExpiresAt.After(now)
}
