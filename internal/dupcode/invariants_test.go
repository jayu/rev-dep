package dupcode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/parser"
)

// These tests exist to survive refactors of the parser. They do not assert on any
// particular duplicate; they assert on properties that must hold whatever the scanner
// looks like inside, checked against real source files rather than handwritten snippets.

const fixtureProject = "../../__fixtures__/duplicatedCodeProject"

// realSourceFiles returns source to scan: the fixture project plus this repository's own
// TypeScript fixtures, which between them cover JSX, template literals, regex literals,
// decorators, generics and every import form the project supports.
func realSourceFiles(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, root := range []string{fixtureProject, "../../__fixtures__"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
			default:
				return nil
			}
			if strings.Contains(path, "node_modules") || info.Size() > 400_000 {
				return nil
			}
			content, err := os.ReadFile(path)
			if err == nil {
				out[path] = content
			}
			return nil
		})
	}
	if len(out) < 20 {
		t.Fatalf("expected a meaningful corpus, found %d files", len(out))
	}
	return out
}

// TestBlockNormalizationIsSelfConsistent is the load-bearing invariant of the whole
// design: the canonical bytes the scanner recorded for a block must be exactly what you
// get by normalising that block's original text on its own.
//
// Everything downstream depends on it. The hash is taken over the scanner's canonical
// buffer while the confirmation compares raw source through Compare, so if these two ever
// disagree the tool would either bucket things it then refuses to confirm, or - worse -
// confirm a pair whose canonical forms differ.
func TestBlockNormalizationIsSelfConsistent(t *testing.T) {
	files := realSourceFiles(t)

	for _, blinding := range []Blinding{Blinding{}, Blinding{Identifiers: true}} {
		for path, content := range files {
			scan := parser.ScanCodeBlocks(content, parser.BlockScanOptions{
				Blinding: blinding,
				AllowJSX: allowsJSX(path),
			}, nil)

			for _, b := range scan.Blocks {
				fromScan := string(scan.Norm[b.NormStart:b.NormEnd])
				standalone := string(Normalize(content[b.Start:b.End], blinding))
				if fromScan != standalone {
					t.Fatalf("%s [%s] block at %d:%d normalises differently in isolation\n scan:       %q\n standalone: %q",
						path, blinding, b.Start, b.End, truncate(fromScan), truncate(standalone))
				}
			}
		}
	}
}

// TestBlockBoundariesAreWellFormed checks that every reported block is a real, balanced
// region of the file rather than an artefact of a desynced scan.
func TestBlockBoundariesAreWellFormed(t *testing.T) {
	files := realSourceFiles(t)

	for path, content := range files {
		scan := parser.ScanCodeBlocks(content, parser.BlockScanOptions{
			Blinding: parser.Blinding{},
			AllowJSX: allowsJSX(path),
		}, nil)

		for _, b := range scan.Blocks {
			if b.Start >= b.End || int(b.End) > len(content) {
				t.Fatalf("%s: block offsets out of range: %+v (file is %d bytes)", path, b, len(content))
			}
			text := content[b.Start:b.End]
			switch b.Kind {
			case parser.BlockBrace:
				if text[0] != '{' || text[len(text)-1] != '}' {
					t.Errorf("%s: brace block is not brace-delimited: %q", path, truncate(string(text)))
				}
			case parser.BlockJSX:
				if text[0] != '<' || text[len(text)-1] != '>' {
					t.Errorf("%s: JSX block is not angle-delimited: %q", path, truncate(string(text)))
				}
			}
			if b.NormEnd <= b.NormStart || int(b.NormEnd) > len(scan.Norm) {
				t.Fatalf("%s: canonical offsets out of range: %+v", path, b)
			}
		}
	}
}

// TestNestedBlocksAreProperlyContained checks the stack discipline: a block that closes
// while another is open must lie entirely inside it.
func TestNestedBlocksAreProperlyContained(t *testing.T) {
	files := realSourceFiles(t)

	for path, content := range files {
		scan := parser.ScanCodeBlocks(content, parser.BlockScanOptions{
			Blinding: parser.Blinding{},
			AllowJSX: allowsJSX(path),
		}, nil)

		// Blocks close innermost-first, so any later block that starts before an
		// earlier one must also end after it - never straddle it.
		for i := range scan.Blocks {
			for j := i + 1; j < len(scan.Blocks); j++ {
				a, b := scan.Blocks[i], scan.Blocks[j]
				straddles := (a.Start < b.Start && a.End > b.Start && a.End < b.End) ||
					(b.Start < a.Start && b.End > a.Start && b.End < a.End)
				if straddles {
					t.Fatalf("%s: blocks partially overlap: %+v and %+v", path, a, b)
				}
			}
		}
	}
}

// TestReportedOccurrencesAllCompareEqual verifies the result end to end: whatever the
// hashing and bucketing did, every occurrence in a group must genuinely be the same code
// as the snippet that represents it.
func TestReportedOccurrencesAllCompareEqual(t *testing.T) {
	for _, blinding := range []Blinding{Blinding{}, Blinding{Identifiers: true}} {
		dups, _, err := Detect(Options{Cwd: fixtureProject, Blinding: blinding, MinTokens: 5, MinLines: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(dups) == 0 {
			t.Fatalf("[%s] fixture project produced no duplications", blinding)
		}

		for _, d := range dups {
			for _, o := range d.Occurrences {
				content, err := os.ReadFile(filepath.Join(fixtureProject, o.File))
				if err != nil {
					t.Fatal(err)
				}
				if int(o.End) > len(content) {
					t.Fatalf("[%s] %s:%d offsets past end of file", blinding, o.File, o.Line)
				}
				text := content[o.Start:o.End]
				if !Compare([]byte(d.Snippet), text, blinding) {
					t.Errorf("[%s] %s:%d does not compare equal to the reported snippet:\n snippet: %q\n found:   %q",
						blinding, o.File, o.Line, truncate(d.Snippet), truncate(string(text)))
				}
			}
		}
	}
}

// TestOccurrencesWithinAGroupDoNotOverlap guards against a group being built from a block
// and something inside itself.
func TestOccurrencesWithinAGroupDoNotOverlap(t *testing.T) {
	dups, _, err := Detect(Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 5, MinLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dups {
		for i := range d.Occurrences {
			for j := i + 1; j < len(d.Occurrences); j++ {
				a, b := d.Occurrences[i], d.Occurrences[j]
				if a.File != b.File {
					continue
				}
				if a.Start < b.End && b.Start < a.End {
					t.Errorf("group has overlapping occurrences in %s: %d-%d and %d-%d",
						a.File, a.Start, a.End, b.Start, b.End)
				}
			}
		}
	}
}

// TestReportedLocationsMatchLineAndColumn checks the line/column arithmetic against the
// file it points into.
func TestReportedLocationsMatchLineAndColumn(t *testing.T) {
	dups, _, err := Detect(Options{Cwd: fixtureProject, Blinding: Blinding{}, MinTokens: 5, MinLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dups {
		for _, o := range d.Occurrences {
			content, err := os.ReadFile(filepath.Join(fixtureProject, o.File))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(content), "\n")
			if o.Line < 1 || o.Line > len(lines) {
				t.Fatalf("%s: line %d out of range", o.File, o.Line)
			}
			line := lines[o.Line-1]
			if o.Col < 1 || o.Col > len(line) {
				t.Fatalf("%s:%d: column %d out of range for %q", o.File, o.Line, o.Col, line)
			}
			if c := line[o.Col-1]; c != '{' && c != '<' {
				t.Errorf("%s:%d:%d points at %q, not the start of a block", o.File, o.Line, o.Col, c)
			}
		}
	}
}

func truncate(s string) string {
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

// TestCountOnlyMatchesFullReport pins the promise CountOnly makes: it drops presentation,
// never analysis. The config run reports its numbers, so those numbers have to be the ones
// a full report would have shown.
func TestCountOnlyMatchesFullReport(t *testing.T) {
	for _, blinding := range []Blinding{Blinding{}, Blinding{Identifiers: true}} {
		for _, opts := range []Options{
			{MinTokens: 5, MinLines: 1},
			{MinTokens: 5, MinLines: 1, MinDepth: 1},
			{MinTokens: 5, MinLines: 1, MinStatements: 2},
			{MinTokens: 5, MinLines: 1, MinDuplicates: 3},
		} {
			opts.Cwd = fixtureProject
			opts.Blinding = blinding

			full, _, err := Detect(opts)
			if err != nil {
				t.Fatal(err)
			}
			opts.CountOnly = true
			counted, _, err := Detect(opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(full) != len(counted) {
				t.Fatalf("[%s] CountOnly found %d duplications, full report found %d",
					blinding, len(counted), len(full))
			}
			for i := range full {
				if len(full[i].Occurrences) != len(counted[i].Occurrences) {
					t.Errorf("[%s] #%d: %d occurrences under CountOnly, %d in full report",
						blinding, i+1, len(counted[i].Occurrences), len(full[i].Occurrences))
				}
				if full[i].FileCount != counted[i].FileCount {
					t.Errorf("[%s] #%d: FileCount %d vs %d", blinding, i+1, counted[i].FileCount, full[i].FileCount)
				}
				// Distinct-file counting downstream relies on FileID, which must be
				// populated in both modes even though File is not.
				for j, o := range counted[i].Occurrences {
					if o.FileID != full[i].Occurrences[j].FileID {
						t.Errorf("[%s] #%d occurrence %d: FileID %d vs %d",
							blinding, i+1, j, o.FileID, full[i].Occurrences[j].FileID)
					}
					if o.Start != full[i].Occurrences[j].Start {
						t.Errorf("[%s] #%d occurrence %d: offsets differ", blinding, i+1, j)
					}
				}
			}
		}
	}
}
