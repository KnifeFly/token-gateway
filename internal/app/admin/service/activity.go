package service

import (
	"context"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListUsageLogs returns operator-safe request-level usage rows.
func (s *Service) ListUsageLogs(ctx context.Context, actor adminapp.Actor, filter adminapp.UsageLogFilter) (adminapp.ListResponse[adminapp.UsageLogView], error) {
	if err := s.Authorize(actor, "read", "usage"); err != nil {
		return adminapp.ListResponse[adminapp.UsageLogView]{}, err
	}
	if s.commercial == nil {
		return adminapp.ListResponse[adminapp.UsageLogView]{}, apperr.ConfigUnavailable("commercial reporting is unavailable")
	}
	report, err := s.commercial.UsageLogReport(ctx, reporting.UsageLogFilter{
		TenantID:     filter.TenantID,
		ProjectID:    filter.ProjectID,
		APIKeyID:     filter.APIKeyID,
		RequestID:    filter.RequestID,
		Model:        filter.Model,
		ProviderType: filter.ProviderType,
		ChannelID:    filter.ChannelID,
		Status:       filter.Status,
		Currency:     filter.Currency,
		From:         filter.From,
		To:           filter.To,
		Limit:        normalizeLimit(filter.Limit),
	})
	if err != nil {
		return adminapp.ListResponse[adminapp.UsageLogView]{}, err
	}
	views := make([]adminapp.UsageLogView, 0, len(report.Rows))
	for _, row := range report.Rows {
		views = append(views, usageLogView(row))
	}
	return adminapp.ListResponse[adminapp.UsageLogView]{Data: views}, nil
}

// GetUsageLog returns one operator-safe usage log detail by request id.
func (s *Service) GetUsageLog(ctx context.Context, actor adminapp.Actor, requestID string) (adminapp.UsageLogDetail, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || strings.Contains(requestID, "/") {
		return adminapp.UsageLogDetail{}, apperr.NotFound("usage log not found")
	}
	response, err := s.ListUsageLogs(ctx, actor, adminapp.UsageLogFilter{RequestID: requestID, Limit: 1})
	if err != nil {
		return adminapp.UsageLogDetail{}, err
	}
	if len(response.Data) == 0 {
		return adminapp.UsageLogDetail{}, apperr.NotFound("usage log not found")
	}
	detail := adminapp.UsageLogDetail{Usage: response.Data[0]}
	if s.commercial != nil && detail.Usage.TenantID != "" {
		report, err := s.commercial.TenantUsageReport(ctx, reporting.TenantUsageFilter{
			TenantID:  detail.Usage.TenantID,
			ProjectID: detail.Usage.ProjectID,
			RequestID: detail.Usage.RequestID,
			Currency:  detail.Usage.Currency,
			Limit:     10,
		})
		if err == nil {
			detail.Ledger = customerLedgerLines(report)
		}
	}
	return detail, nil
}

// ListTaskLogs returns operator-safe async task rows.
func (s *Service) ListTaskLogs(ctx context.Context, actor adminapp.Actor, filter adminapp.TaskLogFilter) (adminapp.ListResponse[adminapp.TaskLogView], error) {
	if err := s.Authorize(actor, "read", "task"); err != nil {
		return adminapp.ListResponse[adminapp.TaskLogView]{}, err
	}
	if s.tasks == nil {
		return adminapp.ListResponse[adminapp.TaskLogView]{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	tasks, err := s.tasks.ListTasks(ctx, tasksvc.TaskListFilter{
		TaskID:       filter.TaskID,
		TenantID:     filter.TenantID,
		ProjectID:    filter.ProjectID,
		APIKeyID:     filter.APIKeyID,
		RequestID:    filter.RequestID,
		Model:        filter.Model,
		ProviderType: filter.ProviderType,
		ChannelID:    filter.ChannelID,
		Status:       tasksvc.Status(strings.TrimSpace(filter.Status)),
		Cursor:       strings.TrimSpace(filter.Cursor),
		From:         filter.From,
		To:           filter.To,
		Limit:        normalizeLimit(filter.Limit),
	})
	if err != nil {
		return adminapp.ListResponse[adminapp.TaskLogView]{}, err
	}
	views := make([]adminapp.TaskLogView, 0, len(tasks))
	for _, task := range tasks {
		views = append(views, safeTaskLog(task, false))
	}
	return adminapp.ListResponse[adminapp.TaskLogView]{Data: views}, nil
}

// GetTaskLog returns one operator-safe task detail by task id.
func (s *Service) GetTaskLog(ctx context.Context, actor adminapp.Actor, taskID string) (adminapp.TaskLogDetail, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || strings.Contains(taskID, "/") {
		return adminapp.TaskLogDetail{}, apperr.NotFound("task log not found")
	}
	if err := s.Authorize(actor, "read", "task"); err != nil {
		return adminapp.TaskLogDetail{}, err
	}
	if s.tasks == nil {
		return adminapp.TaskLogDetail{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	task, ok, err := s.tasks.GetTask(ctx, taskID)
	if err != nil {
		return adminapp.TaskLogDetail{}, err
	}
	if !ok {
		return adminapp.TaskLogDetail{}, apperr.NotFound("task log not found")
	}
	return adminapp.TaskLogDetail{Task: safeTaskLog(*task, true)}, nil
}

func usageLogView(row reporting.UsageLogRow) adminapp.UsageLogView {
	return adminapp.UsageLogView{
		RequestID:          row.RequestID,
		TenantID:           row.TenantID,
		ProjectID:          row.ProjectID,
		APIKeyID:           row.APIKeyID,
		Model:              row.Model,
		ProviderType:       row.ProviderType,
		ChannelID:          row.ChannelID,
		Status:             row.Status,
		SettlementStatus:   row.SettlementStatus,
		LedgerEntryID:      row.LedgerEntryID,
		SettlementKind:     row.SettlementKind,
		InputTokens:        row.InputTokens,
		OutputTokens:       row.OutputTokens,
		TotalTokens:        row.TotalTokens,
		AmountMicros:       row.AmountMicros,
		Currency:           row.Currency,
		BalanceAfterMicros: row.BalanceAfterMicros,
		CreatedAt:          row.CreatedAt,
		SettledAt:          row.SettledAt,
	}
}

func safeTaskLog(task tasksvc.Task, includeMetadata bool) adminapp.TaskLogView {
	view := adminapp.TaskLogView{
		TaskID:             task.ID,
		RequestID:          task.RequestID,
		TenantID:           task.TenantID,
		ProjectID:          task.ProjectID,
		APIKeyID:           task.APIKeyID,
		Kind:               string(task.Kind),
		MediaType:          task.MediaType,
		Model:              task.Model,
		Status:             string(task.Status),
		Progress:           task.Progress,
		ProviderType:       task.ProviderType,
		ChannelID:          task.ChannelID,
		ProviderTaskID:     task.ProviderTaskID,
		CallbackConfigured: task.CallbackURL != "",
		SettlementStatus:   taskSettlementStatus(task),
		InputTokens:        task.Usage.InputTokens,
		OutputTokens:       task.Usage.OutputTokens,
		TotalTokens:        task.Usage.TotalTokens,
		ErrorCode:          task.ErrorCode,
		ErrorMessage:       safeTaskErrorMessage(task.ErrorMessage),
		CreatedAt:          task.CreatedAt,
		UpdatedAt:          task.UpdatedAt,
		CompletedAt:        task.CompletedAt,
	}
	if includeMetadata {
		view.Metadata = safeStringMetadata(task.Metadata)
	}
	return view
}

func taskSettlementStatus(task tasksvc.Task) string {
	if task.Usage.TotalTokens > 0 {
		return "settled"
	}
	if task.BalanceHoldID != "" && !tasksvc.IsTerminal(task.Status) {
		return "pending"
	}
	if task.BalanceHoldID != "" && tasksvc.IsTerminal(task.Status) {
		return "terminal_without_usage"
	}
	return "not_applicable"
}

func safeStringMetadata(metadata map[string]string) map[string]string {
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if sensitiveKey(key) {
			continue
		}
		out[key] = safeShort(value)
	}
	return out
}

func safeTaskErrorMessage(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "provider error redacted"
}
