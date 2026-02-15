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
		if exactMatchImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/features/feature-a.ts" {
			t.Errorf("Expected exact match import ID to be the resolved path, got '%s'", exactMatchImport.ID)
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
		if wildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/features/feature-from-dist.js" {
			t.Errorf("Expected wildcard import ID to be the resolved path, got '%s'", wildcardImport.ID)
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
		if conditionalImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/utils/helper.ts" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", conditionalImport.ID)
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
		if conditionalImportProd.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/utils/helper.js" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", conditionalImportProd.ID)
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
		if conditionalImportDefault.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/utils/helper.ts" {
			t.Errorf("Expected conditional import ID to be the resolved path, got '%s'", conditionalImportDefault.ID)
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
		if wildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/something.ts" {
			t.Errorf("Expected wildcard import ID to be the resolved path, got '%s'", wildcardImport.ID)
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
		if rootWildcardImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/config/setup.config.js" {
			t.Errorf("Expected root wildcard import ID to be the resolved path, got '%s'", rootWildcardImport.ID)
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
		if directorySwapImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/some/files/xyz.js" {
			t.Errorf("Expected directory swap import ID to be the swapped file path, got '%s'", directorySwapImport.ID)
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
		t.Error("Import request should exist but be marked as not resolved")
	} else if multipleWildcardImport.ResolvedType == NotResolvedModule {
		t.Log("Multiple wildcards correctly excluded (expected behavior)")
	} else {
		t.Errorf("Multiple wildcards NOT excluded - this reveals a bug! Resolved as: %+v", multipleWildcardImport)
		// Patterns with multiple wildcards should be excluded during parsing per package.json exports spec
	}
}

func TestPackageJsonExportsDeepNestedConditionalExports(t *testing.T) {
	// Test deeply nested exports map structure with different condition sets
	// This verifies complex conditional export resolution with nested conditions

	cwd := "__fixtures__/mockMonorepo/packages/consumer-package/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	// Test 1: development + node condition
	minimalTreeDevNode, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"development", "node"}, true)
	importsDevNode := minimalTreeDevNode["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepDevNodeImport := findImportByRequest(importsDevNode, "exported-package/deep")
	if deepDevNodeImport == nil {
		t.Errorf("Expected deep dev+node import for 'exported-package/deep' not found")
	} else {
		if deepDevNodeImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected deep dev+node import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(deepDevNodeImport.ResolvedType))
		}
		if deepDevNodeImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/deep/node.ts" {
			t.Errorf("Expected deep dev+node import ID to be node.ts path, got '%s'", deepDevNodeImport.ID)
		}
	}

	// Test 2: development + default condition (nested)
	minimalTreeDevDefault, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"development"}, true)
	importsDevDefault := minimalTreeDevDefault["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepDevDefaultImport := findImportByRequest(importsDevDefault, "exported-package/deep")
	if deepDevDefaultImport == nil {
		t.Errorf("Expected deep dev+default import for 'exported-package/deep' not found")
	} else {
		if deepDevDefaultImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected deep dev+default import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(deepDevDefaultImport.ResolvedType))
		}
		if deepDevDefaultImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/deep/dev-default.ts" {
			t.Errorf("Expected deep dev+default import ID to be dev-default.ts path, got '%s'", deepDevDefaultImport.ID)
		}
	}

	// Test 3: production + browser condition
	minimalTreeProdBrowser, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"production", "browser"}, true)
	importsProdBrowser := minimalTreeProdBrowser["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepProdBrowserImport := findImportByRequest(importsProdBrowser, "exported-package/deep")
	if deepProdBrowserImport == nil {
		t.Errorf("Expected deep prod+browser import for 'exported-package/deep' not found")
	} else {
		if deepProdBrowserImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected deep prod+browser import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(deepProdBrowserImport.ResolvedType))
		}
		if deepProdBrowserImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/deep/browser.js" {
			t.Errorf("Expected deep prod+browser import ID to be browser.js path, got '%s'", deepProdBrowserImport.ID)
		}
	}

	// Test 4: production + default condition (nested)
	minimalTreeProdDefault, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{"production"}, true)
	importsProdDefault := minimalTreeProdDefault["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepProdDefaultImport := findImportByRequest(importsProdDefault, "exported-package/deep")
	if deepProdDefaultImport == nil {
		t.Errorf("Expected deep prod+default import for 'exported-package/deep' not found")
	} else {
		if deepProdDefaultImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected deep prod+default import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(deepProdDefaultImport.ResolvedType))
		}
		if deepProdDefaultImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/dist/deep/prod-default.js" {
			t.Errorf("Expected deep prod+default import ID to be prod-default.js path, got '%s'", deepProdDefaultImport.ID)
		}
	}

	// Test 5: default condition (no environment specified)
	minimalTreeDefault, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)
	importsDefault := minimalTreeDefault["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepDefaultImport := findImportByRequest(importsDefault, "exported-package/deep")
	if deepDefaultImport == nil {
		t.Errorf("Expected deep default import for 'exported-package/deep' not found")
	} else {
		if deepDefaultImport.ResolvedType != MonorepoModule {
			t.Errorf("Expected deep default import type to be MonorepoModule, got '%s'", ResolvedImportTypeToString(deepDefaultImport.ResolvedType))
		}
		if deepDefaultImport.ID != "__fixtures__/mockMonorepo/packages/exported-package/src/deep/fallback.ts" {
			t.Errorf("Expected deep default import ID to be fallback.ts path, got '%s'", deepDefaultImport.ID)
		}
	}

	// Test 6: blocked path in deep nested structure
	minimalTreeBlocked, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)
	importsBlocked := minimalTreeBlocked["__fixtures__/mockMonorepo/packages/consumer-package/index.ts"]

	deepBlockedImport := findImportByRequest(importsBlocked, "exported-package/deep/blocked")
	if deepBlockedImport == nil {
		t.Error("Import request should exist but be marked as not resolved")
	} else if deepBlockedImport.ResolvedType == NotResolvedModule {
		t.Log("Deep blocked path correctly excluded (expected behavior)")
	} else {
		t.Errorf("Deep blocked path NOT excluded - this reveals a bug! Resolved as: %+v", deepBlockedImport)
		// Blocked paths should be excluded during parsing per package.json exports spec
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
	if blockedImport == nil {
		t.Error("Import request should exist but be marked as not resolved")
	} else if blockedImport.ResolvedType == NotResolvedModule {
		t.Log("Blocked path correctly excluded (expected behavior)")
	} else {
		t.Errorf("Blocked path NOT excluded - this reveals a bug! Resolved as: %+v", blockedImport)
		// Blocked paths should be excluded during parsing per package.json exports spec
	}

	// Test that blocked with wildcard has no effect because there is more specific export
	blockedWildcardImport := findImportByRequest(imports, "exported-package/blocked/something")
	if blockedWildcardImport == nil {
		t.Error("Import request should exist but be marked as not resolved")
	} else if blockedWildcardImport.ResolvedType == MonorepoModule {
		t.Log("Blocked wildcard path correctly excluded (expected behavior)")
	} else {
		t.Errorf("Blocked wildcard path NOT excluded - this reveals a bug! Resolved as: %+v", blockedWildcardImport)
		// Blocked wildcard paths should be excluded during parsing per package.json exports spec
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
func TestPackageJsonExportsBaseUrlWildcard(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/packages/baseurl-consumer/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	// Test: Resolve monorepo package when baseUrl wildcard is configured
	imports := minimalTree["__fixtures__/mockMonorepo/packages/baseurl-consumer/import-baseurl.ts"]

	if len(imports) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to the monorepo package index file
	expectedPath := "__fixtures__/mockMonorepo/packages/baseurl-package/index.ts"
	if imports[0].ID != expectedPath || imports[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedPath, imports[0].ID, ResolvedImportTypeToString(imports[0].ResolvedType))
	}
}

func TestPackageJsonExportsNoMain(t *testing.T) {
	cwd := "__fixtures__/mockMonorepo/" // Run from monorepo root
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, true)

	// Test 0: Simple import test
	imports0 := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/import-no-exports-simple.ts"]

	if len(imports0) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to package root index file
	expectedIndex := "__fixtures__/mockMonorepo/packages/no-exports-package/index.ts"
	if imports0[0].ID != expectedIndex || imports0[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedIndex, imports0[0].ID, ResolvedImportTypeToString(imports0[0].ResolvedType))
	}

	// Test 1: Resolve index file from package without exports/main
	imports := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/import-no-exports-index.ts"]

	if len(imports) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to package root index file
	expectedIndex = "__fixtures__/mockMonorepo/packages/no-exports-package/index.ts"
	if imports[0].ID != expectedIndex || imports[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedIndex, imports[0].ID, ResolvedImportTypeToString(imports[0].ResolvedType))
	}

	// Test 2: Resolve non-index file from package without exports/main
	imports2 := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/import-no-exports-utils.ts"]

	if len(imports2) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to utils.ts file (file takes precedence over directory)
	expectedUtils := "__fixtures__/mockMonorepo/packages/no-exports-package/utils.ts"
	if imports2[0].ID != expectedUtils || imports2[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedUtils, imports2[0].ID, ResolvedImportTypeToString(imports2[0].ResolvedType))
	}

	// Test 3: Resolve utils/index.ts from package without exports/main
	imports3 := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/import-no-exports-utils-index.ts"]

	if len(imports3) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to utils.ts (file takes precedence over directory)
	expectedUtilsIndex := "__fixtures__/mockMonorepo/packages/no-exports-package/utils.ts"
	if imports3[0].ID != expectedUtilsIndex || imports3[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedUtilsIndex, imports3[0].ID, ResolvedImportTypeToString(imports3[0].ResolvedType))
	}

	// Test 4: Resolve lib/index.ts from package without exports/main (different directory)
	imports4 := minimalTree["__fixtures__/mockMonorepo/packages/consumer-package/import-no-exports-lib-index.ts"]

	if len(imports4) == 0 {
		t.Errorf("Expected at least one import, got none")
		return
	}

	// Expected: Should resolve to lib/index.ts
	expectedLibIndex := "__fixtures__/mockMonorepo/packages/no-exports-package/lib/index.ts"
	if imports4[0].ID != expectedLibIndex || imports4[0].ResolvedType != MonorepoModule {
		t.Errorf("Expected %s with MonorepoModule type, got '%s' with type %s", expectedLibIndex, imports4[0].ID, ResolvedImportTypeToString(imports4[0].ResolvedType))
	}
}
