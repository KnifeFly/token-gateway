package service

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListModels returns customer-visible models.
func (s *Service) ListModels(ctx context.Context, principal portalapp.Principal) (portalapp.ModelListResponse, error) {
	snapshot, err := s.currentSnapshot(ctx)
	if err != nil {
		return portalapp.ModelListResponse{}, err
	}

	models := snapshot.ListModels()
	out := make([]portalapp.ModelSummary, 0, len(models))
	for _, model := range models {
		if !model.Enabled || !modelAllowedForView(principal.AllowedModels, model.PublicModel, model) {
			continue
		}
		out = append(out, modelSummary(model))
	}
	return portalapp.ModelListResponse{Object: "list", Data: out}, nil
}

// GetModelSchema returns a visible model schema.
func (s *Service) GetModelSchema(ctx context.Context, principal portalapp.Principal, modelName string) (portalapp.ModelSchemaResponse, error) {
	snapshot, err := s.currentSnapshot(ctx)
	if err != nil {
		return portalapp.ModelSchemaResponse{}, err
	}
	modelName = strings.Trim(modelName, "/ ")
	if modelName == "" || strings.Contains(modelName, "/") {
		return portalapp.ModelSchemaResponse{}, apperr.NotFound("model schema not found")
	}
	model, found := snapshot.LookupModel(modelName)
	if !found || !model.Enabled || !modelAllowedForView(principal.AllowedModels, modelName, model) {
		return portalapp.ModelSchemaResponse{}, apperr.NotFound("model not found")
	}
	return portalapp.ModelSchemaResponse{Model: model.PublicModel, Version: snapshot.Ref().Version, Schema: modelSchema(model)}, nil
}

func modelSummary(model engine.ModelView) portalapp.ModelSummary {
	return portalapp.ModelSummary{
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

func (s *Service) currentSnapshot(ctx context.Context) (engine.SnapshotView, error) {
	if s == nil || s.snapshot == nil {
		return nil, apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	state := &engine.RequestState{Incoming: engine.IncomingRequest{Header: http.Header{}}, Metadata: map[string]string{}, Internal: map[string]any{}}
	if err := s.snapshot.Attach(ctx, state); err != nil {
		return nil, err
	}
	return state.Snapshot, nil
}
