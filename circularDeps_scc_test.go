package main

import (
	"reflect"
	"testing"
)

func TestFindCircularDependenciesSCCStableOrdering(t *testing.T) {
	sorted := []string{"A", "B", "C", "D", "E"}

	deps1 := MinimalDependencyTree{
		"A": {
			{ID: "B", ImportKind: NotTypeOrMixedImport},
			{ID: "C", ImportKind: NotTypeOrMixedImport},
		},
		"B": {
			{ID: "C", ImportKind: NotTypeOrMixedImport},
		},
		"C": {
			{ID: "A", ImportKind: NotTypeOrMixedImport},
		},
		"D": {
			{ID: "E", ImportKind: NotTypeOrMixedImport},
		},
		"E": {
			{ID: "D", ImportKind: NotTypeOrMixedImport},
		},
	}

	deps2 := MinimalDependencyTree{
		"A": {
			{ID: "C", ImportKind: NotTypeOrMixedImport},
			{ID: "B", ImportKind: NotTypeOrMixedImport},
		},
		"B": {
			{ID: "C", ImportKind: NotTypeOrMixedImport},
		},
		"C": {
			{ID: "A", ImportKind: NotTypeOrMixedImport},
		},
		"D": {
			{ID: "E", ImportKind: NotTypeOrMixedImport},
		},
		"E": {
			{ID: "D", ImportKind: NotTypeOrMixedImport},
		},
	}

	cycles1 := FindCircularDependenciesSCC(deps1, sorted, false)
	cycles2 := FindCircularDependenciesSCC(deps2, sorted, false)

	expected := [][]string{
		{"A", "B", "C", "A"},
		{"D", "E", "D"},
	}

	if !reflect.DeepEqual(cycles1, expected) {
		t.Errorf("unexpected cycles for deps1: %v", cycles1)
	}
	if !reflect.DeepEqual(cycles2, expected) {
		t.Errorf("unexpected cycles for deps2: %v", cycles2)
	}
}

func TestFindCircularDependenciesSCCTypeImports(t *testing.T) {
	sorted := []string{"A", "B"}
	deps := MinimalDependencyTree{
		"A": {
			{ID: "B", ImportKind: NotTypeOrMixedImport},
		},
		"B": {
			{ID: "A", ImportKind: OnlyTypeImport},
		},
	}

	cyclesWithTypes := FindCircularDependenciesSCC(deps, sorted, false)
	expectedWithTypes := [][]string{
		{"A", "B", "A"},
	}
	if !reflect.DeepEqual(cyclesWithTypes, expectedWithTypes) {
		t.Errorf("unexpected cycles (types allowed): %v", cyclesWithTypes)
	}

	cyclesWithoutTypes := FindCircularDependenciesSCC(deps, sorted, true)
	if len(cyclesWithoutTypes) != 0 {
		t.Errorf("expected no cycles when type imports are ignored, got: %v", cyclesWithoutTypes)
	}
}

func TestFindCircularDependenciesSCCSelfLoop(t *testing.T) {
	sorted := []string{"A"}
	deps := MinimalDependencyTree{
		"A": {
			{ID: "A", ImportKind: NotTypeOrMixedImport},
		},
	}

	cycles := FindCircularDependenciesSCC(deps, sorted, false)
	expected := [][]string{
		{"A", "A"},
	}
	if !reflect.DeepEqual(cycles, expected) {
		t.Errorf("unexpected self-loop cycles: %v", cycles)
	}
}

func TestFindCircularDependenciesSCCCompleteGraph(t *testing.T) {
	sorted := []string{"a.ts", "b.ts", "c.ts"}
	deps := MinimalDependencyTree{
		"a.ts": {
			{ID: "b.ts", ImportKind: NotTypeOrMixedImport},
			{ID: "c.ts", ImportKind: NotTypeOrMixedImport},
		},
		"b.ts": {
			{ID: "a.ts", ImportKind: NotTypeOrMixedImport},
			{ID: "c.ts", ImportKind: NotTypeOrMixedImport},
		},
		"c.ts": {
			{ID: "a.ts", ImportKind: NotTypeOrMixedImport},
			{ID: "b.ts", ImportKind: NotTypeOrMixedImport},
		},
	}

	cycles := FindCircularDependenciesSCC(deps, sorted, false)
	expected := [][]string{
		{"a.ts", "b.ts", "a.ts"},
	}
	if !reflect.DeepEqual(cycles, expected) {
		t.Errorf("unexpected cycles for complete graph: %v", cycles)
	}
}
