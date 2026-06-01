package portal

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/billing/reporting"
	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	tasksvc "github.com/KnifeFly/token-gateway/internal/task"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

const (
	defaultPortalLimit = 50
	maxPortalLimit     = 200
)

// Service coordinates customer-facing Portal APIs within one tenant/project scope.
type Service struct {
	admin     *admin.Service
	reporting *reporting.Service
	tasks     tasksvc.Repository
	snapshots SnapshotRefresher
}

// SnapshotRefresher activates customer-visible control-plane changes in the runtime snapshot.
type SnapshotRefresher interface {
	RefreshSnapshot(ctx context.Context) error
}

// ServiceOption configures Portal service dependencies.
type ServiceOption func(*Service)

// WithSnapshotRefresher publishes and swaps the runtime snapshot after mutable Portal changes.
func WithSnapshotRefresher(refresher SnapshotRefresher) ServiceOption {
	return func(s *Service) {
		s.snapshots = refresher
	}
}

// NewService returns a Portal service.
func NewService(adminService *admin.Service, reportingService *reporting.Service, tasks tasksvc.Repository, opts ...ServiceOption) *Service {
	service := &Service{admin: adminService, reporting: reportingService, tasks: tasks}
	for _, opt := range opts {
		opt(service)
	}
	return service
}

// ListModels returns enabled models visible to the current principal.
func (s *Service) ListModels(snapshot engine.SnapshotView, principal Principal) (ModelListResponse, error) {
	if snapshot == nil {
		return ModelListResponse{}, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	models := snapshot.ListModels()
	out := make([]ModelSummary, 0, len(models))
	for _, model := range models {
		if !model.Enabled || !modelAllowedForView(principal.AllowedModels, model.PublicModel, model) {
			continue
		}
		out = append(out, modelSummary(model))
	}
	return ModelListResponse{Object: "list", Data: out}, nil
}

// GetModelSchema returns a model schema when visible to the current principal.
func (s *Service) GetModelSchema(snapshot engine.SnapshotView, principal Principal, modelName string) (ModelSchemaResponse, error) {
	if snapshot == nil {
		return ModelSchemaResponse{}, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	modelName = strings.Trim(modelName, "/ ")
	if modelName == "" || strings.Contains(modelName, "/") {
		return ModelSchemaResponse{}, apperr.NotFound("model schema not found")
	}
	model, found := snapshot.LookupModel(modelName)
	if !found || !model.Enabled || !modelAllowedForView(principal.AllowedModels, modelName, model) {
		return ModelSchemaResponse{}, apperr.NotFound("model not found")
	}
	return ModelSchemaResponse{Model: model.PublicModel, Version: snapshot.Ref().Version, Schema: modelSchema(model)}, nil
}

// Credits returns customer-facing balance and used-credit buckets.
func (s *Service) Credits(ctx context.Context, principal Principal, currency string) (CreditsResponse, error) {
	if s == nil || s.reporting == nil {
		return CreditsResponse{}, apperr.ConfigUnavailable("reporting service is unavailable")
	}
	report, err := s.reporting.TenantUsageReport(ctx, reporting.TenantUsageFilter{
		TenantID:  principal.TenantID,
		ProjectID: principal.ProjectID,
		Currency:  currency,
		Limit:     1,
	})
	if err != nil {
		return CreditsResponse{}, err
	}
	bucket := creditsBucket(report)
	return CreditsResponse{
		Success: true,
		Message: "ok",
		Data: map[string]CreditsBucket{
			"token":   bucket,
			"user":    bucket,
			"account": bucket,
		},
	}, nil
}

// Usage returns customer-visible usage and charge summaries.
func (s *Service) Usage(ctx context.Context, principal Principal, filter reporting.TenantUsageFilter) (UsageResponse, error) {
	if s == nil || s.reporting == nil {
		return UsageResponse{}, apperr.ConfigUnavailable("reporting service is unavailable")
	}
	filter.TenantID = principal.TenantID
	filter.ProjectID = principal.ProjectID
	filter.Limit = normalizeLimit(filter.Limit)
	report, err := s.reporting.TenantUsageReport(ctx, filter)
	if err != nil {
		return UsageResponse{}, err
	}
	return usageResponse(report), nil
}

// ListAPIKeys returns safe API key metadata for the current tenant/project.
func (s *Service) ListAPIKeys(ctx context.Context, principal Principal) (APIKeyListResponse, error) {
	if s == nil || s.admin == nil {
		return APIKeyListResponse{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	keys, err := s.admin.ListAPIKeys(ctx, principal.TenantID, principal.ProjectID)
	if err != nil {
		return APIKeyListResponse{}, err
	}
	out := make([]APIKey, 0, len(keys))
	for _, key := range keys {
		if key.TenantID != principal.TenantID || key.ProjectID != principal.ProjectID {
			continue
		}
		out = append(out, safeAPIKey(key))
	}
	return APIKeyListResponse{Data: out}, nil
}

// CreateAPIKey creates a derived key scoped to the current tenant/project.
func (s *Service) CreateAPIKey(ctx context.Context, principal Principal, request APIKeyCreateRequest) (APIKeyCreateResponse, error) {
	if s == nil || s.admin == nil {
		return APIKeyCreateResponse{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	allowedModels := cleanStrings(request.AllowedModels)
	if len(allowedModels) == 0 {
		allowedModels = cleanStrings(principal.AllowedModels)
	}
	if !allowedModelsSubset(principal.AllowedModels, allowedModels) {
		return APIKeyCreateResponse{}, apperr.Forbidden("allowed_models cannot exceed current api key permissions")
	}
	key, err := s.admin.CreateAPIKey(ctx, admin.APIKey{
		TenantID:      principal.TenantID,
		ProjectID:     principal.ProjectID,
		Name:          strings.TrimSpace(request.Name),
		AllowedModels: allowedModels,
	})
	if err != nil {
		return APIKeyCreateResponse{}, err
	}
	if err := s.refreshSnapshot(ctx); err != nil {
		return APIKeyCreateResponse{}, err
	}
	return APIKeyCreateResponse{APIKey: safeAPIKey(*key), PlaintextKey: key.PlaintextKey}, nil
}

// DisableAPIKey disables a same-tenant/project key other than the current caller key.
func (s *Service) DisableAPIKey(ctx context.Context, principal Principal, keyID string) (APIKey, error) {
	if s == nil || s.admin == nil {
		return APIKey{}, apperr.ConfigUnavailable("admin service is unavailable")
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return APIKey{}, apperr.InvalidArgument("api key id is required")
	}
	if keyID == principal.APIKeyID {
		return APIKey{}, apperr.Forbidden("current api key cannot disable itself")
	}
	keys, err := s.admin.ListAPIKeys(ctx, principal.TenantID, principal.ProjectID)
	if err != nil {
		return APIKey{}, err
	}
	var found bool
	for _, key := range keys {
		if key.ID == keyID && key.TenantID == principal.TenantID && key.ProjectID == principal.ProjectID {
			found = true
			break
		}
	}
	if !found {
		return APIKey{}, apperr.NotFound("api key not found")
	}
	disabled, err := s.admin.DisableAPIKey(ctx, keyID)
	if err != nil {
		return APIKey{}, err
	}
	if err := s.refreshSnapshot(ctx); err != nil {
		return APIKey{}, err
	}
	return safeAPIKey(*disabled), nil
}

// ListTasks returns current tenant/project async tasks.
func (s *Service) ListTasks(ctx context.Context, principal Principal, status string, limit int, cursor string) (TaskListResponse, error) {
	if s == nil || s.tasks == nil {
		return TaskListResponse{}, apperr.ConfigUnavailable("task repository is unavailable")
	}
	limit = normalizeLimit(limit)
	tasks, err := s.tasks.ListTasks(ctx, tasksvc.TaskListFilter{
		TenantID:  principal.TenantID,
		ProjectID: principal.ProjectID,
		Status:    tasksvc.Status(strings.TrimSpace(status)),
		Cursor:    strings.TrimSpace(cursor),
		Limit:     limit + 1,
	})
	if err != nil {
		return TaskListResponse{}, err
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
	return TaskListResponse{Data: out, NextCursor: nextCursor}, nil
}

// GetTask returns one tenant/project scoped task.
func (s *Service) GetTask(ctx context.Context, principal Principal, taskID string) (map[string]any, error) {
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

func modelSummary(model engine.ModelView) ModelSummary {
	return ModelSummary{
		ID:               model.PublicModel,
		Object:           "model",
		Type:             modelType(model),
		Category:         model.Category,
		DisplayName:      displayName(model),
		Description:      model.Description,
		Aliases:          append([]string(nil), model.Aliases...),
		Tags:             append([]string(nil), model.Tags...),
		ProviderFamily:   model.ProviderFamily,
		Owner:            "platform",
		Capabilities:     capabilities(model),
		InputModalities:  inputModalities(model),
		OutputModalities: outputModalities(model),
		ContextWindow:    model.ContextWindow,
		MaxOutputTokens:  model.MaxOutputTokens,
		Status:           model.Status,
		Async:            model.Protocol == engine.ProtocolUnified,
		Deprecated:       model.Deprecated,
	}
}

func capabilities(model engine.ModelView) []string {
	if len(model.Capabilities) > 0 {
		out := append([]string(nil), model.Capabilities...)
		sort.Strings(out)
		return out
	}
	if model.Capability == "" {
		return []string{string(model.Protocol)}
	}
	parts := strings.FieldsFunc(model.Capability, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		out = append(out, model.Capability)
	}
	sort.Strings(out)
	return out
}

func modelType(model engine.ModelView) string {
	value := strings.ToLower(model.Capability + " " + model.PublicModel)
	switch {
	case strings.Contains(value, "moderation"):
		return "moderation"
	case strings.Contains(value, "embedding"):
		return "embedding"
	case strings.Contains(value, "image"):
		return "image"
	case strings.Contains(value, "video"):
		return "video"
	case strings.Contains(value, "audio") || strings.Contains(value, "speech") || strings.Contains(value, "transcription"):
		return "audio"
	case strings.Contains(value, "multi"):
		return "multimodal"
	default:
		return "text"
	}
}

func inputModalities(model engine.ModelView) []string {
	if len(model.Modalities) > 0 {
		return append([]string(nil), model.Modalities...)
	}
	switch modelType(model) {
	case "image":
		return []string{"text", "image"}
	case "video":
		return []string{"text", "image"}
	case "audio":
		return []string{"text", "audio"}
	default:
		return []string{"text"}
	}
}

func outputModalities(model engine.ModelView) []string {
	if len(model.Modalities) > 0 {
		return append([]string(nil), model.Modalities...)
	}
	switch modelType(model) {
	case "image":
		return []string{"image"}
	case "video":
		return []string{"video"}
	case "audio":
		return []string{"audio", "text"}
	case "embedding":
		return []string{"embedding"}
	case "moderation":
		return []string{"moderation"}
	default:
		return []string{"text"}
	}
}

func modelSchema(model engine.ModelView) map[string]any {
	if len(model.Schema) > 0 && string(model.Schema) != "{}" {
		var schema map[string]any
		if err := json.Unmarshal(model.Schema, &schema); err == nil && len(schema) > 0 {
			return schema
		}
	}
	return map[string]any{
		"type":     "object",
		"required": []string{"model"},
		"properties": map[string]any{
			"model": map[string]any{
				"type":  "string",
				"const": model.PublicModel,
			},
			"model_params": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		},
	}
}

func displayName(model engine.ModelView) string {
	if model.DisplayName != "" {
		return model.DisplayName
	}
	return model.PublicModel
}

func creditsBucket(report *reporting.TenantUsageReport) CreditsBucket {
	var remaining, held int64
	currency := ""
	if report != nil {
		for _, balance := range report.Balances {
			remaining += balance.AvailableMicros
			held += balance.HeldMicros
			if currency == "" {
				currency = balance.Currency
			}
		}
		if currency == "" {
			currency = report.Totals.Currency
		}
	}
	if currency == "" {
		currency = "USD"
	}
	used := int64(0)
	if report != nil {
		used = report.Totals.RevenueMicros
	}
	return CreditsBucket{
		RemainingCredits: microsToCredits(remaining),
		UsedCredits:      microsToCredits(used),
		HeldCredits:      microsToCredits(held),
		UnlimitedCredits: false,
		Currency:         currency,
	}
}

func usageResponse(report *reporting.TenantUsageReport) UsageResponse {
	response := UsageResponse{NextCursor: nil}
	if report == nil {
		return response
	}
	response.GeneratedAt = report.GeneratedAt
	response.Currency = report.Totals.Currency
	response.Totals = UsageTotals{
		Requests:     report.Totals.Requests,
		InputTokens:  report.Totals.InputTokens,
		OutputTokens: report.Totals.OutputTokens,
		CreditsUsed:  microsToCredits(report.Totals.RevenueMicros),
	}
	for _, row := range report.Usage {
		response.Items = append(response.Items, UsageItem{
			Model:        row.Model,
			Capability:   row.ProviderType,
			Status:       "settled",
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			CreditsUsed:  microsToCredits(row.AmountMicros),
		})
	}
	if len(response.Items) == 0 {
		for _, line := range report.Ledger {
			response.Items = append(response.Items, UsageItem{
				RequestID:   line.RequestID,
				Status:      line.SettlementKind,
				CreditsUsed: microsToCredits(-line.AmountMicros),
				CreatedAt:   line.CreatedAt,
			})
		}
	}
	if response.Currency == "" && len(response.Items) > 0 {
		response.Currency = report.Filter.Currency
	}
	return response
}

func safeAPIKey(key admin.APIKey) APIKey {
	return APIKey{
		ID:            key.ID,
		Name:          key.Name,
		Enabled:       key.Enabled,
		AllowedModels: append([]string(nil), key.AllowedModels...),
		CreatedAt:     key.CreatedAt,
		RevokedAt:     key.RevokedAt,
	}
}

func safeTaskObject(task *tasksvc.Task) map[string]any {
	object := tasksvc.TaskObject(task)
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

func microsToCredits(value int64) float64 {
	return float64(value) / 1_000_000
}

func allowedModelsSubset(parent []string, child []string) bool {
	if modelAllowed(parent, "*") {
		return true
	}
	for _, model := range child {
		if !modelAllowed(parent, model) {
			return false
		}
	}
	return true
}

func modelAllowed(allowed []string, model string) bool {
	model = strings.TrimSpace(model)
	for _, value := range allowed {
		value = strings.TrimSpace(value)
		if value == "*" || value == model {
			return true
		}
	}
	return false
}

func modelAllowedForView(allowed []string, requested string, model engine.ModelView) bool {
	if modelAllowed(allowed, model.PublicModel) || modelAllowed(allowed, requested) {
		return true
	}
	for _, alias := range model.Aliases {
		if modelAllowed(allowed, alias) {
			return true
		}
	}
	return false
}

func cleanStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
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

func (s *Service) refreshSnapshot(ctx context.Context) error {
	if s == nil || s.snapshots == nil {
		return nil
	}
	return s.snapshots.RefreshSnapshot(ctx)
}
