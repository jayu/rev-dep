package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type TestCase struct {
	request    string
	moduleName string
}

func TestGetNodeModuleName(t *testing.T) {

	cases := []TestCase{
		{
			request:    "@org/name",
			moduleName: "@org/name",
		},
		{
			request:    "@org/name/src/file",
			moduleName: "@org/name",
		},
		{
			request:    "@org/name/src/file.ts",
			moduleName: "@org/name",
		},
		{
			request:    "name",
			moduleName: "name",
		},
		{
			request:    "name/src/file",
			moduleName: "name",
		},
		{
			request:    "name/src/file.ts",
			moduleName: "name",
		},
	}

	for _, testCase := range cases {
		name := GetNodeModuleName(testCase.request)

		if name != testCase.moduleName {
			t.Errorf("Module name '%s' incorrectly parsed to '%s'. Should be '%s'", testCase.request, name, testCase.moduleName)
		}

	}

}

func TestUsedNodeModules(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := false
	nodeModulesListMissing := false
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}
	t.Run("should print flat list of node modules", func(t *testing.T) {

		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		// TODO command should return also dep5 - used by not defined in pkg json
		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print count of node modules", func(t *testing.T) {
		nodeModulesCountFlag := true

		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "6\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print node modules grouped by files", func(t *testing.T) {
		nodeModulesGroupByFile := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n file.ts\n    ➞ dep4\n    ➞ dep5\n\n\n index.ts\n    ➞ @types/dep-types-2\n    ➞ dep2\n\n\n package.json\n    ➞ dep1\n\n\n tsconfig.json\n    ➞ @types/dep-types-1\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print node modules grouped by modules", func(t *testing.T) {
		nodeModulesGroupByModule := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n @types/dep-types-1\n    ➞ tsconfig.json\n\n\n @types/dep-types-2\n    ➞ index.ts\n\n\n dep1\n    ➞ package.json\n\n\n dep2\n    ➞ index.ts\n\n\n dep4\n    ➞ file.ts\n\n\n dep5\n    ➞ file.ts\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})
}

func TestUnusedNodeModules(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := true
	nodeModulesListMissing := false
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}
	t.Run("should print flat list of node modules", func(t *testing.T) {
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "dep3\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should print count of node modules", func(t *testing.T) {
		nodeModulesCountFlag := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "1\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestMissingNodeModules(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := false
	nodeModulesListMissing := true
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}

	t.Run("should print flat list of node modules", func(t *testing.T) {
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "dep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print count of node modules", func(t *testing.T) {
		nodeModulesCountFlag := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "1\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print node modules groupe by file", func(t *testing.T) {
		nodeModulesGroupByFile := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n file.ts\n    ➞ dep5\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})

	t.Run("should print node modules groupe by module", func(t *testing.T) {
		nodeModulesGroupByModule := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n dep5\n    ➞ file.ts\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})
}

func TestUnusedAdditionalFlags(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := true
	nodeModulesListMissing := false
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}
	t.Run("should detect binary in text file", func(t *testing.T) {
		nodeModulesFilesWithBinaries := []string{"fileWithBinary.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in text file", func(t *testing.T) {
		nodeModulesFilesWithModules := []string{"fileWithModule.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in package json scripts", func(t *testing.T) {
		nodeModulesFilesWithModules := []string{"fileWithModule.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in package json field", func(t *testing.T) {
		nodeModulesPkgJsonFieldsWithBinaries := []string{"some-tool-settings"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestUsedAdditionalFlags(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := false
	nodeModulesListMissing := false
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}
	t.Run("should detect binary in text file", func(t *testing.T) {
		nodeModulesFilesWithBinaries := []string{"fileWithBinary.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep3\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in text file", func(t *testing.T) {
		nodeModulesFilesWithModules := []string{"fileWithModule.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep3\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in package json scripts", func(t *testing.T) {
		nodeModulesFilesWithModules := []string{"fileWithModule.txt"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep3\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should detect node module in package json field", func(t *testing.T) {
		nodeModulesPkgJsonFieldsWithBinaries := []string{"some-tool-settings"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep3\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestIncludeExclude(t *testing.T) {
	currentDir, _ = os.Getwd()
	nodeModulesCwd := filepath.Join(currentDir, "__fixtures__/nodeModulesCmd")
	nodeModulesIgnoreType := false
	nodeModulesEntryPoints := []string{}
	nodeModulesCountFlag := false
	nodeModulesListUnused := false
	nodeModulesListMissing := false
	nodeModulesGroupByModule := false
	nodeModulesGroupByFile := false
	nodeModulesPkgJsonFieldsWithBinaries := []string{}
	nodeModulesFilesWithBinaries := []string{}
	nodeModulesFilesWithModules := []string{}
	nodeModulesIncludeModules := []string{}
	nodeModulesExcludeModules := []string{}
	t.Run("should list only included modules in used modules cmd", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"@types/dep-types-2", "dep4"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-2\ndep4\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should not list excluded modules in used modules cmd", func(t *testing.T) {
		nodeModulesExcludeModules := []string{"@types/dep-types-2", "dep4"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "@types/dep-types-1\ndep1\ndep2\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should list only included modules in unused modules cmd", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"dep3"}
		nodeModulesListUnused := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "dep3\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should not list excluded modules in unused modules cmd", func(t *testing.T) {
		nodeModulesExcludeModules := []string{"dep3"}
		nodeModulesListUnused := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should list only included modules in missing modules cmd", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"dep5"}
		nodeModulesListMissing := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "dep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should not list excluded modules in missing modules cmd", func(t *testing.T) {
		nodeModulesExcludeModules := []string{"dep5"}
		nodeModulesListMissing := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})
}

func TestSortPathsToNodeModulesByNestingLevel(t *testing.T) {
	t.Run("Different nesting level", func(t *testing.T) {
		input := []string{"node_modules/inherits/package.json", "node_modules/google-gax/node_modules/@grpc/proto-loader/node_modules/protobufjs/cli/node_modules/inherits/package.json", "node_modules/protobufjs/cli/node_modules/inherits/package.json"}
		expected := []string{"node_modules/inherits/package.json", "node_modules/protobufjs/cli/node_modules/inherits/package.json", "node_modules/google-gax/node_modules/@grpc/proto-loader/node_modules/protobufjs/cli/node_modules/inherits/package.json"}

		SortPathsToNodeModulesByNestingLevel(input)

		if !reflect.DeepEqual(input, expected) {
			t.Errorf("Array not sorted correctly \n'%s'. \nExpected \n'%s'", input, expected)
		}
	})

	t.Run("Different path length", func(t *testing.T) {
		input := []string{"node_modules/short/package.json", "node_modules/long-package-name/package.json"}
		expected := []string{"node_modules/short/package.json", "node_modules/long-package-name/package.json"}

		SortPathsToNodeModulesByNestingLevel(input)

		if !reflect.DeepEqual(input, expected) {
			t.Errorf("Array not sorted correctly \n'%s'. \nExpected \n'%s'", input, expected)
		}
	})

	t.Run("Different path alphabetical order", func(t *testing.T) {
		input := []string{"node_modules/abcd/package.json", "node_modules/efgh/package.json"}
		expected := []string{"node_modules/abcd/package.json", "node_modules/efgh/package.json"}

		SortPathsToNodeModulesByNestingLevel(input)

		if !reflect.DeepEqual(input, expected) {
			t.Errorf("Array not sorted correctly \n'%s'. \nExpected \n'%s'", input, expected)
		}
	})
}
