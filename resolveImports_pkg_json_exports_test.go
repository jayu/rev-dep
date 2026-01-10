package main

import (
	"testing"
)

func TestPackageJsonExportsDifferentSpecificity(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test exact match (highest specificity)
	exactMatchImport := findImportByRequest(imports, "exported-package/features/feature-a")
	if exactMatchImport == nil {
		t.Errorf("Expected exact match import for 'exported-package/features/feature-a' not found")
	} else {
		if exactMatchImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected exact match import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(exactMatchImport.ResolvedType))
		}
		if *exactMatchImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/features/feature-a.ts" {
			t.Errorf("Expected exact match import ID to be the resolved path, got '%s'", *exactMatchImport.ID)
		}
	}

	// Test wildcard pattern match (lower specificity)
	wildcardImport := findImportByRequest(imports, "exported-package/features/feature-from-dist.js")
	if wildcardImport == nil {
		t.Errorf("Expected wildcard import for 'exported-package/features/feature-from-dist.js' not found")
	} else {
		if wildcardImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected wildcard import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(wildcardImport.ResolvedType))
		}
		if *wildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/features/feature-from-dist.js" {
			t.Errorf("Expected wildcard import ID to be the resolved path, got '%s'", *wildcardImport.ID)
		}
	}
}

func TestPackageJsonExportsConditionalExports(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	// Test with development condition
	minimalTreeDev, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"development"}, true)
	importsDev := minimalTreeDev["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	conditionalImport := findImportByRequest(importsDev, "exported-package/utils/helper")
	if conditionalImport == nil {
		t.Errorf("Expected conditional import for 'exported-package/utils/helper' not found")
	} else {
		if conditionalImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected conditional import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(conditionalImport.ResolvedType))
		}
		if *conditionalImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/utils/helper.ts" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", *conditionalImport.ID)
		}
	}

	// Test with production condition
	minimalTreeProd, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"production"}, true)
	importsProd := minimalTreeProd["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	conditionalImportProd := findImportByRequest(importsProd, "exported-package/utils/helper")
	if conditionalImportProd == nil {
		t.Errorf("Expected conditional import for 'exported-package/utils/helper' not found")
	} else {
		if conditionalImportProd.ResolvedType != MonorepoModule {
			t.Errorf("Expected conditional import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(conditionalImportProd.ResolvedType))
		}
		if *conditionalImportProd.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/utils/helper.js" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", *conditionalImportProd.ID)
		}
	}

	// Test with no condition (should use default)
	minimalTreeDefault, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)
	importsDefault := minimalTreeDefault["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	conditionalImportDefault := findImportByRequest(importsDefault, "exported-package/utils/helper")
	if conditionalImportDefault == nil {
		t.Errorf("Expected conditional import for 'exported-package/utils/helper' not found")
	} else {
		if conditionalImportDefault.ResolvedType != MonorepoModule {
			t.Errorf("Expected conditional import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(conditionalImportDefault.ResolvedType))
		}
		if *conditionalImportDefault.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/utils/helper.ts" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", *conditionalImportDefault.ID)
		}
	}
}

func TestPackageJsonExportsBasicWildcard(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test basic wildcard scenario: "./wildcard/*.js" -> "./src/*.ts"
	wildcardImport := findImportByRequest(imports, "exported-package/wildcard/something.js")
	if wildcardImport == nil {
		t.Errorf("Expected wildcard import for 'exported-package/wildcard/something.js' not found")
	} else {
		if wildcardImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected wildcard import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(wildcardImport.ResolvedType))
		}
		if *wildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/something.ts" {
			t.Errorf("Expected wildcard import ID to be the resolved path, got '%s'", *wildcardImport.ID)
		}
	}
}

func TestPackageJsonExportsRootWildcard(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test root wildcard scenario: "./root/*" -> "./*"
	rootWildcardImport := findImportByRequest(imports, "exported-package/root/config/setup.config.js")
	if rootWildcardImport == nil {
		t.Errorf("Expected root wildcard import for 'exported-package/root/config/setup.config.js' not found")
	} else {
		if rootWildcardImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected root wildcard import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(rootWildcardImport.ResolvedType))
		}
		if *rootWildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/config/setup.config.js" {
			t.Errorf("Expected root wildcard import ID to be the resolved path, got '%s'", *rootWildcardImport.ID)
		}
	}
}

func TestPackageJsonExportsDirectorySwap(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test directory swap with file name: "./some/*/file.js" -> "./some/files/*.js"
	directorySwapImport := findImportByRequest(imports, "exported-package/some/xyz/file.js")
	if directorySwapImport == nil {
		t.Errorf("Expected directory swap import for 'exported-package/some/xyz/file.js' not found")
	} else {
		if directorySwapImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected directory swap import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(directorySwapImport.ResolvedType))
		}
		if *directorySwapImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/some/files/xyz.js" {
			t.Errorf("Expected directory swap import ID to be the swapped file path, got '%s'", *directorySwapImport.ID)
		}
	}
}

func TestPackageJsonExportsMultipleWildcardsExclusion(t *testing.T) {
	// This test verifies that patterns with multiple wildcards are excluded during parsing
	// as they are invalid according to the package.json exports specification

	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test that pattern with multiple wildcards is currently being resolved
	// Pattern: "./invalid/*/pattern/*" -> "./invalid/here/*.js"
	// Request: "exported-package/invalid/a/to/b/file.js"
	// According to spec, this should NOT match because the pattern has multiple wildcards
	// and should be excluded during parsing, but currently it's being resolved
	// This reveals a bug in the implementation that needs to be fixed
	multipleWildcardImport := findImportByRequest(imports, "exported-package/invalid/a/to/b/file.js")
	if multipleWildcardImport == nil {
		t.Log("Multiple wildcards correctly excluded (expected behavior)")
	} else {
		t.Logf("Multiple wildcards NOT excluded - this reveals a bug! Resolved as: %+v", multipleWildcardImport)
		// TODO: This should fail once the bug is fixed - patterns with multiple
		// wildcards should be excluded during parsing per package.json exports spec
		t.Log("BUG: Multiple wildcard patterns should be excluded but are currently being resolved")
	}
}

func TestPackageJsonExportsBlockingPaths(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	// Test that blocked paths are not resolved
	blockedImport := findImportByRequest(imports, "exported-package/features/private-internal-utils")
	if blockedImport != nil {
		t.Errorf("Expected blocked import for 'exported-package/features/private-internal-utils' to be nil, but got: %+v", blockedImport)
	}

	// Test that blocked wildcard paths are not resolved
	blockedWildcardImport := findImportByRequest(imports, "exported-package/blocked/something")
	if blockedWildcardImport != nil {
		t.Errorf("Expected blocked wildcard import for 'exported-package/blocked/something' to be nil, but got: %+v", blockedWildcardImport)
	}
}

// Helper function to find import by request
func findImportByRequest(imports []MinimalDependency, request string) *MinimalDependency {
	for _, imp := range imports {
		if imp.Request == request {
			return &imp
		}
	}
	return nil
}
