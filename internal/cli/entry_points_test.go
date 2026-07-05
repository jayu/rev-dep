package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/pathutil"
)

// fold is a test helper: fold entryFiles (a subset of allFiles) into patterns.
func fold(allFiles, entryFiles []string) []string {
	set := map[string]bool{}
	for _, f := range entryFiles {
		set[f] = true
	}
	return foldEntryPatterns(allFiles, set)
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
		func() detectorPreset { return detectorsScaffold })
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
		func() detectorPreset { return detectorsScaffold })
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
	// The sibling test files stay in dev (and test/ does not over-fold to test/** because it
	// contains the ignored fixtures subtree).
	if !slices.Equal(dev, []string{"test/analyze.test.ts", "test/regressions.test.ts"}) {
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
