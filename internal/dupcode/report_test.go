package dupcode

import (
	"strings"
	"testing"
	"time"

	"rev-dep-go/internal/parser"
)

func renderReport(t *testing.T, dups []Duplication, stats Stats, blinding Blinding) string {
	t.Helper()
	var sb strings.Builder
	PrintReport(&sb, dups, stats, blinding)
	return sb.String()
}

func sampleDups() []Duplication {
	return []Duplication{
		{
			Snippet: "{\n    const a = 1;\n    return a;\n  }",
			Occurrences: []Occurrence{
				{File: "src/a.ts", FileID: 1, Line: 10, Col: 3},
				{File: "src/b.ts", FileID: 2, Line: 44, Col: 5},
				{File: "src/b.ts", FileID: 2, Line: 90, Col: 5},
			},
			FileCount: 2, NormLen: 200, Lines: 4, Depth: 1, Statements: 2,
			IsStatementBlock: true,
			Kind:             parser.BlockBrace,
		},
		{
			Snippet:     "<Card>\n  <Body />\n</Card>",
			Occurrences: []Occurrence{{File: "src/C.tsx", FileID: 3, Line: 7, Col: 9}, {File: "src/D.tsx", FileID: 4, Line: 2, Col: 1}},
			FileCount:   2, NormLen: 150, Lines: 3, Depth: 1, Statements: 1,
			Kind: parser.BlockJSX,
		},
	}
}

func TestPrintReportListsEveryOccurrence(t *testing.T) {
	out := renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})

	for _, want := range []string{
		"src/a.ts:10:3", "src/b.ts:44:5", "src/b.ts:90:5", "src/C.tsx:7:9", "src/D.tsx:2:1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing location %q:\n%s", want, out)
		}
	}
}

func TestPrintReportHeaderStatesCountsAndMetrics(t *testing.T) {
	out := renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})

	if !strings.Contains(out, "#1  3 duplicates in 2 files") {
		t.Errorf("first header does not state duplicate and file counts:\n%s", out)
	}
	if !strings.Contains(out, "code block, 4 lines, depth 1, 2 statements") {
		t.Errorf("first header does not state the metrics:\n%s", out)
	}
	if !strings.Contains(out, "#2  2 duplicates in 2 files") {
		t.Errorf("second header wrong:\n%s", out)
	}
	if !strings.Contains(out, "JSX block") {
		t.Errorf("JSX kind not shown:\n%s", out)
	}
}

func TestPrintReportSingularForms(t *testing.T) {
	dups := []Duplication{{
		Snippet:     "{ x() }",
		Occurrences: []Occurrence{{File: "a.ts", Line: 1, Col: 1}},
		FileCount:   1, Lines: 1, Statements: 1, IsStatementBlock: true, Kind: parser.BlockBrace,
	}}
	out := renderReport(t, dups, Stats{Files: 1}, Blinding{})

	for _, want := range []string{"1 duplicate in 1 file", "1 line", "1 statement"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected singular %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "1 duplicates") || strings.Contains(out, "1 lines") {
		t.Errorf("plural used for a count of one:\n%s", out)
	}
}

func TestPrintReportShowsSnippetWithGutter(t *testing.T) {
	out := renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})
	if !strings.Contains(out, "  | {") {
		t.Errorf("snippet is not rendered with a gutter:\n%s", out)
	}
	// The body sits at indent 4 and its closing brace at indent 2, so 2 is the common
	// indent: it comes off, and the one level of relative indentation is preserved.
	if !strings.Contains(out, "  |   const a = 1;") {
		t.Errorf("common indentation was not stripped to the closing brace:\n%s", out)
	}
	if !strings.Contains(out, "  | }") {
		t.Errorf("closing brace should end up flush with the opening one:\n%s", out)
	}
}

func TestPrintReportEmptyResult(t *testing.T) {
	out := renderReport(t, nil, Stats{Files: 42, Total: 3 * time.Millisecond}, Blinding{Identifiers: true})
	if !strings.Contains(out, "No duplicated code found") {
		t.Errorf("empty report wrong:\n%s", out)
	}
	if !strings.Contains(out, "identifiers comparison") {
		t.Errorf("empty report does not name the blinding:\n%s", out)
	}
	if !strings.Contains(out, "42 files") {
		t.Errorf("empty report does not state how much was scanned:\n%s", out)
	}
}

func TestPrintReportTotalsLine(t *testing.T) {
	out := renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})
	if !strings.Contains(out, "2 duplicated patterns in 4 files across 5 occurrences.") {
		t.Errorf("totals line wrong:\n%s", out)
	}
}

func TestBlindingStringNames(t *testing.T) {
	if got := (Blinding{}).String(); got != "exact" {
		t.Errorf("zero value names %q, want exact", got)
	}
	if got := (Blinding{Identifiers: true}).String(); got != "identifiers" {
		t.Errorf("identifier blinding names %q", got)
	}
}

// TestPrintReportSeparatesFoundFromScanned pins the distinction the headline used to blur:
// the "in N files" of the found-sentence counts files that CONTAIN duplicates, matching
// what a config run reports, while how much was searched is stated separately.
func TestPrintReportSeparatesFoundFromScanned(t *testing.T) {
	// Four distinct files hold duplicates; 12 were scanned.
	out := renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})

	if !strings.Contains(out, "Found 2 duplicated patterns in 4 files (5 occurrences, exact comparison)") {
		t.Errorf("headline does not report what was found:\n%s", out)
	}
	if !strings.Contains(out, "Scanned 12 files.") {
		t.Errorf("scanned count is not stated as its own fact:\n%s", out)
	}
	if strings.Contains(out, "patterns in 12 files") {
		t.Errorf("the scanned count leaked into the found-sentence:\n%s", out)
	}
}

func TestPrintReportStatesIgnoredFiles(t *testing.T) {
	out := renderReport(t, sampleDups(), Stats{Files: 12, IgnoredFiles: 3}, Blinding{})
	if !strings.Contains(out, "Scanned 12 files, 3 ignored.") {
		t.Errorf("ignored count not reported:\n%s", out)
	}

	// With nothing ignored the clause is left off rather than printing a zero.
	out = renderReport(t, sampleDups(), Stats{Files: 12}, Blinding{})
	if strings.Contains(out, "0 ignored") {
		t.Errorf("a zero ignored-count should not be printed:\n%s", out)
	}
}
