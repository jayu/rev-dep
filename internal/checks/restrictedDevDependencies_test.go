package checks

import "testing"

func TestFindDevDependenciesInProduction_IgnoreTypeImports(t *testing.T) {
	rulePath := "/repo"
	devDependencies := map[string]bool{
		"eslint": true,
	}

	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{
				Request:      "eslint",
				ResolvedType: NodeModule,
				ImportKind:   OnlyTypeImport,
			},
		},
	}

	violationsIgnore := FindDevDependenciesInProduction(
		ruleTree,
		[]string{"src/server.ts"},
		true,
		rulePath,
		devDependencies,
	)
	if len(violationsIgnore) != 0 {
		t.Fatalf("expected 0 violations with ignoreTypeImports=true, got %d: %+v", len(violationsIgnore), violationsIgnore)
	}

	violationsInclude := FindDevDependenciesInProduction(
		ruleTree,
		[]string{"src/server.ts"},
		false,
		rulePath,
		devDependencies,
	)
	if len(violationsInclude) != 1 {
		t.Fatalf("expected 1 violation with ignoreTypeImports=false, got %d: %+v", len(violationsInclude), violationsInclude)
	}
	if violationsInclude[0].DevDependency != "eslint" {
		t.Fatalf("expected eslint violation, got %+v", violationsInclude[0])
	}
}
