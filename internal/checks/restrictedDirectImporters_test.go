package checks

import (
	"reflect"
	"testing"

	"rev-dep-go/internal/rules"
)

// directImportersTree covers the behaviors FindRestrictedDirectImporters must get right. Direct
// importers of config/secret.ts are reader.ts (value), leak.ts (value), proxy.ts (value) and
// typing.ts (type-only). consumer.ts imports proxy.ts, so it reaches secret ONLY transitively and
// must never be reported. client.ts directly imports the "axios" node module.
//
//	src/allowed/reader.ts -> config/secret.ts            (direct, value)
//	src/feature/leak.ts   -> config/secret.ts            (direct, value)
//	src/deep/proxy.ts     -> config/secret.ts            (direct, value)
//	src/deep/consumer.ts  -> src/deep/proxy.ts           (transitive to secret, NOT direct)
//	src/types/typing.ts   -> config/secret.ts            (direct, type-only)
//	src/net/client.ts     -> axios (node module)         (direct module import)
func directImportersTree() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/allowed/reader.ts": {
			{ID: "/repo/config/secret.ts", Request: "../../config/secret", ResolvedType: UserModule},
		},
		"/repo/src/feature/leak.ts": {
			{ID: "/repo/config/secret.ts", Request: "../../config/secret", ResolvedType: UserModule},
		},
		"/repo/src/deep/proxy.ts": {
			{ID: "/repo/config/secret.ts", Request: "../../config/secret", ResolvedType: UserModule},
		},
		"/repo/src/deep/consumer.ts": {
			{ID: "/repo/src/deep/proxy.ts", Request: "./proxy", ResolvedType: UserModule},
		},
		"/repo/src/types/typing.ts": {
			{ID: "/repo/config/secret.ts", Request: "../../config/secret", ResolvedType: UserModule, ImportKind: OnlyTypeImport},
		},
		"/repo/src/net/client.ts": {
			{Request: "axios", ResolvedType: NodeModule},
		},
		"/repo/config/secret.ts": {},
	}
}

func dv(importer, file string) RestrictedDirectImporterViolation {
	return RestrictedDirectImporterViolation{ViolationType: "file", ImporterFile: importer, File: file}
}

func dvModule(importer, moduleName, request string) RestrictedDirectImporterViolation {
	return RestrictedDirectImporterViolation{ViolationType: "module", ImporterFile: importer, Module: moduleName, ImportRequest: request}
}

const secret = "/repo/config/secret.ts"

var restrictedDirectImportersScenarios = []struct {
	name string
	tree MinimalDependencyTree
	opts *rules.RestrictedDirectImportersDetectionOptions
	want []RestrictedDirectImporterViolation
}{
	{
		name: "allowlist_reports_disallowed_direct_importers",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:           true,
			Files:             []string{"config/**"},
			AllowImporters:    []string{"src/allowed/**"},
			IgnoreTypeImports: true,
		},
		// Direct value importers: reader (allowed), leak, proxy. typing is type-only (ignored);
		// consumer reaches secret only transitively.
		want: []RestrictedDirectImporterViolation{
			dv("/repo/src/deep/proxy.ts", secret),
			dv("/repo/src/feature/leak.ts", secret),
		},
	},
	{
		name: "transitive_importer_never_reported_even_when_it_matches_deny",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:           true,
			Files:             []string{"config/**"},
			DenyImporters:     []string{"src/deep/**"}, // matches proxy.ts AND consumer.ts
			IgnoreTypeImports: true,
		},
		// consumer.ts matches the deny pattern but imports secret only transitively -> not reported.
		want: []RestrictedDirectImporterViolation{dv("/repo/src/deep/proxy.ts", secret)},
	},
	{
		name: "denylist_reports_matching_direct_importer",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:       true,
			Files:         []string{"config/**"},
			DenyImporters: []string{"src/feature/**"},
		},
		want: []RestrictedDirectImporterViolation{dv("/repo/src/feature/leak.ts", secret)},
	},
	{
		name: "denylist_no_match_yields_nothing",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:       true,
			Files:         []string{"config/**"},
			DenyImporters: []string{"does/not/match/**"},
		},
		want: []RestrictedDirectImporterViolation{},
	},
	{
		name: "type_import_ignored",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:           true,
			Files:             []string{"config/**"},
			DenyImporters:     []string{"src/**"},
			IgnoreTypeImports: true,
		},
		// typing.ts is type-only -> ignored; the three value importers are reported.
		want: []RestrictedDirectImporterViolation{
			dv("/repo/src/allowed/reader.ts", secret),
			dv("/repo/src/deep/proxy.ts", secret),
			dv("/repo/src/feature/leak.ts", secret),
		},
	},
	{
		name: "type_import_counted",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:           true,
			Files:             []string{"config/**"},
			DenyImporters:     []string{"src/types/**"},
			IgnoreTypeImports: false,
		},
		want: []RestrictedDirectImporterViolation{dv("/repo/src/types/typing.ts", secret)},
	},
	{
		name: "ignore_matches_suppresses_importer",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:           true,
			Files:             []string{"config/**"},
			DenyImporters:     []string{"src/**"},
			IgnoreMatches:     []string{"src/deep/**"},
			IgnoreTypeImports: true,
		},
		// proxy.ts suppressed by ignoreMatches; typing.ts is type-only.
		want: []RestrictedDirectImporterViolation{
			dv("/repo/src/allowed/reader.ts", secret),
			dv("/repo/src/feature/leak.ts", secret),
		},
	},
	{
		name: "module_target_denylist",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:       true,
			Modules:       []string{"axios"},
			DenyImporters: []string{"src/net/**"},
		},
		want: []RestrictedDirectImporterViolation{dvModule("/repo/src/net/client.ts", "axios", "axios")},
	},
	{
		name: "module_target_allowlist_allows_importer",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:        true,
			Modules:        []string{"ax*"},
			AllowImporters: []string{"src/net/**"},
		},
		want: []RestrictedDirectImporterViolation{},
	},
	{
		name: "disabled_yields_nothing",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled:       false,
			Files:         []string{"config/**"},
			DenyImporters: []string{"src/**"},
		},
		want: []RestrictedDirectImporterViolation{},
	},
	{
		name: "no_policy_yields_nothing",
		tree: directImportersTree(),
		opts: &rules.RestrictedDirectImportersDetectionOptions{
			Enabled: true,
			Files:   []string{"config/**"},
		},
		want: []RestrictedDirectImporterViolation{},
	},
}

func TestFindRestrictedDirectImporters_Scenarios(t *testing.T) {
	for _, sc := range restrictedDirectImportersScenarios {
		t.Run(sc.name, func(t *testing.T) {
			got := FindRestrictedDirectImporters(sc.tree, sc.opts, "/repo")
			if !reflect.DeepEqual(got, sc.want) {
				t.Errorf("got %+v, want %+v", got, sc.want)
			}
		})
	}
}

// globMatchingTree exercises glob selectivity of every path/module field:
//   - user.entity.ts matches `**/*.entity.ts`; user.service.ts must NOT match.
//   - db/client.ts imports @legacy/orm (matches `@legacy/*`) and @modern/orm (must NOT match).
func globMatchingTree() MinimalDependencyTree {
	return MinimalDependencyTree{
		"/repo/src/api/handler.ts": {
			{ID: "/repo/src/gen/user.entity.ts", Request: "../gen/user.entity", ResolvedType: UserModule},
			{ID: "/repo/src/gen/user.service.ts", Request: "../gen/user.service", ResolvedType: UserModule},
		},
		"/repo/src/lib/util.ts": {
			{ID: "/repo/src/gen/user.entity.ts", Request: "../gen/user.entity", ResolvedType: UserModule},
		},
		"/repo/src/db/client.ts": {
			{Request: "@legacy/orm", ResolvedType: NodeModule},
			{Request: "@modern/orm", ResolvedType: NodeModule},
		},
		"/repo/src/gen/user.entity.ts":  {},
		"/repo/src/gen/user.service.ts": {},
	}
}

func TestFindRestrictedDirectImporters_GlobMatching(t *testing.T) {
	entity := "/repo/src/gen/user.entity.ts"

	cases := []struct {
		name string
		opts *rules.RestrictedDirectImportersDetectionOptions
		want []RestrictedDirectImporterViolation
	}{
		{
			// `files` glob selects only *.entity.ts; user.service.ts (imported by handler) is not a target.
			name: "files_glob_selects_targets",
			opts: &rules.RestrictedDirectImportersDetectionOptions{
				Enabled:       true,
				Files:         []string{"**/*.entity.ts"},
				DenyImporters: []string{"src/**"},
			},
			want: []RestrictedDirectImporterViolation{
				dv("/repo/src/api/handler.ts", entity),
				dv("/repo/src/lib/util.ts", entity),
			},
		},
		{
			// `allowImporters` glob whitelists src/api/**; src/lib/util.ts is the disallowed importer.
			name: "allowImporters_glob",
			opts: &rules.RestrictedDirectImportersDetectionOptions{
				Enabled:        true,
				Files:          []string{"**/*.entity.ts"},
				AllowImporters: []string{"src/api/**"},
			},
			want: []RestrictedDirectImporterViolation{dv("/repo/src/lib/util.ts", entity)},
		},
		{
			// `denyImporters` glob targets only src/lib/**.
			name: "denyImporters_glob",
			opts: &rules.RestrictedDirectImportersDetectionOptions{
				Enabled:       true,
				Files:         []string{"**/*.entity.ts"},
				DenyImporters: []string{"src/lib/**"},
			},
			want: []RestrictedDirectImporterViolation{dv("/repo/src/lib/util.ts", entity)},
		},
		{
			// `modules` scoped glob matches @legacy/orm but not @modern/orm.
			name: "modules_scoped_glob",
			opts: &rules.RestrictedDirectImportersDetectionOptions{
				Enabled:       true,
				Modules:       []string{"@legacy/*"},
				DenyImporters: []string{"src/**"},
			},
			want: []RestrictedDirectImporterViolation{dvModule("/repo/src/db/client.ts", "@legacy/orm", "@legacy/orm")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FindRestrictedDirectImporters(globMatchingTree(), tc.opts, "/repo")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
