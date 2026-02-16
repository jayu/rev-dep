package main

import (
	"testing"
)

func TestShouldResolveFileIfDirWithTheSameNameExists(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithTheSameNameAsDir.ts"]
	_, fileWithIndexExists := minimalTree["__fixtures__/mockProject/src/fileDirTheSameName/index.ts"]

	if !fileWithIndexExists {
		t.Errorf("Contrary file for this test does not exists")
	}

	if imports[0].ID != "__fixtures__/mockProject/src/fileDirTheSameName.ts" {
		t.Errorf("Should resolve file instead of directory with index file")
	}
}

func TestShouldResolveFileIfDirWithTheSameNameExistsOutOfCwd(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithTheSameNameAsDirOutsideCwd.ts"]

	// For this test file outside of CWD won't be in minimal tree, as it was not resolved and not looked up
	if imports[0].ID != "__fixtures__/fileDirTheSameName.ts" {
		t.Errorf("Should resolve file instead of directory with index file")
	}
}

func TestShouldResolveImportToFileWhenNodeModuleWithTheSamePrefixExists(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithSamePathAsNodeModule.ts"]

	// For this test file outside of CWD won't be in minimal tree, as it was not resolved and not looked up
	if imports[0].ID != "__fixtures__/mockProject/lodash/file.ts" {
		t.Errorf("Should resolve file instead of node module with the same prefix")
	}
}

func TestShouldResolveFilesWithAmbiguousImportsByOrderingExtensions(t *testing.T) {
	cwd := "__fixtures__/ambiguousImports/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	imports := minimalTree["__fixtures__/ambiguousImports/test.ts"]

	if imports[0].ID != "__fixtures__/ambiguousImports/1/file.ts" {
		t.Errorf("Should resolve file ts extension, resolved '%v'", imports[0].ID)
	}
	if imports[1].ID != "__fixtures__/ambiguousImports/2/file.tsx" {
		t.Errorf("Should resolve file tsx extension, resolved '%v'", imports[1].ID)

	}
	if imports[2].ID != "__fixtures__/ambiguousImports/3/file.js" {
		t.Errorf("Should resolve file js extension, resolved '%v'", imports[2].ID)

	}
}

func TestParsingTsConfig(t *testing.T) {
	t.Run("Should not crash on empty config file", func(t *testing.T) {
		tsConfig := `{}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     []string{},
			Cwd:             "/root/",
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile("/root/")

		aliasesCount := len(resolver.tsConfigParsed.aliases)

		if aliasesCount != 0 {
			t.Errorf("Aliases should be empty")
		}
	})

	t.Run("Should parse single alias", func(t *testing.T) {
		tsConfig := `{
			"compilerOptions": {
			  "paths": {
				  "@/dir/*": ["./app/dir/*"]
				}
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     []string{},
			Cwd:             "/root/",
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile("/root/")

		aliasesCount := len(resolver.tsConfigParsed.aliases)
		_, hasAlias := resolver.tsConfigParsed.aliases["@/dir/*"]

		if aliasesCount != 1 {
			t.Errorf("Aliases count not match")
		}

		if !hasAlias {
			t.Errorf("Aliases not found")
		}
	})

	t.Run("Should not crash on config file with single line comments", func(t *testing.T) {
		tsConfig := `{
		  // comment
			"compilerOptions" : {
			  "paths": {
				  "@/dir/*": ["./app/dir/*"]
				}
		  // comment
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     []string{},
			Cwd:             "/root/",
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile("/root/")

		aliasesCount := len(resolver.tsConfigParsed.aliases)
		_, hasAlias := resolver.tsConfigParsed.aliases["@/dir/*"]

		if aliasesCount != 1 {
			t.Errorf("Aliases count not match")
		}

		if !hasAlias {
			t.Errorf("Aliases not found")
		}
	})

	t.Run("Should not crash on config file with multi line comments", func(t *testing.T) {
		tsConfig := `{
		  /*  comment
        comment 
			*/
			"compilerOptions" : {
			"paths": {
				  "@/dir/*": ["./app/dir/*"]
				}
		  /*  comment
        comment 
			*/
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     []string{},
			Cwd:             "/root/",
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile("/root/")

		aliasesCount := len(resolver.tsConfigParsed.aliases)
		_, hasAlias := resolver.tsConfigParsed.aliases["@/dir/*"]

		if aliasesCount != 1 {
			t.Errorf("Aliases count not match")
		}

		if !hasAlias {
			t.Errorf("Aliases not found")
		}
	})

	t.Run("Should filter out non-relative aliases", func(t *testing.T) {
		tsConfig := `{
			"compilerOptions": {
			  "paths": {
				  "@/valid/*": ["./src/*"],
				  "@/another-valid": ["../lib/*"],
				  "@/invalid": ["node_modules/package"],
				  "@/also-invalid": ["/absolute/path"]
				}
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     []string{},
			Cwd:             "/root/",
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile("/root/")

		aliasesCount := len(resolver.tsConfigParsed.aliases)

		// Should only have 2 aliases (the ones with relative paths)
		if aliasesCount != 2 {
			t.Errorf("Expected 2 aliases, got %d", aliasesCount)
		}

		// Check that valid aliases are present
		if _, hasValid := resolver.tsConfigParsed.aliases["@/valid/*"]; !hasValid {
			t.Errorf("@/valid/* alias should be present")
		}

		if _, hasAnotherValid := resolver.tsConfigParsed.aliases["@/another-valid"]; !hasAnotherValid {
			t.Errorf("@/another-valid alias should be present")
		}

		// Check that invalid aliases are filtered out
		if _, hasInvalid := resolver.tsConfigParsed.aliases["@/invalid"]; hasInvalid {
			t.Errorf("@/invalid alias should be filtered out")
		}

		if _, hasAlsoInvalid := resolver.tsConfigParsed.aliases["@/also-invalid"]; hasAlsoInvalid {
			t.Errorf("@/also-invalid alias should be filtered out")
		}
	})
}

func TestResolve(t *testing.T) {
	t.Run("Should resolve aliased import", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
				"paths": {
						"@/dir/*": ["./app/dir/*"]
					}
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.js")

		resolvedPath, _, err := resolver.ResolveModule("@/dir/fileA", cwd+"app/index.js")

		if resolvedPath != cwd+"app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/dir/fileA.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})
	t.Run("Should resolve baseUrl import", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.js")

		resolvedPath, _, err := resolver.ResolveModule("app/dir/fileA", cwd+"app/index.js")

		if resolvedPath != cwd+"app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/dir/fileA.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve non-wildcard alias with file extension", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "db/index.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions": {
			  "paths": {
				  "db": ["./db/index.ts"]
				}
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.js")

		resolvedPath, _, err := resolver.ResolveModule("db", cwd+"app/index.js")

		if resolvedPath != cwd+"db/index.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"db/index.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve relative import", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.js")

		resolvedPath, _, err := resolver.ResolveModule("./dir/fileA", cwd+"app/index.js")

		if resolvedPath != cwd+"app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/dir/fileA.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve relative import to parent dir", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/dir/fileA.ts")

		resolvedPath, _, err := resolver.ResolveModule("../index", cwd+"app/dir/fileA.ts")

		if resolvedPath != cwd+"app/index.js" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/index.js", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve import to file using other ts-supported extension", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("./dir/fileA.jsx", cwd+"app/index.ts")

		if resolvedPath != cwd+"app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/dir/fileA.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})
}

func TestRelativeImports(t *testing.T) {

	t.Run("Should resolve relative import to parent dir", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/fileA.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/dir/fileA.ts")

		resolvedPath, _, err := resolver.ResolveModule("../index", cwd+"app/dir/fileA.ts")

		if resolvedPath != cwd+"app/index.js" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/index.js", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve directory import to index file", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/index.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.js")

		resolvedPath, _, err := resolver.ResolveModule("./dir", cwd+"app/index.js")

		if resolvedPath != cwd+"app/dir/index.ts" {
			t.Errorf("Path not resolved correctly, expected %s, got %s", cwd+"app/dir/index.ts", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve import '.' to current dir index", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/dir/index.ts",
			cwd + "app/index.js",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`
		pkgConfig := `{}` // Assuming an empty pkgConfig for this test

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte(pkgConfig),
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/dir/file.ts")

		resolvedPath, _, err := resolver.ResolveModule(".", cwd+"app/dir/file.ts")

		if resolvedPath != cwd+"app/dir/index.ts" {
			t.Errorf("Path not resolved correctly for '.', got %v", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})

	t.Run("Should resolve import '..' to parent dir index", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/index.ts",
			cwd + "app/dir/file.ts",
		}
		tsConfig := `{
			"compilerOptions" : {
			  "baseUrl": "."
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/dir/file.ts")

		resolvedPath, _, err := resolver.ResolveModule("..", cwd+"app/dir/file.ts")

		if resolvedPath != cwd+"app/index.ts" {
			t.Errorf("Path not resolved correctly for '..', got %v", resolvedPath)
		}

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
	})
}

func TestGetNodeModulesFromPkg(t *testing.T) {
	pkgJson := `
	{
	  "dependencies" :{
			"react": "0.0.0",
			"node": "0.0.0"
		},
		"devDependencies" :{
			"@types/react": "0.0.0",
			"@types/node": "0.0.0"
		}
	}
	`

	modules := GetNodeModulesFromPkgJson([]byte(pkgJson))
	expectedModules := []string{"react", "node", "@types/react", "@types/node"}

	for _, module := range expectedModules {
		_, has := modules[module]
		if !has {
			t.Errorf("Module '%s' not found in package.json", module)
		}
	}
}

func TestResolveNodeModules(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := false
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "", []string{}, FollowMonorepoPackagesValue{})

	nodeModulesImports := minimalTree[cwd+"src/nodeModules.ts"]

	module1Ok := nodeModulesImports[0].ID == "@types/node" && nodeModulesImports[0].ResolvedType == NodeModule
	module2Ok := nodeModulesImports[1].ID == "node:fs" && nodeModulesImports[1].ResolvedType == BuiltInModule
	module3Ok := nodeModulesImports[2].ID == "react" && nodeModulesImports[2].ResolvedType == NodeModule
	module4Ok := nodeModulesImports[3].ID == "path" && nodeModulesImports[3].ResolvedType == BuiltInModule
	module5Ok := nodeModulesImports[4].ID == "" && nodeModulesImports[4].ResolvedType == NotResolvedModule

	results := []bool{module1Ok, module2Ok, module3Ok, module4Ok, module5Ok}

	for idx, isOk := range results {
		if !isOk {
			t.Errorf("Module %d not resolved correctly. ID: '%s', Type: %s", idx, nodeModulesImports[idx].ID, ResolvedImportTypeToString(nodeModulesImports[idx].ResolvedType))
		}
	}
}

func TestModuleSuffixes(t *testing.T) {
	t.Run("Should parse moduleSuffixes from tsconfig", func(t *testing.T) {
		tsConfig := `{
			"compilerOptions": {
				"moduleSuffixes": [".ios", ".native", ""]
			}
		}`

		parsed := ParseTsConfigContent([]byte(tsConfig))

		if len(parsed.moduleSuffixes) != 3 {
			t.Fatalf("Expected 3 moduleSuffixes, got %d", len(parsed.moduleSuffixes))
		}
		if parsed.moduleSuffixes[0] != ".ios" {
			t.Errorf("Expected first suffix '.ios', got '%s'", parsed.moduleSuffixes[0])
		}
		if parsed.moduleSuffixes[1] != ".native" {
			t.Errorf("Expected second suffix '.native', got '%s'", parsed.moduleSuffixes[1])
		}
		if parsed.moduleSuffixes[2] != "" {
			t.Errorf("Expected third suffix '', got '%s'", parsed.moduleSuffixes[2])
		}
	})

	t.Run("Should have nil moduleSuffixes when not configured", func(t *testing.T) {
		tsConfig := `{
			"compilerOptions": {
				"paths": {}
			}
		}`

		parsed := ParseTsConfigContent([]byte(tsConfig))

		if parsed.moduleSuffixes != nil {
			t.Errorf("Expected nil moduleSuffixes, got %v", parsed.moduleSuffixes)
		}
	})

	t.Run("Should resolve suffixed file with moduleSuffixes", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/button.ios.tsx",
			cwd + "app/button.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {
				"moduleSuffixes": [".ios", ""]
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("./button", cwd+"app/index.ts")

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
		if resolvedPath != cwd+"app/button.ios.tsx" {
			t.Errorf("Expected %s, got %s", cwd+"app/button.ios.tsx", resolvedPath)
		}
	})

	t.Run("Should fallback to unsuffixed file when suffixed file does not exist", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/button.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {
				"moduleSuffixes": [".ios", ""]
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("./button", cwd+"app/index.ts")

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
		if resolvedPath != cwd+"app/button.tsx" {
			t.Errorf("Expected %s, got %s", cwd+"app/button.tsx", resolvedPath)
		}
	})

	t.Run("Should resolve suffixed index file in directory", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/components/index.ios.tsx",
			cwd + "app/components/index.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {
				"moduleSuffixes": [".ios", ""]
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("./components", cwd+"app/index.ts")

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
		if resolvedPath != cwd+"app/components/index.ios.tsx" {
			t.Errorf("Expected %s, got %s", cwd+"app/components/index.ios.tsx", resolvedPath)
		}
	})

	t.Run("Should not change behavior when no moduleSuffixes configured", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/button.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("./button", cwd+"app/index.ts")

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
		if resolvedPath != cwd+"app/button.tsx" {
			t.Errorf("Expected %s, got %s", cwd+"app/button.tsx", resolvedPath)
		}
	})

	t.Run("Should resolve aliases combined with moduleSuffixes", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "src/button.ios.tsx",
			cwd + "src/button.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {
				"paths": {
					"@/*": ["./src/*"]
				},
				"moduleSuffixes": [".ios", ""]
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		resolvedPath, _, err := resolver.ResolveModule("@/button", cwd+"app/index.ts")

		if err != nil {
			t.Errorf("Error during path resolution: %v", err)
		}
		if resolvedPath != cwd+"src/button.ios.tsx" {
			t.Errorf("Expected %s, got %s", cwd+"src/button.ios.tsx", resolvedPath)
		}
	})

	t.Run("Should not resolve unsuffixed when empty string not in moduleSuffixes", func(t *testing.T) {
		cwd := "/root/"
		filePaths := []string{
			cwd + "app/button.tsx",
			cwd + "app/index.ts",
		}
		tsConfig := `{
			"compilerOptions": {
				"moduleSuffixes": [".ios"]
			}
		}`

		rm := NewResolverManager(FollowMonorepoPackagesValue{}, []string{}, RootParams{
			TsConfigContent: []byte(tsConfig),
			PkgJsonContent:  []byte{},
			SortedFiles:     filePaths,
			Cwd:             cwd,
		}, []GlobMatcher{})
		resolver := rm.GetResolverForFile(cwd + "app/index.ts")

		_, _, err := resolver.ResolveModule("./button", cwd+"app/index.ts")

		if err == nil {
			t.Errorf("Expected FileNotFound error when empty string not in moduleSuffixes")
		}
	})
}

// createTestResolverManager creates a minimal ResolverManager for variant detection tests.
func createTestResolverManager(cwd string, files []string, moduleSuffixes []string) *ResolverManager {
	tsConfig := &TsConfigParsed{
		aliases:          map[string]string{},
		aliasesRegexps:   []RegExpArrItem{},
		wildcardPatterns: []WildcardPattern{},
		moduleSuffixes:   moduleSuffixes,
	}
	filesAndExtensions := &map[string]string{}
	for _, f := range files {
		addFilePathToFilesAndExtensions(f, filesAndExtensions)
	}
	return &ResolverManager{
		rootResolver: &ModuleResolver{
			tsConfigParsed: tsConfig,
			resolverRoot:   cwd,
		},
		filesAndExtensions: filesAndExtensions,
	}
}

func TestDetectModuleSuffixVariants(t *testing.T) {
	t.Run("Should detect platform variants with multiple suffixes", func(t *testing.T) {
		cwd := "/root/app/"
		files := []string{
			cwd + "button.ios.tsx",
			cwd + "button.android.tsx",
			cwd + "button.tsx",
			cwd + "utils.tsx",
		}

		rm := createTestResolverManager(cwd, files, []string{".ios", ".android", ""})
		variants := DetectModuleSuffixVariants(files, rm)

		// button.ios.tsx, button.android.tsx, button.tsx are all variants of each other
		if !variants[cwd+"button.ios.tsx"] {
			t.Error("Expected button.ios.tsx to be a variant")
		}
		if !variants[cwd+"button.android.tsx"] {
			t.Error("Expected button.android.tsx to be a variant")
		}
		if !variants[cwd+"button.tsx"] {
			t.Error("Expected button.tsx to be a variant (empty suffix)")
		}
		// utils.tsx has no other variant, so it should NOT be detected
		if variants[cwd+"utils.tsx"] {
			t.Error("Expected utils.tsx to NOT be a variant")
		}
	})

	t.Run("Should return empty map when no moduleSuffixes configured", func(t *testing.T) {
		files := []string{"/root/app/button.tsx"}
		rm := createTestResolverManager("/root/app/", files, []string{})
		variants := DetectModuleSuffixVariants(files, rm)

		if len(variants) != 0 {
			t.Errorf("Expected 0 variants, got %d", len(variants))
		}
	})

	t.Run("Should not detect variant when suffix is not in config", func(t *testing.T) {
		cwd := "/root/app/"
		files := []string{
			cwd + "button.ios.tsx",
			cwd + "button.web.tsx",
		}

		rm := createTestResolverManager(cwd, files, []string{".ios", ""})
		variants := DetectModuleSuffixVariants(files, rm)

		// button.ios.tsx: suffix .ios, base = button, check button (empty suffix) → not in filesAndExtensions
		// So button.ios.tsx is NOT a variant (no other suffix variant exists)
		if variants[cwd+"button.ios.tsx"] {
			t.Error("Expected button.ios.tsx to NOT be a variant when no other configured suffix file exists")
		}
		// button.web.tsx: no configured suffix matches .web, empty suffix gives base=button.web, check button.web.ios → no
		if variants[cwd+"button.web.tsx"] {
			t.Error("Expected button.web.tsx to NOT be a variant (.web not in config)")
		}
	})

	t.Run("Should detect variant when only two suffixes exist", func(t *testing.T) {
		cwd := "/root/app/"
		files := []string{
			cwd + "button.ios.tsx",
			cwd + "button.tsx",
		}

		rm := createTestResolverManager(cwd, files, []string{".ios", ""})
		variants := DetectModuleSuffixVariants(files, rm)

		if !variants[cwd+"button.ios.tsx"] {
			t.Error("Expected button.ios.tsx to be a variant")
		}
		if !variants[cwd+"button.tsx"] {
			t.Error("Expected button.tsx to be a variant")
		}
	})

	t.Run("Should detect variants with four suffixes including native", func(t *testing.T) {
		cwd := "/root/app/"
		files := []string{
			cwd + "FormContent.ios.tsx",
			cwd + "FormContent.android.tsx",
			cwd + "types.ts",
		}

		rm := createTestResolverManager(cwd, files, []string{".ios", ".android", ".native", ""})
		variants := DetectModuleSuffixVariants(files, rm)

		if !variants[cwd+"FormContent.ios.tsx"] {
			t.Error("Expected FormContent.ios.tsx to be a variant")
		}
		if !variants[cwd+"FormContent.android.tsx"] {
			t.Error("Expected FormContent.android.tsx to be a variant")
		}
		if variants[cwd+"types.ts"] {
			t.Error("Expected types.ts to NOT be a variant")
		}
	})
}

func TestSpecialCharactersInAliases(t *testing.T) {
	// Test that aliases with special regexp characters like $ are properly escaped
	// This test verifies the fix for the issue where $base/* would cause regexp compilation errors

	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	// Create a temporary tsconfig with special characters in alias names
	tempTsConfig := `{
		"compilerOptions": {
			"paths": {
				"$base/*": ["./src/*"],
				"$test/*": ["./test/*"],
				"foo+bar/*": ["./lib/*"]
			}
		}
	}`

	// This should not panic or cause regexp compilation errors
	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, tempTsConfig, "", []string{}, FollowMonorepoPackagesValue{})

	// If we get here without panicking, the test passes
	// The actual resolution might not find files since these are test aliases,
	// but the important thing is that regexp compilation doesn't fail
	if minimalTree == nil {
		t.Errorf("Expected minimalTree to be created, but got nil")
	}
}
