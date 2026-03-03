package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindOrphanFiles(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/orphanFilesProject")

	// Create a minimal dependency tree for testing
	// Note: index.ts imports helper.ts, helper.ts imports constants.ts
	// So index.ts is an entry point, constants.ts is referenced, orphan.ts is not referenced
	minimalTree := MinimalDependencyTree{
		"src/index.ts": {
			{ID: "src/utils/helper.ts", ImportKind: NotTypeOrMixedImport},
		},
		"src/utils/helper.ts": {
			{ID: "src/utils/constants.ts", ImportKind: NotTypeOrMixedImport},
		},
		"src/utils/constants.ts": {}, // Referenced by helper.ts
		"src/utils/orphan.ts":    {}, // Not imported by any file
		"src/config.json":        {}, // Config file, should be valid entry point
		"README.md":              {}, // Should be ignored by default patterns
	}

	t.Run("should find orphan files", func(t *testing.T) {
		validEntryPoints := []string{"**/*config*", "**/*.md", "src/index.ts"}
		graphExclude := []string{}
		ignoreTypeImports := false

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd, nil, nil)

		expected := []string{"src/utils/orphan.ts"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan files %v, got %v", expected, orphanFiles)
		}
	})

	t.Run("should respect graph exclude patterns", func(t *testing.T) {
		validEntryPoints := []string{"**/*config*", "**/*.md", "src/index.ts"}
		graphExclude := []string{"src/utils/orphan.ts"}
		ignoreTypeImports := false

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd, nil, nil)

		// Orphan file should be excluded from results
		expected := []string{}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected no orphan files when excluded, got %v", orphanFiles)
		}
	})

	t.Run("should respect valid entry points", func(t *testing.T) {
		validEntryPoints := []string{"**/*config*", "**/*.md", "src/utils/orphan.ts", "src/index.ts"}
		graphExclude := []string{}
		ignoreTypeImports := false

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd, nil, nil)

		// Orphan file should not be considered orphan since it's a valid entry point
		expected := []string{}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected no orphan files when orphan.ts is entry point, got %v", orphanFiles)
		}
	})

	t.Run("should ignore type imports when ignoreTypeImports is true", func(t *testing.T) {
		// Create tree with type-only imports
		typeImportKind := OnlyTypeImport
		minimalTreeWithTypeImports := MinimalDependencyTree{
			"src/index.ts": {
				{ID: "src/utils/orphan.ts", ImportKind: typeImportKind}, // type-only import
			},
			"src/utils/orphan.ts": {}, // Only imported via type-only import
		}

		validEntryPoints := []string{"src/index.ts"}
		graphExclude := []string{}
		ignoreTypeImports := true

		orphanFiles := FindOrphanFiles(minimalTreeWithTypeImports, validEntryPoints, graphExclude, ignoreTypeImports, testCwd, nil, nil)

		// orphan.ts should be considered orphan since type-only imports are ignored
		expected := []string{"src/utils/orphan.ts"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan.ts to be orphan when type imports are ignored, got %v", orphanFiles)
		}
	})
}

func TestFindOrphanFiles_ModuleSuffixVariants(t *testing.T) {
	currentDir, _ := os.Getwd()
	testCwd := filepath.Join(currentDir, "__fixtures__/orphanFilesProject")

	t.Run("should not report module-suffix variants as orphans", func(t *testing.T) {
		// button.ios.tsx is imported (resolved), button.android.tsx and button.tsx are unreferenced variants
		minimalTree := MinimalDependencyTree{
			"src/index.ts": {
				{ID: "src/button.ios.tsx", ImportKind: NotTypeOrMixedImport},
			},
			"src/button.ios.tsx":     {},
			"src/button.android.tsx": {},
			"src/button.tsx":         {},
			"src/button.web.tsx":     {}, // not listed in variants, should be reported as orphan
			"src/orphan.ts":          {}, // truly orphan
		}

		variants := map[string]bool{
			"src/button.ios.tsx":     true,
			"src/button.android.tsx": true,
			"src/button.tsx":         true,
		}

		orphanFiles := FindOrphanFiles(
			minimalTree,
			[]string{"src/index.ts"},
			[]string{},
			false,
			testCwd,
			variants,
			nil,
		)

		expected := []string{"src/button.web.tsx", "src/orphan.ts"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan files %v, got %v", expected, orphanFiles)
		}
	})

	t.Run("should report orphans normally when no variants", func(t *testing.T) {
		minimalTree := MinimalDependencyTree{
			"src/index.ts": {
				{ID: "src/button.ios.tsx", ImportKind: NotTypeOrMixedImport},
			},
			"src/button.ios.tsx":     {},
			"src/button.android.tsx": {},
		}

		orphanFiles := FindOrphanFiles(
			minimalTree,
			[]string{"src/index.ts"},
			[]string{},
			false,
			testCwd,
			nil,
			nil,
		)

		expected := []string{"src/button.android.tsx"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan files %v, got %v", expected, orphanFiles)
		}
	})
}

// TestFindOrphanFiles_WorkspaceEntryPoints verifies that workspace package entry
// point files are not falsely flagged as orphans in a pnpm monorepo.
//
// When followMonorepoPackages is enabled, files like packages/llm-clients/src/index.ts
// are reachable only via cross-package imports. Without the additionalEntryPointFiles
// exclusion they appear unreferenced and get flagged as orphans.
func TestFindOrphanFiles_WorkspaceEntryPoints(t *testing.T) {
	testCwd := "/workspace"

	// index.ts is the workspace package entry point — not directly imported within
	// the scanned tree but reachable from outside via package exports.
	minimalTree := MinimalDependencyTree{
		"/workspace/packages/llm-clients/src/index.ts":   {},
		"/workspace/packages/llm-clients/src/internal.ts": {},
	}

	// internal.ts is referenced by index.ts (so not orphan regardless).
	minimalTree["/workspace/packages/llm-clients/src/index.ts"] = []MinimalDependency{
		{ID: "/workspace/packages/llm-clients/src/internal.ts", ResolvedType: UserModule},
	}

	workspaceEntryPoints := map[string]bool{
		"/workspace/packages/llm-clients/src/index.ts": true,
	}

	t.Run("workspace entry point not flagged as orphan with additionalEntryPointFiles", func(t *testing.T) {
		orphans := FindOrphanFiles(minimalTree, []string{}, []string{}, false, testCwd, nil, workspaceEntryPoints)
		for _, o := range orphans {
			if o == "/workspace/packages/llm-clients/src/index.ts" {
				t.Errorf("index.ts should not be flagged as orphan — it is a workspace entry point")
			}
		}
	})

	t.Run("workspace entry point falsely flagged without additionalEntryPointFiles (regression guard)", func(t *testing.T) {
		orphans := FindOrphanFiles(minimalTree, []string{}, []string{}, false, testCwd, nil, nil)
		found := false
		for _, o := range orphans {
			if o == "/workspace/packages/llm-clients/src/index.ts" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected index.ts to be falsely flagged as orphan (regression guard), got: %v", orphans)
		}
	})
}
