package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func captureJSONOutput(t *testing.T, result *ConfigProcessingResult, cwd string) jsonOutput {
	t.Helper()

	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	results := []RuleResult{}
	for _, rr := range result.RuleResults {
		results = append(results, rr)
	}

	output := jsonOutput{
		Version: "1.0",
		Rules:   []jsonRuleResult{},
	}
	if result.HasFailures {
		output.HasFailures = true
	}
	output.FixSummary.FixedFilesCount = result.FixedFilesCount
	output.FixSummary.FixedImportsCount = result.FixedImportsCount
	output.FixSummary.DeletedFilesCount = result.DeletedFilesCount
	output.FixSummary.FixableIssuesCount = result.FixableIssuesCount
	output.FixSummary.UnfixableAliasingCount = result.UnfixableAliasingCount

	for _, ruleResult := range result.RuleResults {
		output.Rules = append(output.Rules, buildJSONRuleResult(ruleResult, cwd))
	}

	json.NewEncoder(os.Stdout).Encode(output)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	var parsed jsonOutput
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw output: %s", err, buf.String())
	}
	return parsed
}

func TestJSONOutput_AllChecksPassing(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures: false,
		RuleResults: []RuleResult{
			{
				RulePath:      "src/",
				FileCount:     42,
				EnabledChecks: []string{"circular-imports", "orphan-files", "module-boundaries", "unused-node-modules", "missing-node-modules", "import-conventions", "unresolved-imports", "unused-exports", "restricted-dev-dependencies-usage", "restricted-imports"},
			},
		},
	}

	output := captureJSONOutput(t, result, cwd)

	if output.Version != "1.0" {
		t.Errorf("expected version '1.0', got '%s'", output.Version)
	}
	if output.HasFailures {
		t.Error("expected hasFailures to be false")
	}
	if len(output.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(output.Rules))
	}

	rule := output.Rules[0]
	if rule.Path != "src/" {
		t.Errorf("expected path 'src/', got '%s'", rule.Path)
	}
	if rule.FileCount != 42 {
		t.Errorf("expected fileCount 42, got %d", rule.FileCount)
	}

	// All checks should be present and passing
	checks := []struct {
		name   string
		result *jsonCheckResult
	}{
		{"circularDependencies", rule.Checks.CircularDependencies},
		{"orphanFiles", rule.Checks.OrphanFiles},
		{"moduleBoundaries", rule.Checks.ModuleBoundaries},
		{"unusedNodeModules", rule.Checks.UnusedNodeModules},
		{"missingNodeModules", rule.Checks.MissingNodeModules},
		{"importConventions", rule.Checks.ImportConventions},
		{"unresolvedImports", rule.Checks.UnresolvedImports},
		{"unusedExports", rule.Checks.UnusedExports},
		{"restrictedDevDependenciesUsage", rule.Checks.RestrictedDevDependenciesUsage},
		{"restrictedImports", rule.Checks.RestrictedImports},
	}

	for _, c := range checks {
		if c.result == nil {
			t.Errorf("expected %s to be present", c.name)
			continue
		}
		if c.result.Status != "pass" {
			t.Errorf("expected %s status 'pass', got '%s'", c.name, c.result.Status)
		}
		if len(c.result.Issues) != 0 {
			t.Errorf("expected %s to have 0 issues, got %d", c.name, len(c.result.Issues))
		}
	}
}

func TestJSONOutput_WithFailures(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
			{
				RulePath:  "src/",
				FileCount: 10,
				EnabledChecks: []string{
					"circular-imports",
					"orphan-files",
					"module-boundaries",
					"unused-exports",
					"import-conventions",
					"unresolved-imports",
					"unused-node-modules",
					"missing-node-modules",
					"restricted-dev-dependencies-usage",
					"restricted-imports",
				},
				CircularDependencies: [][]string{
					{"src/a.ts", "src/b.ts", "src/a.ts"},
				},
				OrphanFiles: []string{"src/orphan.ts"},
				ModuleBoundaryViolations: []ModuleBoundaryViolation{
					{
						RuleName:      "no-cross-feature",
						FilePath:      "src/featureA/index.ts",
						ImportPath:    "src/featureB/utils.ts",
						ViolationType: "not_allowed",
					},
				},
				UnusedExports: []UnusedExport{
					{FilePath: "src/utils.ts", ExportName: "unusedHelper", IsType: false},
					{FilePath: "src/types.ts", ExportName: "OldType", IsType: true},
				},
				ImportConventionViolations: []ImportConventionViolation{
					{FilePath: "src/comp.ts", ImportRequest: "../utils", ViolationType: "should-be-aliased"},
				},
				UnresolvedImports: []UnresolvedImport{
					{FilePath: "src/index.ts", Request: "./missing"},
				},
				UnusedNodeModules: []string{"lodash"},
				MissingNodeModules: []MissingNodeModuleResult{
					{ModuleName: "axios", ImportedFrom: []string{"src/api.ts"}},
				},
				RestrictedDevDependenciesUsageViolations: []RestrictedDevDependenciesUsageViolation{
					{DevDependency: "jest", FilePath: "src/prod.ts", EntryPoint: "src/index.ts"},
				},
				RestrictedImportsViolations: []RestrictedImportViolation{
					{ViolationType: "module", ImporterFile: "src/a.ts", EntryPoint: "src/index.ts", DeniedModule: "react-dom", ImportRequest: "react-dom/client"},
				},
			},
		},
		FixableIssuesCount: 3,
	}

	output := captureJSONOutput(t, result, cwd)

	if !output.HasFailures {
		t.Error("expected hasFailures to be true")
	}

	rule := output.Rules[0]

	// Circular dependencies
	if rule.Checks.CircularDependencies.Status != "fail" {
		t.Error("expected circularDependencies to fail")
	}
	if len(rule.Checks.CircularDependencies.Issues) != 1 {
		t.Fatalf("expected 1 circular dependency issue, got %d", len(rule.Checks.CircularDependencies.Issues))
	}

	// Orphan files
	if rule.Checks.OrphanFiles.Status != "fail" {
		t.Error("expected orphanFiles to fail")
	}
	if len(rule.Checks.OrphanFiles.Issues) != 1 {
		t.Fatalf("expected 1 orphan file issue, got %d", len(rule.Checks.OrphanFiles.Issues))
	}

	// Module boundaries
	if rule.Checks.ModuleBoundaries.Status != "fail" {
		t.Error("expected moduleBoundaries to fail")
	}
	if len(rule.Checks.ModuleBoundaries.Issues) != 1 {
		t.Fatalf("expected 1 module boundary issue, got %d", len(rule.Checks.ModuleBoundaries.Issues))
	}

	// Unused exports
	if rule.Checks.UnusedExports.Status != "fail" {
		t.Error("expected unusedExports to fail")
	}
	if len(rule.Checks.UnusedExports.Issues) != 2 {
		t.Fatalf("expected 2 unused export issues, got %d", len(rule.Checks.UnusedExports.Issues))
	}

	// Import conventions
	if rule.Checks.ImportConventions.Status != "fail" {
		t.Error("expected importConventions to fail")
	}

	// Unresolved imports
	if rule.Checks.UnresolvedImports.Status != "fail" {
		t.Error("expected unresolvedImports to fail")
	}

	// Unused node modules
	if rule.Checks.UnusedNodeModules.Status != "fail" {
		t.Error("expected unusedNodeModules to fail")
	}

	// Missing node modules
	if rule.Checks.MissingNodeModules.Status != "fail" {
		t.Error("expected missingNodeModules to fail")
	}

	// Restricted dev deps
	if rule.Checks.RestrictedDevDependenciesUsage.Status != "fail" {
		t.Error("expected restrictedDevDependenciesUsage to fail")
	}

	// Restricted imports
	if rule.Checks.RestrictedImports.Status != "fail" {
		t.Error("expected restrictedImports to fail")
	}

	// Fix summary
	if output.FixSummary.FixableIssuesCount != 3 {
		t.Errorf("expected fixableIssuesCount 3, got %d", output.FixSummary.FixableIssuesCount)
	}
}

func TestJSONOutput_OnlyEnabledChecks(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures: false,
		RuleResults: []RuleResult{
			{
				RulePath:      "src/",
				FileCount:     5,
				EnabledChecks: []string{"circular-imports", "orphan-files"},
			},
		},
	}

	output := captureJSONOutput(t, result, cwd)

	rule := output.Rules[0]

	// Enabled checks should be present
	if rule.Checks.CircularDependencies == nil {
		t.Error("expected circularDependencies to be present")
	}
	if rule.Checks.OrphanFiles == nil {
		t.Error("expected orphanFiles to be present")
	}

	// Disabled checks should be nil (omitted from JSON)
	if rule.Checks.ModuleBoundaries != nil {
		t.Error("expected moduleBoundaries to be nil")
	}
	if rule.Checks.UnusedExports != nil {
		t.Error("expected unusedExports to be nil")
	}
	if rule.Checks.ImportConventions != nil {
		t.Error("expected importConventions to be nil")
	}
}

func TestJSONOutput_IssueFieldValues(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
			{
				RulePath:  ".",
				FileCount: 5,
				EnabledChecks: []string{
					"module-boundaries",
					"unused-exports",
					"missing-node-modules",
				},
				ModuleBoundaryViolations: []ModuleBoundaryViolation{
					{RuleName: "boundary-rule", FilePath: "src/a.ts", ImportPath: "src/b.ts", ViolationType: "denied"},
				},
				UnusedExports: []UnusedExport{
					{FilePath: "src/utils.ts", ExportName: "MyType", IsType: true},
				},
				MissingNodeModules: []MissingNodeModuleResult{
					{ModuleName: "express", ImportedFrom: []string{"src/server.ts", "src/app.ts"}},
				},
			},
		},
	}

	output := captureJSONOutput(t, result, cwd)
	rule := output.Rules[0]

	// Validate module boundary issue fields via raw JSON
	mbRaw, _ := json.Marshal(rule.Checks.ModuleBoundaries.Issues[0])
	var mbIssue jsonModuleBoundaryIssue
	json.Unmarshal(mbRaw, &mbIssue)
	if mbIssue.RuleName != "boundary-rule" {
		t.Errorf("expected ruleName 'boundary-rule', got '%s'", mbIssue.RuleName)
	}
	if mbIssue.ViolationType != "denied" {
		t.Errorf("expected violationType 'denied', got '%s'", mbIssue.ViolationType)
	}

	// Validate unused export issue fields
	ueRaw, _ := json.Marshal(rule.Checks.UnusedExports.Issues[0])
	var ueIssue jsonUnusedExportIssue
	json.Unmarshal(ueRaw, &ueIssue)
	if ueIssue.ExportName != "MyType" {
		t.Errorf("expected exportName 'MyType', got '%s'", ueIssue.ExportName)
	}
	if !ueIssue.IsType {
		t.Error("expected isType to be true")
	}

	// Validate missing node module issue fields
	mnmRaw, _ := json.Marshal(rule.Checks.MissingNodeModules.Issues[0])
	var mnmIssue jsonMissingNodeModuleIssue
	json.Unmarshal(mnmRaw, &mnmIssue)
	if mnmIssue.ModuleName != "express" {
		t.Errorf("expected moduleName 'express', got '%s'", mnmIssue.ModuleName)
	}
	if len(mnmIssue.ImportedFrom) != 2 {
		t.Errorf("expected 2 importedFrom entries, got %d", len(mnmIssue.ImportedFrom))
	}
}

func TestJSONOutput_FixSummary(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures:            false,
		FixedFilesCount:        5,
		FixedImportsCount:      12,
		DeletedFilesCount:      3,
		FixableIssuesCount:     2,
		UnfixableAliasingCount: 1,
		RuleResults: []RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				EnabledChecks: []string{"circular-imports"},
			},
		},
	}

	output := captureJSONOutput(t, result, cwd)

	if output.FixSummary.FixedFilesCount != 5 {
		t.Errorf("expected fixedFilesCount 5, got %d", output.FixSummary.FixedFilesCount)
	}
	if output.FixSummary.FixedImportsCount != 12 {
		t.Errorf("expected fixedImportsCount 12, got %d", output.FixSummary.FixedImportsCount)
	}
	if output.FixSummary.DeletedFilesCount != 3 {
		t.Errorf("expected deletedFilesCount 3, got %d", output.FixSummary.DeletedFilesCount)
	}
	if output.FixSummary.FixableIssuesCount != 2 {
		t.Errorf("expected fixableIssuesCount 2, got %d", output.FixSummary.FixableIssuesCount)
	}
	if output.FixSummary.UnfixableAliasingCount != 1 {
		t.Errorf("expected unfixableAliasingCount 1, got %d", output.FixSummary.UnfixableAliasingCount)
	}
}
