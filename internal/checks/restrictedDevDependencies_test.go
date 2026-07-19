package checks

import "testing"

// TestFindDevDependenciesInProduction_PerFileSets verifies that the per-file dev dependency lookup
// flags violations against each file's own set — mirroring nearest-package (each file's own package
// devDeps) plus includeDevDepsFromRoot (root devDeps available everywhere).
func TestFindDevDependenciesInProduction_PerFileSets(t *testing.T) {
	rulePath := "/repo"
	ruleTree := MinimalDependencyTree{
		"/repo/app/server.ts": {
			{ID: "/repo/shared/thing.ts", Request: "../shared/thing", ResolvedType: UserModule},
			{Request: "app-dev", ResolvedType: NodeModule},
			{Request: "root-dev", ResolvedType: NodeModule},
		},
		"/repo/shared/thing.ts": {
			{Request: "shared-dev", ResolvedType: NodeModule},
		},
	}

	// nearest-package: each file resolves against its own package devDeps. Root devDeps are added
	// everywhere (includeDevDepsFromRoot).
	rootDevDeps := map[string]bool{"root-dev": true}
	devDepsForFile := func(filePath string) map[string]bool {
		base := map[string]bool{}
		switch {
		case filePath == "/repo/app/server.ts":
			base = map[string]bool{"app-dev": true}
		case filePath == "/repo/shared/thing.ts":
			base = map[string]bool{"shared-dev": true}
		}
		merged := map[string]bool{}
		for k := range base {
			merged[k] = true
		}
		for k := range rootDevDeps {
			merged[k] = true
		}
		return merged
	}

	violations := FindDevDependenciesInProduction(ruleTree, []string{"app/server.ts"}, false, rulePath, devDepsForFile)

	got := map[string]string{} // devDependency -> filePath
	for _, v := range violations {
		got[v.DevDependency] = v.FilePath
	}

	want := map[string]string{
		"app-dev":    "/repo/app/server.ts",   // app's own devDep used in prod
		"root-dev":   "/repo/app/server.ts",   // root devDep used in prod (includeDevDepsFromRoot)
		"shared-dev": "/repo/shared/thing.ts", // shared's own devDep used in prod (nearest-package)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d violations, got %d: %+v", len(want), len(got), violations)
	}
	for dep, file := range want {
		if got[dep] != file {
			t.Errorf("expected %s flagged on %s, got %q", dep, file, got[dep])
		}
	}
}

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

	devDepsForFile := func(string) map[string]bool { return devDependencies }

	violationsIgnore := FindDevDependenciesInProduction(
		ruleTree,
		[]string{"src/server.ts"},
		true,
		rulePath,
		devDepsForFile,
	)
	if len(violationsIgnore) != 0 {
		t.Fatalf("expected 0 violations with ignoreTypeImports=true, got %d: %+v", len(violationsIgnore), violationsIgnore)
	}

	violationsInclude := FindDevDependenciesInProduction(
		ruleTree,
		[]string{"src/server.ts"},
		false,
		rulePath,
		devDepsForFile,
	)
	if len(violationsInclude) != 1 {
		t.Fatalf("expected 1 violation with ignoreTypeImports=false, got %d: %+v", len(violationsInclude), violationsInclude)
	}
	if violationsInclude[0].DevDependency != "eslint" {
		t.Fatalf("expected eslint violation, got %+v", violationsInclude[0])
	}
}
