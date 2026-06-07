package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPISplitBaseline(t *testing.T) {
	root := filepath.Join("..", "..", "api", "openapi")
	required := []string{
		"gateway-public.yaml",
		"portal-public.yaml",
		"portal-bff.yaml",
		"admin-bff.yaml",
		"control.yaml",
	}
	for _, name := range required {
		t.Run(name, func(t *testing.T) {
			doc := readOpenAPIDoc(t, filepath.Join(root, name))
			if doc["openapi"] == nil {
				t.Fatalf("%s is missing openapi version", name)
			}
			if doc["info"] == nil {
				t.Fatalf("%s is missing info", name)
			}
			if doc["paths"] == nil {
				t.Fatalf("%s is missing paths", name)
			}
			if doc["components"] == nil {
				t.Fatalf("%s is missing components", name)
			}
		})
	}
}

func TestBFFContractsStayOnConsoleSurface(t *testing.T) {
	root := filepath.Join("..", "..", "api", "openapi")
	tests := []struct {
		name      string
		prefix    string
		forbidden []string
		security  string
	}{
		{
			name:      "portal-bff.yaml",
			prefix:    "/api/portal/v1/",
			forbidden: []string{"/v1/", "/admin/", "/api/admin/v1/"},
			security:  "portalSession",
		},
		{
			name:      "admin-bff.yaml",
			prefix:    "/api/admin/v1/",
			forbidden: []string{"/v1/", "/admin/", "/api/portal/v1/"},
			security:  "adminSession",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := readOpenAPIDoc(t, filepath.Join(root, tt.name))
			paths := getMap(t, doc, "paths")
			if len(paths) == 0 {
				t.Fatal("BFF contract must define at least one path")
			}
			for pathName := range paths {
				if !strings.HasPrefix(pathName, tt.prefix) {
					t.Fatalf("path %s must stay under %s", pathName, tt.prefix)
				}
				for _, forbidden := range tt.forbidden {
					if strings.HasPrefix(pathName, forbidden) {
						t.Fatalf("path %s must not claim %s", pathName, forbidden)
					}
				}
			}
			components := getMap(t, doc, "components")
			securitySchemes := getMap(t, components, "securitySchemes")
			if _, ok := securitySchemes[tt.security]; !ok {
				t.Fatalf("%s is missing %s security scheme", tt.name, tt.security)
			}
			if _, ok := securitySchemes["csrfHeader"]; !ok {
				t.Fatalf("%s is missing csrfHeader security scheme", tt.name)
			}
		})
	}
}

func TestAggregateMentionsOpenAPISplit(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "design", "ai_gateway_openapi.yaml")
	doc := readOpenAPIDoc(t, path)
	split := asMap(doc["x-token-gateway-contract-split"])
	if split == nil {
		t.Fatal("aggregate OpenAPI must mention the split contract directory")
	}
	if split["source_directory"] != "api/openapi" {
		t.Fatalf("source_directory = %v", split["source_directory"])
	}
}

func readOpenAPIDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		t.Fatalf("%s is invalid OpenAPI YAML: %v", path, err)
	}
	return doc
}
