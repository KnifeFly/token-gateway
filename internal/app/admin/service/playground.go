package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// RunPlayground validates an Admin-scoped payload and returns safe execution metadata.
func (s *Service) RunPlayground(ctx context.Context, actor adminapp.Actor, request adminapp.PlaygroundRunRequest, opts adminapp.MutationOptions) (adminapp.PlaygroundRunResult, error) {
	auditRequest := playgroundAuditRequest{
		Model:     request.Model,
		ChannelID: request.ChannelID,
		Mode:      request.Mode,
		Stream:    request.Stream,
		Debug:     request.Debug,
		Payload:   request.Payload,
	}
	return mutate(ctx, s, actor, opts, "write", "playground", strings.TrimSpace(request.Model), auditRequest, func() (adminapp.PlaygroundRunResult, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.PlaygroundRunResult{}, err
		}
		return s.adminPlaygroundDryRun(request, cfg), nil
	})
}

// PreviewPlaygroundImport returns sanitized import metadata without executing a payload.
func (s *Service) PreviewPlaygroundImport(ctx context.Context, actor adminapp.Actor, request adminapp.PlaygroundImportPreviewRequest, opts adminapp.MutationOptions) (adminapp.PlaygroundImportPreview, error) {
	return mutate(ctx, s, actor, opts, "write", "playground", "import", request, func() (adminapp.PlaygroundImportPreview, error) {
		payload, fields, redacted, err := sanitizePlaygroundPayload(request.Payload, "")
		if err != nil {
			return adminapp.PlaygroundImportPreview{}, err
		}
		model, _ := payloadString(payload, "model")
		mode, _ := payloadString(payload, "mode")
		return adminapp.PlaygroundImportPreview{
			Model:          model,
			Mode:           normalizePlaygroundMode(mode),
			Payload:        playgroundJSON(payload),
			PayloadFields:  fields,
			RedactedFields: redacted,
			Safe:           len(redacted) == 0,
			Message:        "导入预览已脱敏，未执行请求。",
			PreviewedAt:    s.now(),
		}, nil
	})
}

// ExportPlayground returns a sanitized portable payload without secret or raw debug fields.
func (s *Service) ExportPlayground(ctx context.Context, actor adminapp.Actor, request adminapp.PlaygroundExportRequest, opts adminapp.MutationOptions) (adminapp.PlaygroundExport, error) {
	return mutate(ctx, s, actor, opts, "write", "playground", strings.TrimSpace(request.Model), request, func() (adminapp.PlaygroundExport, error) {
		model := strings.TrimSpace(request.Model)
		if model == "" {
			return adminapp.PlaygroundExport{}, apperr.InvalidArgument("model is required")
		}
		payload, _, redacted, err := sanitizePlaygroundPayload(request.Payload, model)
		if err != nil {
			return adminapp.PlaygroundExport{}, err
		}
		return adminapp.PlaygroundExport{
			Model:         model,
			Mode:          normalizePlaygroundMode(request.Mode),
			Payload:       playgroundJSON(payload),
			OmittedFields: redacted,
			ExportedAt:    s.now(),
		}, nil
	})
}

type playgroundAuditRequest struct {
	Model     string          `json:"model"`
	ChannelID string          `json:"channel_id,omitempty"`
	Mode      string          `json:"mode,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
	Debug     bool            `json:"debug,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func (s *Service) adminPlaygroundDryRun(request adminapp.PlaygroundRunRequest, cfg *configadmin.SnapshotConfig) adminapp.PlaygroundRunResult {
	start := time.Now()
	modelID := strings.TrimSpace(request.Model)
	payload, fields, payloadErr := parsePlaygroundPayload(request.Payload, modelID)
	if payloadErr == nil && modelID == "" {
		modelID, _ = payloadString(payload, "model")
	}
	result := adminapp.PlaygroundRunResult{
		RequestID:     newID("pg"),
		Scope:         "admin",
		Status:        "ready",
		Message:       "结构校验通过，已完成安全校验，未调用上游。",
		Model:         modelID,
		Mode:          normalizePlaygroundMode(request.Mode),
		Stream:        request.Stream,
		PayloadFields: safePayloadFields(fields),
		Debug: adminapp.PlaygroundDebug{
			LatencyMillis: elapsedMillis(start),
			Usage:         estimatePlaygroundUsage(request.Payload, fields),
		},
		Result: adminapp.PlaygroundSafeResult{
			Object:  "playground.dry_run",
			Summary: "安全校验已完成，结果不包含原始提示词、响应或凭证。",
		},
		RanAt: s.now(),
	}
	if payloadErr != nil {
		result.Status = "invalid"
		result.Message = "请求 JSON 无法解析。"
		result.Debug.SafeErrorCode = "invalid_payload"
		result.Debug.SafeErrorMessage = "请求 JSON 无法解析。"
		return result
	}
	model, ok := findModel(cfg.Models, modelID)
	if !ok || !model.Enabled {
		result.Status = "invalid"
		result.Message = "模型不存在或未启用。"
		result.Debug.SafeErrorCode = "model_not_available"
		result.Debug.SafeErrorMessage = "模型不存在或未启用。"
		return result
	}
	result.Schema = playgroundSchemaSummary(configModelSchema(model), payload)
	if len(result.Schema.MissingRequired) > 0 {
		result.Status = "invalid"
		result.Message = "请求缺少结构定义要求的字段。"
		result.Debug.SafeErrorCode = "schema_required_missing"
		result.Debug.SafeErrorMessage = "请求缺少结构定义要求的字段。"
		return result
	}
	routeID, channel, channelOK := adminPlaygroundChannel(model.PublicModel, strings.TrimSpace(request.ChannelID), cfg)
	result.Debug.RouteID = routeID
	if !channelOK {
		result.Status = "warning"
		result.Message = "结构校验通过，但没有可用渠道候选。"
		result.Debug.SafeErrorCode = "route_candidate_missing"
		result.Debug.SafeErrorMessage = "没有可用渠道候选。"
		return result
	}
	channelTest := channelReadiness(channel, s.now())
	result.Debug.ChannelID = channel.ID
	result.Debug.ProviderType = channel.ProviderType
	result.Debug.ChannelTest = &channelTest
	if channelTest.Status != "ready" {
		result.Status = "warning"
		result.Message = "结构校验通过，但渠道测试需要处理。"
		result.Debug.SafeErrorCode = "channel_not_ready"
		result.Debug.SafeErrorMessage = "渠道测试需要处理。"
	}
	return result
}

func channelReadiness(channel configadmin.ChannelConfig, testedAt time.Time) adminapp.ChannelTestResult {
	status := "ready"
	message := "渠道配置已通过供应商测试前置检查。"
	if !channel.Enabled {
		status = "disabled"
		message = "渠道已停用。"
	} else if !credentialConfigured(channel) {
		status = "warning"
		message = "供应商凭证未配置。"
	} else if len(channel.Models) == 0 {
		status = "warning"
		message = "渠道没有模型覆盖。"
	}
	return adminapp.ChannelTestResult{
		ChannelID:            channel.ID,
		Status:               status,
		Message:              message,
		CredentialConfigured: credentialConfigured(channel),
		ModelCount:           len(channel.Models),
		TestedAt:             testedAt,
	}
}

func adminPlaygroundChannel(modelID string, channelID string, cfg *configadmin.SnapshotConfig) (string, configadmin.ChannelConfig, bool) {
	if channelID != "" {
		channel, ok := findChannel(cfg.Channels, channelID)
		if !ok || !channelSupportsModel(channel, modelID) {
			return "", configadmin.ChannelConfig{}, false
		}
		return "", channel, true
	}
	for _, route := range cfg.Routes {
		if route.PublicModel != modelID {
			continue
		}
		candidates := append([]configadmin.RouteCandidate(nil), route.Candidates...)
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Priority != candidates[j].Priority {
				return candidates[i].Priority < candidates[j].Priority
			}
			return candidates[i].ChannelID < candidates[j].ChannelID
		})
		for _, candidate := range candidates {
			channel, ok := findChannel(cfg.Channels, candidate.ChannelID)
			if ok && channelSupportsModel(channel, modelID) {
				return route.ID, channel, true
			}
		}
	}
	for _, channel := range cfg.Channels {
		if channelSupportsModel(channel, modelID) {
			return "", channel, true
		}
	}
	return "", configadmin.ChannelConfig{}, false
}

func channelSupportsModel(channel configadmin.ChannelConfig, modelID string) bool {
	for _, model := range channel.Models {
		if model.PublicModel == modelID {
			return true
		}
	}
	return false
}

func sanitizePlaygroundPayload(raw json.RawMessage, modelID string) (map[string]any, []string, []string, error) {
	payload, fields, err := parsePlaygroundPayload(raw, modelID)
	if err != nil {
		return nil, nil, nil, err
	}
	redacted := make([]string, 0)
	for _, field := range fields {
		if playgroundSensitiveField(field) {
			delete(payload, field)
			redacted = append(redacted, field)
		}
	}
	if strings.TrimSpace(modelID) != "" {
		payload["model"] = modelID
	}
	return payload, sortedPayloadFields(payload), redacted, nil
}

func parsePlaygroundPayload(raw json.RawMessage, modelID string) (map[string]any, []string, error) {
	payload := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, nil, apperr.InvalidArgument("payload must be a json object", apperr.WithCause(err))
		}
	}
	if strings.TrimSpace(modelID) != "" {
		if existing, ok := payloadString(payload, "model"); ok && existing != modelID {
			return nil, nil, apperr.InvalidArgument("payload model must match request model")
		}
		payload["model"] = modelID
	}
	return payload, sortedPayloadFields(payload), nil
}

func payloadString(payload map[string]any, field string) (string, bool) {
	value, ok := payload[field]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return strings.TrimSpace(text), ok
}

func sortedPayloadFields(payload map[string]any) []string {
	fields := make([]string, 0, len(payload))
	for field := range payload {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func safePayloadFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if !playgroundSensitiveField(field) {
			out = append(out, field)
		}
	}
	return out
}

func playgroundSensitiveField(field string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	if sensitiveKey(field) {
		return true
	}
	switch field {
	case "messages", "input", "inputs", "content", "image", "images", "audio", "file", "files", "url", "base64", "response":
		return true
	default:
		return strings.Contains(field, "prompt") ||
			strings.Contains(field, "secret") ||
			strings.Contains(field, "url") ||
			strings.Contains(field, "callback")
	}
}

func playgroundJSON(payload map[string]any) json.RawMessage {
	content, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return content
}

func normalizePlaygroundMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "chat", "responses", "embeddings", "images", "audio":
		return value
	default:
		return "chat"
	}
}

func playgroundSchemaSummary(schema map[string]any, payload map[string]any) adminapp.PlaygroundSchemaSummary {
	required := schemaStringList(schema["required"])
	properties := schemaProperties(schema)
	missing := make([]string, 0)
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			missing = append(missing, field)
		}
	}
	return adminapp.PlaygroundSchemaSummary{
		Required:        required,
		AcceptedFields:  properties,
		MissingRequired: missing,
	}
}

func schemaStringList(value any) []string {
	var out []string
	switch typed := value.(type) {
	case []string:
		out = append(out, typed...)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
	}
	sort.Strings(out)
	return out
}

func schemaProperties(schema map[string]any) []string {
	raw, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	fields := make([]string, 0, len(raw))
	for field := range raw {
		if !playgroundSensitiveField(field) {
			fields = append(fields, field)
		}
	}
	sort.Strings(fields)
	return fields
}

func estimatePlaygroundUsage(raw json.RawMessage, fields []string) adminapp.PlaygroundUsage {
	input := len(raw) / 4
	if input < len(fields)*4 {
		input = len(fields) * 4
	}
	return adminapp.PlaygroundUsage{
		InputTokens:  input,
		OutputTokens: 0,
		TotalTokens:  input,
	}
}

func elapsedMillis(start time.Time) int64 {
	elapsed := time.Since(start).Milliseconds()
	if elapsed < 1 {
		return 1
	}
	return elapsed
}
