package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/pathutil"
)

// fold is a test helper: fold entryFiles (a subset of allFiles) into patterns using the strict
// (100%) coverage rule with no blocked set.
func fold(allFiles, entryFiles []string) []string {
	entrySet := map[string]bool{}
	for _, entryFile := range entryFiles {
		entrySet[entryFile] = true
	}
	return foldEntryPatterns(allFiles, entrySet, nil, 100)
}

func TestFoldEntryPatterns(t *testing.T) {
	cases := []struct {
		name     string
		allFiles []string
		entries  []string
		want     []string
	}{
		{
			name:     "empty entry set",
			allFiles: []string{"a.ts", "b.ts"},
			entries:  nil,
			want:     nil,
		},
		{
			name:     "single root file",
			allFiles: []string{"index.ts", "lib.ts"},
			entries:  []string{"index.ts"},
			want:     []string{"index.ts"},
		},
		{
			name:     "some root files are entries",
			allFiles: []string{"a.ts", "b.ts", "c.ts"},
			entries:  []string{"a.ts", "c.ts"},
			want:     []string{"a.ts", "c.ts"},
		},
		{
			name:     "directory fully covered folds to dir/**",
			allFiles: []string{"pages/home.ts", "pages/about.ts"},
			entries:  []string{"pages/home.ts", "pages/about.ts"},
			want:     []string{"pages/**"},
		},
		{
			name:     "directory partially covered lists files",
			allFiles: []string{"pages/home.ts", "pages/_app.ts"},
			entries:  []string{"pages/home.ts"},
			want:     []string{"pages/home.ts"},
		},
		{
			name:     "nested fully covered folds at the top",
			allFiles: []string{"pages/home.ts", "pages/blog/index.ts", "pages/blog/post.ts"},
			entries:  []string{"pages/home.ts", "pages/blog/index.ts", "pages/blog/post.ts"},
			want:     []string{"pages/**"},
		},
		{
			name:     "deep nesting all covered folds to the top dir",
			allFiles: []string{"app/a.ts", "app/b/c.ts", "app/b/d/e.ts"},
			entries:  []string{"app/a.ts", "app/b/c.ts", "app/b/d/e.ts"},
			want:     []string{"app/**"},
		},
		{
			name:     "parent has a non-entry file but subdirs fold independently",
			allFiles: []string{"a/b/c.ts", "a/b/d.ts", "a/e.ts", "a/f.ts"},
			entries:  []string{"a/b/c.ts", "a/b/d.ts", "a/e.ts"},
			want:     []string{"a/b/**", "a/e.ts"},
		},
		{
			name:     "directory with only covered subdirs folds",
			allFiles: []string{"pages/a/x.ts", "pages/b/y.ts"},
			entries:  []string{"pages/a/x.ts", "pages/b/y.ts"},
			want:     []string{"pages/**"},
		},
		{
			name:     "directory with only-covered and one-uncovered subdir",
			allFiles: []string{"pages/a/x.ts", "pages/b/y.ts", "pages/b/z.ts"},
			entries:  []string{"pages/a/x.ts", "pages/b/y.ts"}, // pages/b/z.ts not an entry
			want:     []string{"pages/a/**", "pages/b/y.ts"},
		},
		{
			name:     "two separate top-level dirs both fold",
			allFiles: []string{"pages/a.ts", "app/b.ts"},
			entries:  []string{"pages/a.ts", "app/b.ts"},
			want:     []string{"app/**", "pages/**"},
		},
		{
			name:     "root never folds to bare ** even when everything is an entry",
			allFiles: []string{"a.ts", "pages/b.ts", "pages/c.ts"},
			entries:  []string{"a.ts", "pages/b.ts", "pages/c.ts"},
			want:     []string{"a.ts", "pages/**"},
		},
		{
			name:     "non-entry files inside a folded dir prevent folding",
			allFiles: []string{"pages/a.ts", "pages/b.ts", "pages/sub/keep.ts"},
			entries:  []string{"pages/a.ts", "pages/b.ts"}, // pages/sub/keep.ts not an entry
			want:     []string{"pages/a.ts", "pages/b.ts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fold(tc.allFiles, tc.entries)
			if !slices.Equal(got, tc.want) {
				t.Errorf("fold(%v, %v)\n  got  %v\n  want %v", tc.allFiles, tc.entries, got, tc.want)
			}
		})
	}
}

// setOf builds a membership set from paths.
func setOf(paths ...string) map[string]bool {
	set := map[string]bool{}
	for _, path := range paths {
		set[path] = true
	}
	return set
}

func TestFoldEntryPatterns_CoverageThreshold(t *testing.T) {
	// A directory of 10 files where 9 are entry points (90% covered).
	allFiles := []string{
		"pages/a.ts", "pages/b.ts", "pages/c.ts", "pages/d.ts", "pages/e.ts",
		"pages/f.ts", "pages/g.ts", "pages/h.ts", "pages/i.ts", "pages/internal.ts",
	}
	entries := setOf("pages/a.ts", "pages/b.ts", "pages/c.ts", "pages/d.ts", "pages/e.ts",
		"pages/f.ts", "pages/g.ts", "pages/h.ts", "pages/i.ts")

	if got := foldEntryPatterns(allFiles, entries, nil, 90); !slices.Equal(got, []string{"pages/**"}) {
		t.Errorf("at 90%% coverage expected [pages/**], got %v", got)
	}
	// At 95% the directory is not covered enough — every entry file is listed instead.
	got95 := foldEntryPatterns(allFiles, entries, nil, 95)
	if slices.Contains(got95, "pages/**") || len(got95) != 9 {
		t.Errorf("at 95%% expected 9 individual files, got %v", got95)
	}
	// 100% reproduces the strict rule: still expanded.
	if got100 := foldEntryPatterns(allFiles, entries, nil, 100); len(got100) != 9 {
		t.Errorf("at 100%% expected 9 individual files, got %v", got100)
	}
}

func TestFoldEntryPatterns_BlockedPreventsFold(t *testing.T) {
	// 9 prod entries plus one file from another set (a test). Even though the prod files alone are
	// 100% of the non-blocked files, the blocked file forbids folding the directory into prod/**.
	allFiles := []string{
		"d/1.ts", "d/2.ts", "d/3.ts", "d/4.ts", "d/5.ts",
		"d/6.ts", "d/7.ts", "d/8.ts", "d/9.ts", "d/x.test.ts",
	}
	prod := setOf("d/1.ts", "d/2.ts", "d/3.ts", "d/4.ts", "d/5.ts", "d/6.ts", "d/7.ts", "d/8.ts", "d/9.ts")
	blocked := setOf("d/x.test.ts")

	if got := foldEntryPatterns(allFiles, prod, blocked, 80); slices.Contains(got, "d/**") {
		t.Errorf("blocked file should prevent d/** fold, got %v", got)
	}
	// Without the block, the same 9/10 ratio folds at 80%.
	if got := foldEntryPatterns(allFiles, prod, nil, 80); !slices.Equal(got, []string{"d/**"}) {
		t.Errorf("without block expected [d/**] at 80%%, got %v", got)
	}
}

func TestFoldableDirs_ReportsCoverage(t *testing.T) {
	allFiles := []string{"pages/a.ts", "pages/b.ts", "pages/c.ts", "pages/d.ts", "pages/internal.ts"}
	entries := setOf("pages/a.ts", "pages/b.ts", "pages/c.ts", "pages/d.ts") // 4/5 = 80%

	folded := foldableDirs(allFiles, entries, nil, 80)
	if len(folded) != 1 || folded[0].dir != "pages" || folded[0].coverage != 80 {
		t.Errorf("expected [{pages 80}], got %v", folded)
	}
	if folded := foldableDirs(allFiles, entries, nil, 85); len(folded) != 0 {
		t.Errorf("expected no folds at 85%%, got %v", folded)
	}
}

func TestFoldThresholdOptions_DedupAndSkip(t *testing.T) {
	// pages: 24/25 prod entries = 96% (folds at 95, not 100).
	// tests: 22/25 dev entries = 88% (folds at 85, not 90).
	universe := []string{}
	prod := map[string]bool{}
	dev := map[string]bool{}
	for i := 0; i < 25; i++ {
		pagesFile := fmt.Sprintf("pages/f%02d.ts", i)
		testsFile := fmt.Sprintf("tests/g%02d.ts", i)
		universe = append(universe, pagesFile, testsFile)
		if i < 24 {
			prod[pagesFile] = true
		}
		if i < 22 {
			dev[testsFile] = true
		}
	}
	analysis := packageEntryAnalysis{universe: universe, prodSet: prod, devSet: dev, ignoreSet: map[string]bool{}}

	options := foldThresholdOptions([]analyzedPackage{{rulePath: ".", analysis: analysis}})

	// 95 unlocks pages; 90 unlocks nothing new (skipped); 85 unlocks tests; 80 nothing new (skipped).
	if len(options) != 2 {
		t.Fatalf("expected 2 threshold options (95, 85), got %d: %+v", len(options), options)
	}
	if options[0].threshold != 95 || len(options[0].newDirs) != 1 || options[0].newDirs[0].dir != "pages" || options[0].newDirs[0].coverage != 96 {
		t.Errorf("option 0: expected 95%% -> pages@96, got %+v", options[0])
	}
	if options[1].threshold != 85 || len(options[1].newDirs) != 1 || options[1].newDirs[0].dir != "tests" || options[1].newDirs[0].coverage != 88 {
		t.Errorf("option 1: expected 85%% -> tests@88, got %+v", options[1])
	}
}

func TestFoldThresholdOptions_NoneWhenFullyStrict(t *testing.T) {
	// A directory that is either fully covered or well below any threshold — nothing to offer.
	analysis := packageEntryAnalysis{
		universe:  []string{"src/index.ts", "src/a.ts", "src/b.ts", "src/c.ts"},
		prodSet:   setOf("src/index.ts"), // 1/4 = 25%, far below 80%
		devSet:    map[string]bool{},
		ignoreSet: map[string]bool{},
	}
	if options := foldThresholdOptions([]analyzedPackage{{rulePath: ".", analysis: analysis}}); len(options) != 0 {
		t.Errorf("expected no options for a low-coverage dir, got %+v", options)
	}
}

func TestJoinPkgDir(t *testing.T) {
	cases := []struct{ rulePath, dir, want string }{
		{".", "pages", "pages"},
		{"", "pages", "pages"},
		{"apps/web", "pages", "apps/web/pages"},
		{"apps/web", "", "apps/web"},
	}
	for _, tc := range cases {
		if got := joinPkgDir(tc.rulePath, tc.dir); got != tc.want {
			t.Errorf("joinPkgDir(%q, %q) = %q, want %q", tc.rulePath, tc.dir, got, tc.want)
		}
	}
}

func TestClassifyEntryPoint(t *testing.T) {
	dev := []string{
		"index.test.ts",
		"src/Button.spec.tsx",
		"components/Card.stories.tsx",
		"bench/x.bench.ts",
		"e2e/login.e2e.ts",
		"cypress/support/commands.cy.ts",
		"test/helper.ts",
		"src/__tests__/a.ts",
		"scripts/build.ts",
		"packages/x/scripts/gen.ts",
		"benchmarks/run.ts",
		"__mocks__/fs.ts",
		".storybook/main.ts",
		"examples/demo/app.ts",
		"vite.config.ts",  // *.config.*
		"jest.config.js",  // *.config.*
		"next.config.mjs", // *.config.*
		"types.d.ts",      // *.d.ts
		"src/global.d.ts", // nested *.d.ts
	}
	ignore := []string{
		"fixtures/data.ts",
		"__fixtures__/sample.ts",
		"src/fixtures/user.ts",
		"test/fixtures/x.ts",            // ignore (fixtures) takes precedence over dev (test)
		"__fixtures__/types.d.ts",       // .d.ts inside fixtures still ignored
		"__snapshots__/a.ts",            // snapshots are ignored
		"components/__snapshots__/b.ts", // nested snapshots
		"snapshots/c.ts",
	}
	prod := []string{
		"index.ts",
		"src/server.ts",
		"src/pages/home.tsx",
		"lib/util.ts",
		"test-utils/render.ts", // "test-utils" is not the exact segment "test"
		"src/latest.ts",        // contains "test" substring but not ".test." nor a dev segment
		"contest/main.ts",      // "contest" is not "test"
	}
	for _, p := range dev {
		if got := classifyEntryPoint(p); got != entryDev {
			t.Errorf("expected %q to be DEV, got %d", p, got)
		}
	}
	for _, p := range ignore {
		if got := classifyEntryPoint(p); got != entryIgnore {
			t.Errorf("expected %q to be IGNORE, got %d", p, got)
		}
	}
	for _, p := range prod {
		if got := classifyEntryPoint(p); got != entryProd {
			t.Errorf("expected %q to be PRODUCTION, got %d", p, got)
		}
	}
}

func TestCollapseEntryPatterns(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "scattered test files collapse per suffix",
			in: []string{
				"app/a/foo.server.test.ts",
				"app/b/bar.client.test.ts",
				"app/c/baz.test.ts",
			},
			want: []string{"**/*.test.ts"},
		},
		{
			name: "different extensions collapse independently",
			in: []string{
				"app/a/x.test.ts",
				"app/b/y.test.ts",
				"app/c/Widget.test.tsx",
				"app/d/Panel.test.tsx",
			},
			want: []string{"**/*.test.ts", "**/*.test.tsx"},
		},
		{
			name: "any compound suffix collapses, not just classifier markers",
			in: []string{
				"ui/A.stories.tsx",
				"ui/B.stories.tsx",
				"jest.config.js",
				"ecosystem.config.js",
				"a/x.fixture.ts", // not a classifier marker, still collapses on suffix
				"b/y.fixture.ts",
			},
			want: []string{"**/*.config.js", "**/*.fixture.ts", "**/*.stories.tsx"},
		},
		{
			name: "a lone suffix or lone dir glob stays as-is",
			in:   []string{"index.test.ts", "scripts/**"},
			want: []string{"index.test.ts", "scripts/**"},
		},
		{
			name: "recognized dir globs collapse to **/<leaf>/**",
			in: []string{
				"app/broadcast/mocks/**",
				"app/chat/services/mocks/**",
				"app/notes/mocks/**",
				"x/y/__tests__/**",
				"a/b/__tests__/**",
			},
			want: []string{"**/__tests__/**", "**/mocks/**"},
		},
		{
			name: "unrecognized dir leaf and non-compound files pass through",
			in: []string{
				"app/chat/webhook/**", // "webhook" not a recognized dir name
				"app/retail/webhook/**",
				"**/*.d.ts",
				"cypress/utils.ts", // no compound suffix
				"a/x.test.ts",
				"b/y.test.ts",
			},
			want: []string{
				"**/*.d.ts", "**/*.test.ts",
				"app/chat/webhook/**", "app/retail/webhook/**",
				"cypress/utils.ts",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := collapseEntryPatterns(tc.in, devEntryDirNames, true)
			if !slices.Equal(got, tc.want) {
				t.Errorf("collapseEntryPatterns(%v)\n  got  %v\n  want %v", tc.in, got, tc.want)
			}
		})
	}
}

// With file collapse disabled (the ignore set), directory globs still collapse but file suffixes
// must not — a stray ".test.ts" in a fixtures dir may not broaden into "**/*.test.ts".
func TestCollapseEntryPatterns_FilesDisabled(t *testing.T) {
	in := []string{
		"src/__snapshots__/**",
		"lib/__snapshots__/**",
		"__fixtures__/a.test.ts",
		"__fixtures__/b.test.ts",
	}
	want := []string{"**/__snapshots__/**", "__fixtures__/a.test.ts", "__fixtures__/b.test.ts"}
	got := collapseEntryPatterns(in, ignoreEntryDirNames, false)
	if !slices.Equal(got, want) {
		t.Errorf("collapseEntryPatterns(files disabled)\n  got  %v\n  want %v", got, want)
	}
}

// End-to-end: test files scattered across production directories collapse to one **/*.test.ts glob
// instead of one entry per file.
func TestDetectPackageEntryPoints_ScatteredTestsCollapse(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-tests")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"p"}`)
	// Production code interleaved with tests, so no directory is fully covered by tests.
	write("app/account/settings.ts", `export const s=1;`)
	write("app/account/settings.server.test.ts", `export const t=1;`)
	write("app/ats/service.ts", `export const s=1;`)
	write("app/ats/service.server.test.ts", `export const t=1;`)
	write("utils/date.ts", `export const d=1;`)
	write("utils/date.test.ts", `export const t=1;`)

	_, dev, _ := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))
	if !slices.Equal(dev, []string{"**/*.test.ts"}) {
		t.Errorf("expected scattered tests to collapse to [**/*.test.ts], got %v", dev)
	}
}

// Integration: entry-point detection wired through initConfigInteractive populates the rule.
func TestInitConfigInteractive_EntryPointsPopulated(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-wire")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"app"}`)
	write("index.ts", `import './src/util';`)
	write("src/util.ts", `export const u=1;`)
	write("pages/home.ts", `export const h=1;`)
	write("pages/about.ts", `export const a=1;`)
	write("index.test.ts", `export const t=1;`)
	write("fixtures/sample.ts", `export const s=1;`)

	yes := func() bool { return true }
	result, err := initConfigInteractive(dir,
		func(projectStructure, standalonePackages) standaloneSelection { return standaloneAll },
		yes,
		func() detectorPreset { return detectorsScaffold },
		strictFoldThreshold)
	if err != nil {
		t.Fatalf("initConfigInteractive: %v", err)
	}

	if !result.entryPointsDetected || result.entryPointPackageCount != 1 {
		t.Fatalf("expected entry-point detection on 1 package, got detected=%v count=%d", result.entryPointsDetected, result.entryPointPackageCount)
	}
	rule := result.rules[0]
	if !slices.Equal(rule.ProdEntryPoints, []string{"index.ts", "pages/**"}) {
		t.Errorf("prod entry points: got %v", rule.ProdEntryPoints)
	}
	if !slices.Equal(rule.DevEntryPoints, []string{"index.test.ts"}) {
		t.Errorf("dev entry points: got %v", rule.DevEntryPoints)
	}
	if !slices.Equal(rule.IgnoreEntryPoints, []string{"fixtures/**"}) {
		t.Errorf("ignore entry points: got %v", rule.IgnoreEntryPoints)
	}
}

// Integration: the fold-threshold question is asked after the entry-points question and before the
// detectors question, and the chosen threshold is applied to the generated rule.
func TestInitConfigInteractive_FoldPromptOrderAndApplied(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-fold")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"app"}`)
	// pages/: 9 entry points + 1 imported (non-entry) = 90% covered, so it only folds below 100%.
	write("pages/index.tsx", `import './_app';`)
	write("pages/_app.tsx", `export const a=1;`) // imported -> not an entry point
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		write("pages/"+name+".tsx", `export const x=1;`)
	}

	var order []string
	askStandalone := func(projectStructure, standalonePackages) standaloneSelection { return standaloneAll }
	askEntry := func() bool { order = append(order, "entry"); return true }
	askDetectors := func() detectorPreset { order = append(order, "detectors"); return detectorsScaffold }
	askFold := func(options []foldThresholdOption) int {
		order = append(order, "fold")
		if len(options) == 0 {
			t.Fatal("expected at least one fold-threshold option")
		}
		return options[0].threshold // accept the mildest offered fold
	}

	result, err := initConfigInteractive(dir, askStandalone, askEntry, askDetectors, askFold)
	if err != nil {
		t.Fatalf("initConfigInteractive: %v", err)
	}

	if !slices.Equal(order, []string{"entry", "fold", "detectors"}) {
		t.Errorf("prompt order = %v, want [entry fold detectors]", order)
	}
	if !slices.Equal(result.rules[0].ProdEntryPoints, []string{"pages/**"}) {
		t.Errorf("expected pages/** after folding at the chosen threshold, got %v", result.rules[0].ProdEntryPoints)
	}
}

// Integration: the monorepo root rule is skipped; only workspace packages get entry points.
func TestInitConfigInteractive_EntryPointsSkipMonorepoRoot(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-mono")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"root","workspaces":["packages/*"]}`)
	write("packages/api/package.json", `{"name":"@m/api"}`)
	write("packages/api/index.ts", `import './lib';`)
	write("packages/api/lib.ts", `export const x=1;`)

	result, err := initConfigInteractive(dir,
		func(projectStructure, standalonePackages) standaloneSelection { return standaloneAll },
		func() bool { return true },
		func() detectorPreset { return detectorsScaffold },
		strictFoldThreshold)
	if err != nil {
		t.Fatalf("initConfigInteractive: %v", err)
	}

	// rules[0] = monorepo root "." (boundaries only, skipped), rules[1] = packages/api.
	if result.rules[0].Path != "." || len(result.rules[0].ProdEntryPoints) != 0 || len(result.rules[0].DevEntryPoints) != 0 {
		t.Errorf("monorepo root rule should have no entry points, got prod=%v dev=%v", result.rules[0].ProdEntryPoints, result.rules[0].DevEntryPoints)
	}
	var apiRule *config.Rule
	for i := range result.rules {
		if result.rules[i].Path == "packages/api" {
			apiRule = &result.rules[i]
		}
	}
	if apiRule == nil {
		t.Fatalf("expected a rule for packages/api, rules: %v", rulePaths(result.rules))
	}
	if !slices.Equal(apiRule.ProdEntryPoints, []string{"index.ts"}) {
		t.Errorf("packages/api prod entry points: got %v", apiRule.ProdEntryPoints)
	}
	if result.entryPointPackageCount != 1 {
		t.Errorf("expected 1 package processed (root skipped), got %d", result.entryPointPackageCount)
	}
}

// End-to-end: build a small package on disk and detect + classify + fold its entry points.
func TestDetectPackageEntryPoints_EndToEnd(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)

	writeFile := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	writeFile("package.json", `{"name":"pkg"}`)
	writeFile("index.ts", `import './lib';`)                     // entry (no importer) -> prod
	writeFile("lib.ts", `export const x = 1;`)                   // imported by index -> not an entry
	writeFile("pages/home.ts", `export const h=1;`)              // entry -> prod, folds
	writeFile("pages/about.ts", `export const a=1;`)             // entry -> prod, folds
	writeFile("scripts/build.ts", `export const b=1;`)           // entry -> dev (scripts/)
	writeFile("index.test.ts", `export const t=1;`)              // entry -> dev (.test.)
	writeFile("__fixtures__/a.ts", `import './helper';`)         // entry -> ignore (fixtures)
	writeFile("__fixtures__/helper.ts", `export const g=1;`)     // imported sibling -> NOT an entry, still ignore
	writeFile("__fixtures__/vendor/lib.ts", `export const v=1;`) // entry -> ignore

	prod, dev, ignore := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))

	wantProd := []string{"index.ts", "pages/**"}
	wantDev := []string{"index.test.ts", "scripts/**"}
	// The whole fixtures tree folds despite the imported (non-entry) helper and the nested vendor dir.
	wantIgnore := []string{"__fixtures__/**"}
	if !slices.Equal(prod, wantProd) {
		t.Errorf("prod entry points:\n  got  %v\n  want %v", prod, wantProd)
	}
	if !slices.Equal(dev, wantDev) {
		t.Errorf("dev entry points:\n  got  %v\n  want %v", dev, wantDev)
	}
	if !slices.Equal(ignore, wantIgnore) {
		t.Errorf("ignore entry points:\n  got  %v\n  want %v", ignore, wantIgnore)
	}
}

// Regression for the real internal-usage-analyzer/test/fixtures case: a fixtures directory whose
// files import each other (so most are NOT entry points) plus a nested vendor dir with non-source
// files must still fold into a single "test/fixtures/**" glob, not one entry per file.
func TestDetectPackageEntryPoints_FixturesFoldWholeTree(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-fix")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"p"}`)
	// A prod entry so the package isn't fixtures-only.
	write("src/index.ts", `export const i=1;`)
	// Real test/ layout: test files live next to the fixtures dir, so test/ is mixed.
	write("test/analyze.test.ts", `export const a=1;`)
	write("test/regressions.test.ts", `export const r=1;`)
	// Fixtures that import each other: helper/private are imported -> not entry points.
	write("test/fixtures/class-uses-helper.ts", `import './internal-helper'; import './private-helper';`)
	write("test/fixtures/internal-helper.ts", `export const h=1;`)
	write("test/fixtures/private-helper.ts", `export const p=1;`)
	write("test/fixtures/standalone.ts", `export const s=1;`)
	// Nested vendor dir with non-source files alongside a source file.
	write("test/fixtures/vendor/redux/combineReducers.ts", `export const c=1;`)
	write("test/fixtures/vendor/redux/LICENSE", `MIT`)
	write("test/fixtures/vendor/README.md", `# vendor`)

	_, dev, ignore := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))
	// The whole fixtures tree folds despite imported (non-entry) helpers and the nested vendor dir.
	if !slices.Equal(ignore, []string{"test/fixtures/**"}) {
		t.Errorf("expected fixtures to fold to [test/fixtures/**], got %v", ignore)
	}
	// The sibling test files are dev (and test/ does not over-fold to test/** because it contains
	// the ignored fixtures subtree); sharing the .test.ts suffix, they collapse to one glob.
	if !slices.Equal(dev, []string{"**/*.test.ts"}) {
		t.Errorf("expected dev test files, got %v", dev)
	}
}

// Declaration files and config files are dev entry points; multiple .d.ts collapse to **/*.d.ts.
func TestDetectPackageEntryPoints_DeclarationAndConfigFiles(t *testing.T) {
	newPkg := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "rev-dep-ep-dts")
		if err != nil {
			t.Fatalf("temp: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		for rel, content := range files {
			p := filepath.Join(dir, rel)
			_ = os.MkdirAll(filepath.Dir(p), 0755)
			if err := os.WriteFile(p, []byte(content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
		}
		return dir
	}

	t.Run("multiple .d.ts collapse to **/*.d.ts; config file is dev", func(t *testing.T) {
		dir := newPkg(t, map[string]string{
			"package.json":   `{"name":"p"}`,
			"index.ts":       `import './lib';`, // prod entry
			"lib.ts":         `export const l=1;`,
			"src/types.d.ts": `export type A = number;`,
			"global.d.ts":    `declare const x: number;`,
			"vite.config.ts": `export default {};`, // .config. -> dev
		})
		prod, dev, _ := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))
		if !slices.Equal(prod, []string{"index.ts"}) {
			t.Errorf("prod: got %v", prod)
		}
		if !slices.Equal(dev, []string{"**/*.d.ts", "vite.config.ts"}) {
			t.Errorf("dev: got %v", dev)
		}
	})

	t.Run("single .d.ts is listed as-is", func(t *testing.T) {
		dir := newPkg(t, map[string]string{
			"package.json": `{"name":"p"}`,
			"index.ts":     `export const i=1;`, // prod entry
			"types.d.ts":   `export type A = number;`,
		})
		_, dev, _ := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))
		if !slices.Equal(dev, []string{"types.d.ts"}) {
			t.Errorf("dev: got %v", dev)
		}
	})
}

// Regression: non-source files (HTML, CSS, ...) must not block folding, since they are not part
// of the analyzed file universe.
func TestDetectPackageEntryPoints_IgnoresNonSourceFiles(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-ep-ext")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	write("package.json", `{"name":"p"}`)
	write("pages/home.tsx", `export const h=1;`)
	write("pages/about.tsx", `export const a=1;`)
	write("pages/index.html", `<html></html>`) // non-source, must not block the fold
	write("pages/styles.css", `body{}`)        // non-source, must not block the fold

	prod, _, _ := detectPackageEntryPoints(pathutil.StandardiseDirPath(dir))
	if !slices.Equal(prod, []string{"pages/**"}) {
		t.Errorf("expected pages/** despite non-source files, got %v", prod)
	}
}
