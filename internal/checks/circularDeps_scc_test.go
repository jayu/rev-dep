package checks

import (
	"reflect"
	"testing"
)

func TestFindCircularDependenciesSCCStableOrdering(t *testing.T) {
	sorted := []string{"A", "B", "C", "D", "E"}

	deps1 := MinimalDependencyTree{
		"A": {{ID: "B", ImportKind: NotTypeOrMixedImport}, {ID: "C", ImportKind: NotTypeOrMixedImport}},
		"B": {{ID: "C", ImportKind: NotTypeOrMixedImport}},
		"C": {{ID: "A", ImportKind: NotTypeOrMixedImport}},
		"D": {{ID: "E", ImportKind: NotTypeOrMixedImport}},
		"E": {{ID: "D", ImportKind: NotTypeOrMixedImport}},
	}

	deps2 := MinimalDependencyTree{
		"A": {{ID: "C", ImportKind: NotTypeOrMixedImport}, {ID: "B", ImportKind: NotTypeOrMixedImport}},
		"B": {{ID: "C", ImportKind: NotTypeOrMixedImport}},
		"C": {{ID: "A", ImportKind: NotTypeOrMixedImport}},
		"D": {{ID: "E", ImportKind: NotTypeOrMixedImport}},
		"E": {{ID: "D", ImportKind: NotTypeOrMixedImport}},
	}

	expected := [][]string{{"A", "B", "C", "A"}, {"D", "E", "D"}}

	if cycles := FindCircularDependenciesSCC(deps1, sorted, false); !reflect.DeepEqual(cycles, expected) {
		t.Errorf("unexpected cycles for deps1: %v", cycles)
	}
	if cycles := FindCircularDependenciesSCC(deps2, sorted, false); !reflect.DeepEqual(cycles, expected) {
		t.Errorf("unexpected cycles for deps2: %v", cycles)
	}
}

func TestFindCircularDependenciesSCCTypeImports(t *testing.T) {
	sorted := []string{"A", "B"}
	deps := MinimalDependencyTree{
		"A": {{ID: "B", ImportKind: NotTypeOrMixedImport}},
		"B": {{ID: "A", ImportKind: OnlyTypeImport}},
	}

	if cycles := FindCircularDependenciesSCC(deps, sorted, false); !reflect.DeepEqual(cycles, [][]string{{"A", "B", "A"}}) {
		t.Errorf("unexpected cycles (types allowed): %v", cycles)
	}
	if cycles := FindCircularDependenciesSCC(deps, sorted, true); len(cycles) != 0 {
		t.Errorf("expected no cycles when type imports are ignored, got: %v", cycles)
	}
}

func TestFindCircularDependenciesSCCSelfLoop(t *testing.T) {
	sorted := []string{"A"}
	deps := MinimalDependencyTree{
		"A": {{ID: "A", ImportKind: NotTypeOrMixedImport}},
	}

	if cycles := FindCircularDependenciesSCC(deps, sorted, false); !reflect.DeepEqual(cycles, [][]string{{"A", "A"}}) {
		t.Errorf("unexpected self-loop cycles: %v", cycles)
	}
}

func TestFindCircularDependenciesSCCCompleteGraph(t *testing.T) {
	sorted := []string{"a.ts", "b.ts", "c.ts"}
	deps := MinimalDependencyTree{
		"a.ts": {{ID: "b.ts", ImportKind: NotTypeOrMixedImport}, {ID: "c.ts", ImportKind: NotTypeOrMixedImport}},
		"b.ts": {{ID: "a.ts", ImportKind: NotTypeOrMixedImport}, {ID: "c.ts", ImportKind: NotTypeOrMixedImport}},
		"c.ts": {{ID: "a.ts", ImportKind: NotTypeOrMixedImport}, {ID: "b.ts", ImportKind: NotTypeOrMixedImport}},
	}

	if cycles := FindCircularDependenciesSCC(deps, sorted, false); !reflect.DeepEqual(cycles, [][]string{{"a.ts", "b.ts", "a.ts"}}) {
		t.Errorf("unexpected cycles for complete graph: %v", cycles)
	}
}
