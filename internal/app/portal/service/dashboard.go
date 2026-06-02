package service

import (
	"context"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
)

// Dashboard returns Portal dashboard read models scoped to the current project.
func (s *Service) Dashboard(ctx context.Context, principal portalapp.Principal) (portalapp.Dashboard, error) {
	credits, err := s.Credits(ctx, principal, "")
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	usage, err := s.Usage(ctx, principal, portalapp.UsageFilter{Limit: 5})
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	apiKeys, err := s.ListAPIKeys(ctx, principal)
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	tasks, err := s.ListTasks(ctx, principal, "", 5, "")
	if err != nil {
		return portalapp.Dashboard{}, err
	}
	return portalapp.Dashboard{
		GeneratedAt:    s.now(),
		Credits:        credits,
		Usage:          usage,
		APIKeyCount:    len(apiKeys.Data),
		ActiveKeyCount: activeAPIKeyCount(apiKeys.Data),
		TaskSummary:    summarizeTasks(tasks.Data),
		RecentTasks:    tasks.Data,
	}, nil
}

// Onboarding returns the current first-run checklist.
func (s *Service) Onboarding(ctx context.Context, principal portalapp.Principal) (portalapp.OnboardingState, error) {
	keys, err := s.ListAPIKeys(ctx, principal)
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	models, err := s.ListModels(ctx, principal)
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	usage, err := s.Usage(ctx, principal, portalapp.UsageFilter{Limit: 1})
	if err != nil {
		return portalapp.OnboardingState{}, err
	}
	return portalapp.OnboardingState{
		GeneratedAt: s.now(),
		Steps: []portalapp.OnboardingStep{
			{ID: "login", Title: "Sign in", Complete: true},
			{ID: "models", Title: "Review available models", Complete: len(models.Data) > 0},
			{ID: "api_keys", Title: "Create a derived API key", Complete: len(keys.Data) > 1},
			{ID: "first_request", Title: "Send first request", Complete: usage.Totals.Requests > 0},
		},
	}, nil
}

// ProjectSettings returns safe project-scoped settings.
func (s *Service) ProjectSettings(principal portalapp.Principal) portalapp.ProjectSettings {
	return portalapp.ProjectSettings{
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		APIKeyID:      principal.APIKeyID,
		AllowedModels: append([]string(nil), principal.AllowedModels...),
		GeneratedAt:   s.now(),
	}
}

func activeAPIKeyCount(keys []portalapp.APIKey) int {
	var count int
	for _, key := range keys {
		if key.Enabled && key.RevokedAt == nil {
			count++
		}
	}
	return count
}

func summarizeTasks(tasks []map[string]any) portalapp.TaskSummary {
	summary := portalapp.TaskSummary{Total: len(tasks)}
	for _, task := range tasks {
		switch strings.ToLower(strings.TrimSpace(anyString(task["status"]))) {
		case "queued", "pending":
			summary.Queued++
		case "processing", "running":
			summary.Processing++
		case "completed", "succeeded":
			summary.Completed++
		case "failed", "cancelled", "canceled", "expired":
			summary.Failed++
		}
	}
	return summary
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
