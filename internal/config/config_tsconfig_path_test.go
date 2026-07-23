package config

import (
	"strings"
	"testing"
)

func TestParseConfig_TsConfigPathValidation(t *testing.T) {
	cases := []struct {
		name      string
		value     string // raw JSON value for tsConfigPath
		wantError string // "" means the config must parse
	}{
		{name: "valid relative path", value: `"tsconfig.build.json"`, wantError: ""},
		{name: "valid nested path", value: `"config/tsconfig.prod.json"`, wantError: ""},
		{name: "empty string", value: `""`, wantError: "cannot be empty"},
		{name: "parent escape", value: `"../tsconfig.json"`, wantError: "'../'"},
		{name: "absolute path", value: `"/etc/tsconfig.json"`, wantError: "absolute path"},
		{name: "wrong type", value: `123`, wantError: "must be a string"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := `{
				"configVersion": "1.3",
				"workspaces": [
					{ "path": ".", "tsConfigPath": ` + c.value + ` }
				]
			}`

			parsed, err := ParseConfig([]byte(cfg))

			if c.wantError == "" {
				if err != nil {
					t.Fatalf("expected config to parse, got error: %v", err)
				}
				want := strings.Trim(c.value, `"`)
				if parsed.Rules[0].TsConfigPath != want {
					t.Errorf("TsConfigPath = %q, want %q", parsed.Rules[0].TsConfigPath, want)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantError)
			}
			if !strings.Contains(err.Error(), c.wantError) {
				t.Errorf("error %q does not contain %q", err.Error(), c.wantError)
			}
		})
	}
}
