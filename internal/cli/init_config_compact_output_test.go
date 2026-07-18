package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitConfig_GeneratesCompactDetectors verifies that config init writes detector declarations in
// the compact form: enabled detectors as bare booleans, disabled scaffold detectors as `false`, and
// never a redundant "enabled": true.
func TestInitConfig_GeneratesCompactDetectors(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-init-compact")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	// Scaffold: circular + unresolved enabled, the rest listed but disabled.
	res, err := InitConfig(dir, InitOptions{Detectors: DetectorsScaffold})
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	content, err := os.ReadFile(res.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(content)

	if strings.Contains(got, `"enabled": true`) {
		t.Errorf("generated config should not contain a redundant \"enabled\": true:\n%s", got)
	}
	for _, want := range []string{
		`"circularImportsDetection": true`,
		`"unresolvedImportsDetection": true`,
		`"orphanFilesDetection": false`,
		`"unusedNodeModulesDetection": false`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected generated config to contain %q, got:\n%s", want, got)
		}
	}
}
