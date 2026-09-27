package checks

import (
	"reflect"
	"testing"

	"rev-dep-go/internal/rules"
)

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

	opts := &rules.RestrictedImportsDetectionOptions{
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

	opts := &rules.RestrictedImportsDetectionOptions{
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

	opts := &rules.RestrictedImportsDetectionOptions{
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

// ignoreMatches carves a specific denied file out of a broader denyFiles pattern - the
// file-path counterpart of TestFindRestrictedImports_DenyModulesAndIgnore.
func TestFindRestrictedImports_DenyFilesAndIgnore(t *testing.T) {
	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{ID: "/repo/src/secrets/private-keys.ts", Request: "./secrets/private-keys", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
			{ID: "/repo/src/secrets/public-keys.ts", Request: "./secrets/public-keys", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/secrets/private-keys.ts": {},
		"/repo/src/secrets/public-keys.ts":  {},
	}

	opts := &rules.RestrictedImportsDetectionOptions{
		Enabled:       true,
		EntryPoints:   []string{"src/server.ts"},
		DenyFiles:     []string{"src/secrets/*"},
		IgnoreMatches: []string{"src/secrets/public-keys.ts"},
	}

	violations := FindRestrictedImports(ruleTree, opts, "/repo")
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation (public-keys carved out), got %d: %+v", len(violations), violations)
	}
	if violations[0].DeniedFile != "/repo/src/secrets/private-keys.ts" {
		t.Fatalf("expected denied file private-keys.ts, got %q", violations[0].DeniedFile)
	}
}

func TestFindRestrictedImports_GraphExclude(t *testing.T) {
	ruleTree := MinimalDependencyTree{
		"/repo/src/server.ts": {
			{ID: "/repo/src/service.ts", Request: "./service", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/service.ts": {
			{ID: "/repo/src/ui/view.tsx", Request: "./ui/view", ResolvedType: UserModule, ImportKind: NotTypeOrMixedImport},
		},
		"/repo/src/ui/view.tsx": {},
	}

	opts := &rules.RestrictedImportsDetectionOptions{
		Enabled:      true,
		EntryPoints:  []string{"src/server.ts"},
		GraphExclude: []string{"src/service.ts"},
		DenyFiles:    []string{"**/*.tsx"},
	}

	violations := FindRestrictedImports(ruleTree, opts, "/repo")
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations with graphExclude, got %d: %+v", len(violations), violations)
	}
}

func builtInEntryTree() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/main.ts": {
			{ID: "/repo/src/domain/service.ts", Request: "./domain/service", ResolvedType: UserModule},
		},
		"/repo/src/domain/service.ts": {
			{ID: "fs", Request: "fs", ResolvedType: BuiltInModule},
			{ID: "node:fs", Request: "node:fs", ResolvedType: BuiltInModule},
			{ID: "fs", Request: "fs/promises", ResolvedType: BuiltInModule},
			{ID: "node:fs", Request: "node:fs/promises", ResolvedType: BuiltInModule},
			{ID: "path", Request: "path", ResolvedType: BuiltInModule},
			{ID: "bun:sqlite", Request: "bun:sqlite", ResolvedType: BuiltInModule},
			{ID: "axios", Request: "axios", ResolvedType: NodeModule},
			{ID: "node:net", Request: "node:net", ResolvedType: BuiltInModule, ImportKind: OnlyTypeImport},
		},
	}
}

func TestFindRestrictedImports_DenyBuiltInModules(t *testing.T) {
	allFs := []string{"fs|fs", "fs|fs/promises", "node:fs|node:fs", "node:fs|node:fs/promises"}

	cases := []struct {
		denyModules       []string
		ignoreTypeImports bool
		want              []string
	}{
		{denyModules: []string{"fs"}, want: []string{"fs|fs", "fs|fs/promises"}},
		{denyModules: []string{"node:fs"}, want: []string{"node:fs|node:fs", "node:fs|node:fs/promises"}},
		{denyModules: []string{"fs", "node:fs"}, want: allFs},
		{denyModules: []string{"fs/*"}, want: []string{"fs|fs/promises"}},
		{denyModules: []string{"node:*"}, want: []string{"node:fs|node:fs", "node:fs|node:fs/promises", "node:net|node:net"}},
		{denyModules: []string{"node:*"}, ignoreTypeImports: true, want: []string{"node:fs|node:fs", "node:fs|node:fs/promises"}},
		{denyModules: []string{"bun:*"}, want: []string{"bun:sqlite|bun:sqlite"}},
		{denyModules: []string{"builtin:*"}, want: append(append([]string{"bun:sqlite|bun:sqlite"}, allFs...), "node:net|node:net", "path|path")},
		{denyModules: []string{"dns", "net"}, ignoreTypeImports: true, want: []string{}},
	}

	for _, tc := range cases {
		opts := &rules.RestrictedImportsDetectionOptions{
			Enabled:           true,
			EntryPoints:       []string{"src/main.ts"},
			DenyModules:       tc.denyModules,
			IgnoreTypeImports: tc.ignoreTypeImports,
		}
		violations := FindRestrictedImports(builtInEntryTree(), opts, "/repo")

		got := []string{}
		for _, v := range violations {
			if v.ViolationType != "module" || v.ImporterFile != "/repo/src/domain/service.ts" || v.EntryPoint != "/repo/src/main.ts" {
				t.Errorf("denyModules=%v: unexpected violation %+v", tc.denyModules, v)
			}
			got = append(got, v.DeniedModule+"|"+v.ImportRequest)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("denyModules=%v ignoreTypeImports=%v: got %v, want %v", tc.denyModules, tc.ignoreTypeImports, got, tc.want)
		}
	}
}

func frontendLeakTree() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/frontend/app.tsx": {
			{ID: "/repo/src/frontend/button.tsx", Request: "./button", ResolvedType: UserModule},
			{ID: "/repo/src/shared/format.ts", Request: "../shared/format", ResolvedType: UserModule},
			{ID: "react", Request: "react", ResolvedType: NodeModule},
		},
		"/repo/src/frontend/button.tsx": {
			{ID: "react", Request: "react", ResolvedType: NodeModule},
		},
		"/repo/src/shared/format.ts": {
			{ID: "/repo/src/backend/config.ts", Request: "../backend/config", ResolvedType: UserModule},
		},
		"/repo/src/backend/config.ts": {
			{ID: "fs", Request: "fs", ResolvedType: BuiltInModule},
			{ID: "node:path", Request: "node:path", ResolvedType: BuiltInModule},
		},
		"/repo/src/server/main.ts": {
			{ID: "/repo/src/backend/config.ts", Request: "../backend/config", ResolvedType: UserModule},
		},
	}
}

func TestFindRestrictedImports_FrontendEntryPointReachesBuiltIns(t *testing.T) {
	opts := &rules.RestrictedImportsDetectionOptions{
		Enabled:     true,
		EntryPoints: []string{"src/frontend/app.tsx"},
		DenyModules: []string{"builtin:*"},
	}

	got := FindRestrictedImports(frontendLeakTree(), opts, "/repo")
	want := []RestrictedImportViolation{
		{ViolationType: "module", ImporterFile: "/repo/src/backend/config.ts", EntryPoint: "/repo/src/frontend/app.tsx", DeniedModule: "fs", ImportRequest: "fs"},
		{ViolationType: "module", ImporterFile: "/repo/src/backend/config.ts", EntryPoint: "/repo/src/frontend/app.tsx", DeniedModule: "node:path", ImportRequest: "node:path"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFindRestrictedImports_IgnoreMatchesBuiltInModules(t *testing.T) {
	tree := MinimalDependencyTree{
		"/repo/src/main.ts": {
			{ID: "/repo/src/service.ts", Request: "./service", ResolvedType: UserModule},
		},
		"/repo/src/service.ts": {
			{ID: "fs", Request: "fs", ResolvedType: BuiltInModule},
			{ID: "node:fs", Request: "node:fs/promises", ResolvedType: BuiltInModule},
			{ID: "path", Request: "path", ResolvedType: BuiltInModule},
			{ID: "node:path", Request: "node:path", ResolvedType: BuiltInModule},
			{ID: "bun:sqlite", Request: "bun:sqlite", ResolvedType: BuiltInModule},
			{ID: "axios", Request: "axios", ResolvedType: NodeModule},
		},
	}

	cases := []struct {
		ignoreMatches []string
		want          []string
	}{
		{nil, []string{"axios", "bun:sqlite", "fs", "node:fs/promises", "node:path", "path"}},
		{[]string{"path"}, []string{"axios", "bun:sqlite", "fs", "node:fs/promises", "node:path"}},
		{[]string{"node:path"}, []string{"axios", "bun:sqlite", "fs", "node:fs/promises", "path"}},
		{[]string{"path", "node:path"}, []string{"axios", "bun:sqlite", "fs", "node:fs/promises"}},
		{[]string{"node:*"}, []string{"axios", "bun:sqlite", "fs", "path"}},
		{[]string{"node:*", "!node:path"}, []string{"axios", "bun:sqlite", "fs", "node:path", "path"}},
	}

	for _, tc := range cases {
		opts := &rules.RestrictedImportsDetectionOptions{
			Enabled:       true,
			EntryPoints:   []string{"src/main.ts"},
			DenyModules:   []string{"builtin:*", "axios"},
			IgnoreMatches: tc.ignoreMatches,
		}
		got := []string{}
		for _, v := range FindRestrictedImports(tree, opts, "/repo") {
			got = append(got, v.ImportRequest)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ignoreMatches=%v: got %v, want %v", tc.ignoreMatches, got, tc.want)
		}
	}
}
