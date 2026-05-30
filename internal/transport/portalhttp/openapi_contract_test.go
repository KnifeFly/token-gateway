package portalhttp

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIIncludesPortalContract(t *testing.T) {
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
		"/v1/portal/models":                    {"get"},
		"/v1/portal/models/{model}/schema":     {"get"},
		"/v1/portal/credits":                   {"get"},
		"/v1/portal/usage":                     {"get"},
		"/v1/portal/api-keys":                  {"get", "post"},
		"/v1/portal/api-keys/{key_id}/disable": {"post"},
		"/v1/portal/tasks":                     {"get"},
		"/v1/portal/tasks/{task_id}":           {"get"},
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
		"PortalUsageResponse",
		"PortalAPIKey",
		"PortalAPIKeyListResponse",
		"PortalAPIKeyCreateRequest",
		"PortalAPIKeyCreateResponse",
		"PortalTaskListResponse",
	}
	for _, schema := range requiredSchemas {
		if _, ok := doc.Components.Schemas[schema]; !ok {
			t.Fatalf("OpenAPI schema %s is missing", schema)
		}
	}
	if _, ok := doc.Components.SecuritySchemes["bearerAuth"]; !ok {
		t.Fatal("OpenAPI bearerAuth security scheme is missing")
	}
}

type openAPIDocument struct {
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]any `yaml:"securitySchemes"`
		Schemas         map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}
