package cli

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/node"
)

func TestObsoleteFindingsRenderEveryIdentity(t *testing.T) {
	isType := true
	cases := []struct {
		detector string
		issue    baseline.Issue
		console  string
		list     string
	}{
		{"circular-imports", baseline.Issue{Files: []string{"src/a.ts", "src/b.ts"}}, "Files: src/a.ts, src/b.ts", "  Files: src/a.ts, src/b.ts\n"},
		{"orphan-files", baseline.Issue{File: "src/old.ts"}, "src/old.ts", "  src/old.ts\n"},
		{"missing-node-modules", baseline.Issue{File: "src/main.ts", Module: "missing"}, "missing (imported from: src/main.ts)", "  missing  src/main.ts\n"},
		{"dev-deps-usage-on-prod", baseline.Issue{File: "src/main.ts", Module: "dev-only"}, "    dev-only\n     - src/main.ts\n", "  dev-only  src/main.ts\n"},
		{"unused-node-modules", baseline.Issue{PackageJSON: "package.json", Module: "unused"}, "unused (package.json)", "  unused  package.json\n"},
		{"unused-exports", baseline.Issue{File: "src/types.ts", Export: "OldType", IsType: &isType}, "    src/types.ts\n     - OldType (type)\n", "  OldType  src/types.ts\n"},
		{"unresolved-imports", baseline.Issue{File: "src/main.ts", Request: "./absent", Kind: "literal"}, "    src/main.ts\n     - ./absent\n", "  ./absent  src/main.ts\n"},
		{"unresolved-imports", baseline.Issue{File: "src/main.ts", Request: "base +\nname", Kind: "nonliteral"}, "    src/main.ts\n     - base +\\nname - non-literal\n", "  base +\\nname - non-literal  src/main.ts\n"},
		{"module-boundaries", baseline.Issue{File: "src/main.ts", Target: "server/private.ts"}, "src/main.ts -> server/private.ts", "  server/private.ts  src/main.ts\n"},
		{"import-conventions", baseline.Issue{File: "src/main.ts", Request: "@utils/a", ViolationType: "should-be-relative"}, "    src/main.ts\n     - [should-be-relative] : \"@utils/a\"\n", "  @utils/a  src/main.ts\n"},
		{"restricted-imports", baseline.Issue{Kind: "file", Target: "server/private.ts"}, "     ➞ server/private.ts\n", "  server/private.ts\n"},
		{"restricted-imports", baseline.Issue{Kind: "module", Request: "forbidden/subpath"}, "     ➞ forbidden/subpath\n", "  forbidden/subpath\n"},
		{"restricted-importers", baseline.Issue{EntryPoint: "src/main.ts", Kind: "file", Target: "server/private.ts"}, "    src/main.ts\n     ➞ server/private.ts\n", "  server/private.ts  src/main.ts\n"},
		{"restricted-importers", baseline.Issue{EntryPoint: "src/main.ts", Kind: "module", Module: "forbidden"}, "    src/main.ts\n     ➞ forbidden\n", "  module forbidden  src/main.ts\n"},
		{"restricted-direct-importers", baseline.Issue{File: "src/main.ts", Kind: "file", Target: "server/private.ts"}, "    src/main.ts\n     ➞ server/private.ts\n", "  server/private.ts  src/main.ts\n"},
		{"restricted-direct-importers", baseline.Issue{File: "src/main.ts", Kind: "module", Request: "forbidden/subpath"}, "    src/main.ts\n     ➞ forbidden/subpath\n", "  module forbidden/subpath  src/main.ts\n"},
	}
	covered := map[string]bool{}
	for _, test := range cases {
		t.Run(test.detector+"/"+test.issue.Kind, func(t *testing.T) {
			result := &baseline.Result{
				Path: "baseline.json", Status: "different", Counts: baseline.Counts{Obsolete: 1},
				Obsolete: []baseline.Finding{{Workspace: "packages/app", Detector: test.detector, Issue: test.issue}},
			}
			console := formatFindingsBaselineConsole(result, "/project", false, nil)
			list := formatFindingsBaselineIssuesList(result, "/project")
			if !strings.Contains(console, test.console) || !strings.Contains(list, test.list) {
				t.Fatalf("identity is not readable:\nconsole:\n%s\nissues-list:\n%s", console, list)
			}
			for _, output := range []string{console, list} {
				if !strings.HasPrefix(output, "Baseline snapshot differs") || !strings.Contains(output, "Obsolete findings:") {
					t.Fatalf("missing summary, scope or detector: %s", output)
				}
				if strings.Contains(output, `"file":`) || strings.Contains(output, `"kind":`) || strings.Contains(output, ":0:0") {
					t.Fatalf("rendered JSON or invented source positions: %s", output)
				}
			}
		})
		covered[test.detector] = true
	}
	for _, detector := range baseline.Detectors() {
		if !covered[detector] {
			t.Errorf("obsolete findings lack rendering coverage for %s", detector)
		}
	}
}

func TestObsoleteFindingsGroupingAndAlignment(t *testing.T) {
	result := &baseline.Result{
		Path: "baseline.json", Status: "different", Counts: baseline.Counts{Obsolete: 3},
		Obsolete: []baseline.Finding{
			{Workspace: "packages/gone", Detector: "orphan-files", Issue: baseline.Issue{File: "packages/gone/old.ts"}},
			{Workspace: ".", Detector: "missing-node-modules", Issue: baseline.Issue{File: "src/b.ts", Module: "longer-package"}},
			{Workspace: ".", Detector: "missing-node-modules", Issue: baseline.Issue{File: "src/a.ts", Module: "pkg"}},
		},
	}
	list := formatFindingsBaselineIssuesList(result, "/project")
	want := "Missing Node Modules Issues (2):\n  pkg             src/a.ts\n  longer-package  src/b.ts\n"
	if !strings.Contains(list, want) || !strings.Contains(list, "Orphan Files Issues (1):") {
		t.Fatalf("issues list lost alignment or workspace scopes: %s", list)
	}
	console := formatFindingsBaselineConsole(result, "/project", false, nil)
	if !strings.Contains(console, "Workspace: . (root)") || !strings.Contains(console, "Workspace: packages/gone") {
		t.Fatalf("console lost workspace groups: %s", console)
	}
	// Display ordering must be independent of the supplied findings order and
	// formatting must preserve the result used by JSON output.
	original := slices.Clone(result.Obsolete)
	slices.Reverse(result.Obsolete)
	if got := formatFindingsBaselineIssuesList(result, "/project"); got != list {
		t.Fatalf("issues-list order changed: %s", got)
	}
	if got := formatFindingsBaselineConsole(result, "/project", false, nil); got != console {
		t.Fatalf("console order changed: %s", got)
	}
	slices.Reverse(result.Obsolete)
	for i := range original {
		if baseline.FindingKey(original[i]) != baseline.FindingKey(result.Obsolete[i]) {
			t.Fatal("formatting mutated baseline findings")
		}
	}
}

func TestObsoleteIssuesListUsesCurrentOrdering(t *testing.T) {
	// Supply workspaces and findings in reverse order. The detector order must
	// take precedence over workspace order, and rows use location then label.
	var findings []baseline.Finding
	var current []jsonRuleResult
	for _, workspace := range []string{"packages/z", "packages/a"} {
		file := workspace + "/main.ts"
		rule := config.RuleResult{
			RulePath: workspace, FileCount: 2,
			EnabledChecks: []string{"unused-exports", "missing-node-modules", "orphan-files"},
			OrphanFiles:   []string{"/project/" + workspace + "/orphan.ts"},
			UnusedExports: []checks.UnusedExport{{FilePath: "/project/" + file, ExportName: "oldExport"}},
		}
		isType := false
		findings = append(findings,
			baseline.Finding{Workspace: workspace, Detector: "unused-exports", Issue: baseline.Issue{File: file, Export: "oldExport", IsType: &isType}},
			baseline.Finding{Workspace: workspace, Detector: "orphan-files", Issue: baseline.Issue{File: workspace + "/orphan.ts"}},
		)
		for _, missing := range []struct{ file, module string }{
			{workspace + "/other.ts", "aaa"}, {file, "beta"}, {file, "alpha"},
		} {
			findings = append(findings, baseline.Finding{
				Workspace: workspace, Detector: "missing-node-modules", Issue: baseline.Issue{File: missing.file, Module: missing.module},
			})
			rule.MissingNodeModules = append(rule.MissingNodeModules, node.MissingNodeModuleResult{
				ModuleName: missing.module, ImportedFrom: []string{"/project/" + missing.file},
			})
		}
		current = append(current, buildJSONRuleResult(rule, "/project", nil))
	}
	result := &baseline.Result{Path: "baseline.json", Status: "different", Obsolete: findings}
	obsolete := formatFindingsBaselineIssuesList(result, "/project")
	assertOrder := func(output string, values []string) {
		t.Helper()
		previous := -1
		for _, value := range values {
			index := strings.Index(output, value)
			if index <= previous {
				t.Fatalf("missing or incorrectly ordered %q: %s", value, output)
			}
			previous = index
		}
	}
	assertOrder(obsolete, []string{"Orphan Files Issues (2)", "Missing Node Modules Issues (6)", "Unused Exports Issues (2)"})
	if strings.Contains(obsolete, "[workspace:") {
		t.Fatalf("obsolete findings were split into workspace groups: %s", obsolete)
	}
	regular := formatIssuesListOutput(current)
	assertOrder(regular, []string{"Orphan Files Issues", "Missing Node Modules Issues", "Unused Exports Issues"})
	rows := []string{
		"alpha  packages/a/main.ts", "beta   packages/a/main.ts", "aaa    packages/a/other.ts",
		"alpha  packages/z/main.ts", "beta   packages/z/main.ts", "aaa    packages/z/other.ts",
	}
	assertOrder(regular, rows)
	assertOrder(obsolete, rows)

	slices.Reverse(result.Obsolete)
	if got := formatFindingsBaselineIssuesList(result, "/project"); got != obsolete {
		t.Fatalf("ordering depends on baseline record order: %s", got)
	}
}

func TestObsoleteFindingsUseSharedDetectorLimits(t *testing.T) {
	result := &baseline.Result{Path: "baseline.json", Status: "different"}
	for _, workspace := range []string{"packages/a", "packages/b"} {
		for i := 0; i < maxIssuesToList+2; i++ {
			result.Obsolete = append(result.Obsolete, baseline.Finding{
				Workspace: workspace, Detector: "orphan-files",
				Issue: baseline.Issue{File: fmt.Sprintf("%s/old-%d.ts", workspace, i)},
			})
			result.Obsolete = append(result.Obsolete, baseline.Finding{
				Workspace: workspace, Detector: "unused-node-modules",
				Issue: baseline.Issue{PackageJSON: workspace + "/package.json", Module: fmt.Sprintf("unused-%d", i)},
			})
		}
	}
	limited := formatFindingsBaselineConsole(result, "/project", false, nil)
	if strings.Count(limited, "    - ") != 4*maxIssuesToList ||
		strings.Count(limited, "and 2 more orphan file issues") != 2 ||
		strings.Count(limited, "and 2 more unused node module issues") != 2 {
		t.Fatalf("obsolete findings do not use the usual per-workspace detector limits: %s", limited)
	}
	full := formatFindingsBaselineConsole(result, "/project", true, nil)
	if strings.Count(full, "    - ") != len(result.Obsolete) || strings.Contains(full, "more orphan file issues") || strings.Contains(full, "more unused node module issues") {
		t.Fatalf("--list-all-issues did not show every finding: %s", full)
	}
}
