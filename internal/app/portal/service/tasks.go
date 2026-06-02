package service

import (
	"context"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListTasks returns project-scoped tasks.
func (s *Service) ListTasks(ctx context.Context, principal portalapp.Principal, filter portalapp.TaskFilter) (portalapp.TaskListResponse, error) {
	if s == nil || s.tasks == nil {
		return portalapp.TaskListResponse{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	limit := normalizeLimit(filter.Limit)
	tasks, err := s.tasks.ListTasks(ctx, tasksvc.TaskListFilter{
		TenantID:     principal.TenantID,
		ProjectID:    principal.ProjectID,
		APIKeyID:     strings.TrimSpace(filter.APIKeyID),
		RequestID:    strings.TrimSpace(filter.RequestID),
		Model:        strings.TrimSpace(filter.Model),
		ProviderType: strings.TrimSpace(filter.ProviderType),
		ChannelID:    strings.TrimSpace(filter.ChannelID),
		Status:       tasksvc.Status(strings.TrimSpace(filter.Status)),
		Cursor:       strings.TrimSpace(filter.Cursor),
		From:         filter.From,
		To:           filter.To,
		Limit:        limit + 1,
	})
	if err != nil {
		return portalapp.TaskListResponse{}, err
	}
	var nextCursor *string
	if len(tasks) > limit {
		cursor := tasks[limit-1].ID
		nextCursor = &cursor
		tasks = tasks[:limit]
	}
	out := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, safeTaskObject(&task))
	}
	return portalapp.TaskListResponse{Data: out, NextCursor: nextCursor}, nil
}

// GetTask returns one project-scoped task.

func (s *Service) GetTask(ctx context.Context, principal portalapp.Principal, taskID string) (map[string]any, error) {
	if s == nil || s.tasks == nil {
		return nil, apperr.ConfigUnavailable("task repository is unavailable")
	}
	taskID = strings.Trim(taskID, "/ ")
	if taskID == "" || strings.Contains(taskID, "/") {
		return nil, apperr.NotFound("task not found")
	}
	task, ok, err := s.tasks.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !ok || task.TenantID != principal.TenantID || task.ProjectID != principal.ProjectID {
		return nil, apperr.NotFound("task not found")
	}
	return safeTaskObject(task), nil
}

func safeTaskObject(task *tasksvc.Task) map[string]any {
	object := tasksvc.TaskObject(task)
	object["request_id"] = task.RequestID
	object["api_key_id"] = task.APIKeyID
	object["created_at"] = task.CreatedAt.Format(time.RFC3339)
	object["updated_at"] = task.UpdatedAt.Format(time.RFC3339)
	if metadata, ok := object["metadata"].(map[string]string); ok {
		object["metadata"] = safeMetadata(metadata)
	}
	if metadata, ok := object["provider_metadata"].(map[string]string); ok {
		object["provider_metadata"] = safeMetadata(metadata)
	}
	return object
}

func safeMetadata(metadata map[string]string) map[string]string {
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") {
			continue
		}
		out[key] = value
	}
	return out
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultPortalLimit
	}
	if limit > maxPortalLimit {
		return maxPortalLimit
	}
	return limit
}
