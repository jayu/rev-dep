package dupcode

import (
	"fmt"
	"strings"
	"testing"

	"rev-dep-go/internal/parser"
)

// This file pins the exact result of running the detector over the committed fixture
// project. It is deliberately literal: any change to the scanner, the normaliser, the
// hashing or the reporting that moves a boundary, drops a match or invents one will fail
// here with a readable diff, which is what makes a parser refactor safe to attempt.
//
// If a change to the fixture or an intentional behaviour change breaks these, update the
// expectations - but read the diff first.

type expectedDup struct {
	kind       parser.BlockKind
	lines      int
	depth      int
	statements int
	locations  []string // "path:line:col", sorted
	snippetHas string
}

func checkExpectations(t *testing.T, blinding Blinding, opts Options, want []expectedDup) {
	t.Helper()
	opts.Cwd = fixtureProject
	opts.Blinding = blinding
	got, _, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("[%s] got %d duplications, want %d:\n%s", blinding, len(got), len(want), describe(got))
	}
	for i, w := range want {
		g := got[i]
		if g.Kind != w.kind {
			t.Errorf("[%s] #%d kind = %v, want %v", blinding, i+1, g.Kind, w.kind)
		}
		if g.Lines != w.lines {
			t.Errorf("[%s] #%d lines = %d, want %d", blinding, i+1, g.Lines, w.lines)
		}
		if g.Depth != w.depth {
			t.Errorf("[%s] #%d depth = %d, want %d", blinding, i+1, g.Depth, w.depth)
		}
		if g.Statements != w.statements {
			t.Errorf("[%s] #%d statements = %d, want %d", blinding, i+1, g.Statements, w.statements)
		}
		var locs []string
		for _, o := range g.Occurrences {
			locs = append(locs, fmt.Sprintf("%s:%d:%d", o.File, o.Line, o.Col))
		}
		if strings.Join(locs, " ") != strings.Join(w.locations, " ") {
			t.Errorf("[%s] #%d locations =\n  %v\nwant\n  %v", blinding, i+1, locs, w.locations)
		}
		if !strings.Contains(g.Snippet, w.snippetHas) {
			t.Errorf("[%s] #%d snippet does not contain %q:\n%s", blinding, i+1, w.snippetHas, truncate(g.Snippet))
		}
	}
}

func describe(dups []Duplication) string {
	var b strings.Builder
	for i, d := range dups {
		fmt.Fprintf(&b, "  #%d kind=%v lines=%d depth=%d stmts=%d\n", i+1, d.Kind, d.Lines, d.Depth, d.Statements)
		for _, o := range d.Occurrences {
			fmt.Fprintf(&b, "      %s:%d:%d\n", o.File, o.Line, o.Col)
		}
		fmt.Fprintf(&b, "      snippet: %s\n", truncate(d.Snippet))
	}
	return b.String()
}

func TestCharacterizationLiteral(t *testing.T) {
	checkExpectations(t, Blinding{}, Options{MinTokens: 10, MinLines: 3}, []expectedDup{
		{
			// Identical JSX tree in two components, nested two levels deep.
			// JSX is an expression: three levels of elements, no statements.
			kind: parser.BlockJSX, lines: 7, depth: 3, statements: 0,
			locations:  []string{"src/components/Card.tsx:4:3", "src/components/Panel.tsx:4:3"},
			snippetHas: `<section className="panel panel--bordered">`,
		},
		{
			// adminApi is the same code as userApi but re-indented and commented
			// differently - this is the formatting/comment agnosticism guard.
			// Four statements: two consts, the `if` block, and the return.
			kind: parser.BlockBrace, lines: 9, depth: 2, statements: 4,
			locations:  []string{"src/services/adminApi.ts:3:53", "src/services/userApi.ts:4:84"},
			snippetHas: "TransportError",
		},
		{
			// A flat object literal: long values, no nesting. depth 0 is what
			// --min-depth uses to filter this kind of match out.
			// A flat object literal: one level, and no statements to count.
			kind: parser.BlockBrace, lines: 5, depth: 1, statements: 0,
			locations:  []string{"src/services/teamApi.ts:15:29", "src/services/userApi.ts:13:29"},
			snippetHas: "displayNamePreferenceForHeaderArea",
		},
	})
}

func TestCharacterizationStructural(t *testing.T) {
	checkExpectations(t, Blinding{Identifiers: true}, Options{MinTokens: 10, MinLines: 3}, []expectedDup{
		{
			// teamApi joins the group here: same shape, renamed locals. Three copies
			// sorts it above the two-copy patterns.
			kind: parser.BlockBrace, lines: 9, depth: 2, statements: 4,
			locations: []string{
				"src/services/adminApi.ts:3:53",
				"src/services/teamApi.ts:6:84",
				"src/services/userApi.ts:4:84",
			},
			snippetHas: "TransportError",
		},
		{
			// JSX is an expression: three levels of elements, no statements.
			kind: parser.BlockJSX, lines: 7, depth: 3, statements: 0,
			locations:  []string{"src/components/Card.tsx:4:3", "src/components/Panel.tsx:4:3"},
			snippetHas: `<section className="panel panel--bordered">`,
		},
		{
			// A flat object literal: one level, and no statements to count.
			kind: parser.BlockBrace, lines: 5, depth: 1, statements: 0,
			locations:  []string{"src/services/teamApi.ts:15:29", "src/services/userApi.ts:13:29"},
			snippetHas: "displayNamePreferenceForHeaderArea",
		},
	})
}

// TestCharacterizationFilters pins how each filter narrows that same fixture, so a
// refactor cannot quietly change what any of them means.
func TestCharacterizationFilters(t *testing.T) {
	count := func(opts Options) int {
		t.Helper()
		opts.Cwd = fixtureProject
		dups, _, err := Detect(opts)
		if err != nil {
			t.Fatal(err)
		}
		return len(dups)
	}

	base := Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 3}
	cases := []struct {
		name string
		opts Options
		want int
	}{
		{"baseline", base, 3},
		// Depth counts the block itself, so 1 admits everything.
		{"min-depth 1", withMinDepth(base, 1), 3},
		// Drops the flat object literal, which has nothing nested inside it.
		{"min-depth 2", withMinDepth(base, 2), 2},
		// The statement floor applies to the API body only. The object literal and the
		// JSX element are expressions, so it does not touch them at any value.
		{"min-statements 1", withMinStatements(base, 1), 3},
		{"min-statements 4", withMinStatements(base, 4), 3},
		{"min-statements 5", withMinStatements(base, 5), 2},
		// Nothing in the fixture is copied three times in literal blinding.
		{"min-duplicates 3", withMinDuplicates(base, 3), 0},
		// The JSX tree and the API body are both multi-line; the object is 5 lines.
		{"min-lines 7", withMinLines(base, 7), 2},
		// Only the two larger patterns clear a higher character floor.
		{"min-tokens 50", withMinTokens(base, 50), 2},
	}
	for _, tc := range cases {
		if got := count(tc.opts); got != tc.want {
			t.Errorf("%s: got %d duplications, want %d", tc.name, got, tc.want)
		}
	}
}

func withMinDepth(o Options, v int) Options      { o.MinDepth = v; return o }
func withMinStatements(o Options, v int) Options { o.MinStatements = v; return o }
func withMinDuplicates(o Options, v int) Options { o.MinDuplicates = v; return o }
func withMinLines(o Options, v int) Options      { o.MinLines = v; return o }
func withMinTokens(o Options, v int) Options     { o.MinTokens = v; return o }
