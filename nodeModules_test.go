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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "\n @types/dep-types-1\n    ➞ tsconfig.json\n\n\n @types/dep-types-2\n    ➞ index.ts\n\n\n dep1\n    ➞ package.json\n\n\n dep2\n    ➞ index.ts\n\n\n dep4\n    ➞ file.ts\n\n\n dep5\n    ➞ file.ts\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should print node modules with files count", func(t *testing.T) {
		nodeModulesGroupByModuleFilesCount := true
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			false,
			false,
			nodeModulesGroupByModuleFilesCount,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-1 (1 files)\n@types/dep-types-2 (1 files)\ndep1 (1 files)\ndep2 (1 files)\ndep4 (1 files)\ndep5 (1 files)\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})
}

func TestUnusedNodeModules(t *testing.T) {
	currentDir, _ := os.Getwd()
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "1\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestMissingNodeModules(t *testing.T) {
	currentDir, _ := os.Getwd()
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "\n dep5\n    ➞ file.ts\n\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}

	})
}

func TestUnusedAdditionalFlags(t *testing.T) {
	currentDir, _ := os.Getwd()
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestUsedAdditionalFlags(t *testing.T) {
	currentDir, _ := os.Getwd()
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep3\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

}

func TestIncludeExclude(t *testing.T) {
	currentDir, _ := os.Getwd()
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
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
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})
}

func TestWildcardIncludeExclude(t *testing.T) {
	currentDir, _ := os.Getwd()
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

	t.Run("should support wildcard prefix matching (suffix wildcard)", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"*types-2", "dep*"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			[]string{},
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-2\ndep1\ndep2\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should support wildcard suffix matching (prefix wildcard)", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"@types/*", "*4"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			[]string{},
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep4\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should support wildcard contains matching (both prefix and suffix)", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"*dep*"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			[]string{},
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep4\ndep5\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should support wildcard exclusion", func(t *testing.T) {
		nodeModulesExcludeModules := []string{"@types/*", "dep*"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			[]string{},
			nodeModulesExcludeModules,
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "\n"

		if result != expected {
			t.Errorf("Incorrect modules list '%s'. Expected '%s'", result, expected)
		}
	})

	t.Run("should support mixed wildcard and exact patterns", func(t *testing.T) {
		nodeModulesIncludeModules := []string{"@types/dep-types-1", "*dep*"}
		result, _ := NodeModulesCmd(
			nodeModulesCwd,
			nodeModulesIgnoreType,
			nodeModulesEntryPoints,
			nodeModulesCountFlag,
			nodeModulesListUnused,
			nodeModulesListMissing,
			nodeModulesGroupByModule,
			nodeModulesGroupByFile,
			false,
			nodeModulesPkgJsonFieldsWithBinaries,
			nodeModulesFilesWithBinaries,
			nodeModulesFilesWithModules,
			nodeModulesIncludeModules,
			[]string{},
			"",
			"",
			[]string{},
			FollowMonorepoPackagesValue{},
		)

		expected := "@types/dep-types-1\n@types/dep-types-2\ndep1\ndep2\ndep4\ndep5\n"

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
func TestTsConfigTypesComplexRealWorld(t *testing.T) {
	currentDir, _ := os.Getwd()
	tsconfigTypesComplexCwd := filepath.Join(currentDir, "__fixtures__/tsconfigTypesComplex")

	t.Run("should handle complex tsconfig with mixed types", func(t *testing.T) {
		result, _ := NodeModulesCmd(
			tsconfigTypesComplexCwd,
			false,                         // ignoreType
			[]string{},                    // entryPoints
			false,                         // countFlag
			true,                          // listUnused
			false,                         // listMissing
			false,                         // groupByModule
			false,                         // groupByFile
			false,                         // groupByModuleFilesCount
			[]string{},                    // pkgJsonFieldsWithBinaries
			[]string{},                    // filesWithBinaries
			[]string{},                    // filesWithModules
			[]string{},                    // modulesToInclude
			[]string{},                    // modulesToExclude
			"",                            // packageJson
			"",                            // tsconfigJson
			[]string{},                    // conditionNames
			FollowMonorepoPackagesValue{}, // followMonorepoPackages
		)

		// This test reproduces the original bug scenario:
		// - Complex tsconfig with mixed types (booleans, strings, arrays, objects)
		// - @types/google.maps and @types/node are referenced in tsconfig.json, so they should NOT be in the unused list
		// - @types/unused-types is NOT referenced anywhere, so it SHOULD be in the unused list
		expected := "@types/unused-types\n"

		if result != expected {
			t.Errorf("Incorrect unused modules list '%s'. Expected '%s'", result, expected)
		}
	})
}
