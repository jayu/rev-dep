package main

import (
	"reflect"
	"testing"
)

func TestFindCircularDepsWithTypeImports(t *testing.T) {
	cwd := "__fixtures__/mockProject/"

	minimalDepsTree, sortedFiles, _ := GetMinimalDepsTreeForCwd(cwd, false, []string{}, []string{}, "", "", []string{}, false)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	expectedCircularDeps := [][]string{
		{cwd + "moduleSrc/fileA.tsx", cwd + "src/types.ts", cwd + "moduleSrc/fileA.tsx"},
		{cwd + "moduleSrc/anotherFileForCycle.js", cwd + "moduleSrc/oneMoreFileForCycle.tsx", cwd + "moduleSrc/fileForCycle.ts", cwd + "moduleSrc/anotherFileForCycle.js"},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}

func TestFindCircularDepsWithoutTypeImports(t *testing.T) {
	cwd := "__fixtures__/mockProject/"

	minimalDepsTree, sortedFiles, _ := GetMinimalDepsTreeForCwd(cwd, true, []string{}, []string{}, "", "", []string{}, false)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	expectedCircularDeps := [][]string{
		{cwd + "moduleSrc/anotherFileForCycle.js", cwd + "moduleSrc/oneMoreFileForCycle.tsx", cwd + "moduleSrc/fileForCycle.ts", cwd + "moduleSrc/anotherFileForCycle.js"},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}

func TestFindMultipleCircularDepsFromSameNode(t *testing.T) {
	cwd := "__fixtures__/multipleCyclesFromSameNode/"

	minimalDepsTree, sortedFiles, _ := GetMinimalDepsTreeForCwd(cwd, false, []string{}, []string{}, "", "", []string{}, false)

	circularDeps := FindCircularDependencies(minimalDepsTree, sortedFiles, false)

	// This test case starts with _index.ts file to assert search order that discovers two cycles from fileA
	expectedCircularDeps := [][]string{
		{cwd + "_index.ts", cwd + "fileA.ts", cwd + "fileB.ts", cwd + "_index.ts"},
		{cwd + "_index.ts", cwd + "fileA.ts", cwd + "fileC.ts", cwd + "_index.ts"},
	}

	equals := reflect.DeepEqual(circularDeps, expectedCircularDeps)
	if !equals {
		t.Errorf("\nCircular deps not equal\n %s\n----vs----\n\n%s", FormatCircularDependencies(circularDeps, cwd, minimalDepsTree), FormatCircularDependencies(expectedCircularDeps, cwd, minimalDepsTree))
	}
}
