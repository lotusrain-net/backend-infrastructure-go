package api_test

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIDeclaresBaseRoutesAndSharedSchemas(t *testing.T) {
	content, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	spec := string(content)
	for _, required := range []string{
		"openapi: 3.1.0",
		"/health/live:",
		"/health/ready:",
		"/metrics:",
		"SuccessEnvelope:",
		"ErrorEnvelope:",
		"PaginationMeta:",
		"X-Request-ID:",
	} {
		if !strings.Contains(spec, required) {
			t.Errorf("OpenAPI spec is missing %q", required)
		}
	}
}

func TestOpenAPIErrorEnvelopePreservesCodeMsgDataContract(t *testing.T) {
	content, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	spec := string(content)
	start := strings.Index(spec, "    ErrorEnvelope:")
	if start < 0 {
		t.Fatal("ErrorEnvelope schema is missing")
	}
	section := spec[start:]
	if next := strings.Index(section[1:], "\n    PaginationMeta:"); next >= 0 {
		section = section[:next+1]
	}
	for _, property := range []string{"code:", "msg:", "data:"} {
		if !strings.Contains(section, property) {
			t.Errorf("ErrorEnvelope is missing %s", property)
		}
	}
}
