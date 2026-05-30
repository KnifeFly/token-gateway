package controlhttp

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIIncludesImplementedAdminContract(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "design", "ai_gateway_openapi.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var doc openAPIDocument
	if err := yaml.Unmarshal(content, &doc); err != nil {
		t.Fatalf("OpenAPI yaml is invalid: %v", err)
	}
	requiredPaths := map[string][]string{
		"/admin/tenants":                   {"post"},
		"/admin/projects":                  {"post"},
		"/admin/api-keys":                  {"get", "post"},
		"/admin/api-keys/{key_id}/disable": {"post"},
		"/admin/models":                    {"post"},
		"/admin/channels":                  {"post"},
		"/admin/routes":                    {"post"},
		"/admin/prices":                    {"post"},
		"/admin/limits":                    {"post"},
		"/admin/plugin-bindings":           {"post"},
		"/admin/snapshots/publish":         {"post"},
		"/admin/snapshots/rollback":        {"post"},
		"/admin/emergency/providers/{provider_type}/{action}": {"post"},
		"/admin/emergency/channels/{channel_id}/{action}":     {"post"},
	}
	for path, methods := range requiredPaths {
		pathItem, ok := doc.Paths[path]
		if !ok {
			t.Fatalf("OpenAPI path %s is missing", path)
		}
		for _, method := range methods {
			if _, ok := pathItem[method]; !ok {
				t.Fatalf("OpenAPI path %s method %s is missing", path, method)
			}
		}
	}
	requiredSchemas := []string{
		"AdminTenant",
		"AdminProject",
		"AdminAPIKey",
		"AdminModelConfig",
		"AdminChannelConfig",
		"AdminRoutePolicyConfig",
		"AdminPriceRuleConfig",
		"AdminLimitRuleConfig",
		"AdminPluginBindingConfig",
		"AdminRuntimeSnapshot",
		"AdminEmergencyAction",
	}
	for _, schema := range requiredSchemas {
		if _, ok := doc.Components.Schemas[schema]; !ok {
			t.Fatalf("OpenAPI schema %s is missing", schema)
		}
	}
	if _, ok := doc.Components.SecuritySchemes["adminTokenAuth"]; !ok {
		t.Fatal("OpenAPI adminTokenAuth security scheme is missing")
	}
}

type openAPIDocument struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]any `yaml:"securitySchemes"`
		Schemas         map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}
