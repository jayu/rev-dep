package dupcode

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/plural"
)

// PrintReport writes the duplications in the order Detect returned them - most copies first.
func PrintReport(w io.Writer, dups []Duplication, stats Stats, blinding Blinding) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if len(dups) == 0 {
		fmt.Fprintf(bw, "%s No duplicated code found (%s comparison).\n", emoji.Success, blinding)
		fmt.Fprintf(bw, "%s in %s.\n", scannedSummary(stats), roundDuration(stats.Total))
		return
	}

	totalOccurrences := 0
	// By file id rather than path, so the figure is right under CountOnly.
	affected := map[int32]struct{}{}
	for _, d := range dups {
		totalOccurrences += len(d.Occurrences)
		for _, o := range d.Occurrences {
			affected[o.FileID] = struct{}{}
		}
	}

	// The headline counts what was FOUND. How much was searched is a separate fact and is stated as
	// one - mixing the two made this line disagree with the config run's.
	fmt.Fprintf(bw, "Found %s in %s (%s, %s comparison)\n",
		plural.Count(len(dups), "duplicated pattern", "duplicated patterns"),
		plural.Count(len(affected), "file", "files"),
		plural.Count(totalOccurrences, "occurrence", "occurrences"),
		blinding)
	fmt.Fprintf(bw, "%s.\n\n", scannedSummary(stats))

	for i, d := range dups {
		fmt.Fprintf(bw, "#%d  %s in %s  -  %s\n",
			i+1,
			plural.Count(len(d.Occurrences), "duplicate", "duplicates"),
			plural.Count(d.FileCount, "file", "files"),
			FindingMetrics(d),
		)

		for _, o := range d.Occurrences {
			fmt.Fprintf(bw, "    %s:%d:%d\n", o.File, o.Line, o.Col)
		}

		bw.WriteString("\n")
		writeSnippet(bw, d.Snippet, "  ")
		bw.WriteString("\n")
	}

	fmt.Fprintf(bw, "%s in %s across %s.\n",
		plural.Count(len(dups), "duplicated pattern", "duplicated patterns"),
		plural.Count(len(affected), "file", "files"),
		plural.Count(totalOccurrences, "occurrence", "occurrences"))
}

// FindingMetrics describes the shape of a duplication.
//
// Statements are reported only for blocks that have them: "0 statements" beside an object
// literal would read as a measurement rather than the category error it is.
//
// Exported because the snapshot delta says the same thing about the same findings.
func FindingMetrics(d Duplication) string {
	kind := "code block"
	if d.Kind == parser.BlockJSX {
		kind = "JSX block"
	}
	metrics := fmt.Sprintf("%s, %s, depth %d", kind, plural.Count(d.Lines, "line", "lines"), d.Depth)
	if d.IsStatementBlock {
		metrics += ", " + plural.Count(d.Statements, "statement", "statements")
	}
	return metrics
}

// writeSnippet strips the common indentation and draws a gutter, so a block lifted out of five
// levels of nesting still reads. indent offsets the gutter itself.
func writeSnippet(bw *bufio.Writer, snippet string, indent string) {
	if snippet == "" {
		// CountOnly runs carry no snippet.
		return
	}
	lines := strings.Split(snippet, "\n")

	common := -1
	for i, l := range lines {
		if i == 0 || strings.TrimSpace(l) == "" {
			continue // the first line starts at the '{', so it has no indentation to measure
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if common < 0 || n < common {
			common = n
		}
	}
	if common < 0 {
		common = 0
	}

	for i, l := range lines {
		if i > 0 && len(l) >= common {
			l = l[common:]
		}
		bw.WriteString(indent)
		bw.WriteString("| ")
		bw.WriteString(strings.TrimRight(l, " \t\r"))
		bw.WriteString("\n")
	}
}

// scannedSummary is what makes a disagreement with a config run diagnosable: if the two report
// different numbers, the file counts say whether they were looking at the same thing.
func scannedSummary(s Stats) string {
	out := fmt.Sprintf("Scanned %s", plural.Count(s.Files, "file", "files"))
	if s.IgnoredFiles > 0 {
		out += fmt.Sprintf(", %d ignored", s.IgnoredFiles)
	}
	return out
}

// roundDuration reports in milliseconds rather than nanoseconds.
func roundDuration(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(10 * time.Microsecond)
	default:
		return d.Round(time.Microsecond)
	}
}
