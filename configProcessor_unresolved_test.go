package main

import (
	"os"
	"path/filepath"
	"testing"
)

func loadAndProcessUnresolvedConfig(t *testing.T, testCwd string, cfg string) *ConfigProcessingResult {
	t.Helper()

	configPath := filepath.Join(testCwd, "unresolved-config.json")
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(configPath)
	})

	configs, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&configs[0], testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	return result
}

func countUnresolvedByRequest(unresolved []UnresolvedImport, request string) int {
	count := 0
	for _, u := range unresolved {
		if u.Request == request {
			count++
		}
	}
	return count
}

func hasUnresolvedForFileAndRequest(unresolved []UnresolvedImport, fileSuffix string, request string) bool {
	for _, u := range unresolved {
		if filepath.ToSlash(u.FilePath) == filepath.ToSlash(fileSuffix) && u.Request == request {
			return true
		}
	}
	return false
}

func TestConfigProcessor_UnresolvedImports(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__", "configProcessorProject")

	t.Run("baseline unresolved imports are detected", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": { "enabled": true }
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		if len(result.RuleResults) == 0 {
			t.Fatalf("Expected at least one rule result")
		}

		unresolved := result.RuleResults[0].UnresolvedImports
		if len(unresolved) == 0 {
			t.Fatalf("Expected unresolved imports in fixture project, got none")
		}

		if countUnresolvedByRequest(unresolved, "non-existent-module") == 0 {
			t.Errorf("Expected unresolved request non-existent-module to be present")
		}
		if countUnresolvedByRequest(unresolved, "non-existent-pkg") == 0 {
			t.Errorf("Expected unresolved request non-existent-pkg to be present")
		}
	})

	t.Run("ignore map suppresses exact file-request pair only", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"src/index.ts": "non-existent-module"
						}
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports

		srcIndexPath := filepath.ToSlash(filepath.Join(testCwd, "src", "index.ts"))
		if hasUnresolvedForFileAndRequest(unresolved, srcIndexPath, "non-existent-module") {
			t.Errorf("Expected src/index.ts -> non-existent-module to be suppressed")
		}

		if countUnresolvedByRequest(unresolved, "non-existent-pkg") == 0 {
			t.Errorf("Expected unrelated unresolved request non-existent-pkg to remain")
		}
	})

	t.Run("ignoreFiles suppresses all unresolved imports for matching files", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignoreFiles": ["**/broken-import.ts"]
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports

		for _, u := range unresolved {
			if filepath.Base(u.FilePath) == "broken-import.ts" {
				t.Errorf("Expected broken-import.ts to be fully suppressed, found %q", u.Request)
			}
		}

		if countUnresolvedByRequest(unresolved, "non-existent-module") == 0 {
			t.Errorf("Expected unresolved request from non-ignored files to remain")
		}
	})

	t.Run("ignoreImports suppresses matching requests globally", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignoreImports": ["non-existent-module"]
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports

		if countUnresolvedByRequest(unresolved, "non-existent-module") != 0 {
			t.Errorf("Expected non-existent-module to be fully suppressed")
		}
		if countUnresolvedByRequest(unresolved, "non-existent-pkg") == 0 {
			t.Errorf("Expected non-existent-pkg to remain")
		}
	})

	t.Run("combined ignore options suppress all configured violations", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"src/index.ts": "non-existent-module"
						},
						"ignoreFiles": ["**/broken-import.ts"],
						"ignoreImports": ["non-existent-module", "non-existent-pkg"]
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		if len(unresolved) != 0 {
			t.Fatalf("Expected all unresolved imports to be suppressed, got %d", len(unresolved))
		}
	})

	t.Run("ignore map path is resolved relative to rule path", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": "packages/subpkg",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"src/broken-import.ts": "non-existent-pkg"
						}
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		if len(unresolved) != 0 {
			t.Fatalf("Expected unresolved import to be suppressed with rule-relative ignore path, got %d", len(unresolved))
		}
	})

	t.Run("ignoreFiles glob is resolved relative to rule path", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": "packages/subpkg",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignoreFiles": ["src/broken-import.ts"]
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		if len(unresolved) != 0 {
			t.Fatalf("Expected unresolved import to be suppressed with rule-relative ignoreFiles glob, got %d", len(unresolved))
		}
	})
}
