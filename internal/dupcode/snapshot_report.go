package dupcode

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/plural"
)

// PrintDelta writes what changed relative to the snapshot. On a project with two hundred
// acknowledged duplicates, the four lines that moved are the point.
//
// Every reported finding is rendered FROM dups, the run's own result. The snapshot decides
// which findings to show; it cannot supply their content, because it stores a hash, a file list
// and a label. A delta built from snapshot entries could only print those three things.
//
// Resolved entries are the exception, unavoidably: that code no longer exists in the run.
func PrintDelta(w io.Writer, delta *Delta, stats Stats, blinding Blinding, snapshotPath string) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if delta.CanonicalFormChanged {
		fmt.Fprintf(bw, "%s The canonical form changed since this snapshot was written (v%s -> v%s).\n",
			"!", delta.SnapshotFormVersion, CanonicalFormVersion)
		fmt.Fprintf(bw, "  Code that did not change may still appear below; re-check before re-baselining.\n\n")
	}
	if len(delta.ParameterChanges) > 0 {
		fmt.Fprintf(bw, "Settings differ from the snapshot: %s.\n", strings.Join(delta.ParameterChanges, ", "))
		fmt.Fprintf(bw, "  Fewer or more duplications will qualify than when it was written.\n\n")
	}

	if delta.IsClean() {
		fmt.Fprintf(bw, "%s Duplicated code matches the snapshot: %s acknowledged, nothing new.\n",
			emoji.Success, plural.Count(delta.Unchanged, "pattern", "patterns"))
		fmt.Fprintf(bw, "%s.\n", scannedSummary(stats))
		return
	}

	fmt.Fprintf(bw, "Duplicated code changed since the snapshot: %s.\n\n", DeltaSummary(delta))
	for _, line := range flattenDelta(delta) {
		writeDeltaLine(bw, line)
	}

	fmt.Fprintf(bw, "%d unchanged. %s.\n", delta.Unchanged, scannedSummary(stats))
	writeVerdict(bw, delta)
	fmt.Fprintf(bw, "Snapshot: %s\n", snapshotPath)
}

// writeVerdict reports both directions when both happened: a run that resolves three patterns
// and introduces one is a common shape, and saying only "increased" hides work that was done.
func writeVerdict(bw *bufio.Writer, delta *Delta) {
	bw.WriteString("\n")
	switch {
	case delta.HasMoreDuplication() && delta.HasImprovements():
		fmt.Fprintf(bw, "%s Duplication increased.\n", emoji.Error)
		fmt.Fprintf(bw, "%s Some was also resolved, so the snapshot is out of date either way.\n",
			emoji.Success)
		bw.WriteString("   Fix what is new, or acknowledge all of it with --update-snapshot.\n")
	case delta.HasMoreDuplication():
		fmt.Fprintf(bw, "%s Duplication increased. Fix it, or acknowledge it with --update-snapshot.\n",
			emoji.Error)
	case len(delta.Moved) > 0 && !delta.HasImprovements():
		// Same amount of duplication, different files.
		fmt.Fprintf(bw, "%s Duplication moved to other files. Nothing was added, but the "+
			"snapshot names the wrong ones - refresh it with --update-snapshot.\n", emoji.Warning)
	case len(delta.Moved) > 0:
		fmt.Fprintf(bw, "%s Duplication moved to other files, and %s some was resolved. "+
			"Refresh the snapshot with --update-snapshot.\n", emoji.Warning, emoji.Success)
	default:
		fmt.Fprintf(bw, "%s Duplication decreased, so the snapshot is out of date. "+
			"Refresh it with --update-snapshot.\n", emoji.Success)
	}
}

// entryMarker is the verdict for ONE entry, from what changed about it rather than from the
// category it sits in.
//
// The two differ: a pattern gaining a copy in two files while disappearing from a third is
// filed under "spread" on its net count, and the category says only that it got worse. Some of
// it got better, and a reader deciding what to do next needs both facts.
func entryMarker(changes []FileChange) string {
	gained, lost := false, false
	for _, c := range changes {
		switch {
		case c.Now > c.Was:
			gained = true
		case c.Now < c.Was:
			lost = true
		}
	}
	switch {
	case gained && lost:
		return marker(emoji.Error, emoji.Success)
	case gained:
		return marker(emoji.Error)
	case lost:
		return marker(emoji.Success)
	default:
		// No file-level story to tell.
		return marker()
	}
}

// marker pads a verdict to a fixed column so the kind and hash line up.
//
// Glyphs arrive as separate arguments so this is arithmetic on a COUNT, not a measurement.
// Measuring was the bug: a rune count is right for ✅ and wrong for ⚠️, which is two runes and
// one cell. Every glyph in the emoji package is emoji.Cells wide by construction.
func marker(glyphs ...string) string {
	const slots = 2 // the most an entry can carry: gained AND lost
	return strings.Join(glyphs, "") + strings.Repeat(" ", (slots-len(glyphs))*emoji.Cells)
}

// A delta is computed as six categories but READ as one list. Grouping by category forces the
// reader through it in an order nobody chose - a resolved pattern and a new one four screens
// apart - and spends a heading saying what the entry line now says for itself.
const (
	kindNew      = "new"
	kindSpread   = "spread"
	kindMoved    = "moved"
	kindFewer    = "fewer"
	kindUnderMin = "under-min"
	kindResolved = "resolved"
)

// kindWidth pads the kind column.
const kindWidth = 9 // len("under-min")

type deltaLine struct {
	// rank orders the list worst first, so it can be abandoned where the reader stops caring.
	rank    int
	kind    string
	marker  string
	entry   SnapshotEntry
	was     *SnapshotEntry
	changes []FileChange
	finding *Duplication
	note    string
}

func flattenDelta(delta *Delta) []deltaLine {
	var out []deltaLine

	for _, e := range delta.New {
		note := ""
		if e.RevealedBy != "" {
			note = fmt.Sprintf("was nested inside %s, which is no longer duplicated - "+
				"this code did not change", e.RevealedBy)
		}
		out = append(out, deltaLine{
			rank: 0, kind: kindNew,
			// Everything about a new entry is new.
			marker: marker(emoji.Error),
			entry:  e.SnapshotEntry, finding: e.Finding, note: note,
		})
	}
	for _, c := range delta.Spread {
		out = append(out, lineForChange(1, kindSpread, c, entryMarker(c.FileChanges), ""))
	}
	for _, c := range delta.Moved {
		// Neither better nor worse. Every move gains in one file and loses in another, so a file-level
		// marker would always read as both, which overstates it. It still fails the check: a baseline
		// naming the wrong files is one nobody can review.
		out = append(out, lineForChange(2, kindMoved, c, marker(emoji.Warning), ""))
	}
	for _, c := range delta.Shrunk {
		out = append(out, lineForChange(3, kindFewer, c, entryMarker(c.FileChanges), ""))
	}
	for _, c := range delta.Dropped {
		// The copies that remain are the point, so the note says how many are left.
		note := fmt.Sprintf("still duplicated %s - below --min-duplicates %d, not gone",
			plural.Count(c.Now.Occurrences, "time", "times"), delta.MinDuplicates)
		out = append(out, lineForChange(4, kindUnderMin, c, entryMarker(c.FileChanges), note))
	}
	for _, e := range delta.Resolved {
		out = append(out, deltaLine{
			rank: 5, kind: kindResolved, marker: marker(emoji.Success), entry: e,
		})
	}

	// Within a rank, biggest first, then by hash so the report is the same on every run.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.entry.Occurrences != b.entry.Occurrences {
			return a.entry.Occurrences > b.entry.Occurrences
		}
		return a.entry.Hash < b.entry.Hash
	})
	return out
}

func lineForChange(rank int, kind string, c EntryChange, mark, note string) deltaLine {
	was := c.Was
	return deltaLine{
		rank: rank, kind: kind, marker: mark,
		entry: c.Now, was: &was, changes: c.FileChanges, finding: c.Finding, note: note,
	}
}

// writeDeltaLine prints one changed pattern: what kind of change, how its copy count moved,
// where every copy is now, and the code.
func writeDeltaLine(bw *bufio.Writer, l deltaLine) {
	counts := fmt.Sprintf("%s in %s",
		plural.Count(l.entry.Occurrences, "copy", "copies"),
		plural.Count(len(l.entry.Files), "file", "files"))
	if l.was != nil {
		counts = fmt.Sprintf("%d -> %d copies", l.was.Occurrences, l.entry.Occurrences)
	}

	if l.finding == nil {
		// The code is gone, so the snapshot's record is the only source and there are no metrics.
		fmt.Fprintf(bw, "  %s  %-*s  %s  was %s\n", l.marker, kindWidth, l.kind, l.entry.Hash, counts)
		writeVanishedBody(bw, l.entry, l.note)
		return
	}
	fmt.Fprintf(bw, "  %s  %-*s  %s  %s  -  %s\n", l.marker, kindWidth, l.kind,
		l.entry.Hash, counts, FindingMetrics(*l.finding))
	writeDeltaBody(bw, l.finding, l.changes, l.note)
}

// writeDeltaBody works at two levels of precision, because that is what the data supports.
//
// A file the snapshot did not list has every occurrence new, so those are marked individually.
// A file merely holding MORE copies cannot be resolved that far: the copies are identical by
// construction, so none is "the new one", and marking an arbitrary line would be a guess dressed
// as a fact. Those are reported per file with both counts.
func writeDeltaBody(bw *bufio.Writer, d *Duplication, changes []FileChange, note string) {
	newFiles := map[string]bool{}
	for _, c := range changes {
		if c.IsNew() {
			newFiles[c.Path] = true
		}
	}
	// The marker column only appears when something is marked.
	marked := len(changes) > 0

	for _, o := range d.Occurrences {
		prefix := "      "
		if marked {
			prefix = "        "
			if newFiles[o.File] {
				prefix = "      + "
			}
		}
		fmt.Fprintf(bw, "%s%s:%d:%d\n", prefix, o.File, o.Line, o.Col)
	}
	for _, c := range changes {
		if c.IsNew() {
			continue // already marked at each of its locations above
		}
		fmt.Fprintf(bw, "      %s\n", describeFileChange(c))
	}

	if note != "" {
		fmt.Fprintf(bw, "      (%s)\n", note)
	}
	if d.Snippet != "" {
		bw.WriteString("\n")
		writeSnippet(bw, d.Snippet, "      ")
	}
	bw.WriteString("\n")
}

// describeFileChange covers the files where no single occurrence can be pointed at.
func describeFileChange(c FileChange) string {
	switch {
	case c.IsGone():
		return fmt.Sprintf("- %s: %s gone, none left here", c.Path,
			plural.Count(c.Was, "copy", "copies"))
	case c.Now > c.Was:
		return fmt.Sprintf("+ %s: %d -> %d copies here", c.Path, c.Was, c.Now)
	default:
		return fmt.Sprintf("- %s: %d -> %d copies here", c.Path, c.Was, c.Now)
	}
}

// writeVanishedBody: the code is gone, so the snapshot's record is all there is. The label is
// worth showing here precisely because it is all there is.
func writeVanishedBody(bw *bufio.Writer, e SnapshotEntry, note string) {
	for _, f := range e.SortedFiles() {
		if n := e.Files[f]; n > 1 {
			fmt.Fprintf(bw, "      %s (%s)\n", f, plural.Count(n, "copy", "copies"))
			continue
		}
		fmt.Fprintf(bw, "      %s\n", f)
	}
	if e.Label != "" {
		fmt.Fprintf(bw, "      | %s\n", e.Label)
	}
	if note != "" {
		fmt.Fprintf(bw, "      (%s)\n", note)
	}
	bw.WriteString("\n")
}

// DeltaSummary is the one-line form, for where a full listing would not be useful.
func DeltaSummary(delta *Delta) string { return DeltaCountsSummary(CountsOf(delta)) }

// DeltaCounts is a delta reduced to its sizes, and the ONE shape those sizes have.
//
// Everything reporting a delta without listing it reads this struct. They used to carry their
// own copies, which is how a field went missing: a delta whose only change was below-threshold
// reported "nothing new" beside a failing status. The json tags are here because both JSON
// outputs embed this directly rather than translating it.
type DeltaCounts struct {
	New    int `json:"new"`
	Spread int `json:"spread"`
	Moved  int `json:"moved"`
	Fewer  int `json:"fewer"`
	// BelowThreshold is Delta.Dropped: still duplicated, but below MinDuplicates.
	BelowThreshold int `json:"belowThreshold"`
	Resolved       int `json:"resolved"`
	Unchanged      int `json:"unchanged"`
}

// CountsOf is the only way a DeltaCounts should be built. Constructing one field by field is
// what let a category be forgotten, and the compiler cannot catch a missing field in a literal.
func CountsOf(d *Delta) DeltaCounts {
	if d == nil {
		return DeltaCounts{}
	}
	return DeltaCounts{
		New:            len(d.New),
		Spread:         len(d.Spread),
		Moved:          len(d.Moved),
		Fewer:          len(d.Shrunk),
		BelowThreshold: len(d.Dropped),
		Resolved:       len(d.Resolved),
		Unchanged:      d.Unchanged,
	}
}

func (c DeltaCounts) IsClean() bool {
	return c.Changed() == 0
}

// Changed adds up every category, which is what a caller counting issues wants - and what it
// gets wrong when it sums the categories it happens to remember.
func (c DeltaCounts) Changed() int {
	return c.New + c.Spread + c.Moved + c.Fewer + c.BelowThreshold + c.Resolved
}

func DeltaCountsSummary(c DeltaCounts) string {
	if c.IsClean() {
		return fmt.Sprintf("%s acknowledged, nothing new", plural.Count(c.Unchanged, "pattern", "patterns"))
	}
	var parts []string
	for _, part := range []struct {
		n    int
		text string
	}{
		{c.New, "new"},
		{c.Spread, "spread further"},
		{c.Moved, "moved"},
		{c.Fewer, "with fewer copies"},
		{c.BelowThreshold, "below the threshold"},
		{c.Resolved, "resolved"},
	} {
		if part.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", part.n, part.text))
		}
	}
	return fmt.Sprintf("%s (%d unchanged)", strings.Join(parts, ", "), c.Unchanged)
}
