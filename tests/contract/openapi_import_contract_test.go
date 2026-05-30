package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPIImportPreflight(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "design", "ai_gateway_openapi.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		t.Fatalf("OpenAPI yaml is invalid: %v", err)
	}
	if doc["openapi"] == nil {
		t.Fatal("OpenAPI version is missing")
	}

	paths := getMap(t, doc, "paths")
	components := getMap(t, doc, "components")
	if len(paths) == 0 {
		t.Fatal("OpenAPI paths are empty")
	}
	if len(getMap(t, components, "schemas")) == 0 {
		t.Fatal("OpenAPI components.schemas are empty")
	}
	if len(getMap(t, components, "securitySchemes")) == 0 {
		t.Fatal("OpenAPI components.securitySchemes are empty")
	}

	refs := collectRefs(doc)
	for _, ref := range refs {
		if strings.HasPrefix(ref, "#/") && !resolveLocalRef(doc, ref) {
			t.Fatalf("OpenAPI local ref does not resolve: %s", ref)
		}
	}

	globalBearerAuth := securityListHasScheme(doc["security"], "bearerAuth")
	seenOperationIDs := make(map[string]string)
	for pathName, pathValue := range paths {
		pathItem := asMap(pathValue)
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			operation := asMap(pathItem[method])
			if len(operation) == 0 {
				continue
			}
			operationID, _ := operation["operationId"].(string)
			if operationID == "" {
				t.Fatalf("%s %s is missing operationId", strings.ToUpper(method), pathName)
			}
			if previous, ok := seenOperationIDs[operationID]; ok {
				t.Fatalf("duplicate operationId %q on %s %s and %s", operationID, strings.ToUpper(method), pathName, previous)
			}
			seenOperationIDs[operationID] = strings.ToUpper(method) + " " + pathName
			security, hasOperationSecurity := operation["security"]
			if strings.HasPrefix(pathName, "/v1/portal/") && hasOperationSecurity && !securityListHasScheme(security, "bearerAuth") {
				t.Fatalf("%s %s must require bearerAuth", strings.ToUpper(method), pathName)
			}
			if strings.HasPrefix(pathName, "/v1/portal/") && !hasOperationSecurity && !globalBearerAuth {
				t.Fatalf("%s %s must inherit global bearerAuth", strings.ToUpper(method), pathName)
			}
		}
	}
}

func getMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	child := asMap(parent[key])
	if child == nil {
		t.Fatalf("OpenAPI %s must be an object", key)
	}
	return child
}

func collectRefs(value any) []string {
	var refs []string
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			if ref, _ := typed["$ref"].(string); ref != "" {
				refs = append(refs, ref)
			}
			for _, child := range typed {
				walk(child)
			}
		case map[any]any:
			if ref, _ := typed["$ref"].(string); ref != "" {
				refs = append(refs, ref)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

func resolveLocalRef(root map[string]any, ref string) bool {
	current := any(root)
	for _, rawPart := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(rawPart, "~1", "/"), "~0", "~")
		currentMap := asMap(current)
		if currentMap == nil {
			return false
		}
		next, ok := currentMap[part]
		if !ok {
			return false
		}
		current = next
	}
	return true
}

func asMap(value any) map[string]any {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]any:
		return typed
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil
			}
			out[keyString] = child
		}
		return out
	default:
		return nil
	}
}

func securityListHasScheme(value any, scheme string) bool {
	security, ok := value.([]any)
	if !ok {
		return false
	}
	for _, requirement := range security {
		for name := range asMap(requirement) {
			if name == scheme {
				return true
			}
		}
	}
	return false
}
