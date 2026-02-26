package main

import "testing"

func TestFindRestrictedImports_DenyFiles(t *testing.T) {
	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{ID: "/repo/src/service.ts", Request: "./service", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/service.ts": {
			{ID: "/repo/src/ui/view.tsx", Request: "./ui/view", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/ui/view.tsx": {},
	}

	opts := &RestrictedImportsDetectionOptions{
		Enabled:     true,
		EntryPoints: []string{"src/server.ts"},
		DenyFiles:   []string{"**/*.tsx"},
	}

	violations := FindRestrictedImports(ruleTree, opts, "/repo")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}

	v := violations[0]
	if v.ViolationType != "file" {
		t.Fatalf("expected file violation, got %q", v.ViolationType)
	}
	if v.ImporterFile != "/repo/src/service.ts" {
		t.Fatalf("expected importer /repo/src/service.ts, got %q", v.ImporterFile)
	}
	if v.DeniedFile != "/repo/src/ui/view.tsx" {
		t.Fatalf("expected denied file /repo/src/ui/view.tsx, got %q", v.DeniedFile)
	}
	if v.EntryPoint != "/repo/src/server.ts" {
		t.Fatalf("expected entry point /repo/src/server.ts, got %q", v.EntryPoint)
	}
}

func TestFindRestrictedImports_IgnoreTypeImports(t *testing.T) {
	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{ID: "/repo/src/types.ts", Request: "./types", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/types.ts": {
			{ID: "/repo/src/ui/view.tsx", Request: "./ui/view", ResolvedType: UserModule, ImportKind: OnlyTypeImport},
		},
		"/repo/src/ui/view.tsx": {},
	}

	opts := &RestrictedImportsDetectionOptions{
		Enabled:           true,
		EntryPoints:       []string{"src/server.ts"},
		DenyFiles:         []string{"**/*.tsx"},
		IgnoreTypeImports: true,
	}

	violations := FindRestrictedImports(ruleTree, opts, "/repo")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations when ignoreTypeImports=true, got %d: %+v", len(violations), violations)
	}
}

func TestFindRestrictedImports_DenyModulesAndIgnore(t *testing.T) {
	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{ID: "/repo/src/service.ts", Request: "./service", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/service.ts": {
			{Request: "react/jsx-runtime", ResolvedType: NodeModule, ImportKind: NotTypeOrMixedImport},
			{Request: "react-dom/client", ResolvedType: NodeModule, ImportKind: NotTypeOrMixedImport},
		},
	}

	opts := &RestrictedImportsDetectionOptions{
		Enabled:       true,
		EntryPoints:   []string{"src/server.ts"},
		DenyModules:   []string{"react", "react-*"},
		IgnoreMatches: []string{"react-dom"},
	}

	violations := FindRestrictedImports(ruleTree, opts, "/repo")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}

	v := violations[0]
	if v.ViolationType != "module" {
		t.Fatalf("expected module violation, got %q", v.ViolationType)
	}
	if v.DeniedModule != "react" {
		t.Fatalf("expected denied module react, got %q", v.DeniedModule)
	}
	if v.ImportRequest != "react/jsx-runtime" {
		t.Fatalf("expected request react/jsx-runtime, got %q", v.ImportRequest)
	}
}
