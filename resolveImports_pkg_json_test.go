package main

import (
	"testing"
)

func TestResolvePackageJsonImports(t *testing.T) {
	cwd := "/root/"
	filePaths := []string{
		cwd + "src/index.ts",
		cwd + "src/utils.ts",
		cwd + "dist/index.js",
		cwd + "dist/utils.js",
	}

	pkgJson := `{
		"imports": {
			"#simple": "./src/index.ts",
			"#wildcard/*.js": "./src/*.ts",
			"#deep/wildcard/*.js": "./src/*.ts",
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
			t.Fatalf("Expected no error, got %v", err)
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
			t.Fatalf("Expected no error, got %v", err)
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
			t.Fatalf("Expected no error, got %v", err)
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
			t.Fatalf("Expected no error, got %v", err)
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
			t.Fatalf("Expected no error, got %v", err)
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
			t.Fatalf("Expected no error, got %v", err)
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

	t.Run("Package.json imports should not take precedence over tsconfig paths", func(t *testing.T) {
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
			t.Fatalf("Expected no error, got %v", err)
		}
		// Should match package.json (index.ts) not tsconfig (utils.ts)
		if path != cwd+"src/utils.ts" {
			t.Errorf("Expected package.json priority (src/utils.ts), got %s", path)
		}
	})
}
