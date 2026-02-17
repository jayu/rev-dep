package main

import (
	"testing"
)

func TestResolvePackageJsonImports(t *testing.T) {
	cwd := "/root/"
	filePaths := []string{
		cwd + "src/index.ts",
		cwd + "src/utils.ts",
		cwd + "src/file.ts",
		cwd + "dist/index.js",
		cwd + "dist/utils.js",
		cwd + "src/specific/file.ts",
		cwd + "#wildcard/file.js",
		cwd + "src/dirs/someDir.ts",
		cwd + "runtime/helpers/classApplyDescriptorSet.js",
	}

	pkgJson := `{
	"dependencies": {
		"some-dep": "^1.0.0"
	},
	// comments should be supported
	"imports": {
		  "#root/*": "./*",
			"#simple": "./src/index.ts",
			"#wildcard/*.js": "./src/*.ts",
			"#wildcard/specific.js": "./src/specific/file.ts",
			"#deep/wildcard/*.js": "./src/*.ts",
			"#directory/*/index.js": "./src/dirs/*.ts",
			// comments should be supported
			"#multiple/*/wildcards/*.js": "./src/*/*.ts",
			"#some-dep-alias": "some-dep",
			"#conditional": {
				"node": "./dist/index.js",
				"default": "./src/index.ts"
			},
			"#nested": {
				"node": {
					"import": "./dist/index.js",
					"require": "./dist/utils.js"
				},
				"default": "./src/index.ts"
			}
		}
	}`

	t.Run("Should resolve simple import", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#simple", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/index.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/index.ts", path)
		}
	})

	t.Run("Should resolve wildcard import", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#wildcard/utils.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/utils.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/utils.ts", path)
		}
	})

	t.Run("Should resolve deep wildcard import", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#deep/wildcard/utils.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/utils.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/utils.ts", path)
		}
	})

	t.Run("Should resolve wildcard import with explicit extension without duplicating it", func(t *testing.T) {
		localPkgJson := `{
			"imports": {
				"#runtime/helpers/*": "./runtime/helpers/*.js"
			}
		}`
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(localPkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#runtime/helpers/classApplyDescriptorSet.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"runtime/helpers/classApplyDescriptorSet.js" {
			t.Errorf("Expected %s, got %s", cwd+"runtime/helpers/classApplyDescriptorSet.js", path)
		}
	})

	t.Run("Should resolve conditional import (default)", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#conditional", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/index.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/index.ts", path)
		}
	})

	t.Run("Should resolve conditional import (node)", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{"node"}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#conditional", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"dist/index.js" {
			t.Errorf("Expected %s, got %s", cwd+"dist/index.js", path)
		}
	})

	t.Run("Should resolve nested conditional import (node -> require)", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{"node", "require"}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#nested", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"dist/utils.js" {
			t.Errorf("Expected %s, got %s", cwd+"dist/utils.js", path)
		}
	})

	t.Run("Should prioritize conditions based on order", func(t *testing.T) {
		localPkgJson := `{
			"imports": {
				"#foo": {
					"import": "./src/index.ts",
					"node": "./dist/index.js"
				}
			}
		}`

		// Case 1: node first
		rmNode := NewResolverManager(FollowMonorepoPackagesValue{}, []string{"node", "import"}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(localPkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolverNode := rmNode.GetResolverForFile(cwd + "main.ts")
		pathNode, _, _ := resolverNode.ResolveModule("#foo", cwd+"main.ts")
		if pathNode != cwd+"dist/index.js" {
			t.Errorf("Expected dist/index.js when node is prioritized, got %s", pathNode)
		}

		// Case 2: import first
		rmImport := NewResolverManager(FollowMonorepoPackagesValue{}, []string{"import", "node"}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(localPkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolverImport := rmImport.GetResolverForFile(cwd + "main.ts")
		pathImport, _, _ := resolverImport.ResolveModule("#foo", cwd+"main.ts")
		if pathImport != cwd+"src/index.ts" {
			t.Errorf("Expected src/index.ts when import is prioritized, got %s", pathImport)
		}
	})

	t.Run("Should resolve imports according to specifity", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#wildcard/specific.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/specific/file.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/specific/file.ts", path)
		}
	})

	t.Run("Should resolve root wildcard import", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#root/src/specific/file.ts", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"src/specific/file.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/specific/file.ts", path)
		}

		path, _, err = resolver.ResolveModule("#root/dist/utils.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		if path != cwd+"dist/utils.js" {
			t.Errorf("Expected %s, got %s", cwd+"dist/utils.js", path)
		}
	})

	t.Run("Package.json imports should take precedence over tsconfig paths", func(t *testing.T) {
		tsConfig := `{
			"compilerOptions": {
				"paths": {
					"#simple": ["./src/utils.ts"]
				}
			}
		}`
		// pkgJson maps #simple to index.ts
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#simple", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}
		// Should match package.json (index.ts) not tsconfig (utils.ts)
		if path != cwd+"src/index.ts" {
			t.Errorf("Expected package.json priority (src/index.ts), got %s", path)
		}
	})

	t.Run("Should resolve package json imports before ts aliases even for ts global wildcard match", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(`{
				"compilerOptions": {
					"paths": {
						"*": ["./*"]
					}
				}
			}`),
			PkgJsonContent: []byte(pkgJson),
			SortedFiles:    filePaths,
			Cwd:            cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, rtype, err := resolver.ResolveModule("#wildcard/file.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}

		if rtype != UserModule {
			t.Errorf("Expected UserModule, got %s", ResolvedImportTypeToString(rtype))
		}

		if path != cwd+"src/file.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/file.ts", path)
		}

	})

	t.Run("Should resolve pjson import with directory and file swap", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})

		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		path, _, err := resolver.ResolveModule("#directory/someDir/index.js", cwd+"src/main.ts")

		if err != nil {
			t.Fatalf("Expected no error, got %v", *err)
		}

		if path != cwd+"src/dirs/someDir.ts" {
			t.Errorf("Expected %s, got %s", cwd+"src/dirs/someDir.ts", path)
		}
	})

	// TODO examine what happens when this test is run, we don't have any explicit code preventing multiple wildcards
	t.Run("Should not process import with multiple wildcards", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")
		_, _, err := resolver.ResolveModule("#multiple/sth/wildcards/sth.js", cwd+"src/main.ts")

		if err == nil || *err != AliasNotResolved {
			t.Fatalf("Expected error, got %v", err)
		}
	})

	t.Run("Should parse import targets into tree structure", func(t *testing.T) {
		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{"node", "import"}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJson),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")

		// Test simple string target
		simpleTarget := resolver.packageJsonImports.parsedImportTargets["#simple"]
		if simpleTarget == nil || simpleTarget.nodeType != LeafNode || simpleTarget.value != "./src/index.ts" {
			t.Errorf("Expected simple leaf node with value './src/index.ts', got %+v", simpleTarget)
		}

		// Test conditional target
		conditionalTarget := resolver.packageJsonImports.parsedImportTargets["#conditional"]
		if conditionalTarget == nil || conditionalTarget.nodeType != MapNode {
			t.Errorf("Expected conditional map node, got %+v", conditionalTarget)
		}

		// Check node condition
		nodeChild := conditionalTarget.conditionsMap["node"]
		if nodeChild == nil || nodeChild.nodeType != LeafNode || nodeChild.value != "./dist/index.js" {
			t.Errorf("Expected node leaf node with value './dist/index.js', got %+v", nodeChild)
		}

		// Check default condition
		defaultChild := conditionalTarget.conditionsMap["default"]
		if defaultChild == nil || defaultChild.nodeType != LeafNode || defaultChild.value != "./src/index.ts" {
			t.Errorf("Expected default leaf node with value './src/index.ts', got %+v", defaultChild)
		}

		// Test nested conditional target
		nestedTarget := resolver.packageJsonImports.parsedImportTargets["#nested"]
		if nestedTarget == nil || nestedTarget.nodeType != MapNode {
			t.Errorf("Expected nested map node, got %+v", nestedTarget)
		}

		// Check node condition with nested import/require
		nodeNestedChild := nestedTarget.conditionsMap["node"]
		if nodeNestedChild == nil || nodeNestedChild.nodeType != MapNode {
			t.Errorf("Expected node nested map node, got %+v", nodeNestedChild)
		}

		importChild := nodeNestedChild.conditionsMap["import"]
		if importChild == nil || importChild.nodeType != LeafNode || importChild.value != "./dist/index.js" {
			t.Errorf("Expected import leaf node with value './dist/index.js', got %+v", importChild)
		}
	})

	t.Run("Should reject targets with multiple wildcards", func(t *testing.T) {
		pkgJsonWithInvalidTargets := `{
			"imports": {
				"#valid": "./src/*.ts",
				"#invalid-target": "./src/*/*.js",
				"#invalid/*/key/*": "file.js",
				"#valid-wildcard": "./*.ts"
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte("{}"),
			PkgJsonContent:  []byte(pkgJsonWithInvalidTargets),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "src/main.ts")

		// Valid targets should be parsed
		validTarget := resolver.packageJsonImports.parsedImportTargets["#valid"]
		if validTarget == nil || validTarget.nodeType != LeafNode || validTarget.value != "./src/*.ts" {
			t.Errorf("Expected valid leaf node with value './src/*.ts', got %+v", validTarget)
		}

		validWildcardTarget := resolver.packageJsonImports.parsedImportTargets["#valid-wildcard"]
		if validWildcardTarget == nil || validWildcardTarget.nodeType != LeafNode || validWildcardTarget.value != "./*.ts" {
			t.Errorf("Expected valid wildcard leaf node with value './*.ts', got %+v", validWildcardTarget)
		}

		// Invalid targets should be rejected (not present in parsed targets)
		if _, exists := resolver.packageJsonImports.parsedImportTargets["#invalid-target"]; exists {
			t.Errorf("Expected #invalid-target to be rejected due to multiple wildcards in target")
		}

		// Keys with multiple wildcards should also be rejected
		if _, exists := resolver.packageJsonImports.parsedImportTargets["#invalid-key"]; exists {
			t.Errorf("Expected #invalid-key to be rejected due to multiple wildcards in key")
		}

		// Verify that regex patterns are only created for valid entries
		regexCount := 0
		for _, regexItem := range resolver.packageJsonImports.importsRegexps {
			if regexItem.aliasKey == "#valid" || regexItem.aliasKey == "#valid-wildcard" {
				regexCount++
			}
		}
		if regexCount != 2 {
			t.Errorf("Expected 2 regex patterns for valid entries, got %d", regexCount)
		}
	})
}

func TestShouldResolvePJsonAliasToExternalModule(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/mockProject/index.ts"]
	aliasedImport := imports[len(imports)-2]

	if aliasedImport.Request != "#utils-lib" {
		t.Errorf("Expected aliased import request to be '#utils-lib', got '%s'", aliasedImport.Request)
	}

	if aliasedImport.ID != "lodash" {
		t.Errorf("Expected aliased import ID to be 'lodash', got '%s'", aliasedImport.ID)
	}
}

func TestShouldResolvePJsonAliasToNodeModuleWithSubpath(t *testing.T) {
	cwd := "__fixtures__/mockProjectSubpath/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/mockProjectSubpath/index.ts"]

	// Test basic subpath alias: "#my-util-submodule" -> "lodash/files/utils"
	basicSubpathImport := imports[0]
	if basicSubpathImport.Request != "#my-util-submodule" {
		t.Errorf("Expected basic import request to be '#my-util-submodule', got '%s'", basicSubpathImport.Request)
	}
	if basicSubpathImport.ID != "lodash" {
		t.Errorf("Expected basic import ID to be 'lodash', got '%s'", basicSubpathImport.ID)
	}
	if basicSubpathImport.ResolvedType != NodeModule {
		t.Errorf("Expected basic import type to be NodeModule, got '%s'", ResolvedImportTypeToString(basicSubpathImport.ResolvedType))
	}

	// Test wildcard subpath alias: "#my-util-submodule/array.js" -> "lodash/files/utils/array.js"
	wildcardSubpathImport := imports[1]
	if wildcardSubpathImport.Request != "#my-util-submodule/array.js" {
		t.Errorf("Expected wildcard import request to be '#my-util-submodule/array.js', got '%s'", wildcardSubpathImport.Request)
	}
	if wildcardSubpathImport.ID != "lodash" {
		t.Errorf("Expected wildcard import ID to be 'lodash', got '%s'", wildcardSubpathImport.ID)
	}
	if wildcardSubpathImport.ResolvedType != NodeModule {
		t.Errorf("Expected wildcard import type to be NodeModule, got '%s'", ResolvedImportTypeToString(wildcardSubpathImport.ResolvedType))
	}

	// Test deep nested subpath alias: "#my-util-submodule/deep/nested/path.js" -> "lodash/files/utils/deep/nested/path.js"
	deepSubpathImport := imports[2]
	if deepSubpathImport.Request != "#my-util-submodule/deep/nested/path.js" {
		t.Errorf("Expected deep import request to be '#my-util-submodule/deep/nested/path.js', got '%s'", deepSubpathImport.Request)
	}
	if deepSubpathImport.ID != "lodash" {
		t.Errorf("Expected deep import ID to be 'lodash', got '%s'", deepSubpathImport.ID)
	}
	if deepSubpathImport.ResolvedType != NodeModule {
		t.Errorf("Expected deep import type to be NodeModule, got '%s'", ResolvedImportTypeToString(deepSubpathImport.ResolvedType))
	}
}
