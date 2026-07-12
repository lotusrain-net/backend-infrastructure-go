package api_test

import (
	"context"
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

func loadOpenAPI(t *testing.T) *openapi3.T {
	t.Helper()
	document, err := openapi3.NewLoader().LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatalf("load OpenAPI: %v", err)
	}
	return document
}
