package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/node"
)

func TestConfigFormattingPreservesResultOrder(t *testing.T) {
	cwd := t.TempDir()
	file := filepath.Join(cwd, "src/main.ts")
	rule := config.RuleResult{
		RulePath: ".", FileCount: 1,
		EnabledChecks: []string{"import-conventions", "unresolved-imports", "unused-exports", "restricted-imports"},
		ImportConventionViolations: []checks.ImportConventionViolation{
			{FilePath: file, ImportRequest: "@utils/b", ImportIndex: 2},
			{FilePath: file, ImportRequest: "@utils/a", ImportIndex: 1},
		},
		UnresolvedImports: []checks.UnresolvedImport{
			{FilePath: file, Request: "./b", RequestStart: 20},
			{FilePath: file, Request: "./a", RequestStart: 10},
		},
		UnusedExports: []checks.UnusedExport{
			{FilePath: file, ExportName: "B"}, {FilePath: file, ExportName: "A"},
		},
		RestrictedImportsViolations: []checks.RestrictedImportViolation{
			{EntryPoint: file, ViolationType: "module", DeniedModule: "b"},
			{EntryPoint: file, ViolationType: "module", DeniedModule: "a"},
		},
	}
	result := &config.ConfigProcessingResult{RuleResults: []config.RuleResult{rule}, HasFailures: true}
	jsonResult := func() string {
		t.Helper()
		encoded, err := json.Marshal(buildJSONRuleResult(result.RuleResults[0], cwd, nil))
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	before := jsonResult()
	for _, listAll := range []bool{false, true} {
		formatConfigResults(result, cwd, listAll)
		formatIssuesListOutput([]jsonRuleResult{buildJSONRuleResult(result.RuleResults[0], cwd, nil)})
		if after := jsonResult(); after != before {
			t.Fatalf("rendering changed detector results:\nbefore: %s\nafter: %s", before, after)
		}
	}
}

func TestCurrentAndObsoleteFindingsShareDependencyGrouping(t *testing.T) {
	cwd := t.TempDir()
	rule := config.RuleResult{
		RulePath: ".", FileCount: 2, EnabledChecks: []string{"missing-node-modules"},
		MissingNodeModules: []node.MissingNodeModuleResult{{
			ModuleName: "missing", ImportedFrom: []string{filepath.Join(cwd, "src/a.ts"), filepath.Join(cwd, "src/b.ts")},
		}},
	}
	result := &baseline.Result{
		Path: "baseline.json", Status: "different", Counts: baseline.Counts{Obsolete: 2},
		Obsolete: []baseline.Finding{
			{Workspace: ".", Detector: "missing-node-modules", Issue: baseline.Issue{File: "src/b.ts", Module: "missing"}},
			{Workspace: ".", Detector: "missing-node-modules", Issue: baseline.Issue{File: "src/a.ts", Module: "missing"}},
		},
	}
	current := formatConfigRuleChecks(rule, cwd, false)
	obsolete := formatFindingsBaselineConsole(result, cwd, false, nil)
	if !strings.Contains(current, "missing (imported from: src/a.ts and 1 more files)") || !strings.Contains(obsolete, current) {
		t.Fatalf("current and obsolete dependencies use different layouts:\ncurrent:\n%s\nobsolete:\n%s", current, obsolete)
	}
	currentList := formatIssuesListOutput([]jsonRuleResult{buildJSONRuleResult(rule, cwd, nil)})
	obsoleteList := formatFindingsBaselineIssuesList(result, cwd)
	rows := "  missing  src/a.ts\n  missing  src/b.ts\n"
	if !strings.Contains(currentList, rows) || !strings.Contains(obsoleteList, rows) {
		t.Fatalf("current and obsolete dependency rows differ:\ncurrent:\n%s\nobsolete:\n%s", currentList, obsoleteList)
	}
}

func TestConfigFormattersReturnOutputWithoutPrinting(t *testing.T) {
	cwd := t.TempDir()
	baselineResult := &baseline.Result{
		Path: "baseline.json", Status: "different", Counts: baseline.Counts{Obsolete: 1},
		Obsolete: []baseline.Finding{{Workspace: ".", Detector: "orphan-files", Issue: baseline.Issue{File: "src/old.ts"}}},
	}
	result := &config.ConfigProcessingResult{
		Baseline: baselineResult, HasFailures: true,
		RuleResults: []config.RuleResult{{
			RulePath: ".", FileCount: 1, EnabledChecks: []string{"orphan-files"},
			OrphanFiles: []string{filepath.Join(cwd, "src/new.ts")},
		}},
	}
	var console, list string
	printed, err := captureStdout(t, func() error {
		console = formatConfigResults(result, cwd, false)
		list = formatFindingsBaselineIssuesList(baselineResult, cwd) +
			formatIssuesListOutput([]jsonRuleResult{buildJSONRuleResult(result.RuleResults[0], cwd, nil)})
		return nil
	})
	if err != nil || printed != "" {
		t.Fatalf("formatting wrote directly to stdout: %q, %v", printed, err)
	}
	for _, output := range []string{console, list} {
		if strings.Count(output, "Baseline snapshot differs") != 1 ||
			!strings.Contains(output, "src/old.ts") || !strings.Contains(output, "src/new.ts") {
			t.Fatalf("formatting lost part of the composed report: %s", output)
		}
	}
}

func requireOutputOrder(t *testing.T, output string, tokens ...string) {
	t.Helper()
	offset := 0
	for _, token := range tokens {
		index := strings.Index(output[offset:], token)
		if index < 0 {
			t.Fatalf("missing or incorrectly ordered %q:\n%s", token, output)
		}
		offset += index + len(token)
	}
}

func TestDefaultSectionsUseConfigWorkspaceAndDetectorOrder(t *testing.T) {
	cwd := t.TempDir()
	isType := false
	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath: "./packages/z", FileCount: 1,
				DetectorOrder:      []string{"unused-exports", "orphan-files", "missing-node-modules"},
				EnabledChecks:      []string{"unused-exports", "missing-node-modules"},
				UnusedExports:      []checks.UnusedExport{{FilePath: filepath.Join(cwd, "packages/z/main.ts"), ExportName: "newExport"}},
				MissingNodeModules: []node.MissingNodeModuleResult{{ModuleName: "new-package", ImportedFrom: []string{filepath.Join(cwd, "packages/z/main.ts")}}},
			},
			{
				RulePath: "packages/a", FileCount: 1,
				DetectorOrder:      []string{"missing-node-modules", "unused-exports"},
				EnabledChecks:      []string{"missing-node-modules", "unused-exports"},
				UnusedExports:      []checks.UnusedExport{{FilePath: filepath.Join(cwd, "packages/a/main.ts"), ExportName: "newExport"}},
				MissingNodeModules: []node.MissingNodeModuleResult{{ModuleName: "new-package", ImportedFrom: []string{filepath.Join(cwd, "packages/a/main.ts")}}},
			},
		},
		Baseline: &baseline.Result{
			Path: "baseline.json", Status: "different",
			Obsolete: []baseline.Finding{
				{Workspace: "packages/a", Detector: "unused-exports", Issue: baseline.Issue{File: "packages/a/main.ts", Export: "oldExport", IsType: &isType}},
				{Workspace: "packages/a", Detector: "missing-node-modules", Issue: baseline.Issue{File: "packages/a/main.ts", Module: "old-package"}},
				{Workspace: "packages/z", Detector: "missing-node-modules", Issue: baseline.Issue{File: "packages/z/main.ts", Module: "old-package"}},
				{Workspace: "packages/z", Detector: "unused-exports", Issue: baseline.Issue{File: "packages/z/main.ts", Export: "oldExport", IsType: &isType}},
				{Workspace: "packages/z", Detector: "orphan-files", Issue: baseline.Issue{File: "packages/z/old.ts"}},
				{Workspace: "packages/deleted", Detector: "orphan-files", Issue: baseline.Issue{File: "packages/deleted/old.ts"}},
			},
		},
	}
	output := formatConfigResults(result, cwd, false)
	sections := strings.SplitN(output, findingsBaselineHint, 2)
	if len(sections) != 2 {
		t.Fatalf("missing baseline/main sections: %s", output)
	}
	requireOutputOrder(t, sections[0], "Workspace: ./packages/z", "Unused Exports Issues", "Orphan Files Issues", "Missing Node Modules Issues",
		"Workspace: packages/a", "Missing Node Modules Issues", "Unused Exports Issues", "Workspace: packages/deleted")
	requireOutputOrder(t, sections[1], "Workspace: ./packages/z", "Unused Exports Issues", "Missing Node Modules Issues",
		"Workspace: packages/a", "Missing Node Modules Issues", "Unused Exports Issues")
	// Removing the disabled detector's property has no current config position;
	// it follows configured detectors rather than displacing either one.
	result.RuleResults[0].DetectorOrder = []string{"unused-exports", "missing-node-modules"}
	obsolete := formatFindingsBaselineConsole(result.Baseline, cwd, false, result.RuleResults)
	rootSection := strings.SplitN(obsolete, "Workspace: packages/a", 2)[0]
	requireOutputOrder(t, rootSection, "Unused Exports Issues", "Missing Node Modules Issues", "Orphan Files Issues")
}

func TestDefaultImportsKeepSourceOrderAndObsoleteFallback(t *testing.T) {
	cwd := t.TempDir()
	file := filepath.Join(cwd, "main.ts")
	rule := config.RuleResult{
		RulePath: ".", FileCount: 1, EnabledChecks: []string{"import-conventions", "unresolved-imports"},
		ImportConventionViolations: []checks.ImportConventionViolation{
			{FilePath: file, ImportRequest: "@a", ImportIndex: 10},
			{FilePath: file, ImportRequest: "@z", ImportIndex: 2},
		},
		UnresolvedImports: []checks.UnresolvedImport{
			{FilePath: file, Request: "./a", RequestStart: 100},
			{FilePath: file, Request: "./z", RequestStart: 20},
		},
	}
	current := formatConfigRuleChecks(rule, cwd, true)
	requireOutputOrder(t, current, `"@z"`, `"@a"`, "./z", "./a")
	result := &baseline.Result{
		Path: "baseline.json", Status: "different",
		Obsolete: []baseline.Finding{
			{Workspace: ".", Detector: "import-conventions", Issue: baseline.Issue{File: "main.ts", Request: "@z", ViolationType: "should-be-relative"}},
			{Workspace: ".", Detector: "import-conventions", Issue: baseline.Issue{File: "main.ts", Request: "@a", ViolationType: "should-be-relative"}},
			{Workspace: ".", Detector: "unresolved-imports", Issue: baseline.Issue{File: "main.ts", Request: "./z", Kind: "literal"}},
			{Workspace: ".", Detector: "unresolved-imports", Issue: baseline.Issue{File: "main.ts", Request: "./a", Kind: "literal"}},
		},
	}
	obsolete := formatFindingsBaselineConsole(result, cwd, true, []config.RuleResult{rule})
	requireOutputOrder(t, obsolete, `"@a"`, `"@z"`, "./a", "./z")
}

func TestDefaultDevDependencyLimitIsDeterministic(t *testing.T) {
	cwd := t.TempDir()
	rule := config.RuleResult{RulePath: ".", FileCount: 7, EnabledChecks: []string{"dev-deps-usage-on-prod"}}
	for _, file := range []string{"z.ts", "f.ts", "e.ts", "d.ts", "c.ts", "b.ts", "a.ts"} {
		rule.RestrictedDevDependenciesUsageViolations = append(rule.RestrictedDevDependenciesUsageViolations,
			checks.RestrictedDevDependenciesUsageViolation{DevDependency: "dev-only", FilePath: filepath.Join(cwd, file)})
	}
	limited := formatConfigRuleChecks(rule, cwd, false)
	requireOutputOrder(t, limited, "a.ts", "b.ts", "c.ts", "d.ts", "e.ts", "and 2 more")
	if strings.Contains(limited, "f.ts") || strings.Contains(limited, "z.ts") {
		t.Fatalf("limited before sorting: %s", limited)
	}
	if rule.RestrictedDevDependenciesUsageViolations[0].FilePath != filepath.Join(cwd, "z.ts") {
		t.Fatal("sorting mutated diagnostic order")
	}
}

func TestIssuesListRetainsGlobalGroupsAndLocationComparison(t *testing.T) {
	checksFor := func(module, orphan string, line int) jsonChecks {
		return jsonChecks{
			OrphanFiles: &jsonCheckResult{Issues: []interface{}{jsonOrphanFileIssue{FilePath: orphan}}},
			MissingNodeModules: &jsonCheckResult{Issues: []interface{}{jsonMissingNodeModuleIssue{
				ModuleName: module, Locations: []jsonLocation{{FilePath: "shared/main.ts", StartLine: line, StartCol: 1}},
			}}},
		}
	}
	rules := []jsonRuleResult{
		{Path: "packages/z", Checks: checksFor("line-two", "packages/z/orphan.ts", 2)},
		{Path: "packages/a", Checks: checksFor("line-ten", "packages/a/orphan.ts", 10)},
	}
	output := formatIssuesListOutput(rules)
	if strings.Count(output, "Orphan Files Issues (2):") != 1 || strings.Count(output, "Missing Node Modules Issues (2):") != 1 || strings.Contains(output, "workspace:") {
		t.Fatalf("global detector groups changed: %s", output)
	}
	requireOutputOrder(t, output, "Orphan Files Issues", "packages/a/orphan.ts", "packages/z/orphan.ts", "Missing Node Modules Issues", "line-ten", "line-two")
}
