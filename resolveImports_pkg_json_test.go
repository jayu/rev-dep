package main

import (
	"testing"
)

/*
Need more test cases here
- test against pattern with multiple wildcards - they are invalid
- test for pjson import alias for 3rd party modules - this test to make sense should use ResolveImports fn instead of just resolver. Becasue actual node_modules resolution happens in ResolveImports fn
- test for TSAlias for 3rd party modules
*/

// Test for exports blocking paths
// {
//   "exports": {
//     "./features/*": "./dist/features/*.js",
//     "./features/private-internal-utils": null,
//     "./features/*.config.js": null
//   }
// }

/*
 Make sure there are exports tests for the
 - different specifity
 - conditional exports respecing conditions param
 - directory swap with file name
 - multiple wildcards
 - basic wildcard scenario
 - root wildcard scenario
*/

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
	}

	pkgJson := `{
	"dependencies": {
		"some-dep": "^1.0.0"
	},
	"imports": {
		  "#root/*": "./*",
			"#simple": "./src/index.ts",
			"#wildcard/*.js": "./src/*.ts",
			"#wildcard/specific.js": "./src/specific/file.ts",
			"#deep/wildcard/*.js": "./src/*.ts",
			"#directory/*/index.js": "./src/dirs/*.ts",
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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

	t.Run("Should resolve conditional import (default)", func(t *testing.T) {
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{"node"}, RootParams{
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
		rm := NewResolverManager(false, []string{"node", "require"}, RootParams{
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
		rmNode := NewResolverManager(false, []string{"node", "import"}, RootParams{
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
		rmImport := NewResolverManager(false, []string{"import", "node"}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
		rm := NewResolverManager(false, []string{}, RootParams{
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
}
