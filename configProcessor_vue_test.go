package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadAndProcessVueRevDepConfig(t *testing.T, testCwd string, cfg string) *ConfigProcessingResult {
	t.Helper()

	configPath := filepath.Join(testCwd, "rev-dep.vue-test.config.json")
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

func TestConfigProcessor_VueUnresolvedImports(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__", "vueConfigProcessorProject")

	cfg := `{
		"configVersion": "1.6",
		"rules": [
			{
				"path": ".",
				"unresolvedImportsDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcessVueRevDepConfig(t, testCwd, cfg)
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least one rule result")
	}

	unresolved := result.RuleResults[0].UnresolvedImports
	foundMissingPkgInVue := false
	foundAppVueAsUnresolved := false
	for _, item := range unresolved {
		if strings.HasSuffix(filepath.ToSlash(item.FilePath), "/src/App.vue") && item.Request == "non-existent-vue-pkg" {
			foundMissingPkgInVue = true
		}
		if item.Request == "./App.vue" {
			foundAppVueAsUnresolved = true
		}
	}

	if !foundMissingPkgInVue {
		t.Fatalf("Expected unresolved import non-existent-vue-pkg from src/App.vue, got %+v", unresolved)
	}
	if foundAppVueAsUnresolved {
		t.Fatalf("Did not expect ./App.vue to be unresolved, got %+v", unresolved)
	}
}

func TestConfigProcessor_VueModuleBoundaries(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__", "vueConfigProcessorProject")

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

	result := loadAndProcessVueRevDepConfig(t, testCwd, cfg)
	if len(result.RuleResults) == 0 {
		t.Fatalf("Expected at least one rule result")
	}

	violations := result.RuleResults[0].ModuleBoundaryViolations
	found := false
	for _, v := range violations {
		filePath := filepath.ToSlash(v.FilePath)
		importPath := filepath.ToSlash(v.ImportPath)
		if strings.HasSuffix(filePath, "/src/App.vue") &&
			strings.HasSuffix(importPath, "/src/private.ts") &&
			v.RuleName == "no-private-imports" &&
			v.ViolationType == "denied" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Expected denied boundary violation App.vue -> private.ts, got %+v", violations)
	}
}
