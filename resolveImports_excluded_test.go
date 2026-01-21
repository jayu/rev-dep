package main

import (
	"testing"
)

func TestResolveMarksExcludedFilesAsExcludedByUser(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	// exclude the target file that is imported by importNestedFile.ts
	excludeFiles := []string{"src/nested/deeplynested/file.ts"}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, false)

	importer := "__fixtures__/mockProject/src/importNestedFile.ts"
	excluded := "__fixtures__/mockProject/src/nested/deeplynested/file.ts"

	imports, ok := minimalTree[importer]
	if !ok {
		t.Fatalf("Importer %s not found in minimal tree", importer)
	}

	if len(imports) == 0 {
		t.Fatalf("No imports found for %s", importer)
	}

	dep := imports[0]
	if dep.ResolvedType != ExcludedByUser {
		t.Errorf("Expected dependency to be marked ExcludedByUser, got %v", dep.ResolvedType)
	}

	if dep.ID == nil || *dep.ID != excluded {
		t.Errorf("Expected dependency ID to be %s, got %v", excluded, dep.ID)
	}

	// Ensure that excluded file has a placeholder entry in the minimal tree
	depsForExcluded, exists := minimalTree[excluded]
	if !exists {
		t.Fatalf("Excluded file %s not present as placeholder in minimal tree", excluded)
	}

	if len(depsForExcluded) != 0 {
		t.Errorf("Expected placeholder deps for excluded file to be empty, got %d entries", len(depsForExcluded))
	}
}
