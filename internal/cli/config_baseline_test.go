package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/diag"
)

const baselineChildEnv = "REV_DEP_TEST_BASELINE_RUN"

type baselineRunSpec struct {
	Cwd        string
	Format     string
	Update     bool
	Fix        bool
	Workspaces []string
	Lint       bool
	Verbose    bool
	ListAll    bool
}

// Exercise real command exit paths without letting os.Exit terminate the parent.
func TestBaselineCLIChild(t *testing.T) {
	raw := os.Getenv(baselineChildEnv)
	if raw == "" {
		return
	}
	var spec baselineRunSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatal(err)
	}
	runConfigCwd, runConfigFormat = spec.Cwd, spec.Format
	runConfigUpdateSnapshot, runConfigFix = spec.Update, spec.Fix
	runConfigRules, runConfigLint = spec.Workspaces, spec.Lint
	runConfigListAll = spec.ListAll
	diag.SetVerbose(spec.Verbose)
	if err := configRunCmd.RunE(configRunCmd, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func baselineCLI(t *testing.T, spec baselineRunSpec) (string, string, int) {
	t.Helper()
	raw, _ := json.Marshal(spec)
	cmd := exec.Command(os.Args[0], "-test.run=^TestBaselineCLIChild$")
	cmd.Env = append(os.Environ(), baselineChildEnv+"="+string(raw))
	var out, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	code := 0
	if e, ok := err.(*exec.ExitError); ok {
		code = e.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(out.String(), "PASS\n"), stderr.String(), code
}

func writeBaselineProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	baselineWrite(t, root, "package.json", `{"name":"app","version":"1.0.0"}`)
	baselineWrite(t, root, "src/main.ts", "import 'missing-package';\n")
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","missingNodeModulesDetection":true}]}`)
	return root
}

func baselineWrite(t *testing.T, root, name, content string) {
	t.Helper()
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func baselineJSON(t *testing.T, out string) jsonOutput {
	t.Helper()
	var result jsonOutput
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if result.Baseline == nil {
		t.Fatalf("missing baseline outcome: %s", out)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &fields); err != nil {
		t.Fatal(err)
	}
	var baselineFields, counts map[string]json.RawMessage
	if err := json.Unmarshal(fields["baseline"], &baselineFields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(baselineFields["counts"], &counts); err != nil {
		t.Fatal(err)
	}
	for _, object := range []map[string]json.RawMessage{baselineFields, counts} {
		for _, key := range []string{"new", "obsolete"} {
			if _, ok := object[key]; !ok {
				t.Fatalf("baseline output missing %s: %s", key, out)
			}
		}
		for _, key := range []string{"added", "removed"} {
			if _, ok := object[key]; ok {
				t.Fatalf("baseline output still contains %s: %s", key, out)
			}
		}
	}
	if result.Baseline.Counts.New != len(result.Baseline.New) || result.Baseline.Counts.Obsolete != len(result.Baseline.Obsolete) {
		t.Fatalf("baseline counts do not match finding lists: %+v", result.Baseline)
	}
	return result
}

func TestBaselineCLIResolverFailuresUseVerboseWarnings(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T, string)
		message string
	}{
		{"malformed manifest", func(t *testing.T, root string) {
			baselineWrite(t, root, "package.json", `{"name":"app",`)
		}, "package.json"},
		{"empty manifest", func(t *testing.T, root string) {
			baselineWrite(t, root, "package.json", "")
		}, "package.json"},
		{"unexpected manifest read error", func(t *testing.T, root string) {
			file := filepath.Join(root, "package.json")
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(file, 0755); err != nil {
				t.Fatal(err)
			}
		}, "could not read package.json"},
		{"malformed workspace manifest", func(t *testing.T, root string) {
			baselineWrite(t, root, "packages/other/package.json", `{"name":`)
			baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","missingNodeModulesDetection":true},{"path":"packages/other"}]}`)
		}, "package.json"},
		{"malformed discovered workspace manifest", func(t *testing.T, root string) {
			baselineWrite(t, root, "package.json", `{"name":"app","workspaces":["packages/*"]}`)
			baselineWrite(t, root, "packages/other/package.json", `{"name":`)
		}, "package.json"},
		{"invalid configured tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "custom.json", `{"compilerOptions":`)
			baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","tsConfigPath":"custom.json","missingNodeModulesDetection":true}]}`)
		}, "custom.json"},
		{"missing configured tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","tsConfigPath":"missing.json","missingNodeModulesDetection":true}]}`)
		}, "missing.json"},
		{"invalid default tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "tsconfig.json", `{"compilerOptions":`)
		}, "tsconfig.json"},
		{"malformed extended tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "tsconfig.json", `{"extends":"./base.json"}`)
			baselineWrite(t, root, "base.json", `{"compilerOptions":`)
		}, "base.json"},
		{"missing extended tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "tsconfig.json", `{"extends":"./absent.json"}`)
		}, "extends"},
		{"circular extended tsconfig", func(t *testing.T, root string) {
			baselineWrite(t, root, "tsconfig.json", `{"extends":"./base.json"}`)
			baselineWrite(t, root, "base.json", `{"extends":"./tsconfig.json"}`)
		}, "circular tsconfig extends"},
		{"malformed tsconfig package manifest", func(t *testing.T, root string) {
			baselineWrite(t, root, "tsconfig.json", `{"extends":"@scope/config"}`)
			baselineWrite(t, root, "node_modules/@scope/config/package.json", `{"main":`)
			baselineWrite(t, root, "node_modules/@scope/config/tsconfig.json", `{}`)
		}, "could not parse tsconfig package.json"},
		{"unreadable followed source", func(t *testing.T, root string) {
			outside := t.TempDir()
			followed := filepath.Join(outside, "followed.ts")
			baselineWrite(t, outside, "followed.ts", "import 'other-missing';\n")
			if err := os.Chmod(followed, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(followed, 0644) })
			if _, err := os.ReadFile(followed); err == nil {
				t.Skip("current user can read files regardless of file permissions")
			}
			request, err := filepath.Rel(filepath.Join(root, "src"), followed)
			if err != nil {
				t.Fatal(err)
			}
			baselineWrite(t, root, "src/main.ts", fmt.Sprintf("import %q;\n", filepath.ToSlash(request)))
		}, "could not read resolved file"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeBaselineProject(t)
			if _, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true}); code != 0 {
				t.Fatalf("initial capture failed: %s", stderr)
			}
			test.prepare(t, root)
			out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true})
			if code != 0 || !strings.Contains(out, "Baseline snapshot written") || strings.Contains(out, "Warning:") {
				t.Fatalf("quiet capture failed or printed warnings: %d %s %s", code, out, stderr)
			}
			for _, update := range []bool{false, true} {
				out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: update, Verbose: true})
				if code != 0 || !strings.Contains(out, "Warning:") || !strings.Contains(out, test.message) {
					t.Fatalf("resolver failure did not remain a verbose warning (update=%v): %d %s %s", update, code, out, stderr)
				}
			}
		})
	}
}

func TestConfigRunVerboseReportsBrokenExtendsWithoutBaseline(t *testing.T) {
	root := writeBaselineProject(t)
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","workspaces":[{"path":".","unresolvedImportsDetection":true}]}`)
	baselineWrite(t, root, "tsconfig.json", `{"extends":"./missing.json","compilerOptions":{"paths":{"@app/*":["src/*"]}}}`)
	baselineWrite(t, root, "src/main.ts", "import {value} from '@app/value'; console.log(value);\n")
	baselineWrite(t, root, "src/value.ts", "export const value = 1;\n")
	for _, verbose := range []bool{false, true} {
		out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Verbose: verbose})
		if code != 0 {
			t.Fatalf("broken extends discarded valid child aliases: %d %s %s", code, out, stderr)
		}
		if strings.Contains(out, "could not resolve tsconfig extends") != verbose {
			t.Fatalf("extends warning did not respect verbose=%v: %s", verbose, out)
		}
	}
}

func TestBaselineCLIRejectsExistingProjectFiles(t *testing.T) {
	for _, destination := range []string{"src/main.ts", "ignored.ts", "package.json", "tsconfig.json", "base.json", "custom.json", "data.json", "extra.txt"} {
		t.Run(destination, func(t *testing.T) {
			root := writeBaselineProject(t)
			if destination != "src/main.ts" && destination != "package.json" {
				baselineWrite(t, root, destination, `{"projectSetting":true}`)
			}
			baselineWrite(t, root, "src/unused.ts", "export const unused = 1;\n")
			baselineWrite(t, root, "rev-dep.config.json", fmt.Sprintf(`{"configVersion":"2.2","baselineSnapshotPath":%q,"workspaces":[{"path":".","unusedExportsDetection":{"autofix":true}}]}`, destination))
			file := filepath.Join(root, destination)
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			_, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true, Fix: true})
			if code != 1 || !strings.Contains(stderr, "refusing to overwrite") {
				t.Fatalf("dangerous destination accepted: %d %s", code, stderr)
			}
			after, err := os.ReadFile(file)
			if err != nil || string(before) != string(after) {
				t.Fatalf("update replaced source/input %s: %v", destination, err)
			}
			unused, err := os.ReadFile(filepath.Join(root, "src/unused.ts"))
			if err != nil || string(unused) != "export const unused = 1;\n" {
				t.Fatalf("fix ran before rejecting the destination: %v", err)
			}
		})
	}
}

func TestBaselineCLISelectsEveryEntryInNormalizedScope(t *testing.T) {
	root := writeBaselineProject(t)
	baselineWrite(t, root, "src/main.ts", "import 'missing-package';\nimport './absent';\n")
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","missingNodeModulesDetection":true},{"path":"./","unresolvedImportsDetection":true}]}`)
	if _, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true}); code != 0 {
		t.Fatalf("initial capture failed: %s", stderr)
	}
	for _, selected := range []string{".", "./"} {
		out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Workspaces: []string{selected}})
		if code != 0 {
			t.Fatalf("aliased selection reported removals: %d %s %s", code, out, stderr)
		}
		result := baselineJSON(t, out)
		if result.Baseline.Status != "matched" || result.Baseline.Counts.Acknowledged < 2 || len(result.Rules) != 2 || len(result.Baseline.Workspaces) != 1 || result.Baseline.Workspaces[0] != "." {
			t.Fatalf("did not compare every entry in normalized scope: %+v", result)
		}
	}
	// Changes in the aliased entry must still be checked when selecting ".".
	baselineWrite(t, root, "src/main.ts", "import 'missing-package';\nimport './different';\n")
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Workspaces: []string{"."}})
	if code != 1 {
		t.Fatalf("missed change in aliased entry: %d %s %s", code, out, stderr)
	}
	result := baselineJSON(t, out)
	if result.Baseline.Counts.New != 1 || result.Baseline.Counts.Obsolete != 1 {
		t.Fatalf("wrong aliased scope delta: %+v", result.Baseline)
	}
}

func TestBaselineCLIRejectsMalformedFindingShape(t *testing.T) {
	root := writeBaselineProject(t)
	baselineWrite(t, root, "snapshots/findings.json", `{"schemaVersion":"1.0","workspaces":{".":{"missing-node-modules":[{"file":"src/main.ts","module":"missing-package","target":null}]}}}`)
	_, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root})
	if code != 1 || !strings.Contains(stderr, "target must not be null") {
		t.Fatalf("malformed record matched current findings: %d %s", code, stderr)
	}
}

func TestBaselineCLILifecycleAndOutputFormats(t *testing.T) {
	root := writeBaselineProject(t)
	_, errout, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json"})
	if code != 1 || !strings.Contains(errout, "--update-snapshot") {
		t.Fatalf("missing baseline not actionable: %d %s", code, errout)
	}
	out, errout, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Update: true})
	if code != 0 {
		t.Fatalf("capture failed: %s", errout)
	}
	written := baselineJSON(t, out)
	if written.Baseline.Status != "written" || written.Baseline.Counts.Acknowledged != 1 || written.HasFailures {
		t.Fatalf("incorrect capture: %+v", written)
	}
	original, err := os.ReadFile(filepath.Join(root, "snapshots/findings.json"))
	if err != nil {
		t.Fatal(err)
	}
	baselineWrite(t, root, "src/main.ts", "// moved the import\n\nimport 'missing-package';\nimport 'missing-package';\n")
	for _, format := range []string{"", "json", "issues-list"} {
		out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: format})
		if code != 0 {
			t.Fatalf("%s failed matching after line shift: %s", format, stderr)
		}
		if format == "json" {
			result := baselineJSON(t, out)
			if result.HasFailures || result.Baseline.Counts.Acknowledged != 1 || len(result.Rules[0].Checks.MissingNodeModules.Issues) != 0 {
				t.Fatalf("acknowledged findings still fail/display: %+v", result)
			}
		} else if !strings.Contains(out, "Baseline snapshot matched") || strings.Contains(out, "No issues found") {
			t.Fatalf("misleading matched output: %s", out)
		}
	}
	out, _, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Update: true})
	if code != 0 {
		t.Fatalf("recapture failed: %s", out)
	}
	repeated, _ := os.ReadFile(filepath.Join(root, "snapshots/findings.json"))
	if string(original) != string(repeated) {
		t.Fatal("line shifts/repeated imports churned baseline")
	}
	baselineWrite(t, root, "src/main.ts", "import 'different-package';\n")
	for _, format := range []string{"", "json", "issues-list"} {
		out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: format})
		if code != 1 {
			t.Fatalf("%s did not fail on replacement: %d %s %s", format, code, out, stderr)
		}
		if format == "json" {
			result := baselineJSON(t, out)
			if !result.HasFailures || result.Baseline.Counts.New != 1 || result.Baseline.Counts.Obsolete != 1 {
				t.Fatalf("wrong replacement delta: %+v", result.Baseline)
			}
		} else if !strings.Contains(out, "different-package") || !strings.Contains(out, "missing-package") ||
			!strings.Contains(out, "1 new, 1 obsolete, 0 acknowledged findings") || !strings.Contains(out, "Obsolete findings:") {
			t.Fatalf("missing new/obsolete findings: %s", out)
		}
	}
	// Removing the detector compares the old group against an empty set.
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":"."}]}`)
	out, _, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json"})
	result := baselineJSON(t, out)
	if code != 1 || result.Baseline.Counts.Obsolete != 1 || result.Baseline.Counts.New != 0 {
		t.Fatalf("removed detector not asserted: %d %+v", code, result.Baseline)
	}
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Update: true})
	if code != 0 {
		t.Fatalf("empty capture failed: %s %s", out, stderr)
	}
	snapshot, err := baseline.Load(filepath.Join(root, "snapshots/findings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Workspaces) != 0 {
		t.Fatal("empty detector inventory stored")
	}
}

func TestBaselineCLIOptionsAndArrayReordering(t *testing.T) {
	root := writeBaselineProject(t)
	config := func(detections string) string {
		return `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","missingNodeModulesDetection":` + detections + `}]}`
	}
	baselineWrite(t, root, "rev-dep.config.json", config(`[true,{"excludeModules":["unrelated"]}]`))
	_, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true})
	if code != 0 {
		t.Fatal(stderr)
	}
	baselineWrite(t, root, "rev-dep.config.json", config(`[{"excludeModules":["another-unrelated","unrelated"]},true]`))
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json"})
	if code != 0 || baselineJSON(t, out).Baseline.Counts.Acknowledged != 1 {
		t.Fatalf("array/options change without finding changes failed: %s %s", out, stderr)
	}
	baselineWrite(t, root, "rev-dep.config.json", config(`{"excludeModules":["missing-package"]}`))
	out, _, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json"})
	if code != 1 || baselineJSON(t, out).Baseline.Counts.Obsolete != 1 {
		t.Fatal("exclusion removing a finding did not fail")
	}
}

func TestBaselineCLIObsoleteFindingsOutputLimits(t *testing.T) {
	root := writeBaselineProject(t)
	var source strings.Builder
	for i := 0; i < maxIssuesToList+2; i++ {
		fmt.Fprintf(&source, "import 'missing-%02d';\n", i)
	}
	baselineWrite(t, root, "src/main.ts", source.String())
	if _, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true}); code != 0 {
		t.Fatalf("initial capture failed: %s", stderr)
	}
	file := filepath.Join(root, "snapshots/findings.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	baselineWrite(t, root, "src/main.ts", "console.log('clean');\n")
	for _, test := range []struct {
		format  string
		listAll bool
	}{
		{"", false}, {"", true}, {"issues-list", false}, {"json", false},
	} {
		out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: test.format, ListAll: test.listAll})
		if code != 1 {
			t.Fatalf("obsolete findings did not fail: %d %s %s", code, out, stderr)
		}
		if test.format == "json" {
			result := baselineJSON(t, out)
			if len(result.Baseline.Obsolete) != maxIssuesToList+2 {
				t.Fatalf("JSON findings were truncated: %s", out)
			}
		} else {
			if strings.Count(out, "Baseline snapshot differs") != 1 || strings.Count(out, "Obsolete findings:") != 1 {
				t.Fatalf("summary or findings printed twice: %s", out)
			}
			for i := 0; i < maxIssuesToList+2; i++ {
				want := test.format == "issues-list" || test.listAll || i < maxIssuesToList
				if strings.Contains(out, fmt.Sprintf("missing-%02d", i)) != want {
					t.Fatalf("incorrect limit (format=%s, listAll=%v): %s", test.format, test.listAll, out)
				}
			}
			if strings.Contains(out, "and 2 more missing node module issues") != (test.format == "" && !test.listAll) {
				t.Fatalf("incorrect truncation message: %s", out)
			}
			if test.format == "issues-list" && !strings.Contains(out, "missing-00  src/main.ts") {
				t.Fatalf("issues-list labels and paths were not aligned: %s", out)
			}
			if strings.Contains(out, `"module":`) || strings.Contains(out, `"file":`) {
				t.Fatalf("obsolete findings were rendered as JSON: %s", out)
			}
		}
		after, err := os.ReadFile(file)
		if err != nil || string(before) != string(after) {
			t.Fatalf("rendering changed the stored baseline: %v", err)
		}
	}
}

func TestBaselineCLIFixAlwaysRechecksFinalState(t *testing.T) {
	root := writeBaselineProject(t)
	baselineWrite(t, root, "src/main.ts", "export const unused = 1;\n")
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","unusedExportsDetection":{"autofix":true}}]}`)
	_, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true})
	if code != 0 {
		t.Fatal(stderr)
	}
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Fix: true})
	result := baselineJSON(t, out)
	if code != 1 || result.Baseline.Counts.Obsolete != 1 || result.FixSummary.FixedImportsCount == 0 {
		t.Fatalf("fixed acknowledged issue did not produce final-state removal: %s %s", out, stderr)
	}
	baselineWrite(t, root, "src/main.ts", "export const unused = 1;\n")
	out, stderr, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Fix: true, Update: true})
	if code != 0 || baselineJSON(t, out).Baseline.Counts.Acknowledged != 0 {
		t.Fatalf("fix/update captured pre-fix issues: %s %s", out, stderr)
	}
}

func TestBaselineCLIWorkspaceFilteringUsesFullContext(t *testing.T) {
	root := t.TempDir()
	baselineWrite(t, root, "package.json", `{"name":"root","private":true}`)
	baselineWrite(t, root, "packages/app/package.json", `{"name":"app","dependencies":{"lib":"*"}}`)
	baselineWrite(t, root, "packages/lib/package.json", `{"name":"lib","exports":"./src/index.ts"}`)
	baselineWrite(t, root, "packages/app/src/main.ts", "import {value} from 'lib'; console.log(value);\nimport 'missing-package';\n")
	baselineWrite(t, root, "packages/lib/src/index.ts", "export const value = 1;\n")
	baselineWrite(t, root, "rev-dep.config.json", `{"configVersion":"2.2","baselineSnapshotPath":"snapshot.json","workspaces":[{"path":"packages/app","missingNodeModulesDetection":true,"unresolvedImportsDetection":true},{"path":"packages/lib","unusedExportsDetection":true}]}`)
	_, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Update: true})
	if code != 0 {
		t.Fatal(stderr)
	}
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Workspaces: []string{"packages/app"}})
	result := baselineJSON(t, out)
	if code != 0 || result.Baseline.Status != "matched" || len(result.Rules) != 1 || result.Baseline.Counts.Acknowledged != 2 {
		t.Fatalf("filtered resolution differs from full context: %s %s", out, stderr)
	}
	before, _ := os.ReadFile(filepath.Join(root, "snapshot.json"))
	_, stderr, code = baselineCLI(t, baselineRunSpec{Cwd: root, Update: true, Workspaces: []string{"packages/app"}})
	if code != 1 || !strings.Contains(stderr, "cannot be combined") {
		t.Fatalf("partial update allowed: %s", stderr)
	}
	after, _ := os.ReadFile(filepath.Join(root, "snapshot.json"))
	if string(before) != string(after) {
		t.Fatal("partial update erased baseline scopes")
	}
}

func TestBaselineCLISharedUpdateAndIndependentDuplication(t *testing.T) {
	root := writeBaselineProject(t)
	block := "function first(value) {\n const result = value + 1;\n console.log(result);\n return result;\n}\n"
	baselineWrite(t, root, "src/a.ts", block)
	baselineWrite(t, root, "src/b.ts", block)
	config := func(snapshot string) string {
		return `{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":".","missingNodeModulesDetection":true,"duplicatedCodeDetection":{"minTokens":10,"minLines":1` + snapshot + `}}]}`
	}
	baselineWrite(t, root, "rev-dep.config.json", config(`,"snapshotPath":"duplicates.json"`))
	out, stderr, code := baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Update: true})
	if code != 0 {
		t.Fatalf("shared update failed: %s %s", out, stderr)
	}
	for _, file := range []string{"snapshots/findings.json", "duplicates.json"} {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			t.Fatal(err)
		}
	}
	duplication := baselineJSON(t, out).Rules[0].Checks.DuplicatedCode[0]
	if duplication.Snapshot == nil || !duplication.Snapshot.Written || duplication.Snippets == 0 {
		t.Fatalf("initial duplication snapshot not acknowledged: %s", out)
	}
	duplicationPath := filepath.Join(root, "duplicates.json")
	stamp := time.Unix(946684800, 0)
	if err := os.Chtimes(duplicationPath, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	// A findings baseline update must not rewrite a matching duplication snapshot.
	baselineWrite(t, root, "src/main.ts", "import 'missing-package';\nimport 'another-missing-package';\n")
	for _, format := range []string{"", "json", "issues-list"} {
		out, stderr, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: format, Update: true})
		if code != 0 {
			t.Fatalf("unchanged duplication failed during baseline update: %s %s", out, stderr)
		}
		if format == "json" {
			result := baselineJSON(t, out)
			got := result.Rules[0].Checks.DuplicatedCode[0]
			if got.Status != "pass" || got.Snapshot == nil || got.Snapshot.Written || got.Snapshot.Unchanged != duplication.Snippets {
				t.Fatalf("incorrect unchanged duplication result: %s", out)
			}
		} else if format == "" {
			want := fmt.Sprintf("Duplicated Code: snapshot unchanged (%d patterns acknowledged)\n", duplication.Snippets)
			if duplication.Snippets == 1 {
				want = strings.Replace(want, "patterns acknowledged", "pattern acknowledged", 1)
			}
			if !strings.Contains(out, want) || strings.Contains(out, "Duplicated Code: snapshot updated") {
				t.Fatalf("incorrect duplication snapshot message: %s", out)
			}
		} else if strings.Contains(out, "Duplicated Code Issues") {
			t.Fatalf("unchanged duplication reported as an issue: %s", out)
		}
		info, err := os.Stat(duplicationPath)
		if err != nil || !info.ModTime().Equal(stamp) {
			t.Fatalf("baseline update rewrote unchanged duplication snapshot: %v", err)
		}
	}
	out, stderr, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json"})
	if code != 0 {
		t.Fatalf("both snapshots should match: %s %s", out, stderr)
	}
	baselineWrite(t, root, "rev-dep.config.json", config(""))
	for _, update := range []bool{false, true} {
		out, stderr, code = baselineCLI(t, baselineRunSpec{Cwd: root, Format: "json", Update: update})
		result := baselineJSON(t, out)
		if code != 1 || !result.HasFailures || result.Rules[0].Checks.DuplicatedCode[0].Status != "fail" {
			t.Fatalf("duplication without snapshot was acknowledged: %s %s", out, stderr)
		}
	}
}
