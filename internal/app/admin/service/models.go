package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// ListModels returns safe public model catalog read models.
func (s *Service) ListModels(ctx context.Context, actor adminapp.Actor) (adminapp.ListResponse[adminapp.ModelView], error) {
	if err := s.Authorize(actor, "read", "model"); err != nil {
		return adminapp.ListResponse[adminapp.ModelView]{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ListResponse[adminapp.ModelView]{}, err
	}
	views := make([]adminapp.ModelView, 0, len(cfg.Models))
	for _, model := range cfg.Models {
		views = append(views, safeModel(model, cfg))
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].SortOrder != views[j].SortOrder {
			return views[i].SortOrder < views[j].SortOrder
		}
		return views[i].PublicModel < views[j].PublicModel
	})
	return adminapp.ListResponse[adminapp.ModelView]{Data: views}, nil
}

// GetModel returns one safe model catalog read model.
func (s *Service) GetModel(ctx context.Context, actor adminapp.Actor, modelID string) (adminapp.ModelView, error) {
	if err := s.Authorize(actor, "read", "model"); err != nil {
		return adminapp.ModelView{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ModelView{}, err
	}
	model, ok := findModel(cfg.Models, modelID)
	if !ok {
		return adminapp.ModelView{}, apperr.NotFound("model not found")
	}
	return safeModel(model, cfg), nil
}

// UpsertModel writes model configuration and returns a safe model read model.
func (s *Service) UpsertModel(ctx context.Context, actor adminapp.Actor, request configadmin.ModelConfig, opts adminapp.MutationOptions) (adminapp.ModelView, error) {
	return mutate(ctx, s, actor, opts, "write", "model", request.PublicModel, request, func() (adminapp.ModelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		updated, err := s.owner.UpsertModel(ctx, request)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		return safeModel(*updated, cfg), nil
	})
}

// PatchModel updates selected model fields while preserving existing catalog values.
func (s *Service) PatchModel(ctx context.Context, actor adminapp.Actor, modelID string, request configadmin.ModelConfig, opts adminapp.MutationOptions) (adminapp.ModelView, error) {
	request.PublicModel = strings.TrimSpace(modelID)
	return mutate(ctx, s, actor, opts, "write", "model", modelID, request, func() (adminapp.ModelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		current, ok := findModel(cfg.Models, modelID)
		if !ok {
			return adminapp.ModelView{}, apperr.NotFound("model not found")
		}
		merged := mergeModel(current, request)
		updated, err := s.owner.UpsertModel(ctx, merged)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		return safeModel(*updated, cfg), nil
	})
}

// SetModelEnabled updates a model enabled flag through the control-plane owner service.
func (s *Service) SetModelEnabled(ctx context.Context, actor adminapp.Actor, modelID string, enabled bool, opts adminapp.MutationOptions) (adminapp.ModelView, error) {
	request := map[string]any{"id": modelID, "enabled": enabled}
	return mutate(ctx, s, actor, opts, "write", "model", modelID, request, func() (adminapp.ModelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		current, ok := findModel(cfg.Models, modelID)
		if !ok {
			return adminapp.ModelView{}, apperr.NotFound("model not found")
		}
		current.Enabled = enabled
		current.EnabledSet = true
		updated, err := s.owner.UpsertModel(ctx, current)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		return safeModel(*updated, cfg), nil
	})
}

// DeprecateModel marks a model deprecated without disabling existing access.
func (s *Service) DeprecateModel(ctx context.Context, actor adminapp.Actor, modelID string, opts adminapp.MutationOptions) (adminapp.ModelView, error) {
	request := map[string]any{"id": modelID, "deprecated": true}
	return mutate(ctx, s, actor, opts, "write", "model", modelID, request, func() (adminapp.ModelView, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		current, ok := findModel(cfg.Models, modelID)
		if !ok {
			return adminapp.ModelView{}, apperr.NotFound("model not found")
		}
		current.Deprecated = true
		current.Status = "deprecated"
		current.EnabledSet = true
		updated, err := s.owner.UpsertModel(ctx, current)
		if err != nil {
			return adminapp.ModelView{}, err
		}
		return safeModel(*updated, cfg), nil
	})
}

// ListModelChannels returns safe channel coverage for one public model.
func (s *Service) ListModelChannels(ctx context.Context, actor adminapp.Actor, modelID string) (adminapp.ListResponse[adminapp.ModelChannelCoverage], error) {
	model, err := s.GetModel(ctx, actor, modelID)
	if err != nil {
		return adminapp.ListResponse[adminapp.ModelChannelCoverage]{}, err
	}
	return adminapp.ListResponse[adminapp.ModelChannelCoverage]{Data: model.ChannelCoverage}, nil
}

// GetModelSchemaPreview returns a safe Admin schema preview for one public model.
func (s *Service) GetModelSchemaPreview(ctx context.Context, actor adminapp.Actor, modelID string) (adminapp.ModelSchemaPreview, error) {
	if err := s.Authorize(actor, "read", "model"); err != nil {
		return adminapp.ModelSchemaPreview{}, err
	}
	cfg, err := s.snapshotConfig(ctx)
	if err != nil {
		return adminapp.ModelSchemaPreview{}, err
	}
	model, ok := findModel(cfg.Models, modelID)
	if !ok {
		return adminapp.ModelSchemaPreview{}, apperr.NotFound("model not found")
	}
	return adminapp.ModelSchemaPreview{
		Model:   model.PublicModel,
		Version: "admin-config",
		Schema:  configModelSchema(model),
	}, nil
}

// PreviewModelCatalogSync returns non-persistent model catalog differences.
func (s *Service) PreviewModelCatalogSync(ctx context.Context, actor adminapp.Actor, request adminapp.ModelCatalogSyncPreviewRequest, opts adminapp.MutationOptions) (adminapp.ModelCatalogSyncPreview, error) {
	return mutate(ctx, s, actor, opts, "write", "model", "catalog", request, func() (adminapp.ModelCatalogSyncPreview, error) {
		cfg, err := s.snapshotConfig(ctx)
		if err != nil {
			return adminapp.ModelCatalogSyncPreview{}, err
		}
		return modelCatalogSyncPreview(request, cfg), nil
	})
}

func safeModel(model configadmin.ModelConfig, cfg *configadmin.SnapshotConfig) adminapp.ModelView {
	return adminapp.ModelView{
		PublicModel:      model.PublicModel,
		Aliases:          append([]string(nil), model.Aliases...),
		DisplayName:      model.DisplayName,
		Description:      model.Description,
		Protocol:         model.Protocol,
		Capability:       model.Capability,
		Type:             adminModelType(model),
		Category:         model.Category,
		Tags:             append([]string(nil), model.Tags...),
		ProviderFamily:   model.ProviderFamily,
		Modalities:       append([]string(nil), model.Modalities...),
		Capabilities:     adminModelCapabilities(model),
		InputModalities:  adminModelInputModalities(model),
		OutputModalities: adminModelOutputModalities(model),
		ContextWindow:    model.ContextWindow,
		MaxOutputTokens:  model.MaxOutputTokens,
		Status:           model.Status,
		Deprecated:       model.Deprecated,
		SortOrder:        model.SortOrder,
		Metadata:         redactedRawJSON(model.Metadata),
		SchemaAvailable:  schemaAvailable(model.Schema),
		Enabled:          model.Enabled,
		Async:            modelAsync(model),
		PricingSummary:   modelPricingSummary(model.PublicModel, cfg.Prices),
		ChannelCoverage:  modelChannelCoverage(model.PublicModel, cfg.Channels),
	}
}

func findModel(models []configadmin.ModelConfig, modelID string) (configadmin.ModelConfig, bool) {
	modelID = strings.TrimSpace(modelID)
	for _, model := range models {
		if model.PublicModel == modelID {
			return model, true
		}
	}
	return configadmin.ModelConfig{}, false
}

func mergeModel(current configadmin.ModelConfig, patch configadmin.ModelConfig) configadmin.ModelConfig {
	if patch.Aliases != nil {
		current.Aliases = patch.Aliases
	}
	if strings.TrimSpace(patch.DisplayName) != "" {
		current.DisplayName = patch.DisplayName
	}
	if strings.TrimSpace(patch.Description) != "" {
		current.Description = patch.Description
	}
	if strings.TrimSpace(patch.Protocol) != "" {
		current.Protocol = patch.Protocol
	}
	if strings.TrimSpace(patch.Capability) != "" {
		current.Capability = patch.Capability
	}
	if strings.TrimSpace(patch.Category) != "" {
		current.Category = patch.Category
	}
	if patch.Tags != nil {
		current.Tags = patch.Tags
	}
	if strings.TrimSpace(patch.ProviderFamily) != "" {
		current.ProviderFamily = patch.ProviderFamily
	}
	if patch.Modalities != nil {
		current.Modalities = patch.Modalities
	}
	if patch.Capabilities != nil {
		current.Capabilities = patch.Capabilities
	}
	if patch.ContextWindow > 0 {
		current.ContextWindow = patch.ContextWindow
	}
	if patch.MaxOutputTokens > 0 {
		current.MaxOutputTokens = patch.MaxOutputTokens
	}
	if strings.TrimSpace(patch.Status) != "" {
		current.Status = patch.Status
	}
	if patch.Deprecated {
		current.Deprecated = true
	}
	if patch.SortOrder != 0 {
		current.SortOrder = patch.SortOrder
	}
	if patch.Metadata != nil {
		current.Metadata = patch.Metadata
	}
	if patch.Schema != nil {
		current.Schema = patch.Schema
	}
	if patch.EnabledSet {
		current.Enabled = patch.Enabled
	}
	current.EnabledSet = true
	return current
}

func modelPricingSummary(publicModel string, prices []configadmin.PriceRuleConfig) adminapp.ModelPricingSummary {
	for _, price := range prices {
		if price.PublicModel != publicModel || !price.Enabled {
			continue
		}
		components := make([]adminapp.PricingComponentView, 0, len(price.Components))
		for _, component := range price.Components {
			components = append(components, adminapp.PricingComponentView{
				Unit:          string(component.Unit),
				MicrosPerUnit: component.MicrosPerUnit,
			})
		}
		return adminapp.ModelPricingSummary{
			Configured:             true,
			Currency:               price.Currency,
			Category:               price.Category,
			Components:             components,
			InputMicrosPerToken:    price.InputMicrosPerToken,
			OutputMicrosPerToken:   price.OutputMicrosPerToken,
			EstimatedOutputTokens:  price.EstimatedOutputTokens,
			ComponentPriceCount:    len(components),
			LegacyTokenPriceActive: price.InputMicrosPerToken > 0 || price.OutputMicrosPerToken > 0,
		}
	}
	return adminapp.ModelPricingSummary{}
}

func modelChannelCoverage(publicModel string, channels []configadmin.ChannelConfig) []adminapp.ModelChannelCoverage {
	var coverage []adminapp.ModelChannelCoverage
	for _, channel := range channels {
		for _, model := range channel.Models {
			if model.PublicModel != publicModel {
				continue
			}
			coverage = append(coverage, adminapp.ModelChannelCoverage{
				ChannelID:            channel.ID,
				ProviderType:         channel.ProviderType,
				Enabled:              channel.Enabled,
				UpstreamModel:        model.UpstreamModel,
				Capabilities:         append([]string(nil), model.Capabilities...),
				SupportedParameters:  append([]string(nil), model.SupportedParameters...),
				HealthStatus:         model.HealthStatus,
				TestStatus:           model.TestStatus,
				CostConfigStatus:     model.CostConfigStatus,
				CredentialConfigured: credentialConfigured(channel),
			})
		}
	}
	sort.SliceStable(coverage, func(i, j int) bool {
		if coverage[i].Enabled != coverage[j].Enabled {
			return coverage[i].Enabled
		}
		return coverage[i].ChannelID < coverage[j].ChannelID
	})
	return coverage
}

func configModelSchema(model configadmin.ModelConfig) map[string]any {
	if schemaAvailable(model.Schema) {
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

func modelCatalogSyncPreview(request adminapp.ModelCatalogSyncPreviewRequest, cfg *configadmin.SnapshotConfig) adminapp.ModelCatalogSyncPreview {
	currentByModel := make(map[string]configadmin.ModelConfig, len(cfg.Models))
	for _, model := range cfg.Models {
		currentByModel[model.PublicModel] = model
	}
	pricingByModel := make(map[string]bool, len(cfg.Prices))
	for _, price := range cfg.Prices {
		if price.Enabled {
			pricingByModel[price.PublicModel] = true
		}
	}
	coverageByModel := make(map[string]int)
	for _, channel := range cfg.Channels {
		for _, model := range channel.Models {
			coverageByModel[model.PublicModel]++
		}
	}

	seen := make(map[string]bool, len(request.Models))
	preview := adminapp.ModelCatalogSyncPreview{}
	for _, discovered := range request.Models {
		discovered.PublicModel = strings.TrimSpace(discovered.PublicModel)
		if discovered.PublicModel == "" {
			preview.Warnings = append(preview.Warnings, "public_model is required for discovered model")
			continue
		}
		seen[discovered.PublicModel] = true
		current, exists := currentByModel[discovered.PublicModel]
		item := modelCatalogSyncItem(discovered, current, exists, pricingByModel, coverageByModel)
		if !exists {
			preview.Added = append(preview.Added, item)
			continue
		}
		if modelCatalogChanged(discovered, current) {
			preview.Changed = append(preview.Changed, item)
			continue
		}
		preview.Unchanged++
	}
	for _, current := range cfg.Models {
		if seen[current.PublicModel] {
			continue
		}
		preview.Removed = append(preview.Removed, modelCatalogSyncItem(adminapp.ModelCatalogSyncModel{
			PublicModel:    current.PublicModel,
			DisplayName:    current.DisplayName,
			Protocol:       current.Protocol,
			Capability:     current.Capability,
			Category:       current.Category,
			Modalities:     current.Modalities,
			Capabilities:   current.Capabilities,
			ProviderFamily: current.ProviderFamily,
		}, current, true, pricingByModel, coverageByModel))
	}
	return preview
}

func modelCatalogSyncItem(discovered adminapp.ModelCatalogSyncModel, current configadmin.ModelConfig, exists bool, pricingByModel map[string]bool, coverageByModel map[string]int) adminapp.ModelCatalogSyncItem {
	return adminapp.ModelCatalogSyncItem{
		PublicModel:        discovered.PublicModel,
		DisplayName:        discovered.DisplayName,
		Protocol:           discovered.Protocol,
		Capability:         discovered.Capability,
		Category:           discovered.Category,
		Modalities:         append([]string(nil), discovered.Modalities...),
		Capabilities:       append([]string(nil), discovered.Capabilities...),
		ProviderFamily:     discovered.ProviderFamily,
		KnownCatalogModel:  exists,
		PricingConfigured:  pricingByModel[discovered.PublicModel],
		ChannelCoverage:    coverageByModel[discovered.PublicModel],
		CurrentDisplayName: current.DisplayName,
	}
}

func modelCatalogChanged(discovered adminapp.ModelCatalogSyncModel, current configadmin.ModelConfig) bool {
	return nonEmptyDifferent(discovered.DisplayName, current.DisplayName) ||
		nonEmptyDifferent(discovered.Protocol, current.Protocol) ||
		nonEmptyDifferent(discovered.Capability, current.Capability) ||
		nonEmptyDifferent(discovered.Category, current.Category) ||
		nonEmptyDifferent(discovered.ProviderFamily, current.ProviderFamily) ||
		stringSliceDifferent(discovered.Modalities, current.Modalities) ||
		stringSliceDifferent(discovered.Capabilities, current.Capabilities)
}

func nonEmptyDifferent(next string, current string) bool {
	next = strings.TrimSpace(next)
	return next != "" && next != strings.TrimSpace(current)
}

func stringSliceDifferent(next []string, current []string) bool {
	if next == nil {
		return false
	}
	if len(next) != len(current) {
		return true
	}
	nextCopy := append([]string(nil), next...)
	currentCopy := append([]string(nil), current...)
	sort.Strings(nextCopy)
	sort.Strings(currentCopy)
	for i := range nextCopy {
		if nextCopy[i] != currentCopy[i] {
			return true
		}
	}
	return false
}

func adminModelCapabilities(model configadmin.ModelConfig) []string {
	if len(model.Capabilities) > 0 {
		out := append([]string(nil), model.Capabilities...)
		sort.Strings(out)
		return out
	}
	if strings.TrimSpace(model.Capability) == "" {
		return []string{model.Protocol}
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

func adminModelType(model configadmin.ModelConfig) string {
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

func adminModelInputModalities(model configadmin.ModelConfig) []string {
	if len(model.Modalities) > 0 {
		return append([]string(nil), model.Modalities...)
	}
	switch adminModelType(model) {
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

func adminModelOutputModalities(model configadmin.ModelConfig) []string {
	if len(model.Modalities) > 0 {
		return append([]string(nil), model.Modalities...)
	}
	switch adminModelType(model) {
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

func modelAsync(model configadmin.ModelConfig) bool {
	protocol := strings.ToLower(strings.TrimSpace(model.Protocol))
	return strings.Contains(protocol, "unified") || strings.Contains(protocol, "async")
}

func schemaAvailable(schema json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(schema))
	return trimmed != "" && trimmed != "{}" && json.Valid(schema)
}

func redactedRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "{}" {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return redactedJSON(decoded)
}
