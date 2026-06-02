package contract_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestP24ConsoleOpenAPIPaths(t *testing.T) {
	root := filepath.Join("..", "..", "api", "openapi")
	admin := readOpenAPIDoc(t, filepath.Join(root, "admin-bff.yaml"))
	portal := readOpenAPIDoc(t, filepath.Join(root, "portal-bff.yaml"))

	requireOpenAPIPaths(t, admin, []string{
		"/api/admin/v1/channels",
		"/api/admin/v1/channels/{channel_id}",
		"/api/admin/v1/channels/{channel_id}/test",
		"/api/admin/v1/channels/{channel_id}/rotate-credential",
		"/api/admin/v1/channels/{channel_id}/sync-preview",
		"/api/admin/v1/channels/{channel_id}/sync-apply",
		"/api/admin/v1/channels/{channel_id}/health-events",
		"/api/admin/v1/models",
		"/api/admin/v1/models/sync-preview",
		"/api/admin/v1/models/{model_id}",
		"/api/admin/v1/models/{model_id}/channels",
		"/api/admin/v1/models/{model_id}/schema-preview",
		"/api/admin/v1/customer-accounts",
		"/api/admin/v1/customer-accounts/{customer_account_id}",
		"/api/admin/v1/customer-accounts/{customer_account_id}/credit-report",
		"/api/admin/v1/customer-accounts/{customer_account_id}/usage/export",
		"/api/admin/v1/customer-accounts/{customer_account_id}/ledger/export",
		"/api/admin/v1/customer-accounts/{customer_account_id}/manual-adjustment",
		"/api/admin/v1/api-keys",
		"/api/admin/v1/usage-logs",
		"/api/admin/v1/usage-logs/{request_id}",
		"/api/admin/v1/task-logs",
		"/api/admin/v1/task-logs/{task_id}",
		"/api/admin/v1/playground/run",
		"/api/admin/v1/playground/import-preview",
		"/api/admin/v1/playground/export",
	})

	requireOpenAPIPaths(t, portal, []string{
		"/api/portal/v1/models",
		"/api/portal/v1/models/{model}/schema",
		"/api/portal/v1/models/{model}",
		"/api/portal/v1/playground/run",
		"/api/portal/v1/credits",
		"/api/portal/v1/credits/ledger",
		"/api/portal/v1/usage",
		"/api/portal/v1/usage/export",
		"/api/portal/v1/api-keys",
		"/api/portal/v1/api-keys/{key_id}/disable",
		"/api/portal/v1/api-keys/{key_id}/rotate",
		"/api/portal/v1/tasks",
		"/api/portal/v1/tasks/{task_id}",
	})
}

func TestP24ConsoleMutationSecurity(t *testing.T) {
	root := filepath.Join("..", "..", "api", "openapi")
	admin := readOpenAPIDoc(t, filepath.Join(root, "admin-bff.yaml"))
	portal := readOpenAPIDoc(t, filepath.Join(root, "portal-bff.yaml"))

	adminMutations := []struct {
		method string
		path   string
	}{
		{"post", "/api/admin/v1/channels"},
		{"patch", "/api/admin/v1/channels/{channel_id}"},
		{"post", "/api/admin/v1/channels/{channel_id}/test"},
		{"post", "/api/admin/v1/channels/{channel_id}/rotate-credential"},
		{"post", "/api/admin/v1/channels/{channel_id}/sync-preview"},
		{"post", "/api/admin/v1/channels/{channel_id}/sync-apply"},
		{"post", "/api/admin/v1/models"},
		{"patch", "/api/admin/v1/models/{model_id}"},
		{"post", "/api/admin/v1/models/sync-preview"},
		{"post", "/api/admin/v1/models/{model_id}/disable"},
		{"post", "/api/admin/v1/models/{model_id}/deprecate"},
		{"post", "/api/admin/v1/customer-accounts"},
		{"post", "/api/admin/v1/customer-accounts/{customer_account_id}/enable"},
		{"post", "/api/admin/v1/customer-accounts/{customer_account_id}/disable"},
		{"post", "/api/admin/v1/customer-accounts/{customer_account_id}/manual-adjustment"},
		{"post", "/api/admin/v1/customer-accounts/{customer_account_id}/reset-session"},
		{"post", "/api/admin/v1/api-keys"},
		{"post", "/api/admin/v1/api-keys/{key_id}/update"},
		{"post", "/api/admin/v1/api-keys/{key_id}/enable"},
		{"post", "/api/admin/v1/api-keys/{key_id}/disable"},
		{"post", "/api/admin/v1/api-keys/{key_id}/rotate"},
		{"post", "/api/admin/v1/playground/run"},
		{"post", "/api/admin/v1/playground/import-preview"},
		{"post", "/api/admin/v1/playground/export"},
	}
	for _, mutation := range adminMutations {
		requireOperationSecurity(t, admin, mutation.method, mutation.path, []string{
			"adminSession",
			"csrfHeader",
			"idempotencyKey",
			"reasonHeader",
		})
	}

	portalMutations := []struct {
		method string
		path   string
	}{
		{"post", "/api/portal/v1/playground/run"},
		{"post", "/api/portal/v1/api-keys"},
		{"post", "/api/portal/v1/api-keys/{key_id}/disable"},
		{"post", "/api/portal/v1/api-keys/{key_id}/rotate"},
		{"post", "/api/portal/v1/tasks/{task_id}/cancel"},
	}
	for _, mutation := range portalMutations {
		requireOperationSecurity(t, portal, mutation.method, mutation.path, []string{
			"portalSession",
			"csrfHeader",
		})
	}
}

func TestP24SafeSchemasExcludeCutScopeAndSecrets(t *testing.T) {
	root := filepath.Join("..", "..", "api", "openapi")
	admin := readOpenAPIDoc(t, filepath.Join(root, "admin-bff.yaml"))
	portal := readOpenAPIDoc(t, filepath.Join(root, "portal-bff.yaml"))

	adminSafeSchemas := []string{
		"AdminChannelListResponse",
		"AdminChannelView",
		"AdminChannelTestResult",
		"AdminChannelSyncPreview",
		"AdminChannelSyncApplyResult",
		"AdminChannelHealthEventListResponse",
		"AdminModelListResponse",
		"AdminModelView",
		"AdminModelChannelCoverageListResponse",
		"AdminModelSchemaPreview",
		"AdminModelCatalogSyncPreview",
		"AdminCustomerAccountListResponse",
		"AdminCustomerAccountView",
		"AdminCustomerAccountDetail",
		"AdminCustomerCreditReport",
		"AdminCustomerReportExport",
		"AdminAPIKeyListResponse",
		"AdminAPIKeyView",
		"AdminUsageLogListResponse",
		"AdminUsageLogView",
		"AdminUsageLogDetail",
		"AdminTaskLogListResponse",
		"AdminTaskLogView",
		"AdminTaskLogDetail",
		"AdminPlaygroundRunResult",
		"AdminPlaygroundImportPreview",
		"AdminPlaygroundExport",
	}
	portalSafeSchemas := []string{
		"PortalSessionResponse",
		"ModelListResponse",
		"ModelSummary",
		"ModelDetailResponse",
		"ModelSchemaResponse",
		"PlaygroundRunResult",
		"CreditsResponse",
		"CreditLedgerResponse",
		"UsageResponse",
		"UsageExportResponse",
		"APIKeyListResponse",
		"APIKey",
		"TaskListResponse",
		"TaskObject",
		"ProjectSettings",
	}

	for _, schemaName := range adminSafeSchemas {
		assertSafeSchemaPropertyNames(t, admin, "admin-bff.yaml", schemaName)
	}
	for _, schemaName := range portalSafeSchemas {
		assertSafeSchemaPropertyNames(t, portal, "portal-bff.yaml", schemaName)
	}
}

func TestP24GeneratedClientsContainConsoleContracts(t *testing.T) {
	root := filepath.Join("..", "..")
	requireGeneratedClientContains(t, filepath.Join(root, "web", "packages", "api-client", "src", "generated", "admin-bff.ts"), []string{
		`"/api/admin/v1/channels"`,
		`"/api/admin/v1/models"`,
		`"/api/admin/v1/customer-accounts/{customer_account_id}/credit-report"`,
		`"/api/admin/v1/playground/run"`,
		"AdminChannelView",
		"AdminModelView",
		"AdminCustomerCreditReport",
		"AdminCustomerReportExport",
		"AdminPlaygroundRunResult",
	})
	requireGeneratedClientContains(t, filepath.Join(root, "web", "packages", "api-client", "src", "generated", "portal-bff.ts"), []string{
		`"/api/portal/v1/models/{model}"`,
		`"/api/portal/v1/playground/run"`,
		`"/api/portal/v1/credits/ledger"`,
		`"/api/portal/v1/usage/export"`,
		"PlaygroundRunResult",
		"CreditLedgerResponse",
		"UsageExportResponse",
		"TaskObject",
	})
}

func requireOpenAPIPaths(t *testing.T, doc map[string]any, paths []string) {
	t.Helper()
	openAPIPaths := getMap(t, doc, "paths")
	for _, path := range paths {
		if _, ok := openAPIPaths[path]; !ok {
			t.Fatalf("OpenAPI path %s is missing", path)
		}
	}
}

func requireOperationSecurity(t *testing.T, doc map[string]any, method string, path string, schemes []string) {
	t.Helper()
	operation := operationFor(t, doc, method, path)
	security, ok := operation["security"].([]any)
	if !ok {
		t.Fatalf("%s %s is missing operation security", strings.ToUpper(method), path)
	}
	for _, requirement := range security {
		requirementMap := asMap(requirement)
		if securityRequirementHasSchemes(requirementMap, schemes) {
			return
		}
	}
	t.Fatalf("%s %s security must include %s", strings.ToUpper(method), path, strings.Join(schemes, ", "))
}

func operationFor(t *testing.T, doc map[string]any, method string, path string) map[string]any {
	t.Helper()
	paths := getMap(t, doc, "paths")
	pathItem := asMap(paths[path])
	if pathItem == nil {
		t.Fatalf("OpenAPI path %s is missing", path)
	}
	operation := asMap(pathItem[method])
	if operation == nil {
		t.Fatalf("OpenAPI operation %s %s is missing", strings.ToUpper(method), path)
	}
	return operation
}

func securityRequirementHasSchemes(requirement map[string]any, schemes []string) bool {
	for _, scheme := range schemes {
		if _, ok := requirement[scheme]; !ok {
			return false
		}
	}
	return true
}

func assertSafeSchemaPropertyNames(t *testing.T, doc map[string]any, docName string, schemaName string) {
	t.Helper()
	schemas := getMap(t, getMap(t, doc, "components"), "schemas")
	schema := asMap(schemas[schemaName])
	if schema == nil {
		t.Fatalf("%s is missing schema %s", docName, schemaName)
	}

	var walk func(string, map[string]any)
	walk = func(path string, node map[string]any) {
		for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
			for i, child := range asList(node[keyword]) {
				childMap := asMap(child)
				if childMap != nil {
					walk(path+"."+keyword+"["+strconv.Itoa(i)+"]", childMap)
				}
			}
		}
		if items := asMap(node["items"]); items != nil {
			walk(path+"[]", items)
		}
		if additionalProperties := asMap(node["additionalProperties"]); additionalProperties != nil {
			walk(path+".additionalProperties", additionalProperties)
		}
		for propertyName, propertyValue := range asMap(node["properties"]) {
			if reason := forbiddenP24PropertyReason(propertyName); reason != "" {
				t.Fatalf("%s schema %s property %s%s is forbidden: %s", docName, schemaName, path, "."+propertyName, reason)
			}
			childMap := asMap(propertyValue)
			if childMap != nil {
				walk(path+"."+propertyName, childMap)
			}
		}
	}
	walk(schemaName, schema)
}

func forbiddenP24PropertyReason(propertyName string) string {
	name := strings.ToLower(propertyName)
	exact := map[string]string{
		"api_key":           "API keys must only appear in login/create request or one-time create response, not safe DTOs",
		"plaintext_key":     "plaintext keys must only appear in one-time create/rotate responses",
		"key_hash":          "key hashes are internal secrets",
		"api_key_hash":      "key hashes are internal secrets",
		"credential":        "credentials are internal secrets",
		"credential_ref":    "credential references are internal owner-service details",
		"encrypted_api_key": "encrypted credentials are internal secrets",
		"password":          "passwords are internal secrets",
		"access_token":      "tokens are internal secrets",
		"refresh_token":     "tokens are internal secrets",
		"callback_url":      "customer callback URLs are not safe log DTO fields",
		"raw_prompt":        "raw prompts are not safe DTO fields",
		"raw_response":      "raw responses are not safe DTO fields",
		"raw_payload":       "raw payloads are not safe DTO fields",
		"input":             "raw task input is not a safe DTO field",
		"messages":          "raw messages are not safe DTO fields",
		"user_group":        "NewAPI user groups are cut from P24",
		"model_group":       "NewAPI model groups are cut from P24",
		"channel_group":     "NewAPI channel groups are cut from P24",
		"group_ratio":       "NewAPI ratio fields are cut from P24",
		"model_ratio":       "NewAPI ratio fields are cut from P24",
		"channel_ratio":     "NewAPI ratio fields are cut from P24",
	}
	if reason, ok := exact[name]; ok {
		return reason
	}
	if name == "ratio" || strings.HasSuffix(name, "_ratio") || strings.Contains(name, "ratio_") {
		return "P24 cut-scope term ratio must not appear in safe DTOs"
	}
	for _, term := range []string{"payment", "subscription", "redemption", "invite_reward", "deployment"} {
		if strings.Contains(name, term) {
			return "P24 cut-scope term " + term + " must not appear in safe DTOs"
		}
	}
	return ""
}

func requireGeneratedClientContains(t *testing.T, path string, needles []string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	body := string(content)
	for _, needle := range needles {
		if !strings.Contains(body, needle) {
			t.Fatalf("%s must contain %q", path, needle)
		}
	}
}

func asList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return nil
}
