package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// RunPlayground validates a Portal-scoped payload and returns safe execution metadata.
func (s *Service) RunPlayground(ctx context.Context, principal portalapp.Principal, request portalapp.PlaygroundRunRequest) (portalapp.PlaygroundRunResult, error) {
	start := time.Now()
	snapshot, err := s.currentSnapshot(ctx)
	if err != nil {
		return portalapp.PlaygroundRunResult{}, err
	}
	modelID := strings.TrimSpace(request.Model)
	payload, fields, payloadErr := parsePlaygroundPayload(request.Payload, modelID)
	if payloadErr == nil && modelID == "" {
		modelID, _ = payloadString(payload, "model")
	}
	result := portalapp.PlaygroundRunResult{
		RequestID:     "pg_" + s.now().Format("20060102150405.000000000"),
		Scope:         "portal",
		Status:        "ready",
		Message:       "结构校验通过，已完成安全校验，未调用上游。",
		Model:         modelID,
		Mode:          normalizePlaygroundMode(request.Mode),
		Stream:        request.Stream,
		PayloadFields: safePayloadFields(fields),
		Debug: portalapp.PlaygroundDebug{
			LatencyMillis: elapsedMillis(start),
			Usage:         estimatePlaygroundUsage(request.Payload, fields),
		},
		Result: portalapp.PlaygroundSafeResult{
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
		return result, nil
	}
	model, ok := snapshot.LookupModel(modelID)
	if !ok || !model.Enabled || !modelAllowedForView(principal.AllowedModels, modelID, model) {
		result.Status = "invalid"
		result.Message = "模型不在当前项目范围内。"
		result.Debug.SafeErrorCode = "model_not_available"
		result.Debug.SafeErrorMessage = "模型不在当前项目范围内。"
		return result, nil
	}
	result.Schema = playgroundSchemaSummary(modelSchema(model), payload)
	if len(result.Schema.MissingRequired) > 0 {
		result.Status = "invalid"
		result.Message = "请求缺少结构定义要求的字段。"
		result.Debug.SafeErrorCode = "schema_required_missing"
		result.Debug.SafeErrorMessage = "请求缺少结构定义要求的字段。"
		return result, nil
	}
	route, channel, ok := portalPlaygroundChannel(snapshot, model.PublicModel)
	if !ok {
		result.Status = "warning"
		result.Message = "结构校验通过，但没有可用渠道候选。"
		result.Debug.SafeErrorCode = "route_candidate_missing"
		result.Debug.SafeErrorMessage = "没有可用渠道候选。"
		return result, nil
	}
	result.Debug.RouteID = route.ID
	result.Debug.ChannelID = channel.ID
	result.Debug.ProviderType = channel.ProviderType
	if !channel.Enabled {
		result.Status = "warning"
		result.Message = "结构校验通过，但候选渠道当前未启用。"
		result.Debug.SafeErrorCode = "channel_disabled"
		result.Debug.SafeErrorMessage = "候选渠道当前未启用。"
	}
	return result, nil
}

func portalPlaygroundChannel(snapshot engine.SnapshotView, modelID string) (engine.RoutePolicyView, engine.ChannelView, bool) {
	route, ok := snapshot.LookupRoute(modelID)
	if !ok {
		return engine.RoutePolicyView{}, engine.ChannelView{}, false
	}
	candidates := append([]engine.RouteCandidateView(nil), route.Candidates...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority < candidates[j].Priority
		}
		return candidates[i].ChannelID < candidates[j].ChannelID
	})
	for _, candidate := range candidates {
		channel, ok := snapshot.LookupChannel(candidate.ChannelID)
		if ok {
			return route, channel, true
		}
	}
	return engine.RoutePolicyView{}, engine.ChannelView{}, false
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
	switch field {
	case "password", "api_key", "key_hash", "plaintext_key", "encrypted_api_key", "access_token", "refresh_token", "prompt", "response", "payload":
		return true
	case "messages", "input", "inputs", "content", "image", "images", "audio", "file", "files", "url", "base64":
		return true
	default:
		return strings.Contains(field, "secret") ||
			strings.Contains(field, "credential") ||
			strings.Contains(field, "prompt") ||
			strings.Contains(field, "url") ||
			strings.Contains(field, "callback")
	}
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

func playgroundSchemaSummary(schema map[string]any, payload map[string]any) portalapp.PlaygroundSchemaSummary {
	required := schemaStringList(schema["required"])
	properties := schemaProperties(schema)
	missing := make([]string, 0)
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			missing = append(missing, field)
		}
	}
	return portalapp.PlaygroundSchemaSummary{
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

func estimatePlaygroundUsage(raw json.RawMessage, fields []string) portalapp.PlaygroundUsage {
	input := len(raw) / 4
	if input < len(fields)*4 {
		input = len(fields) * 4
	}
	return portalapp.PlaygroundUsage{
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
