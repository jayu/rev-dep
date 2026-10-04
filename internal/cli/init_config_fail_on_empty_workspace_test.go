package cli

import (
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/config"
)

// TestInitConfig_EnablesFailOnEmptyWorkspace verifies that a freshly generated config opts in to
// failOnEmptyWorkspace at the root, so every generated workspace inherits it.
func TestInitConfig_EnablesFailOnEmptyWorkspace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"app"}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	res, err := InitConfig(dir, InitOptions{Detectors: DetectorsScaffold})
	if err != nil {
		t.Fatalf("InitConfig: %v", err)
	}
	content, err := os.ReadFile(res.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	cfg, err := config.ParseConfig(content)
	if err != nil {
		t.Fatalf("generated config does not parse: %v\n%s", err, content)
	}
	if !cfg.FailOnEmptyWorkspace {
		t.Errorf("generated config should set failOnEmptyWorkspace: true at the root:\n%s", content)
	}
	for i, rule := range cfg.Rules {
		if !rule.FailOnEmptyWorkspace {
			t.Errorf("workspaces[%d] (%s) does not inherit failOnEmptyWorkspace", i, rule.Path)
		}
	}
}
