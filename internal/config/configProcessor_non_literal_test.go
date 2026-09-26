package config

import (
	"path/filepath"
	"testing"

	"strings"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/testutil"
)

// The fixture has three dynamic imports in src/index.ts: two non-literal ones
// (`import('./pages/' + name)`, `import(path)`) and one real `import('./real')`.
func nonLiteralFixtureCwd(t *testing.T) string {
	t.Helper()
	cwd, err := testutil.FixturePath("nonLiteralImportsProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	return cwd
}

func TestConfigProcessor_NonLiteralImports(t *testing.T) {
	testCwd := nonLiteralFixtureCwd(t)

	t.Run("not reported by default", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"workspaces": [
				{
					"path": ".",
					"prodEntryPoints": ["src/index.ts"],
					"unresolvedImportsDetection": { "enabled": true }
				}
			]
		}`

		unresolved := loadAndProcessUnresolvedConfig(t, testCwd, cfg).RuleResults[0].UnresolvedImports
		if len(unresolved) != 0 {
			t.Fatalf("expected no unresolved imports by default, got %+v", unresolved)
		}
	})

	t.Run("reported with reportNonLiteralImports", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"workspaces": [
				{
					"path": ".",
					"prodEntryPoints": ["src/index.ts"],
					"unresolvedImportsDetection": { "enabled": true, "reportNonLiteralImports": true }
				}
			]
		}`

		unresolved := loadAndProcessUnresolvedConfig(t, testCwd, cfg).RuleResults[0].UnresolvedImports
		if len(unresolved) != 2 {
			t.Fatalf("expected the two non-literal imports, got %+v", unresolved)
		}
		indexPath := filepath.ToSlash(filepath.Join(testCwd, "src", "index.ts"))
		for _, u := range unresolved {
			if !u.IsNonLiteral {
				t.Fatalf("expected non-literal records, got %+v", u)
			}
			// Request is the expression as written, not a path.
			if u.Request != "'./pages/' + name" && u.Request != "path" {
				t.Fatalf("unexpected non-literal request %q", u.Request)
			}
			if filepath.ToSlash(u.FilePath) != indexPath {
				t.Fatalf("expected the record on src/index.ts, got %q", u.FilePath)
			}
		}
	})

	// A non-literal import is not a dependency of anything: it must not become an edge, a missing
	// package, an orphan file or a used export.
	t.Run("invisible to the other checks", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"workspaces": [
				{
					"path": ".",
					"prodEntryPoints": ["src/index.ts"],
					"unresolvedImportsDetection": { "enabled": true, "reportNonLiteralImports": true },
					"missingNodeModulesDetection": { "enabled": true },
					"unusedNodeModulesDetection": { "enabled": true },
					"orphanFilesDetection": { "enabled": true },
					"unusedExportsDetection": { "enabled": true },
					"circularImportsDetection": { "enabled": true }
				}
			]
		}`

		result := loadAndProcessUnresolvedConfig(t, testCwd, cfg).RuleResults[0]

		for _, missing := range result.MissingNodeModules {
			if strings.Contains(missing.ModuleName, "+") || missing.ModuleName == "path" {
				t.Errorf("non-literal import reported as a missing node module: %+v", missing)
			}
		}
		for _, orphan := range result.OrphanFiles {
			if filepath.Base(orphan) == "real.ts" {
				t.Errorf("a file imported with a literal path became an orphan: %q", orphan)
			}
		}
		for _, unused := range result.UnusedExports {
			if strings.Contains(unused.ExportName, "+") {
				t.Errorf("non-literal import leaked into unused exports: %+v", unused)
			}
		}
		if len(result.CircularDependencies) != 0 {
			t.Errorf("unexpected circular imports: %+v", result.CircularDependencies)
		}

		// src/pages/home.ts is only reachable through `import('./pages/' + name)`, so it stays an
		// orphan: rev-dep cannot know which file that expression names.
		foundHome := false
		for _, orphan := range result.OrphanFiles {
			if filepath.Base(orphan) == "home.ts" {
				foundHome = true
			}
		}
		if !foundHome {
			t.Errorf("expected src/pages/home.ts to stay an orphan, got %+v", result.OrphanFiles)
		}
	})
}

func TestDetectUnresolvedImports_NonLiteralIgnoredLikeAnyRequest(t *testing.T) {
	testCwd := nonLiteralFixtureCwd(t)
	cfg := `{
		"configVersion": "1.3",
		"workspaces": [
			{
				"path": ".",
				"prodEntryPoints": ["src/index.ts"],
				"unresolvedImportsDetection": {
					"enabled": true,
					"reportNonLiteralImports": true,
					"ignoreFiles": ["**/index.ts"]
				}
			}
		]
	}`

	unresolved := loadAndProcessUnresolvedConfig(t, testCwd, cfg).RuleResults[0].UnresolvedImports
	if len(unresolved) != 0 {
		t.Fatalf("ignoreFiles should suppress non-literal records too, got %+v", unresolved)
	}
	var _ []checks.UnresolvedImport = unresolved
}
