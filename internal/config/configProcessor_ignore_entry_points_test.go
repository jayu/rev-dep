package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeIgnoreEntryPointsProject creates a small project on disk and returns its root.
// Layout:
//
//	src/index.ts    - prod entry point, imports and uses `a` from used.ts
//	src/used.ts     - exports `a` (used by index.ts)
//	src/leftover.ts - not imported by anything; exports `b` and `c` (both unused)
func writeIgnoreEntryPointsProject(t *testing.T) string {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "rev-dep-ignore-entry-points-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	files := map[string]string{
		"src/index.ts":    "import { a } from './used';\nconsole.log(a);\n",
		"src/used.ts":     "export const a = 1;\n",
		"src/leftover.ts": "export const b = 2;\nexport const c = 3;\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(tempDir, rel), []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", rel, err)
		}
	}

	return tempDir
}

func loadAndProcessConfigFromJSON(t *testing.T, cwd, cfg string) *ConfigProcessingResult {
	t.Helper()

	configPath := filepath.Join(cwd, "rev-dep.config.json")
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(configPath) })

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	result, err := ProcessConfig(&config, cwd, "", "", false, false)
	if err != nil {
		t.Fatalf("ProcessConfig failed: %v", err)
	}
	return result
}

func containsPathWithSuffix(paths []string, suffix string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

// Baseline: without ignoreEntryPoints, the leftover file is reported as an orphan
// and its exports are reported as unused. This guards the meaningfulness of the
// suppression test below.
func TestConfigProcessor_IgnoreEntryPoints_BaselineReportsLeftover(t *testing.T) {
	cwd := writeIgnoreEntryPointsProject(t)

	cfg := `{
		"configVersion": "1.8",
		"rules": [
			{
				"path": ".",
				"prodEntryPoints": ["src/index.ts"],
				"orphanFilesDetection": { "enabled": true },
				"unusedExportsDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcessConfigFromJSON(t, cwd, cfg)
	ruleResult := result.RuleResults[0]

	if !containsPathWithSuffix(ruleResult.OrphanFiles, "leftover.ts") {
		t.Errorf("Expected leftover.ts to be reported as orphan in baseline, got orphans: %v", ruleResult.OrphanFiles)
	}

	unusedNames := []string{}
	for _, ue := range ruleResult.UnusedExports {
		if strings.HasSuffix(ue.FilePath, "leftover.ts") {
			unusedNames = append(unusedNames, ue.ExportName)
		}
	}
	if len(unusedNames) != 2 {
		t.Errorf("Expected 2 unused exports (b, c) for leftover.ts in baseline, got: %v", unusedNames)
	}
}

// With ignoreEntryPoints, the leftover file must NOT be reported as an orphan and
// its unused exports must NOT be reported.
func TestConfigProcessor_IgnoreEntryPoints_SuppressesOrphanAndUnusedExports(t *testing.T) {
	cwd := writeIgnoreEntryPointsProject(t)

	cfg := `{
		"configVersion": "1.8",
		"rules": [
			{
				"path": ".",
				"prodEntryPoints": ["src/index.ts"],
				"ignoreEntryPoints": ["src/leftover.ts"],
				"orphanFilesDetection": { "enabled": true },
				"unusedExportsDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcessConfigFromJSON(t, cwd, cfg)
	ruleResult := result.RuleResults[0]

	if containsPathWithSuffix(ruleResult.OrphanFiles, "leftover.ts") {
		t.Errorf("Ignored entry point leftover.ts should NOT be reported as orphan, got orphans: %v", ruleResult.OrphanFiles)
	}

	for _, ue := range ruleResult.UnusedExports {
		if strings.HasSuffix(ue.FilePath, "leftover.ts") {
			t.Errorf("Ignored entry point leftover.ts should NOT report unused export %q", ue.ExportName)
		}
	}
}

// The processor captures, per bucket, the entry-point glob patterns that do not
// match any file in the rule's workspace.
func TestConfigProcessor_UnmatchedEntryPointPatterns(t *testing.T) {
	cwd := writeIgnoreEntryPointsProject(t)

	cfg := `{
		"configVersion": "1.8",
		"rules": [
			{
				"path": ".",
				"prodEntryPoints": ["src/index.ts", "src/does-not-exist.ts"],
				"devEntryPoints": ["**/*.test.ts"],
				"ignoreEntryPoints": ["src/leftover.ts", "src/ghost.ts"],
				"orphanFilesDetection": { "enabled": true }
			}
		]
	}`

	result := loadAndProcessConfigFromJSON(t, cwd, cfg)
	unmatched := result.RuleResults[0].UnmatchedEntryPointPatterns

	assertExactlyPatterns(t, "prodEntryPoints", unmatched.ProdEntryPoints, []string{"src/does-not-exist.ts"})
	assertExactlyPatterns(t, "devEntryPoints", unmatched.DevEntryPoints, []string{"**/*.test.ts"})
	assertExactlyPatterns(t, "ignoreEntryPoints", unmatched.IgnoreEntryPoints, []string{"src/ghost.ts"})
}

func assertExactlyPatterns(t *testing.T, bucket string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: expected unmatched %v, got %v", bucket, want, got)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s: expected unmatched %v, got %v", bucket, want, got)
			return
		}
	}
}
