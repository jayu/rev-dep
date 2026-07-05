package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// recordingAsker returns an asker that records whether it was called and returns `answer`.
func recordingAsker(answer standaloneSelection, called *bool) standaloneAsker {
	return func(projectStructure, standalonePackages) standaloneSelection {
		*called = true
		return answer
	}
}

func TestStandaloneChoiceApplies(t *testing.T) {
	mkTemp := func(t *testing.T) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "rev-dep-choice")
		if err != nil {
			t.Fatalf("temp dir: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		return dir
	}
	applies := func(dir string) bool {
		ps := detectProjectStructure(dir)
		return standaloneChoiceApplies(ps, classifyStandalonePackages(dir, ps.standalonePackageDirs))
	}

	t.Run("single root, no standalone -> no choice", func(t *testing.T) {
		dir := mkTemp(t)
		writePackageJson(t, dir, "app")
		if applies(dir) {
			t.Fatalf("did not expect a choice for a plain single-package project")
		}
	})

	t.Run("monorepo, no standalone -> no choice", func(t *testing.T) {
		dir := mkTemp(t)
		os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"root","workspaces":["packages/*"]}`), 0644)
		writePackageJson(t, filepath.Join(dir, "packages", "pkg1"), "@m/pkg1")
		if applies(dir) {
			t.Fatalf("did not expect a choice for a monorepo with no standalone packages")
		}
	})

	t.Run("single root + standalone -> choice", func(t *testing.T) {
		dir := mkTemp(t)
		writePackageJson(t, dir, "app")
		writePackageJson(t, filepath.Join(dir, "services", "api"), "@a/api")
		if !applies(dir) {
			t.Fatalf("expected a choice for single-root + standalone packages")
		}
	})

	t.Run("monorepo + standalone -> choice", func(t *testing.T) {
		dir := mkTemp(t)
		os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"root","workspaces":["packages/*"]}`), 0644)
		writePackageJson(t, filepath.Join(dir, "packages", "pkg1"), "@m/pkg1")
		writePackageJson(t, filepath.Join(dir, "tools", "cli"), "@m/cli")
		if !applies(dir) {
			t.Fatalf("expected a choice for monorepo + standalone packages")
		}
	})

	t.Run("no root, standalone without fixtures -> no choice", func(t *testing.T) {
		dir := mkTemp(t)
		writePackageJson(t, filepath.Join(dir, "pkgs", "a"), "@a/a")
		writePackageJson(t, filepath.Join(dir, "pkgs", "b"), "@a/b")
		if applies(dir) {
			t.Fatalf("did not expect a choice with no base project and nothing to filter")
		}
	})

	t.Run("no root, standalone WITH fixtures -> choice", func(t *testing.T) {
		dir := mkTemp(t)
		writePackageJson(t, filepath.Join(dir, "pkgs", "a"), "@a/a")
		writePackageJson(t, filepath.Join(dir, "__fixtures__", "x"), "fx")
		if !applies(dir) {
			t.Fatalf("expected a choice with no base project but filterable subfolders")
		}
	})

	t.Run("monorepo sub-package -> no choice", func(t *testing.T) {
		dir := mkTemp(t)
		os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"root","workspaces":["packages/*"]}`), 0644)
		pkgDir := filepath.Join(dir, "packages", "pkg1")
		writePackageJson(t, pkgDir, "@m/pkg1")
		ps := detectProjectStructure(pkgDir)
		if !ps.isMonorepoSubPackage {
			t.Fatalf("expected isMonorepoSubPackage=true when run inside a workspace package")
		}
		if standaloneChoiceApplies(ps, classifyStandalonePackages(pkgDir, ps.standalonePackageDirs)) {
			t.Fatalf("did not expect a choice inside a monorepo sub-package")
		}
	})
}

func TestInitConfigInteractive_ChoiceControlsStandalone(t *testing.T) {
	newProject := func(t *testing.T) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "rev-dep-choice-gen")
		if err != nil {
			t.Fatalf("temp dir: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		writePackageJson(t, dir, "app")
		writePackageJson(t, filepath.Join(dir, "services", "api"), "@a/api")
		writePackageJson(t, filepath.Join(dir, "services", "web"), "@a/web")
		return dir
	}

	t.Run("base-only choice excludes standalone packages", func(t *testing.T) {
		dir := newProject(t)
		called := false
		result, err := initConfigInteractive(dir, recordingAsker(standaloneNone, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold })
		if err != nil {
			t.Fatalf("initConfigInteractive: %v", err)
		}
		if !called {
			t.Fatalf("expected the asker to be consulted in a single-root + standalone project")
		}
		if len(result.standalonePackagePaths) != 0 {
			t.Errorf("expected no standalone packages, got %v", result.standalonePackagePaths)
		}
		if got := rulePaths(result.rules); !slices.Equal(got, []string{"."}) {
			t.Errorf("expected only the root rule, got %v", got)
		}
	})

	t.Run("include-all choice adds standalone packages", func(t *testing.T) {
		dir := newProject(t)
		called := false
		result, err := initConfigInteractive(dir, recordingAsker(standaloneAll, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold })
		if err != nil {
			t.Fatalf("initConfigInteractive: %v", err)
		}
		if !called {
			t.Fatalf("expected the asker to be consulted")
		}
		wantStandalone := []string{"services/api", "services/web"}
		if !slices.Equal(result.standalonePackagePaths, wantStandalone) {
			t.Errorf("expected standalone %v, got %v", wantStandalone, result.standalonePackagePaths)
		}
		if got := rulePaths(result.rules); !slices.Equal(got, []string{".", "services/api", "services/web"}) {
			t.Errorf("unexpected rules %v", got)
		}
	})
}

func TestClassifyStandalonePackages(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-classify")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// A mix of real packages and fixture/test-like folders (with and without underscores).
	writePackageJson(t, filepath.Join(dir, "services", "api"), "@a/api")
	writePackageJson(t, filepath.Join(dir, "__fixtures__", "sample-pkg"), "fixture-pkg")
	writePackageJson(t, filepath.Join(dir, "test", "helper-pkg"), "test-pkg")
	writePackageJson(t, filepath.Join(dir, "packages", "core"), "@a/core")

	ps := detectProjectStructure(dir)
	sp := classifyStandalonePackages(dir, ps.standalonePackageDirs)

	if !slices.Equal(sp.all, []string{"__fixtures__/sample-pkg", "packages/core", "services/api", "test/helper-pkg"}) {
		t.Fatalf("unexpected all: %v", sp.all)
	}
	if !slices.Equal(sp.curated, []string{"packages/core", "services/api"}) {
		t.Errorf("unexpected curated: %v", sp.curated)
	}
	if !slices.Equal(sp.filteredOut, []string{"__fixtures__/sample-pkg", "test/helper-pkg"}) {
		t.Errorf("unexpected filteredOut: %v", sp.filteredOut)
	}
	// Patterns recorded in order of first appearance across the sorted list.
	if !slices.Equal(sp.patterns, []string{"__fixtures__", "test"}) {
		t.Errorf("unexpected patterns: %v", sp.patterns)
	}
}

func TestMatchNonPackagePattern(t *testing.T) {
	cases := map[string]string{
		"services/api":            "",
		"packages/core":           "",
		"__fixtures__/a":          "__fixtures__",
		"a/__mocks__/b":           "__mocks__",
		"test/helper":             "test",
		"nested/fixtures/pkg":     "fixtures",
		".next/standalone":        ".next",
		"web/.next/standalone":    ".next",
		"examples/demo-app":       "examples",
		"src/__generated__/thing": "__generated__",
		"test-utils/pkg":          "", // not an exact junk name, not underscore-wrapped
	}
	for relPath, want := range cases {
		if got := matchNonPackagePattern(relPath); got != want {
			t.Errorf("matchNonPackagePattern(%q) = %q, want %q", relPath, got, want)
		}
	}
}

func TestInitConfigInteractive_CuratedSelection(t *testing.T) {
	dir, err := os.MkdirTemp("", "rev-dep-curated")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	writePackageJson(t, dir, "app") // base project (single root package)
	writePackageJson(t, filepath.Join(dir, "services", "api"), "@a/api")
	writePackageJson(t, filepath.Join(dir, "__fixtures__", "x"), "fx")

	called := false
	result, err := initConfigInteractive(dir, recordingAsker(standaloneCurated, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold })
	if err != nil {
		t.Fatalf("initConfigInteractive: %v", err)
	}
	if !called {
		t.Fatalf("expected the asker to be consulted")
	}
	// Curated selection keeps the real package, drops the fixtures one.
	if !slices.Equal(result.standalonePackagePaths, []string{"services/api"}) {
		t.Errorf("expected only services/api curated, got %v", result.standalonePackagePaths)
	}
	if got := rulePaths(result.rules); !slices.Equal(got, []string{".", "services/api"}) {
		t.Errorf("unexpected rules %v", got)
	}
}

func TestInitConfigInteractive_NoBaseCuratedVsAll(t *testing.T) {
	newProject := func(t *testing.T) string {
		t.Helper()
		dir, err := os.MkdirTemp("", "rev-dep-nobase-choice")
		if err != nil {
			t.Fatalf("temp dir: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })
		// No root package.json. Real packages + a fixtures folder.
		writePackageJson(t, filepath.Join(dir, "pkgs", "a"), "@a/a")
		writePackageJson(t, filepath.Join(dir, "__fixtures__", "x"), "fx")
		return dir
	}

	t.Run("asker is consulted and curated drops fixtures (no root rule)", func(t *testing.T) {
		dir := newProject(t)
		called := false
		result, err := initConfigInteractive(dir, recordingAsker(standaloneCurated, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold })
		if err != nil {
			t.Fatalf("initConfigInteractive: %v", err)
		}
		if !called {
			t.Fatalf("expected the asker to be consulted for no-base + filterable subfolders")
		}
		if result.rootRuleCreated {
			t.Errorf("did not expect a root rule when there is no root package.json")
		}
		if !slices.Equal(result.standalonePackagePaths, []string{"pkgs/a"}) {
			t.Errorf("expected curated [pkgs/a], got %v", result.standalonePackagePaths)
		}
		if got := rulePaths(result.rules); !slices.Equal(got, []string{"pkgs/a"}) {
			t.Errorf("expected only pkgs/a, got %v", got)
		}
	})

	t.Run("all includes the fixtures folder too", func(t *testing.T) {
		dir := newProject(t)
		called := false
		result, err := initConfigInteractive(dir, recordingAsker(standaloneAll, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold })
		if err != nil {
			t.Fatalf("initConfigInteractive: %v", err)
		}
		if !slices.Equal(result.standalonePackagePaths, []string{"__fixtures__/x", "pkgs/a"}) {
			t.Errorf("expected all [__fixtures__/x, pkgs/a], got %v", result.standalonePackagePaths)
		}
	})
}

func TestInitConfigInteractive_AskerNotCalledWithoutChoice(t *testing.T) {
	cases := map[string]func(t *testing.T) string{
		"single root, no standalone": func(t *testing.T) string {
			dir, _ := os.MkdirTemp("", "rev-dep-noask-single")
			t.Cleanup(func() { os.RemoveAll(dir) })
			writePackageJson(t, dir, "app")
			return dir
		},
		"no root, standalone only": func(t *testing.T) string {
			dir, _ := os.MkdirTemp("", "rev-dep-noask-noroot")
			t.Cleanup(func() { os.RemoveAll(dir) })
			writePackageJson(t, filepath.Join(dir, "pkgs", "a"), "@a/a")
			return dir
		},
	}

	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := setup(t)
			called := false
			if _, err := initConfigInteractive(dir, recordingAsker(standaloneAll, &called), func() bool { return false }, func() detectorPreset { return detectorsScaffold }); err != nil {
				t.Fatalf("initConfigInteractive: %v", err)
			}
			if called {
				t.Fatalf("the asker must not be consulted when no choice applies")
			}
		})
	}
}
