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

func TestPreferencesDefaultsAndValidation(t *testing.T) {
	defaults := DefaultPreferences()
	if defaults.Theme != ThemeEnterprise || defaults.ColorMode != ColorModeSystem || defaults.AccentColor != nil || defaults.FontScale != FontScaleStandard || defaults.RadiusScale != RadiusScaleCompact {
		t.Fatalf("defaults = %+v", defaults)
	}

	validAccent := "#1a2B3c"
	if err := (Preferences{
		Theme:       ThemeCyberpunk,
		ColorMode:   ColorModeDark,
		AccentColor: &validAccent,
		FontScale:   FontScaleLarge,
		RadiusScale: RadiusScaleRounded,
	}).Validate(); err != nil {
		t.Fatalf("Validate() valid preferences: %v", err)
	}

	for _, value := range []Preferences{
		{Theme: "unknown", ColorMode: ColorModeSystem, FontScale: FontScaleStandard, RadiusScale: RadiusScaleCompact},
		{Theme: ThemeEnterprise, ColorMode: "unknown", FontScale: FontScaleStandard, RadiusScale: RadiusScaleCompact},
		{Theme: ThemeEnterprise, ColorMode: ColorModeSystem, FontScale: "unknown", RadiusScale: RadiusScaleCompact},
		{Theme: ThemeEnterprise, ColorMode: ColorModeSystem, FontScale: FontScaleStandard, RadiusScale: "unknown"},
		{Theme: ThemeEnterprise, ColorMode: ColorModeSystem, AccentColor: stringPointer("#12345"), FontScale: FontScaleStandard, RadiusScale: RadiusScaleCompact},
		{Theme: ThemeEnterprise, ColorMode: ColorModeSystem, AccentColor: stringPointer("blue"), FontScale: FontScaleStandard, RadiusScale: RadiusScaleCompact},
	} {
		if err := value.Validate(); err == nil {
			t.Errorf("Validate() accepted %+v", value)
		}
	}
}

func stringPointer(value string) *string { return &value }
