package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/golden"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/testutil"
)

func TestConfigOutput_Limiting(t *testing.T) {
	// Create a dummy result with 6 orphan files and 6 module boundary violations
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	orphanFiles := []string{
		"src/orphan1.ts",
		"src/orphan2.ts",
		"src/orphan3.ts",
		"src/orphan4.ts",
		"src/orphan5.ts",
		"src/orphan6.ts",
	}

	moduleBoundaryViolations := []checks.ModuleBoundaryViolation{
		{FilePath: "src/file1.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file2.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file3.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file4.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file5.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file6.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
	}

	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath:                 ".",
				FileCount:                10,
				EnabledChecks:            []string{"orphan-files", "module-boundaries"},
				OrphanFiles:              orphanFiles,
				ModuleBoundaryViolations: moduleBoundaryViolations,
			},
		},
	}

	// Capture output with limiting
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, cwd, false)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	output := buf.String()
	golden.Assert(t, output, goldenPath(t, "config_output_limiting.golden"))
}

func TestConfigOutput_UnusedExports_Sorting(t *testing.T) {
	// Create a dummy result with unordered unused exports
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	fileA := "src/components/Button.tsx"
	fileB := "src/utils/helpers.ts"
	fileC := "src/api/client.ts"

	unusedExports := []checks.UnusedExport{
		{FilePath: fileB, ExportName: "formatDate", IsType: false},
		{FilePath: fileA, ExportName: "ButtonProps", IsType: true},
		{FilePath: fileC, ExportName: "fetchData", IsType: false},
		{FilePath: fileA, ExportName: "Button", IsType: false},
		{FilePath: fileB, ExportName: "parseDate", IsType: false},
	}

	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				EnabledChecks: []string{"unused-exports"},
				UnusedExports: unusedExports,
			},
		},
	}

	// Capture output
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, cwd, true) // listAvailable = true to see all

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	output := buf.String()

	golden.Assert(t, output, goldenPath(t, "unused_exports_sorting.golden"))
}

func TestConfigOutput_UnusedExports_Limiting(t *testing.T) {
	// Test that limiting picks the FIRST sorted items
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	// Generate 10 exports across 3 files
	// Sorted Order should be:
	// File A: Export 1, Export 2, Export 3
	// File B: Export 1, Export 2, Export 3
	// File C: Export 1, Export 2, Export 3, Export 4

	files := []string{"src/A.ts", "src/B.ts", "src/C.ts"}
	var unusedExports []checks.UnusedExport

	// Add in reverse order to ensure sorting works
	for i := len(files) - 1; i >= 0; i-- {
		// File C will have 4 exports, others 3
		count := 3
		if i == 2 {
			count = 4
		}

		for j := count; j >= 1; j-- {
			unusedExports = append(unusedExports, checks.UnusedExport{
				FilePath:   files[i],
				ExportName: strings.Repeat("Export", 1) + string(rune('0'+j)), // Export3, Export2...
				IsType:     false,
			})
		}
	}

	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				UnusedExports: unusedExports,
				EnabledChecks: []string{"unused-exports"},
			},
		},
	}

	// Capture output with limiting (default max is 5)
	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, cwd, false) // listAll = false

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)

	output := buf.String()
	golden.Assert(t, output, goldenPath(t, "unused_exports_limiting.golden"))
}

func TestConfigOutput_RestrictedImports_GroupByEntryPoint(t *testing.T) {
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				EnabledChecks: []string{"restricted-imports"},
				RestrictedImportsViolations: []checks.RestrictedImportViolation{
					{
						ViolationType: "file",
						ImporterFile:  "src/server/a.ts",
						EntryPoint:    "src/server.ts",
						DeniedFile:    "src/ui/view.tsx",
					},
					{
						ViolationType: "file",
						ImporterFile:  "src/server/b.ts",
						EntryPoint:    "src/server.ts",
						DeniedFile:    "src/ui/view.tsx",
					},
					{
						ViolationType: "module",
						ImporterFile:  "src/server/a.ts",
						EntryPoint:    "src/server.ts",
						DeniedModule:  "react",
						ImportRequest: "react/jsx-runtime",
					},
					{
						ViolationType: "module",
						ImporterFile:  "src/worker/a.ts",
						EntryPoint:    "src/worker.ts",
						DeniedModule:  "react-dom",
						ImportRequest: "react-dom/client",
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, cwd, true)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "src/server.ts") {
		t.Fatalf("expected grouped output to contain src/server.ts, got:\n%s", output)
	}
	if !strings.Contains(output, "src/worker.ts") {
		t.Fatalf("expected grouped output to contain src/worker.ts, got:\n%s", output)
	}
	if !strings.Contains(output, "➞ src/ui/view.tsx") {
		t.Fatalf("expected grouped file item, got:\n%s", output)
	}
	if !strings.Contains(output, "➞ react/jsx-runtime") {
		t.Fatalf("expected grouped module item, got:\n%s", output)
	}
	if strings.Contains(output, "imports denied file") || strings.Contains(output, "imports denied module") {
		t.Fatalf("expected compact grouped format without importer phrasing, got:\n%s", output)
	}
}

func TestPrintRestrictedImportsResolveHint_UsesViolationIgnoreTypeFlag(t *testing.T) {
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	ruleResult := config.RuleResult{
		RulePath: ".",
		RestrictedImportsViolations: []checks.RestrictedImportViolation{
			{
				ViolationType: "file",
				EntryPoint:    filepath.Join(cwd, "src/entry-file.ts"),
				DeniedFile:    filepath.Join(cwd, "src/denied-file.ts"),
				IgnoreType:    false,
			},
			{
				ViolationType: "module",
				EntryPoint:    filepath.Join(cwd, "src/entry-module.ts"),
				DeniedModule:  "react",
				ImportRequest: "react/jsx-runtime",
				IgnoreType:    true,
				GraphExclude:  []string{"**/*.stories.tsx"},
			},
		},
	}

	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRestrictedImportsResolveHint(ruleResult, cwd)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "resolve --file") {
		t.Fatalf("expected file example in hint, got:\n%s", output)
	}
	if !strings.Contains(output, "resolve --module") {
		t.Fatalf("expected module example in hint, got:\n%s", output)
	}
	if strings.Contains(output, "resolve --file src/denied-file.ts --entry-points src/entry-file.ts --cwd . --ignore-type-imports") {
		t.Fatalf("expected file example to not include --ignore-type-imports, got:\n%s", output)
	}
	if !strings.Contains(output, "resolve --module react/jsx-runtime --entry-points src/entry-module.ts --cwd . --ignore-type-imports") {
		t.Fatalf("expected module example to include --ignore-type-imports, got:\n%s", output)
	}
	if !strings.Contains(output, "--graph-exclude \"**/*.stories.tsx\"") {
		t.Fatalf("expected module example to include --graph-exclude flag, got:\n%s", output)
	}
}

func TestConfigOutput_RestrictedImports_LimitingSortsBeforeTruncation(t *testing.T) {
	cwd, err := testutil.RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				EnabledChecks: []string{"restricted-imports"},
				RestrictedImportsViolations: []checks.RestrictedImportViolation{
					// Intentionally unsorted input.
					{ViolationType: "module", ImporterFile: "src/z.ts", EntryPoint: "src/z-entry.ts", DeniedModule: "react-z", ImportRequest: "react-z"},
					{ViolationType: "module", ImporterFile: "src/a.ts", EntryPoint: "src/a-entry.ts", DeniedModule: "react-a", ImportRequest: "react-a"},
					{ViolationType: "module", ImporterFile: "src/b.ts", EntryPoint: "src/b-entry.ts", DeniedModule: "react-b", ImportRequest: "react-b"},
					{ViolationType: "module", ImporterFile: "src/c.ts", EntryPoint: "src/c-entry.ts", DeniedModule: "react-c", ImportRequest: "react-c"},
					{ViolationType: "module", ImporterFile: "src/d.ts", EntryPoint: "src/d-entry.ts", DeniedModule: "react-d", ImportRequest: "react-d"},
					{ViolationType: "module", ImporterFile: "src/e.ts", EntryPoint: "src/e-entry.ts", DeniedModule: "react-e", ImportRequest: "react-e"},
				},
			},
		},
	}

	var buf bytes.Buffer
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatAndPrintConfigResults(result, cwd, false)

	w.Close()
	os.Stdout = originalStdout
	buf.ReadFrom(r)
	output := buf.String()

	// With sort-before-limit and maxIssuesToList=5, z-entry should be truncated.
	if strings.Contains(output, "src/z-entry.ts") {
		t.Fatalf("expected src/z-entry.ts to be excluded by sorted limiting, got:\n%s", output)
	}
	if !strings.Contains(output, "src/a-entry.ts") {
		t.Fatalf("expected src/a-entry.ts to be included, got:\n%s", output)
	}
	if !strings.Contains(output, "... and 1 more restricted import issues") {
		t.Fatalf("expected limiting summary, got:\n%s", output)
	}
}
