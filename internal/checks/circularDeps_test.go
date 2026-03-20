package checks

import (
	"path/filepath"
	"reflect"
	"testing"

	"rev-dep-go/internal/model"
	"rev-dep-go/internal/resolve"
	"rev-dep-go/internal/testutil"
)

func TestFindCircularDepsWithTypeImports(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root) + string(filepath.Separator)

	minimalDepsTree, sortedFiles, _ := resolve.GetMinimalDepsTreeForCwd(cwd, false, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	expectedCircularDeps := [][]string{
		{
			filepath.Join(cwd, "moduleSrc/fileA.tsx"),
			filepath.Join(cwd, "src/types.ts"),
			filepath.Join(cwd, "moduleSrc/fileA.tsx"),
		},
		{
			filepath.Join(cwd, "moduleSrc/anotherFileForCycle.js"),
			filepath.Join(cwd, "moduleSrc/oneMoreFileForCycle.tsx"),
			filepath.Join(cwd, "moduleSrc/fileForCycle.ts"),
			filepath.Join(cwd, "moduleSrc/anotherFileForCycle.js"),
		},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}

func TestFindCircularDepsWithoutTypeImports(t *testing.T) {
	root, err := testutil.FixturePath("mockProject")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root) + string(filepath.Separator)

	minimalDepsTree, sortedFiles, _ := resolve.GetMinimalDepsTreeForCwd(cwd, true, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	expectedCircularDeps := [][]string{
		{
			filepath.Join(cwd, "moduleSrc/anotherFileForCycle.js"),
			filepath.Join(cwd, "moduleSrc/oneMoreFileForCycle.tsx"),
			filepath.Join(cwd, "moduleSrc/fileForCycle.ts"),
			filepath.Join(cwd, "moduleSrc/anotherFileForCycle.js"),
		},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}

func TestFindMultipleCircularDepsFromSameNode(t *testing.T) {
	root, err := testutil.FixturePath("multipleCyclesFromSameNode")
	if err != nil {
		t.Fatalf("FixturePath: %v", err)
	}
	cwd := filepath.Clean(root) + string(filepath.Separator)

	minimalDepsTree, sortedFiles, _ := resolve.GetMinimalDepsTreeForCwd(cwd, false, []string{}, []string{}, "", "", []string{}, model.FollowMonorepoPackagesValue{}, nil)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	// This test case starts with _index.ts file to assert search order that discovers two cycles from fileA
	expectedCircularDeps := [][]string{
		{filepath.Join(cwd, "_index.ts"), filepath.Join(cwd, "fileA.ts"), filepath.Join(cwd, "fileB.ts"), filepath.Join(cwd, "_index.ts")},
		{filepath.Join(cwd, "_index.ts"), filepath.Join(cwd, "fileA.ts"), filepath.Join(cwd, "fileC.ts"), filepath.Join(cwd, "_index.ts")},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}
