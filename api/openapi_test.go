package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
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
		{http.MethodPost, "/api/v1/users", false},
		{http.MethodPatch, "/api/v1/users/{userID}/active", false},
		{http.MethodGet, "/api/v1/roles", false},
		{http.MethodGet, "/api/v1/permissions", false},
		{http.MethodPost, "/api/v1/users/{userID}/roles/{roleID}", false},
		{http.MethodPost, "/api/v1/roles/{roleID}/permissions/{permissionID}", false},
		{http.MethodGet, "/api/v1/audit-logs", false},
		{http.MethodPost, "/api/v1/task-executions", false},
		{http.MethodGet, "/api/v1/task-executions/{executionID}", false},
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
		"LoginRequest", "RefreshRequest", "TokenPair", "User", "Role", "Permission",
		"AuditEvent", "TaskSubmissionRequest", "TaskExecution",
	} {
		if document.Components.Schemas[schema] == nil {
			t.Errorf("schema %s is missing", schema)
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
