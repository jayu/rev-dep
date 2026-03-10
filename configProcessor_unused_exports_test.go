package main

import (
	"os"
	"path/filepath"
	"testing"
)

func loadAndProcessUnusedExportsConfig(t *testing.T, testCwd string, cfg string) *ConfigProcessingResult {
	t.Helper()

	configPath := filepath.Join(testCwd, "unused-exports-config.json")
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

	result, err := ProcessConfig(&config, testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}

	return result
}

func TestConfigProcessor_UnusedExports(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/unusedExportsProject")

	config, err := LoadConfig(testCwd)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config, testCwd, "package.json", "tsconfig.json", false)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	if len(result.RuleResults) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.RuleResults))
	}

	ruleResult := result.RuleResults[0]

	// Verify that unused-exports is in enabled checks
	hasUnusedExportsCheck := false
	for _, check := range ruleResult.EnabledChecks {
		if check == "unused-exports" {
			hasUnusedExportsCheck = true
			break
		}
	}
	if !hasUnusedExportsCheck {
		t.Fatal("Expected 'unused-exports' in enabled checks")
	}

	// We expect `unusedHelper` from utils.ts and `Bar` from types.ts to be unused
	// `helper` is imported by consumer.ts AND re-exported by index.ts (entry point)
	// `Foo` is imported by consumer.ts AND re-exported by index.ts (entry point)
	// index.ts exports are not reported (it's an entry point)

	unusedExports := ruleResult.UnusedExports

	if len(unusedExports) == 0 {
		t.Fatal("Expected unused exports to be found")
	}

	// Check for the expected unused exports
	unusedNames := make(map[string]string) // exportName -> filePath
	for _, ue := range unusedExports {
		relPath, _ := filepath.Rel(testCwd, ue.FilePath)
		unusedNames[ue.ExportName] = filepath.ToSlash(relPath)
	}

	// unusedHelper from utils.ts should be unused
	if path, ok := unusedNames["unusedHelper"]; !ok {
		t.Error("Expected 'unusedHelper' to be in unused exports")
	} else if path != "src/utils.ts" {
		t.Errorf("Expected 'unusedHelper' from 'src/utils.ts', got from '%s'", path)
	}

	// Bar from types.ts should be unused
	if path, ok := unusedNames["Bar"]; !ok {
		t.Error("Expected 'Bar' to be in unused exports")
	} else if path != "src/types.ts" {
		t.Errorf("Expected 'Bar' from 'src/types.ts', got from '%s'", path)
	}

	// helper should NOT be unused (imported by consumer.ts and re-exported by index.ts)
	if _, ok := unusedNames["helper"]; ok {
		t.Error("Expected 'helper' to NOT be in unused exports")
	}

	// Foo should NOT be unused (imported by consumer.ts and re-exported by index.ts)
	if _, ok := unusedNames["Foo"]; ok {
		t.Error("Expected 'Foo' to NOT be in unused exports")
	}

	// Verify HasFailures is true
	if !result.HasFailures {
		t.Error("Expected HasFailures to be true")
	}

	// Verify there are fixable issues
	if result.FixableIssuesCount == 0 {
		t.Error("Expected FixableIssuesCount > 0")
	}
}

func TestConfigProcessor_UnusedExports_IgnoreOptions(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/unusedExportsProject")

	t.Run("ignoreFiles suppresses all exports from matching files", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignoreFiles": ["**/utils.ts"]
				}
			}]
		}`

		result := loadAndProcessUnusedExportsConfig(t, testCwd, cfg)
		unused := result.RuleResults[0].UnusedExports
		for _, item := range unused {
			if filepath.Base(item.FilePath) == "utils.ts" {
				t.Fatalf("Expected utils.ts exports to be ignored")
			}
		}
	})

	t.Run("ignore map supports file and export globs", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignore": {
						"**/types.ts": "B*"
					}
				}
			}]
		}`

		result := loadAndProcessUnusedExportsConfig(t, testCwd, cfg)
		unused := result.RuleResults[0].UnusedExports
		for _, item := range unused {
			if filepath.Base(item.FilePath) == "types.ts" && item.ExportName == "Bar" {
				t.Fatalf("Expected types.ts#Bar to be ignored")
			}
		}
	})

	t.Run("ignore map supports array of export globs", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.6",
			"rules": [{
				"path": ".",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignore": {
						"**/types.ts": ["B*", "F*"]
					}
				}
			}]
		}`

		result := loadAndProcessUnusedExportsConfig(t, testCwd, cfg)
		unused := result.RuleResults[0].UnusedExports
		for _, item := range unused {
			if filepath.Base(item.FilePath) == "types.ts" && (item.ExportName == "Bar" || item.ExportName == "Foo") {
				t.Fatalf("Expected types.ts exports Bar/Foo to be ignored via ignore array")
			}
		}
	})

	t.Run("ignoreExports supports globs", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.5",
			"rules": [{
				"path": ".",
				"unusedExportsDetection": {
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignoreExports": ["unused*", "B*"]
				}
			}]
		}`

		result := loadAndProcessUnusedExportsConfig(t, testCwd, cfg)
		unused := result.RuleResults[0].UnusedExports
		if len(unused) != 0 {
			t.Fatalf("Expected all unused exports to be ignored, got %d", len(unused))
		}
	})
}

func TestConfigProcessor_UnusedExports_MultipleDetections(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/unusedExportsProject")

	cfg := `{
		"configVersion": "1.6",
		"rules": [{
			"path": ".",
			"unusedExportsDetection": [
				{
					"enabled": true,
					"validEntryPoints": ["src/index.ts"]
				},
				{
					"enabled": true,
					"validEntryPoints": ["src/index.ts"],
					"ignoreExports": ["unusedHelper"]
				}
			]
		}]
	}`

	result := loadAndProcessUnusedExportsConfig(t, testCwd, cfg)
	if len(result.RuleResults) != 1 {
		t.Fatalf("Expected 1 rule result, got %d", len(result.RuleResults))
	}

	if len(result.RuleResults[0].UnusedExports) == 0 {
		t.Fatal("Expected unused exports from multiple detection instances")
	}
}

func TestConfigProcessor_UnusedExportsAutofix(t *testing.T) {
	// Create a temporary copy of the fixture project
	currentDir, _ := os.Getwd()
	srcFixture := filepath.Join(currentDir, "__fixtures__/unusedExportsProject")

	tmpDir, err := os.MkdirTemp("", "unused-exports-autofix-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Copy fixture files to temp dir
	copyFixtureDir(t, srcFixture, tmpDir)

	config, err := LoadConfig(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Run with fix=true
	result, err := ProcessConfig(&config, tmpDir, "package.json", "tsconfig.json", true)
	if err != nil {
		t.Fatalf("Failed to process config: %v", err)
	}

	// Verify fixes were applied
	if result.FixedFilesCount != 2 {
		t.Error("Expected 2 files to be fixed")
	}

	// Read the fixed utils.ts and check that "unusedHelper" no longer has "export"
	utilsContent, err := os.ReadFile(filepath.Join(tmpDir, "src", "utils.ts"))
	if err != nil {
		t.Fatalf("Failed to read fixed utils.ts: %v", err)
	}

	utilsStr := string(utilsContent)
	// The unused helper should have had its export keyword removed
	if contains(utilsStr, "export function unusedHelper") {
		t.Error("Expected 'export' to be removed from 'unusedHelper' after autofix")
	}
	// The used helper should still be exported
	if !contains(utilsStr, "export function helper") {
		t.Error("Expected 'export function helper' to still be present after autofix")
	}
}

// copyFixtureDir recursively copies a directory
func copyFixtureDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("Failed to read dir %s: %v", src, err)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatalf("Failed to create dir %s: %v", dst, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			copyFixtureDir(t, srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				t.Fatalf("Failed to read file %s: %v", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				t.Fatalf("Failed to write file %s: %v", dstPath, err)
			}
		}
	}
}
