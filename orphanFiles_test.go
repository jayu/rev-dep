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
			{ID: stringPtr("src/utils/helper.ts"), ImportKind: nil},
		},
		"src/utils/helper.ts": {
			{ID: stringPtr("src/utils/constants.ts"), ImportKind: nil},
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

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd)

		expected := []string{"src/utils/orphan.ts"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan files %v, got %v", expected, orphanFiles)
		}
	})

	t.Run("should respect graph exclude patterns", func(t *testing.T) {
		validEntryPoints := []string{"**/*config*", "**/*.md", "src/index.ts"}
		graphExclude := []string{"src/utils/orphan.ts"}
		ignoreTypeImports := false

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd)

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

		orphanFiles := FindOrphanFiles(minimalTree, validEntryPoints, graphExclude, ignoreTypeImports, testCwd)

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
				{ID: stringPtr("src/utils/orphan.ts"), ImportKind: &typeImportKind}, // type-only import
			},
			"src/utils/orphan.ts": {}, // Only imported via type-only import
		}

		validEntryPoints := []string{"src/index.ts"}
		graphExclude := []string{}
		ignoreTypeImports := true

		orphanFiles := FindOrphanFiles(minimalTreeWithTypeImports, validEntryPoints, graphExclude, ignoreTypeImports, testCwd)

		// orphan.ts should be considered orphan since type-only imports are ignored
		expected := []string{"src/utils/orphan.ts"}
		if !slices.Equal(orphanFiles, expected) {
			t.Errorf("Expected orphan.ts to be orphan when type imports are ignored, got %v", orphanFiles)
		}
	})
}

// Helper function to create string pointers for tests
func stringPtr(s string) *string {
	return &s
}
