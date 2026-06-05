package config

import (
	"strings"
	"testing"
)

func TestParseConfig_ValidVersion(t *testing.T) {
	content := []byte(`{
        "configVersion": "1.0",
        "workspaces": [{"path": "."}]
    }`)

	config, err := ParseConfig(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if config.ConfigVersion != "1.0" {
		t.Fatalf("expected configVersion 1.0, got %s", config.ConfigVersion)
	}
}

func TestParseConfig_UnsupportedVersion(t *testing.T) {
	content := []byte(`{
        "configVersion": "99.0",
        "workspaces": [{"path": "."}]
    }`)

	_, err := ParseConfig(content)
	if err == nil {
		t.Fatalf("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported configVersion") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// The top-level `rules` field was renamed to `workspaces` in v3. A config still
// using `rules` must fail with a dedicated, actionable error rather than a
// generic "unknown field" message.
func TestParseConfig_LegacyRulesFieldRejected(t *testing.T) {
	content := []byte(`{
        "configVersion": "2.0",
        "rules": [{"path": "."}]
    }`)

	_, err := ParseConfig(content)
	if err == nil {
		t.Fatalf("expected error when using the legacy `rules` field")
	}
	if !strings.Contains(err.Error(), "renamed to `workspaces`") {
		t.Fatalf("expected a dedicated rename error mentioning `workspaces`, got: %v", err)
	}
}
