package config

import (
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/testutil"
)

// TestConfigProcessor_NoRootWorkspaces covers the setup where there is NO root package.json
// declaring monorepo workspaces, but the rev-dep config lists package subdirectories (each
// with its own package.json) as rule paths. In that case the per-package package.json
// dependencies must still be parsed and provided to the resolver, so imports of declared
// node_modules resolve instead of all being reported as unresolved.
func TestConfigProcessor_NoRootWorkspaces(t *testing.T) {
	testCwd, err := testutil.FixturePath("noRootWorkspaces")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

	loadAndProcess := func(t *testing.T, cfg string) *ConfigProcessingResult {
		t.Helper()
		configPath := filepath.Join(testCwd, "no-root-workspaces-config.json")
		if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(configPath) })

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}
		result, err := ProcessConfig(&config, testCwd, false, false)
		if err != nil {
			t.Fatalf("ProcessConfig failed: %v", err)
		}
		return result
	}

	unresolvedRequests := func(unresolved []checks.UnresolvedImport) map[string]int {
		counts := map[string]int{}
		for _, u := range unresolved {
			counts[u.Request]++
		}
		return counts
	}

	cfg := `{
		"configVersion": "1.6",
		"workspaces": [
			{
				"path": "packages/pkg-a",
				"unresolvedImportsDetection": { "enabled": true }
			},
			{
				"path": "packages/pkg-b",
				"unresolvedImportsDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcess(t, cfg)
	if len(result.RuleResults) != 2 {
		t.Fatalf("Expected 2 rule results, got %d", len(result.RuleResults))
	}

	// Rule 0: packages/pkg-a declares "left-pad" -> must resolve, not be unresolved.
	pkgAUnresolved := unresolvedRequests(result.RuleResults[0].UnresolvedImports)
	if pkgAUnresolved["left-pad"] != 0 {
		t.Errorf("Expected 'left-pad' (declared in pkg-a/package.json) to resolve, but it was reported unresolved %d time(s)", pkgAUnresolved["left-pad"])
	}

	// Rule 1: packages/pkg-b declares "chalk" -> must resolve; the undeclared module must NOT.
	pkgBUnresolved := unresolvedRequests(result.RuleResults[1].UnresolvedImports)
	if pkgBUnresolved["chalk"] != 0 {
		t.Errorf("Expected 'chalk' (declared in pkg-b/package.json) to resolve, but it was reported unresolved %d time(s)", pkgBUnresolved["chalk"])
	}
	if pkgBUnresolved["genuinely-undeclared-module"] == 0 {
		t.Errorf("Expected 'genuinely-undeclared-module' (not declared anywhere) to remain unresolved, but it was suppressed")
	}
}
