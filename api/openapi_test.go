package api_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	taskmodule "github.com/jyysy/backend-infrastructure-go/internal/modules/task"
)

func TestOpenAPILoadsAndValidates(t *testing.T) {
	document := loadOpenAPI(t)
	if err := document.Validate(context.Background()); err != nil {
		t.Fatalf("validate OpenAPI: %v", err)
	}
}

func TestBaseEndpointsExplicitlyDisableSecurity(t *testing.T) {
	document := loadOpenAPI(t)
	for _, path := range []string{"/health/live", "/health/ready", "/metrics"} {
		item := document.Paths.Find(path)
		if item == nil || item.Get == nil {
			t.Fatalf("GET %s is missing", path)
		}
		if item.Get.Security == nil || len(*item.Get.Security) != 0 {
			t.Errorf("GET %s must declare security: []", path)
		}
	}
}

func TestOpenAPIProvidesSharedEnvelopeAndPaginationSchemas(t *testing.T) {
	document := loadOpenAPI(t)
	for _, schema := range []string{"SuccessEnvelope", "ErrorEnvelope", "PaginationMeta", "PaginatedResponse"} {
		if document.Components.Schemas[schema] == nil {
			t.Errorf("schema %s is missing", schema)
		}
	}
	errorSchema := document.Components.Schemas["ErrorEnvelope"].Value
	if errorSchema == nil || errorSchema.Properties["code"] == nil || errorSchema.Properties["code"].Value.Type == nil || !errorSchema.Properties["code"].Value.Type.Is("integer") {
		t.Fatal("ErrorEnvelope.code must be an integer")
	}
	if document.Components.Responses["PaginatedResponse"] == nil {
		t.Fatal("reusable PaginatedResponse response component is missing")
	}
}

func TestOpenAPIContainsImplementedCoreOperations(t *testing.T) {
	document := loadOpenAPI(t)
	tests := []struct {
		method string
		path   string
		public bool
	}{
		{http.MethodGet, "/health/live", true},
		{http.MethodGet, "/health/ready", true},
		{http.MethodGet, "/metrics", true},
		{http.MethodPost, "/api/v1/auth/login", true},
		{http.MethodPost, "/api/v1/auth/refresh", true},
		{http.MethodPost, "/api/v1/auth/logout", true},
		{http.MethodGet, "/api/v1/users/me", false},
		{http.MethodGet, "/api/v1/users/me/preferences", false},
		{http.MethodPut, "/api/v1/users/me/preferences", false},
		{http.MethodGet, "/api/v1/users", false},
		{http.MethodPost, "/api/v1/users", false},
		{http.MethodPatch, "/api/v1/users/{userID}", false},
		{http.MethodPost, "/api/v1/users/{userID}/password/reset", false},
		{http.MethodPatch, "/api/v1/users/{userID}/active", false},
		{http.MethodGet, "/api/v1/roles", false},
		{http.MethodPost, "/api/v1/roles", false},
		{http.MethodGet, "/api/v1/roles/{roleID}", false},
		{http.MethodPatch, "/api/v1/roles/{roleID}", false},
		{http.MethodDelete, "/api/v1/roles/{roleID}", false},
		{http.MethodGet, "/api/v1/permissions", false},
		{http.MethodPost, "/api/v1/users/{userID}/roles/{roleID}", false},
		{http.MethodPut, "/api/v1/users/{userID}/roles", false},
		{http.MethodPost, "/api/v1/roles/{roleID}/permissions/{permissionID}", false},
		{http.MethodPut, "/api/v1/roles/{roleID}/permissions", false},
		{http.MethodGet, "/api/v1/audit-logs", false},
		{http.MethodGet, "/api/v1/task-executions", false},
		{http.MethodPost, "/api/v1/task-executions", false},
		{http.MethodGet, "/api/v1/task-executions/{executionID}", false},
		{http.MethodGet, "/api/v1/task-types", false},
	}

	for _, test := range tests {
		operation := operationAt(t, document, test.method, test.path)
		if operation.OperationID == "" {
			t.Errorf("%s %s operationId is empty", test.method, test.path)
		}
		if operation.Security == nil {
			t.Errorf("%s %s must explicitly declare security", test.method, test.path)
			continue
		}
		if test.public && len(*operation.Security) != 0 {
			t.Errorf("%s %s must declare security: []", test.method, test.path)
		}
		if !test.public && len(*operation.Security) == 0 {
			t.Errorf("%s %s must require access authentication", test.method, test.path)
		}
		if !test.public {
			schemes := make(map[string]bool)
			for _, requirement := range *operation.Security {
				for scheme := range requirement {
					schemes[scheme] = true
				}
			}
			if !schemes["bearerAuth"] || !schemes["accessCookie"] {
				t.Errorf("%s %s security = %v, want bearerAuth or accessCookie", test.method, test.path, schemes)
			}
		}
	}
}

func TestPreferencesContractUsesFixedValidatedAccountFields(t *testing.T) {
	document := loadOpenAPI(t)
	for _, schemaName := range []string{"Preferences", "PreferencesEnvelope"} {
		if document.Components.Schemas[schemaName] == nil {
			t.Errorf("schema %s is missing", schemaName)
		}
	}
	schemaRef := document.Components.Schemas["Preferences"]
	if schemaRef == nil || schemaRef.Value == nil {
		t.Fatal("Preferences schema is missing")
	}
	schema := schemaRef.Value
	for _, field := range []string{"theme", "color_mode", "accent_color", "font_scale", "radius_scale"} {
		if schema.Properties[field] == nil || !slices.Contains(schema.Required, field) {
			t.Errorf("Preferences.%s must be present and required", field)
		}
	}
	if schema.Properties["accent_color"].Value.Nullable != true {
		t.Error("Preferences.accent_color must be nullable")
	}
	put := operationAt(t, document, http.MethodPut, "/api/v1/users/me/preferences")
	if put.Responses.Value("422") == nil {
		t.Error("PUT /api/v1/users/me/preferences must document validation failures")
	}
}

func TestEveryOpenAPIOperationExplicitlyDeclaresSecurity(t *testing.T) {
	document := loadOpenAPI(t)
	for path, item := range document.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.Security == nil {
				t.Errorf("%s %s does not explicitly declare security", strings.ToUpper(method), path)
			}
		}
	}
}

func TestOpenAPIProvidesSecuritySchemesAndCoreSchemas(t *testing.T) {
	document := loadOpenAPI(t)
	for _, scheme := range []string{"bearerAuth", "accessCookie", "refreshCookie"} {
		if document.Components.SecuritySchemes[scheme] == nil {
			t.Errorf("security scheme %s is missing", scheme)
		}
	}
	for _, schema := range []string{
		"LoginRequest", "RefreshRequest", "TokenPair", "User", "AuthenticatedUser", "AuthenticatedUserEnvelope", "Role", "RoleDetail", "Permission",
		"UpdateUserRequest", "ResetPasswordRequest", "ReplaceUserRolesRequest", "RoleRequest", "ReplaceRolePermissionsRequest",
		"AuditEvent", "TaskSubmissionRequest", "TaskExecution", "TaskTypeRef",
	} {
		if document.Components.Schemas[schema] == nil {
			t.Errorf("schema %s is missing", schema)
		}
	}
}

func TestIAMManagementSchemasExposeAtomicAndProtectedRoleContracts(t *testing.T) {
	document := loadOpenAPI(t)
	role := document.Components.Schemas["Role"].Value
	if role == nil || role.Properties["is_system"] == nil || !slices.Contains(role.Required, "is_system") {
		t.Fatal("Role.is_system must be a required response field")
	}
	for _, test := range []struct {
		schema string
		field  string
	}{
		{schema: "ReplaceUserRolesRequest", field: "role_ids"},
		{schema: "ReplaceRolePermissionsRequest", field: "permission_ids"},
		{schema: "ResetPasswordRequest", field: "new_password"},
	} {
		schema := document.Components.Schemas[test.schema].Value
		if schema == nil || schema.Properties[test.field] == nil || !slices.Contains(schema.Required, test.field) {
			t.Errorf("%s.%s must be required", test.schema, test.field)
		}
	}
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{http.MethodPatch, "/api/v1/users/{userID}"},
		{http.MethodPost, "/api/v1/users/{userID}/password/reset"},
		{http.MethodPut, "/api/v1/users/{userID}/roles"},
		{http.MethodPut, "/api/v1/roles/{roleID}/permissions"},
	} {
		if operationAt(t, document, endpoint.method, endpoint.path).Responses.Value("422") == nil {
			t.Errorf("%s %s must document validation errors", endpoint.method, endpoint.path)
		}
	}
}

func TestTaskSubmissionSchemaMatchesHTTPContract(t *testing.T) {
	document := loadOpenAPI(t)
	schema := document.Components.Schemas["TaskSubmissionRequest"].Value
	if schema == nil {
		t.Fatal("TaskSubmissionRequest schema is missing")
	}
	for _, required := range []string{"task_type", "payload"} {
		if !slices.Contains(schema.Required, required) {
			t.Errorf("%s must be required", required)
		}
	}
	taskType := schema.Properties["task_type"].Value
	if taskType == nil || taskType.Type == nil || !taskType.Type.Is("string") {
		t.Fatalf("task_type schema = %+v, want string", taskType)
	}
	if len(taskType.Enum) != 0 {
		t.Fatalf("task_type enum = %v, want no static enum for runtime-discovered task types", taskType.Enum)
	}
	if !strings.Contains(taskType.Description, "/api/v1/task-types") {
		t.Fatalf("task_type description = %q, want runtime catalog reference", taskType.Description)
	}
	defaults := map[string]float64{"max_retries": 3, "timeout_seconds": 300, "unique_for_seconds": 0, "process_after_seconds": 0}
	for name, want := range defaults {
		property := schema.Properties[name].Value
		if property == nil || property.Default != want {
			t.Errorf("%s default = %#v, want %v", name, property.Default, want)
		}
	}
	maximums := map[string]float64{
		"max_retries":           taskmodule.MaxSubmissionRetries,
		"timeout_seconds":       taskmodule.MaxSubmissionTimeout.Seconds(),
		"unique_for_seconds":    taskmodule.MaxSubmissionUniqueFor.Seconds(),
		"process_after_seconds": taskmodule.MaxSubmissionProcessAfter.Seconds(),
	}
	for name, want := range maximums {
		property := schema.Properties[name].Value
		if property == nil || property.Max == nil || *property.Max != want {
			t.Errorf("%s maximum = %v, want %v", name, property.Max, want)
		}
	}
	operation := operationAt(t, document, http.MethodPost, "/api/v1/task-executions")
	if operation.Responses.Value("422") == nil {
		t.Error("POST /api/v1/task-executions must document 422 validation response")
	}
	if operation.Responses.Value("404") == nil {
		t.Error("POST /api/v1/task-executions must document missing definition response")
	}
}

func TestValidationResponsesUseRuntimeStatus(t *testing.T) {
	document := loadOpenAPI(t)
	for _, endpoint := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodPost, "/api/v1/users"},
		{http.MethodPatch, "/api/v1/users/{userID}/active"},
		{http.MethodGet, "/api/v1/task-executions"},
		{http.MethodPost, "/api/v1/task-executions"},
	} {
		operation := operationAt(t, document, endpoint.method, endpoint.path)
		if operation.Responses.Value("422") == nil {
			t.Errorf("%s %s must document 422 validation responses", endpoint.method, endpoint.path)
		}
		if operation.Responses.Value("400") != nil {
			t.Errorf("%s %s must not document validation as 400", endpoint.method, endpoint.path)
		}
	}
}

func TestReadCollectionEndpointsDocumentFilterParameters(t *testing.T) {
	document := loadOpenAPI(t)

	users := operationAt(t, document, http.MethodGet, "/api/v1/users")
	if len(users.Parameters) != 4 {
		t.Fatalf("GET /api/v1/users parameters = %d, want 4", len(users.Parameters))
	}
	if schema := users.Parameters[3].Value.Schema.Value; schema == nil || schema.Type == nil || !schema.Type.Is("boolean") {
		t.Fatalf("GET /api/v1/users is_active schema = %+v", schema)
	}

	executions := operationAt(t, document, http.MethodGet, "/api/v1/task-executions")
	if len(executions.Parameters) != 4 {
		t.Fatalf("GET /api/v1/task-executions parameters = %d, want 4", len(executions.Parameters))
	}
	status := executions.Parameters[3].Value.Schema.Value
	if status == nil || len(status.Enum) != 5 {
		t.Fatalf("GET /api/v1/task-executions status schema = %+v", status)
	}

	auditLogs := operationAt(t, document, http.MethodGet, "/api/v1/audit-logs")
	parameters := make(map[string]*openapi3.Schema)
	for _, parameter := range auditLogs.Parameters {
		if parameter.Value == nil || parameter.Value.Schema == nil {
			continue
		}
		parameters[parameter.Value.Name] = parameter.Value.Schema.Value
	}
	for _, name := range []string{"from", "to"} {
		schema := parameters[name]
		if schema == nil || schema.Type == nil || !schema.Type.Is("string") || schema.Format != "date-time" {
			t.Errorf("GET /api/v1/audit-logs %s schema = %+v, want RFC 3339 date-time string", name, schema)
		}
	}
}

func TestCreateUserPasswordMinimumMatchesIAMContract(t *testing.T) {
	document := loadOpenAPI(t)
	schema := document.Components.Schemas["CreateUserRequest"].Value
	if schema == nil || schema.Properties["password"] == nil || schema.Properties["password"].Value == nil {
		t.Fatal("CreateUserRequest.password schema is missing")
	}
	if got := schema.Properties["password"].Value.MinLength; got != 12 {
		t.Fatalf("CreateUserRequest.password minLength = %d, want 12", got)
	}
}

func TestAuthenticatedUserSchemaCarriesPermissionsWithoutPollutingBaseUser(t *testing.T) {
	document := loadOpenAPI(t)
	userSchema := document.Components.Schemas["User"].Value
	if userSchema == nil {
		t.Fatal("User schema is missing")
	}
	if userSchema.Properties["permissions"] != nil {
		t.Fatal("User schema must not include caller-specific permissions")
	}
	schema := document.Components.Schemas["AuthenticatedUser"].Value
	if schema == nil || len(schema.AllOf) != 2 {
		t.Fatalf("AuthenticatedUser schema = %+v", schema)
	}
	permissions := schema.AllOf[1].Value.Properties["permissions"].Value
	if permissions.Type == nil || !permissions.Type.Is("array") || permissions.Items == nil || permissions.Items.Value == nil || permissions.Items.Value.Type == nil || !permissions.Items.Value.Type.Is("string") {
		t.Fatalf("AuthenticatedUser.permissions schema = %+v", permissions)
	}
	operation := operationAt(t, document, http.MethodGet, "/api/v1/users/me")
	response := operation.Responses.Value("200")
	if response == nil || response.Value == nil {
		t.Fatal("GET /api/v1/users/me 200 response is missing")
	}
	schemaRef := response.Value.Content["application/json"].Schema.Ref
	if schemaRef != "#/components/schemas/AuthenticatedUserEnvelope" {
		t.Fatalf("GET /api/v1/users/me schema = %q", schemaRef)
	}
}

func TestTokenResponseDoesNotExposeRefreshToken(t *testing.T) {
	document := loadOpenAPI(t)
	schema := document.Components.Schemas["TokenPair"].Value
	if schema == nil {
		t.Fatal("TokenPair schema is missing")
	}
	if schema.Properties["access_token"] == nil {
		t.Fatal("TokenPair must retain bearer access_token")
	}
	if schema.Properties["refresh_token"] != nil {
		t.Fatal("TokenPair must not expose refresh_token in JSON")
	}
}

func TestAuthResponsesDocumentCookieAndNoStoreHeaders(t *testing.T) {
	document := loadOpenAPI(t)
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/auth/refresh", "/api/v1/auth/logout"} {
		response := operationAt(t, document, http.MethodPost, path).Responses.Value("200")
		if response == nil || response.Value == nil {
			t.Fatalf("POST %s 200 response is missing", path)
		}
		for _, header := range []string{"Set-Cookie", "Cache-Control", "Pragma"} {
			if response.Value.Headers[header] == nil {
				t.Errorf("POST %s must document %s response header", path, header)
			}
		}
	}
}

func operationAt(t *testing.T, document *openapi3.T, method, path string) *openapi3.Operation {
	t.Helper()
	item := document.Paths.Map()[path]
	if item == nil {
		t.Fatalf("%s %s path is missing", method, path)
	}
	operation := item.GetOperation(method)
	if operation == nil {
		t.Fatalf("%s %s operation is missing", method, path)
	}
	return operation
}

func loadOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	document, err := openapi3.NewLoader().LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	return document
}
