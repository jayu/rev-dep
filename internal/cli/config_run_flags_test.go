package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setConfigRunFlags sets the config run flag globals for one test and restores them afterwards.
func setConfigRunFlags(t *testing.T, cwd, format string, lint bool, lintRules []string) {
	t.Helper()
	prevCwd, prevFormat, prevLint, prevLintRules := runConfigCwd, runConfigFormat, runConfigLint, runConfigLintRules
	t.Cleanup(func() {
		runConfigCwd, runConfigFormat, runConfigLint, runConfigLintRules = prevCwd, prevFormat, prevLint, prevLintRules
	})
	runConfigCwd, runConfigFormat, runConfigLint, runConfigLintRules = cwd, format, lint, lintRules
}

// Bad values must fail before the config is even loaded: the directory below has no config, so
// reaching the config loader would produce a different error.
func TestConfigRunRejectsInvalidFlagValuesUpfront(t *testing.T) {
	for _, tc := range []struct {
		name      string
		format    string
		lintRules []string
		wantErr   string
	}{
		{"unknown format", "xml", nil, `invalid --format "xml"`},
		{"format is case-sensitive", "JSON", nil, `invalid --format "JSON"`},
		{"unknown lint rule", "", []string{"nope"}, `invalid --lint-config-rules: unknown lint rule "nope"`},
		{"unknown lint rule with json", "json", []string{"orphan-file-globs", "nope"}, `unknown lint rule "nope"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setConfigRunFlags(t, t.TempDir(), tc.format, false, tc.lintRules)
			err := configRunCmd.RunE(configRunCmd, nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

const configRunLintSubprocessEnv = "REV_DEP_TEST_CONFIG_RUN_LINT"

// runConfigRunChild is the child half of runConfigRunSubprocess: when the parent's env spec is set,
// it runs config run with those flags (exiting through os.Exit like the real command) and reports
// true so the calling test returns.
func runConfigRunChild(t *testing.T) bool {
	spec := os.Getenv(configRunLintSubprocessEnv)
	if spec == "" {
		return false
	}
	parts := strings.SplitN(spec, "|", 4) // cwd|format|lint|fix
	setConfigRunFlags(t, parts[0], parts[1], parts[2] == "lint", nil)
	prevFix := runConfigFix
	t.Cleanup(func() { runConfigFix = prevFix })
	runConfigFix = parts[3] == "fix"
	if err := configRunCmd.RunE(configRunCmd, nil); err != nil {
		t.Fatalf("config run returned an error: %v", err)
	}
	return true
}

// runConfigRunSubprocess re-runs the current test in a child process that executes config run, and
// returns the child's stdout and exit code. The JSON and issues-list paths end in os.Exit, which a
// test can only observe from outside.
func runConfigRunSubprocess(t *testing.T, cwd, format string, lint, fix bool) (string, int) {
	t.Helper()
	flag := func(on bool, name string) string {
		if on {
			return name
		}
		return ""
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
	cmd.Env = append(os.Environ(), configRunLintSubprocessEnv+"="+strings.Join(
		[]string{cwd, format, flag(lint, "lint"), flag(fix, "fix")}, "|"))
	var stdout strings.Builder
	cmd.Stdout = &stdout
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run subprocess: %v", err)
	}
	// The child is a test binary: drop its own PASS/FAIL trailer from stdout.
	out := stdout.String()
	if i := strings.LastIndex(out, "\nPASS"); i >= 0 {
		out = out[:i+1]
	}
	return out, code
}

// writeLintProject writes a project whose checks pass. With deadGlob, its config also carries an
// ignoreFiles glob that matches nothing - a lint error that only --lint-config reports.
func writeLintProject(t *testing.T, deadGlob bool) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	ignore := ""
	if deadGlob {
		ignore = `,"ignoreFiles":["does-not-exist/**"]`
	}
	files := map[string]string{
		"package.json":         `{"name":"p","version":"1.0.0"}`,
		"src/index.ts":         `export {};`,
		".rev-dep.config.json": `{"configVersion":"2.1","workspaces":[{"path":".","prodEntryPoints":["src/index.ts"],"unusedExportsDetection":{"enabled":true` + ignore + `}}]}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// --lint-config used to be silently dropped by --format json and issues-list, so a CI step using
// either passed with a failing config lint. Both formats exit through os.Exit, hence the subprocess.
func TestConfigRunLintsInMachineReadableFormats(t *testing.T) {
	if runConfigRunChild(t) {
		return
	}
	run := func(t *testing.T, cwd, format string, lint bool) (string, int) {
		t.Helper()
		return runConfigRunSubprocess(t, cwd, format, lint, false)
	}

	parseJSON := func(t *testing.T, out string) jsonOutput {
		t.Helper()
		var parsed jsonOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, out)
		}
		return parsed
	}

	t.Run("json reports lint errors and fails", func(t *testing.T) {
		out, code := run(t, writeLintProject(t, true), "json", true)
		got := parseJSON(t, out)
		if got.Lint == nil || got.Lint.Errors == 0 {
			t.Errorf("expected lint errors in the JSON, got %+v", got.Lint)
		}
		if got.HasFailures {
			t.Error("lint findings must not set hasFailures (the checks passed)")
		}
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})

	t.Run("json with a clean config passes", func(t *testing.T) {
		out, code := run(t, writeLintProject(t, false), "json", true)
		if got := parseJSON(t, out); got.Lint == nil || got.Lint.Errors != 0 {
			t.Errorf("expected a lint summary with 0 errors, got %+v", got.Lint)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	t.Run("json without --lint-config has no lint field", func(t *testing.T) {
		out, code := run(t, writeLintProject(t, true), "json", false)
		if strings.Contains(out, `"lint"`) {
			t.Errorf("lint field present although lint was not requested:\n%s", out)
		}
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})

	t.Run("issues-list reports lint errors and fails", func(t *testing.T) {
		out, code := run(t, writeLintProject(t, true), "issues-list", true)
		if !strings.Contains(out, "Config lint:") {
			t.Errorf("expected a config lint summary line:\n%s", out)
		}
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
}

// With --fix, every output format exits 0 when all the issues found were fixable (and so fixed),
// as the default format already did. JSON and issues-list used to exit 1 on hasFailures alone.
func TestConfigRunFixExitCodeMatchesAcrossFormats(t *testing.T) {
	if runConfigRunChild(t) {
		return
	}

	// One unused export, autofixable.
	writeFixableProject := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		files := map[string]string{
			"package.json":         `{"name":"p","version":"1.0.0"}`,
			"src/index.ts":         "import { used } from \"./lib\";\nconsole.log(used);\n",
			"src/lib.ts":           "export const used = 1;\nexport const unused = 2;\n",
			".rev-dep.config.json": `{"configVersion":"2.1","workspaces":[{"path":".","prodEntryPoints":["src/index.ts"],"unusedExportsDetection":{"enabled":true,"autofix":true}}]}`,
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	for _, format := range []string{"", "json", "issues-list"} {
		name := format
		if name == "" {
			name = "default"
		}
		t.Run(name+" without --fix fails", func(t *testing.T) {
			if _, code := runConfigRunSubprocess(t, writeFixableProject(t), format, false, false); code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
		})
		t.Run(name+" with --fix passes once everything is fixed", func(t *testing.T) {
			dir := writeFixableProject(t)
			if _, code := runConfigRunSubprocess(t, dir, format, false, true); code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			lib, err := os.ReadFile(filepath.Join(dir, "src", "lib.ts"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(lib), "export const unused") {
				t.Errorf("--fix did not remove the unused export:\n%s", lib)
			}
		})
	}
}
