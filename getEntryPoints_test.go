package main

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func entryPointsNotEqual(entryPoints []string, expectedEntryPoints []string) string {
	slices.Sort(entryPoints)
	slices.Sort(expectedEntryPoints)
	return fmt.Sprintf("\nEntry points not equal; Given:\n%s\n----vs----\nExpected:\n%s", strings.Join(entryPoints, ", "), strings.Join(expectedEntryPoints, ", "))
}

func TestGetEntryPoints(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := false
	exclude := []string{}
	include := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "")

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{cwd + "index.ts", cwd + "script.js", cwd + "src/importFileA.ts", cwd + "src/nodeModules.ts"}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithExclude(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := false
	exclude := []string{"script.js"}
	include := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "")

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{cwd + "index.ts", cwd + "src/importFileA.ts", cwd + "src/nodeModules.ts"}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithInclude(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := false
	exclude := []string{}
	include := []string{"script.js"}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "")

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{cwd + "script.js"}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithIgnoringTypeImports(t *testing.T) {
	cwd := "__fixtures__/mockProject/"
	ignoreTypeImports := true
	exclude := []string{}
	include := []string{}

	minimalTree, _, _ := GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "")

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{cwd + "index.ts", cwd + "script.js", cwd + "src/importFileA.ts", cwd + "src/nodeModules.ts", cwd + "src/types.ts"}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}
