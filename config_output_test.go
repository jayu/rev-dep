package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/golden"
)

func TestConfigOutput_Limiting(t *testing.T) {
	// Create a dummy result with 6 orphan files and 6 module boundary violations
	cwd, _ := os.Getwd()

	orphanFiles := []string{
		"src/orphan1.ts",
		"src/orphan2.ts",
		"src/orphan3.ts",
		"src/orphan4.ts",
		"src/orphan5.ts",
		"src/orphan6.ts",
	}

	moduleBoundaryViolations := []ModuleBoundaryViolation{
		{FilePath: "src/file1.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file2.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file3.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file4.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file5.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
		{FilePath: "src/file6.ts", ImportPath: "src/forbidden.ts", RuleName: "rule", ViolationType: "not_allowed"},
	}

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
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
	golden.Assert(t, output, "config_output_limiting.golden")
}

func TestConfigOutput_UnusedExports_Sorting(t *testing.T) {
	// Create a dummy result with unordered unused exports
	cwd, _ := os.Getwd()
	fileA := "src/components/Button.tsx"
	fileB := "src/utils/helpers.ts"
	fileC := "src/api/client.ts"

	unusedExports := []UnusedExport{
		{FilePath: fileB, ExportName: "formatDate", IsType: false},
		{FilePath: fileA, ExportName: "ButtonProps", IsType: true},
		{FilePath: fileC, ExportName: "fetchData", IsType: false},
		{FilePath: fileA, ExportName: "Button", IsType: false},
		{FilePath: fileB, ExportName: "parseDate", IsType: false},
	}

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
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

	golden.Assert(t, output, "unused_exports_sorting.golden")
}

func TestConfigOutput_UnusedExports_Limiting(t *testing.T) {
	// Test that limiting picks the FIRST sorted items
	cwd, _ := os.Getwd()

	// Generate 10 exports across 3 files
	// Sorted Order should be:
	// File A: Export 1, Export 2, Export 3
	// File B: Export 1, Export 2, Export 3
	// File C: Export 1, Export 2, Export 3, Export 4

	files := []string{"src/A.ts", "src/B.ts", "src/C.ts"}
	var unusedExports []UnusedExport

	// Add in reverse order to ensure sorting works
	for i := len(files) - 1; i >= 0; i-- {
		// File C will have 4 exports, others 3
		count := 3
		if i == 2 {
			count = 4
		}

		for j := count; j >= 1; j-- {
			unusedExports = append(unusedExports, UnusedExport{
				FilePath:   files[i],
				ExportName: strings.Repeat("Export", 1) + string(rune('0'+j)), // Export3, Export2...
				IsType:     false,
			})
		}
	}

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
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
	golden.Assert(t, output, "unused_exports_limiting.golden")
}

func TestConfigOutput_RestrictedImports_GroupByEntryPoint(t *testing.T) {
	cwd, _ := os.Getwd()

	result := &ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []RuleResult{
			{
				RulePath:      ".",
				FileCount:     10,
				EnabledChecks: []string{"restricted-imports"},
				RestrictedImportsViolations: []RestrictedImportViolation{
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
