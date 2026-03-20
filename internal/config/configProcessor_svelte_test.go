package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/testutil"
)

func loadAndProcessSvelteRevDepConfig(t *testing.T, testCwd string, cfg string) *ConfigProcessingResult {
	t.Helper()

	configPath := filepath.Join(testCwd, "rev-dep.svelte-test.config.json")
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(configPath)
	})

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config, testCwd, "package.json", "tsconfig.json", false, false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	return result
}

func TestConfigProcessor_SvelteUnresolvedImports(t *testing.T) {
	testCwd, err := testutil.FixturePath("svelteConfigProcessorProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

	cfg := `{
		"configVersion": "1.6",
		"rules": [
			{
				"path": ".",
				"unresolvedImportsDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcessSvelteRevDepConfig(t, testCwd, cfg)
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least one rule result")
	}

	unresolved := result.RuleResults[0].UnresolvedImports
	foundMissingPkgInSvelte := false
	foundAppSvelteAsUnresolved := false
	for _, item := range unresolved {
		if strings.HasSuffix(filepath.ToSlash(item.FilePath), "/src/App.svelte") && item.Request == "non-existent-svelte-pkg" {
			foundMissingPkgInSvelte = true
		}
		if item.Request == "./App.svelte" {
			foundAppSvelteAsUnresolved = true
		}
	}

	if !foundMissingPkgInSvelte {
		t.Fatalf("Expected unresolved import non-existent-svelte-pkg from src/App.svelte, got %+v", unresolved)
	}
	if foundAppSvelteAsUnresolved {
		t.Fatalf("Did not expect ./App.svelte to be unresolved, got %+v", unresolved)
	}
}

func TestConfigProcessor_SvelteModuleBoundaries(t *testing.T) {
	testCwd, err := testutil.FixturePath("svelteConfigProcessorProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

	cfg := `{
		"configVersion": "1.6",
		"rules": [
			{
				"path": ".",
				"moduleBoundaries": [
					{
						"name": "no-private-imports",
						"pattern": "src/**",
						"deny": ["**/private.ts"]
					}
				]
			}
		]
	}`

	result := loadAndProcessSvelteRevDepConfig(t, testCwd, cfg)
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least one rule result")
	}

	violations := result.RuleResults[0].ModuleBoundaryViolations
	found := false
	for _, v := range violations {
		filePath := filepath.ToSlash(v.FilePath)
		importPath := filepath.ToSlash(v.ImportPath)
		if strings.HasSuffix(filePath, "/src/App.svelte") &&
			strings.HasSuffix(importPath, "/src/private.ts") &&
			v.RuleName == "no-private-imports" &&
			v.ViolationType == "denied" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected denied boundary violation App.svelte -> private.ts, got %+v", violations)
	}
}
