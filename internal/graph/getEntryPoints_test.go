package graph

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"rev-dep-go/internal/model"
	"rev-dep-go/internal/resolve"
	"rev-dep-go/internal/testutil"
)

func entryPointsNotEqual(entryPoints []string, expectedEntryPoints []string) string {
	slices.Sort(entryPoints)
	slices.Sort(expectedEntryPoints)
	return fmt.Sprintf("\nEntry points not equal; Given:\n%s\n----vs----\nExpected:\n%s", strings.Join(entryPoints, ", "), strings.Join(expectedEntryPoints, ", "))
}

func TestGetEntryPoints(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root)
	ignoreTypeImports := false
	exclude := []string{}
	include := []string{}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{
		filepath.Join(cwd, "index.ts"),
		filepath.Join(cwd, "script.js"),
		filepath.Join(cwd, "src/importFileA.ts"),
		filepath.Join(cwd, "src/nodeModules.ts"),
	}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithExclude(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root)
	ignoreTypeImports := false
	exclude := []string{"script.js"}
	include := []string{}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{
		filepath.Join(cwd, "index.ts"),
		filepath.Join(cwd, "src/importFileA.ts"),
		filepath.Join(cwd, "src/nodeModules.ts"),
	}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithInclude(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root)
	ignoreTypeImports := false
	exclude := []string{}
	include := []string{"script.js"}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{filepath.Join(cwd, "script.js")}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}

func TestGetEntryWithIgnoringTypeImports(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root)
	ignoreTypeImports := true
	exclude := []string{}
	include := []string{}

	minimalTree, _, _ := resolve.GetMinimalDepsTreeForCwd(cwd, ignoreTypeImports, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	entryPoints := GetEntryPoints(minimalTree, exclude, include, cwd)

	expectedEntryPoints := []string{
		filepath.Join(cwd, "index.ts"),
		filepath.Join(cwd, "script.js"),
		filepath.Join(cwd, "src/importFileA.ts"),
		filepath.Join(cwd, "src/nodeModules.ts"),
		filepath.Join(cwd, "src/types.ts"),
	}

	equals := reflect.DeepEqual(entryPoints, expectedEntryPoints)

	if !equals {
		t.Error(entryPointsNotEqual(entryPoints, expectedEntryPoints))
	}
}
