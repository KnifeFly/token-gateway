package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// MySQLRepository persists M4 state in MySQL.
type MySQLRepository struct {
	db *sql.DB
}

// NewMySQLRepository returns a MySQL-backed task repository.
func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

// GetTask returns a task by id.
func (r *MySQLRepository) GetTask(ctx context.Context, taskID string) (*Task, bool, error) {
	task, err := scanTask(r.db.QueryRowContext(ctx, taskSelectSQL+" WHERE id = ?", taskID))
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return task, true, nil
}

// GetTaskByIdempotency returns the task bound to one unexpired idempotency key.
func (r *MySQLRepository) GetTaskByIdempotency(ctx context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*Task, *IdempotencyRecord, bool, error) {
	record, err := scanIdempotency(r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, api_key_id, endpoint, idempotency_key, request_hash, resource_type, resource_id, status, expires_at, created_at, updated_at
FROM task_idempotency_records
WHERE tenant_id = ? AND api_key_id = ? AND endpoint = ? AND idempotency_key = ? AND expires_at > ?`,
		tenantID, apiKeyID, endpoint, key, now))
	if err == sql.ErrNoRows {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	task, ok, err := r.GetTask(ctx, record.ResourceID)
	if err != nil || !ok {
		return nil, nil, false, err
	}
	return task, record, true, nil
}

// CreateTask creates a task and optional idempotency binding.
func (r *MySQLRepository) CreateTask(ctx context.Context, task Task, idempotency *IdempotencyRecord) (*Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	metadata, _ := json.Marshal(task.Metadata)
	usage, _ := json.Marshal(task.Usage)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO tasks (
  id, tenant_id, project_id, api_key_id, request_id, idempotency_key, request_hash,
  kind, media_type, model, status, progress, provider_type, channel_id, provider_task_id,
  input_json, result_json, usage_json, error_code, error_message, callback_url, metadata_json,
  balance_hold_id, created_at, updated_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.TenantID, task.ProjectID, task.APIKeyID, task.RequestID, task.IdempotencyKey, task.RequestHash,
		string(task.Kind), task.MediaType, task.Model, string(task.Status), task.Progress, task.ProviderType, task.ChannelID, task.ProviderTaskID,
		[]byte(task.Input), nullableBytes(task.Result), usage, task.ErrorCode, task.ErrorMessage, task.CallbackURL, metadata,
		task.BalanceHoldID, task.CreatedAt, task.UpdatedAt, task.CompletedAt); err != nil {
		return nil, err
	}
	if idempotency != nil {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO task_idempotency_records (
  id, tenant_id, api_key_id, endpoint, idempotency_key, request_hash, resource_type, resource_id,
  status, expires_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			idempotency.ID, idempotency.TenantID, idempotency.APIKeyID, idempotency.Endpoint, idempotency.IdempotencyKey,
			idempotency.RequestHash, string(idempotency.ResourceType), task.ID, string(IdempotencyStatusBound),
			idempotency.ExpiresAt, idempotency.CreatedAt, idempotency.UpdatedAt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTaskRequired(ctx, task.ID)
}

// GetTaskRequired returns an existing task or the query error.
func (r *MySQLRepository) GetTaskRequired(ctx context.Context, taskID string) (*Task, error) {
	task, ok, err := r.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errTaskNotFound
	}
	return task, nil
}

// UpdateTaskDispatch records the external provider task id.
func (r *MySQLRepository) UpdateTaskDispatch(ctx context.Context, taskID, providerType, channelID, providerTaskID string, status Status) (*Task, error) {
	if _, err := r.db.ExecContext(ctx, `
UPDATE tasks
SET provider_type = ?, channel_id = ?, provider_task_id = ?, status = ?, progress = GREATEST(progress, 1), updated_at = ?
WHERE id = ?`, providerType, channelID, providerTaskID, string(status), time.Now().UTC(), taskID); err != nil {
		return nil, err
	}
	return r.GetTaskRequired(ctx, taskID)
}

// UpdateTaskStatus updates the current task state and terminal fields.
func (r *MySQLRepository) UpdateTaskStatus(ctx context.Context, update TaskStatusUpdate) (*Task, error) {
	usage, _ := json.Marshal(update.Usage)
	if update.Metadata != nil {
		metadata, _ := json.Marshal(update.Metadata)
		if _, err := r.db.ExecContext(ctx, `
	UPDATE tasks
	SET status = ?, progress = ?, result_json = ?, usage_json = ?, error_code = ?, error_message = ?, metadata_json = ?, completed_at = ?, updated_at = ?
	WHERE id = ?`,
			string(update.Status), update.Progress, nullableBytes(update.Result), usage, update.ErrorCode, update.ErrorMessage, metadata, update.CompletedAt, time.Now().UTC(), update.TaskID); err != nil {
			return nil, err
		}
		return r.GetTaskRequired(ctx, update.TaskID)
	}
	if _, err := r.db.ExecContext(ctx, `
	UPDATE tasks
	SET status = ?, progress = ?, result_json = ?, usage_json = ?, error_code = ?, error_message = ?, completed_at = ?, updated_at = ?
	WHERE id = ?`,
		string(update.Status), update.Progress, nullableBytes(update.Result), usage, update.ErrorCode, update.ErrorMessage, update.CompletedAt, time.Now().UTC(), update.TaskID); err != nil {
		return nil, err
	}
	return r.GetTaskRequired(ctx, update.TaskID)
}

// ListProviderTasks returns runnable provider tasks.
func (r *MySQLRepository) ListProviderTasks(ctx context.Context, limit int) ([]Task, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, taskSelectSQL+`
 WHERE provider_task_id <> '' AND status IN ('queued', 'running')
 ORDER BY updated_at ASC
 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

// ListTasks returns tenant/project scoped tasks ordered by newest first.
func (r *MySQLRepository) ListTasks(ctx context.Context, filter TaskListFilter) ([]Task, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	query := taskSelectSQL + ` WHERE tenant_id = ? AND project_id = ?`
	args := []any{filter.TenantID, filter.ProjectID}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	if filter.Cursor != "" {
		cursor, ok, err := r.cursorTask(ctx, filter.Cursor)
		if err != nil {
			return nil, err
		}
		if ok {
			query += ` AND (created_at < ? OR (created_at = ? AND id < ?))`
			args = append(args, cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, filter.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

func (r *MySQLRepository) cursorTask(ctx context.Context, taskID string) (*Task, bool, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false, nil
	}
	task, err := scanTask(r.db.QueryRowContext(ctx, taskSelectSQL+" WHERE id = ?", taskID))
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return task, true, nil
}

// GetFileByIdempotency returns the file bound to one unexpired idempotency key.
func (r *MySQLRepository) GetFileByIdempotency(ctx context.Context, tenantID, apiKeyID, endpoint, key string, now time.Time) (*FileAsset, *IdempotencyRecord, bool, error) {
	record, err := scanIdempotency(r.db.QueryRowContext(ctx, `
SELECT id, tenant_id, api_key_id, endpoint, idempotency_key, request_hash, resource_type, resource_id, status, expires_at, created_at, updated_at
FROM task_idempotency_records
WHERE tenant_id = ? AND api_key_id = ? AND endpoint = ? AND idempotency_key = ? AND expires_at > ?`,
		tenantID, apiKeyID, endpoint, key, now))
	if err == sql.ErrNoRows {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	file, err := scanFile(r.db.QueryRowContext(ctx, fileSelectSQL+" WHERE id = ?", record.ResourceID))
	if err == sql.ErrNoRows {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	return file, record, true, nil
}

// CreateFile creates a file asset and optional idempotency binding.
func (r *MySQLRepository) CreateFile(ctx context.Context, file FileAsset, idempotency *IdempotencyRecord) (*FileAsset, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
	INSERT INTO file_assets (
	  id, tenant_id, project_id, api_key_id, request_id, file_name, original_name, size_bytes,
	  mime_type, upload_path, file_url, download_url, source, content_hash, source_url, transient, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.ID, file.TenantID, file.ProjectID, file.APIKeyID, file.RequestID, file.FileName, file.OriginalName, file.SizeBytes,
		file.MIMEType, file.UploadPath, file.FileURL, file.DownloadURL, file.Source, file.ContentHash, file.SourceURL, file.Transient,
		file.CreatedAt, file.ExpiresAt); err != nil {
		return nil, err
	}
	if idempotency != nil {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO task_idempotency_records (
  id, tenant_id, api_key_id, endpoint, idempotency_key, request_hash, resource_type, resource_id,
  status, expires_at, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			idempotency.ID, idempotency.TenantID, idempotency.APIKeyID, idempotency.Endpoint, idempotency.IdempotencyKey,
			idempotency.RequestHash, string(idempotency.ResourceType), file.ID, string(IdempotencyStatusBound),
			idempotency.ExpiresAt, idempotency.CreatedAt, idempotency.UpdatedAt); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return scanFile(r.db.QueryRowContext(ctx, fileSelectSQL+" WHERE id = ?", file.ID))
}

// FileQuota returns current file usage for one project.
func (r *MySQLRepository) FileQuota(ctx context.Context, tenantID, projectID string, maxFiles int, maxBytes int64) (FileQuota, error) {
	var quota FileQuota
	quota.MaxFiles = maxFiles
	quota.MaxBytes = maxBytes
	if err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(size_bytes), 0)
FROM file_assets
WHERE tenant_id = ? AND project_id = ?`, tenantID, projectID).Scan(&quota.UsedFiles, &quota.UsedBytes); err != nil {
		return FileQuota{}, err
	}
	if maxFiles > 0 {
		quota.RemainingFiles = maxFiles - quota.UsedFiles
		if quota.RemainingFiles < 0 {
			quota.RemainingFiles = 0
		}
	}
	return quota, nil
}

// EnqueueCallback stores a callback event.
func (r *MySQLRepository) EnqueueCallback(ctx context.Context, event CallbackEvent) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO callback_outbox (
  id, task_id, tenant_id, project_id, url, payload_json, status, retry_count, next_retry_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TaskID, event.TenantID, event.ProjectID, event.URL, []byte(event.Payload), string(event.Status),
		event.RetryCount, event.NextRetryAt, event.LastError, event.CreatedAt, event.UpdatedAt)
	return err
}

// ListDueCallbacks returns pending callback events due for delivery.
func (r *MySQLRepository) ListDueCallbacks(ctx context.Context, limit int, now time.Time) ([]CallbackEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, task_id, tenant_id, project_id, url, payload_json, status, retry_count, next_retry_at, last_error, created_at, updated_at
FROM callback_outbox
WHERE status = 'pending' AND next_retry_at <= ?
ORDER BY next_retry_at ASC
LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []CallbackEvent
	for rows.Next() {
		event, err := scanCallback(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}
	return events, rows.Err()
}

// MarkCallbackDelivered marks one callback as delivered.
func (r *MySQLRepository) MarkCallbackDelivered(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE callback_outbox SET status = 'delivered', updated_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

// MarkCallbackFailed records callback retry state.
func (r *MySQLRepository) MarkCallbackFailed(ctx context.Context, id string, nextRetryAt time.Time, lastError string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE callback_outbox
SET status = 'pending', retry_count = retry_count + 1, next_retry_at = ?, last_error = ?, updated_at = ?
WHERE id = ?`, nextRetryAt, lastError, time.Now().UTC(), id)
	return err
}

const taskSelectSQL = `
SELECT id, tenant_id, project_id, api_key_id, request_id, idempotency_key, request_hash,
       kind, media_type, model, status, progress, provider_type, channel_id, provider_task_id,
       input_json, result_json, usage_json, error_code, error_message, callback_url, metadata_json,
       balance_hold_id, created_at, updated_at, completed_at
FROM tasks`

const fileSelectSQL = `
SELECT id, tenant_id, project_id, api_key_id, request_id, file_name, original_name, size_bytes,
       mime_type, upload_path, file_url, download_url, source, content_hash, source_url, transient, created_at, expires_at
	FROM file_assets`

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (*Task, error) {
	var task Task
	var kind, status string
	var input, result, usage, metadata sql.NullString
	var completedAt sql.NullTime
	if err := row.Scan(
		&task.ID, &task.TenantID, &task.ProjectID, &task.APIKeyID, &task.RequestID, &task.IdempotencyKey, &task.RequestHash,
		&kind, &task.MediaType, &task.Model, &status, &task.Progress, &task.ProviderType, &task.ChannelID, &task.ProviderTaskID,
		&input, &result, &usage, &task.ErrorCode, &task.ErrorMessage, &task.CallbackURL, &metadata,
		&task.BalanceHoldID, &task.CreatedAt, &task.UpdatedAt, &completedAt,
	); err != nil {
		return nil, err
	}
	task.Kind = Kind(kind)
	task.Status = Status(status)
	if input.Valid {
		task.Input = []byte(input.String)
	}
	if result.Valid {
		task.Result = []byte(result.String)
	}
	if usage.Valid && usage.String != "" {
		_ = json.Unmarshal([]byte(usage.String), &task.Usage)
	}
	if metadata.Valid && metadata.String != "" {
		_ = json.Unmarshal([]byte(metadata.String), &task.Metadata)
	}
	if task.Metadata == nil {
		task.Metadata = map[string]string{}
	}
	if completedAt.Valid {
		task.CompletedAt = &completedAt.Time
	}
	return &task, nil
}

func scanIdempotency(row scanner) (*IdempotencyRecord, error) {
	var record IdempotencyRecord
	var resourceType, status string
	if err := row.Scan(
		&record.ID, &record.TenantID, &record.APIKeyID, &record.Endpoint, &record.IdempotencyKey,
		&record.RequestHash, &resourceType, &record.ResourceID, &status, &record.ExpiresAt, &record.CreatedAt, &record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	record.ResourceType = ResourceType(resourceType)
	record.Status = IdempotencyStatus(status)
	return &record, nil
}

func scanFile(row scanner) (*FileAsset, error) {
	var file FileAsset
	var expiresAt sql.NullTime
	if err := row.Scan(
		&file.ID, &file.TenantID, &file.ProjectID, &file.APIKeyID, &file.RequestID, &file.FileName, &file.OriginalName, &file.SizeBytes,
		&file.MIMEType, &file.UploadPath, &file.FileURL, &file.DownloadURL, &file.Source, &file.ContentHash, &file.SourceURL, &file.Transient, &file.CreatedAt, &expiresAt,
	); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		file.ExpiresAt = &expiresAt.Time
	}
	return &file, nil
}

func scanCallback(row scanner) (*CallbackEvent, error) {
	var event CallbackEvent
	var status string
	if err := row.Scan(
		&event.ID, &event.TaskID, &event.TenantID, &event.ProjectID, &event.URL, &event.Payload, &status,
		&event.RetryCount, &event.NextRetryAt, &event.LastError, &event.CreatedAt, &event.UpdatedAt,
	); err != nil {
		return nil, err
	}
	event.Status = CallbackStatus(status)
	return &event, nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
