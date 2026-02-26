package main

import "testing"

func TestFindDevDependenciesInProduction_IgnoreTypeImports(t *testing.T) {
	rulePath := "/repo"
	monorepoContext := NewMonorepoContext(rulePath)
	monorepoContext.PackageConfigCache[rulePath] = &PackageJsonConfig{
		DevDependencies: map[string]string{
			"eslint": "^8.0.0",
		},
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
		monorepoContext,
	)
	if len(violationsIgnore) != 0 {
		t.Fatalf("expected 0 violations with ignoreTypeImports=true, got %d: %+v", len(violationsIgnore), violationsIgnore)
	}

	violationsInclude := FindDevDependenciesInProduction(
		ruleTree,
		[]string{"src/server.ts"},
		false,
		rulePath,
		monorepoContext,
	)
	if len(violationsInclude) != 1 {
		t.Fatalf("expected 1 violation with ignoreTypeImports=false, got %d: %+v", len(violationsInclude), violationsInclude)
	}
	if violationsInclude[0].DevDependency != "eslint" {
		t.Fatalf("expected eslint violation, got %+v", violationsInclude[0])
	}
}
