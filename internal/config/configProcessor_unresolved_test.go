package config

import (
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/testutil"
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

func countUnresolvedByRequest(unresolved []checks.UnresolvedImport, request string) int {
	count := 0
	for _, u := range unresolved {
		if u.Request == request {
			count++
		}
	}
	return count
}

func hasUnresolvedForFileAndRequest(unresolved []checks.UnresolvedImport, fileSuffix string, request string) bool {
	for _, u := range unresolved {
		if filepath.ToSlash(u.FilePath) == filepath.ToSlash(fileSuffix) && u.Request == request {
			return true
		}
	}
	return false
}

func TestConfigProcessor_UnresolvedImports(t *testing.T) {
	testCwd, err := testutil.FixturePath("configProcessorProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}

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

	t.Run("ignore map supports glob paths", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"**/broken-import.ts": "non-existent-pkg"
						}
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		for _, u := range unresolved {
			if filepath.Base(u.FilePath) == "broken-import.ts" && u.Request == "non-existent-pkg" {
				t.Fatalf("Expected unresolved import to be suppressed by ignore glob")
			}
		}
	})

	t.Run("ignore map supports glob import values", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"src/index.ts": "non-existent-*"
						}
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		srcIndexPath := filepath.ToSlash(filepath.Join(testCwd, "src", "index.ts"))
		if hasUnresolvedForFileAndRequest(unresolved, srcIndexPath, "non-existent-module") {
			t.Fatalf("Expected unresolved import to be suppressed by glob ignore value")
		}
	})

	t.Run("ignore map supports array of import globs", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.6",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignore": {
							"src/index.ts": ["non-existent-*", "missing-*"]
						}
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		srcIndexPath := filepath.ToSlash(filepath.Join(testCwd, "src", "index.ts"))
		if hasUnresolvedForFileAndRequest(unresolved, srcIndexPath, "non-existent-module") {
			t.Fatalf("Expected unresolved import to be suppressed by ignore array value")
		}
	})

	t.Run("ignoreImports supports glob", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": {
						"enabled": true,
						"ignoreImports": ["non-existent-*"]
					}
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		if len(unresolved) != 0 {
			t.Fatalf("Expected unresolved imports to be suppressed by ignoreImports glob, got %d", len(unresolved))
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

	t.Run("customAssetExtensions suppresses unresolved imports for custom assets", func(t *testing.T) {
		customImporterPath := filepath.Join(testCwd, "src", "custom-asset-import.ts")
		customAssetPath := filepath.Join(testCwd, "src", "logo.custom")

		if err := os.WriteFile(customImporterPath, []byte("import logo from './logo.custom';\nconsole.log(logo);\n"), 0644); err != nil {
			t.Fatalf("Failed to write custom importer fixture: %v", err)
		}
		if err := os.WriteFile(customAssetPath, []byte("asset"), 0644); err != nil {
			t.Fatalf("Failed to write custom asset fixture: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Remove(customImporterPath)
			_ = os.Remove(customAssetPath)
		})

		cfg := `{
			"configVersion": "1.6",
			"customAssetExtensions": ["custom"],
			"rules": [
				{
					"path": ".",
					"unresolvedImportsDetection": { "enabled": true }
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg)
		unresolved := result.RuleResults[0].UnresolvedImports
		customImporterInternalPath := filepath.ToSlash(filepath.Join(testCwd, "src", "custom-asset-import.ts"))
		if hasUnresolvedForFileAndRequest(unresolved, customImporterInternalPath, "./logo.custom") {
			t.Fatalf("Expected unresolved custom asset import to be suppressed by customAssetExtensions")
		}
	})
}
