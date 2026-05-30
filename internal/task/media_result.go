package task

import (
	"encoding/json"
	"net/url"
	"strings"
)

// NormalizeProviderTaskResult makes provider result assets and metadata durable.
func NormalizeProviderTaskResult(result ProviderTaskResult) ProviderTaskResult {
	assets := normalizeResultAssets(result.Assets)
	result.Assets = assets
	result.ProviderMetadata = cleanMetadata(result.ProviderMetadata)
	if len(assets) == 0 && len(result.ProviderMetadata) == 0 {
		return result
	}

	var payload map[string]any
	if len(result.Result) > 0 {
		_ = json.Unmarshal(result.Result, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	if len(assets) > 0 && payload["assets"] == nil {
		payload["assets"] = assets
	}
	if len(assets) > 0 && payload["results"] == nil {
		payload["results"] = urlsFromAssets(assets)
	}
	if len(result.ProviderMetadata) > 0 && payload["provider_metadata"] == nil {
		payload["provider_metadata"] = result.ProviderMetadata
	}
	encoded, err := json.Marshal(payload)
	if err == nil {
		result.Result = encoded
	}
	return result
}

func normalizeResultAssets(assets []ResultAsset) []ResultAsset {
	out := make([]ResultAsset, 0, len(assets))
	for _, asset := range assets {
		asset.URL = strings.TrimSpace(asset.URL)
		if asset.URL == "" || !hasURLScheme(asset.URL) {
			continue
		}
		asset.Type = strings.TrimSpace(asset.Type)
		asset.MIMEType = strings.TrimSpace(asset.MIMEType)
		asset.Provider = strings.TrimSpace(asset.Provider)
		asset.ExpiresAt = strings.TrimSpace(asset.ExpiresAt)
		asset.Metadata = cleanMetadata(asset.Metadata)
		out = append(out, asset)
	}
	return out
}

func urlsFromAssets(assets []ResultAsset) []string {
	urls := make([]string, 0, len(assets))
	for _, asset := range assets {
		if asset.URL != "" {
			urls = append(urls, asset.URL)
		}
	}
	return urls
}

func cleanMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func hasURLScheme(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != ""
}
