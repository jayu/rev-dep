package config

import (
	"strings"
	"testing"
)

// The `algorithm` option was removed in v3 - cycle detection always uses SCC.
// A config still setting it must fail with a dedicated, actionable error rather
// than a generic "unknown field" message.
func TestParseConfig_CircularImportsAlgorithmRejected(t *testing.T) {
	content := []byte(`{
        "configVersion": "2.0",
        "workspaces": [{
            "path": ".",
            "circularImportsDetection": { "enabled": true, "algorithm": "SCC" }
        }]
    }`)

	_, err := ParseConfig(content)
	if err == nil {
		t.Fatalf("expected error when using the removed `algorithm` field")
	}
	if !strings.Contains(err.Error(), "`algorithm` option was removed in v3") {
		t.Fatalf("expected a dedicated removal error, got: %v", err)
	}
}
