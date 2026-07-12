package iam

import (
	"encoding/json"
	"testing"
)

func TestIAMModelsUseOpenAPIJSONFieldNames(t *testing.T) {
	for name, value := range map[string]any{
		"role":       Role{ID: "role-1", Name: "admin", Description: "Administrator"},
		"permission": Permission{ID: "permission-1", Name: "users:read", Description: "Read users"},
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			for _, field := range []string{"id", "name", "description"} {
				if _, ok := fields[field]; !ok {
					t.Fatalf("JSON %s = %s; missing %q", name, encoded, field)
				}
			}
		})
	}
}

func TestCreateUserInputDecodesOpenAPIJSONFieldNames(t *testing.T) {
	var input CreateUserInput
	if err := json.Unmarshal([]byte(`{"email":"a@example.com","username":"alice","password":"long-enough-password","display_name":"Alice"}`), &input); err != nil {
		t.Fatal(err)
	}
	if input.Email != "a@example.com" || input.Username != "alice" || input.Password != "long-enough-password" || input.DisplayName != "Alice" {
		t.Fatalf("decoded input = %+v", input)
	}
}
