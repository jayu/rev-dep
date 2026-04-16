package config

import (
	"strings"
	"testing"
)

func TestParseConfig_ValidVersion(t *testing.T) {
	content := []byte(`{
        "configVersion": "1.0",
        "rules": [{"path": "."}]
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
        "configVersion": "2.0",
        "rules": [{"path": "."}]
    }`)

	_, err := ParseConfig(content)
	if err == nil {
		t.Fatalf("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported configVersion") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
