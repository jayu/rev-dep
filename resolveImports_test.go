package main

import (
	"testing"
)

func TestShouldResolveFileIfDirWithTheSameNameExists(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "")

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithTheSameNameAsDir.ts"]
	_, fileWithIndexExists := minimalTree["__fixtures__/mockProject/src/fileDirTheSameName/index.ts"]

	if !fileWithIndexExists {
		t.Errorf("Contrary file for this test does not exists")
	}

	if *imports[0].ID != "__fixtures__/mockProject/src/fileDirTheSameName.ts" {
		t.Errorf("Should resolve file instead of directory with index file")
	}
}

func TestShouldResolveFileIfDirWithTheSameNameExistsOutOfCwd(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "")

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithTheSameNameAsDirOutsideCwd.ts"]

	// For this test file outside of CWD won't be in minimal tree, as it was not resolved and not looked up
	if *imports[0].ID != "__fixtures__/fileDirTheSameName.ts" {
		t.Errorf("Should resolve file instead of directory with index file")
	}
}

func TestShouldResolveImportToFileWhenNodeModuleWithTheSamePrefixExists(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "")

	imports := minimalTree["__fixtures__/mockProject/src/importFileWithSamePathAsNodeModule.ts"]

	// For this test file outside of CWD won't be in minimal tree, as it was not resolved and not looked up
	if *imports[0].ID != "__fixtures__/mockProject/lodash/file.ts" {
		t.Errorf("Should resolve file instead of node module with the same prefix")
	}
}

func TestShouldResolveFilesWithAmbiguousImportsByOrderingExtensions(t *testing.T) {
	cwd := "__fixtures__/ambiguousImports/"
	ignoreTypeImports := true
	excludeFiles := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "")

	imports := minimalTree["__fixtures__/ambiguousImports/test.ts"]

	if *imports[0].ID != "__fixtures__/ambiguousImports/1/file.ts" {
		t.Errorf("Should resolve file ts extension, resolved '%v'", *imports[0].ID)
	}
	if *imports[1].ID != "__fixtures__/ambiguousImports/2/file.tsx" {
		t.Errorf("Should resolve file tsx extension, resolved '%v'", *imports[1].ID)

	}
	if *imports[2].ID != "__fixtures__/ambiguousImports/3/file.js" {
		t.Errorf("Should resolve file js extension, resolved '%v'", *imports[2].ID)

	}
}

func TestParsingTsConfig(t *testing.T) {
	t.Run("Should not crash on empty config file", func(t *testing.T) {
		tsConfig := `{}`

		importsResolver := NewImportsResolver([]byte(tsConfig), []string{})

		aliasesCount := len(importsResolver.tsConfigParsed.aliases)

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

		importsResolver := NewImportsResolver([]byte(tsConfig), []string{})

		aliasesCount := len(importsResolver.tsConfigParsed.aliases)
		_, hasAlias := importsResolver.tsConfigParsed.aliases["@/dir/*"]

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

		importsResolver := NewImportsResolver([]byte(tsConfig), []string{})

		aliasesCount := len(importsResolver.tsConfigParsed.aliases)
		_, hasAlias := importsResolver.tsConfigParsed.aliases["@/dir/*"]

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

		importsResolver := NewImportsResolver([]byte(tsConfig), []string{})

		aliasesCount := len(importsResolver.tsConfigParsed.aliases)
		_, hasAlias := importsResolver.tsConfigParsed.aliases["@/dir/*"]

		if aliasesCount != 1 {
			t.Errorf("Aliases count not match")
		}

		if !hasAlias {
			t.Errorf("Aliases not found")
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

		importsResolver := NewImportsResolver([]byte(tsConfig), filePaths)

		resolvedPath, err := importsResolver.ResolveModule("@/dir/fileA", "/root/app/index.js", cwd)

		if resolvedPath != "/root/app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly")
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

		importsResolver := NewImportsResolver([]byte(tsConfig), filePaths)

		resolvedPath, err := importsResolver.ResolveModule("app/dir/fileA", "/root/app/index.js", cwd)

		if resolvedPath != "/root/app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly")
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

		importsResolver := NewImportsResolver([]byte(tsConfig), filePaths)

		resolvedPath, err := importsResolver.ResolveModule("./dir/fileA", "/root/app/index.js", cwd)

		if resolvedPath != "/root/app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly")
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

		importsResolver := NewImportsResolver([]byte(tsConfig), filePaths)

		resolvedPath, err := importsResolver.ResolveModule("../index", "/root/app/dir/fileA.ts", cwd)

		if resolvedPath != "/root/app/index.js" {
			t.Errorf("Path not resolved correctly")
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

		importsResolver := NewImportsResolver([]byte(tsConfig), filePaths)

		resolvedPath, err := importsResolver.ResolveModule("./dir/fileA.jsx", "/root/app/index.ts", cwd)

		if resolvedPath != "/root/app/dir/fileA.ts" {
			t.Errorf("Path not resolved correctly")
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

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, excludeFiles, []string{}, "", "")

	nodeModulesImports := minimalTree[cwd+"src/nodeModules.ts"]

	module1Ok := *nodeModulesImports[0].ID == "@types/node" && nodeModulesImports[0].ResolvedType == NodeModule
	module2Ok := *nodeModulesImports[1].ID == "node:fs" && nodeModulesImports[1].ResolvedType == BuiltInModule
	module3Ok := *nodeModulesImports[2].ID == "react" && nodeModulesImports[2].ResolvedType == NodeModule
	module4Ok := *nodeModulesImports[3].ID == "path" && nodeModulesImports[3].ResolvedType == BuiltInModule
	module5Ok := *nodeModulesImports[4].ID == "" && nodeModulesImports[4].ResolvedType == NotResolvedModule

	results := []bool{module1Ok, module2Ok, module3Ok, module4Ok, module5Ok}

	for idx, isOk := range results {
		if !isOk {
			t.Errorf("Module %d not resolved correctly", idx+1)
		}
	}
}
