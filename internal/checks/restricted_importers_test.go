package checks

import (
	"reflect"
	"testing"

	"rev-dep-go/internal/rules"
)

// importersTreeRich is a small tree covering the behaviors FindRestrictedImporters must get right:
//   - src/app/main.ts reaches legacy/core.ts transitively (through src/service.ts)
//   - src/admin/main.ts reaches legacy/core.ts directly
//   - scripts/orphan.ts imports legacy/core.ts but is unreachable from any entry point and is not
//     itself an entry point, so it must never produce a violation
//   - src/types/main.ts imports legacy/core.ts with a type-only import (IgnoreTypeImports)
//   - src/service.ts imports the "moment" node module (modules)
//
//	src/app/main.ts   -> src/service.ts -> legacy/core.ts        (transitive)
//	src/admin/main.ts -> legacy/core.ts                          (direct)
//	scripts/orphan.ts -> legacy/core.ts                          (unreachable from entry points)
//	src/types/main.ts -> legacy/core.ts                          (type-only)
//	src/app/main.ts   -> src/service.ts -> moment (node module)  (via service)
func importersTreeRich() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/app/main.ts": {
			{ID: "/repo/src/service.ts", Request: "../service", ResolvedType: UserModule},
		},
		"/repo/src/service.ts": {
			{ID: "/repo/legacy/core.ts", Request: "../../legacy/core", ResolvedType: UserModule},
			{Request: "moment", ResolvedType: NodeModule},
		},
		"/repo/src/admin/main.ts": {
			{ID: "/repo/legacy/core.ts", Request: "../../legacy/core", ResolvedType: UserModule},
		},
		"/repo/scripts/orphan.ts": {
			{ID: "/repo/legacy/core.ts", Request: "../legacy/core", ResolvedType: UserModule},
		},
		"/repo/src/types/main.ts": {
			{ID: "/repo/legacy/core.ts", Request: "../../legacy/core", ResolvedType: UserModule, ImportKind: OnlyTypeImport},
		},
		"/repo/legacy/core.ts": {},
	}
}

func v(entryPoint, file string) RestrictedImporterViolation {
	return RestrictedImporterViolation{EntryPoint: entryPoint, File: file}
}

func vModule(entryPoint, moduleName string) RestrictedImporterViolation {
	return RestrictedImporterViolation{EntryPoint: entryPoint, Module: moduleName}
}

var restrictedImportersScenarios = []struct {
	name            string
	tree            MinimalDependencyTree
	opts            *rules.RestrictedImportersDetectionOptions
	ruleEntryPoints []string
	want            []RestrictedImporterViolation
}{
	{
		name: "allowlist_transitive_and_unreachable_importer",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"src/admin/**"},
		},
		// app reaches legacy transitively and is not allowed; admin is allowed; orphan is unreachable.
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{v("/repo/src/app/main.ts", "/repo/legacy/core.ts")},
	},
	{
		name: "ignore_matches",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"src/admin/**"},
			IgnoreMatches:      []string{"src/app/**"},
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{},
	},
	{
		name: "type_import_ignored",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"does/not/match/**"},
			IgnoreTypeImports:  true,
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/types/main.ts"},
		want:            []RestrictedImporterViolation{v("/repo/src/app/main.ts", "/repo/legacy/core.ts")},
	},
	{
		name: "type_import_counted",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"does/not/match/**"},
			IgnoreTypeImports:  false,
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/types/main.ts"},
		want: []RestrictedImporterViolation{
			v("/repo/src/app/main.ts", "/repo/legacy/core.ts"),
			v("/repo/src/types/main.ts", "/repo/legacy/core.ts"),
		},
	},
	{
		name: "graph_exclude_breaks_transitive_path",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"src/admin/**"},
			GraphExclude:       []string{"src/service.ts"},
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{},
	},
	{
		name: "module_transitive",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Modules:            []string{"moment"},
			AllowedEntryPoints: []string{"src/admin/**"},
		},
		// app reaches moment through service; admin does not import moment at all.
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{vModule("/repo/src/app/main.ts", "moment")},
	},
	{
		name: "module_allowlisted_entry_ignored",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Modules:            []string{"moment"},
			AllowedEntryPoints: []string{"src/app/**"},
		},
		// only app reaches moment, and app is allowlisted -> nothing.
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{},
	},
	{
		name: "files_and_modules_combined",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            true,
			Files:              []string{"legacy/**"},
			Modules:            []string{"mom*"},
			AllowedEntryPoints: []string{"src/admin/**"},
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		// app reaches both the target file and the target module; admin reaches legacy directly but is
		// allowlisted. Sorted by entryPoint, then file (empty sorts first), then module -> the module
		// violation (empty file) comes before the file violation.
		want: []RestrictedImporterViolation{
			vModule("/repo/src/app/main.ts", "moment"),
			v("/repo/src/app/main.ts", "/repo/legacy/core.ts"),
		},
	},
	{
		name: "no_allowlist_yields_nothing",
		tree: importersTreeRich(),
		// No allowlist -> nothing to enforce (validation also requires it; the check is defensive).
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled: true,
			Files:   []string{"legacy/**"},
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{},
	},
	{
		name: "disabled_yields_nothing",
		tree: importersTreeRich(),
		opts: &rules.RestrictedImportersDetectionOptions{
			Enabled:            false,
			Files:              []string{"legacy/**"},
			AllowedEntryPoints: []string{"src/admin/**"},
		},
		ruleEntryPoints: []string{"src/app/main.ts", "src/admin/main.ts"},
		want:            []RestrictedImporterViolation{},
	},
}

func TestFindRestrictedImporters_Scenarios(t *testing.T) {
	for _, sc := range restrictedImportersScenarios {
		t.Run(sc.name, func(t *testing.T) {
			got := FindRestrictedImporters(sc.tree, sc.opts, "/repo", sc.ruleEntryPoints)
			if !reflect.DeepEqual(got, sc.want) {
				t.Errorf("got %+v, want %+v", got, sc.want)
			}
		})
	}
}
