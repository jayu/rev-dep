package dupcode

import (
	"encoding/json"
	"io"

	"rev-dep-go/internal/parser"
)

// JSON output exists so a run can be compared against another tool, or a previous run, without
// parsing console text. Every field a comparison needs is here: the canonical identity, both
// ends of every occurrence in bytes AND lines, and the metrics the filters act on, so a
// discrepancy can be attributed rather than guessed at.

// JSONOutputVersion lets a consumer tell a tool upgrade from a format change.
//
// Bump it whenever a key is added, removed, renamed or changes meaning, and add the matching
// output-schema/duplicated-code-<version>.schema.json. Minor for additions a current parser can
// ignore, major for anything that would break one. TestDuplicatedCodeJSONSchemaNoDrift fails
// until the schema is updated, so this cannot be forgotten - only decided.
const JSONOutputVersion = "1.0"

// JSONReport is the top-level shape written by --format json.
type JSONReport struct {
	// First field, so a parser checking it does not have to read the rest to find it.
	Version  string       `json:"version"`
	Tool     string       `json:"tool"`
	Cwd      string       `json:"cwd"`
	Settings JSONSettings `json:"settings"`
	// Findings are NOT filtered down to the change when a snapshot is present: a consumer after the
	// delta filters on status, and one tracking total duplication would have had it thrown away.
	Snapshot *JSONSnapshot `json:"snapshot,omitempty"`
	Summary  JSONSummary   `json:"summary"`
	Findings []JSONFinding `json:"findings"`
}

type JSONSnapshot struct {
	Path  string `json:"path"`
	Clean bool   `json:"clean"`
	// dupcode's own DeltaCounts rather than a copy, so a category cannot be present in the report
	// and absent here.
	Counts DeltaCounts `json:"counts"`
	// Resolved have no code left to report: no occurrences, no metrics, no snippet. All that
	// survives is what the snapshot recorded.
	Resolved []JSONSnapshotEntry `json:"resolved,omitempty"`
	// BelowThreshold have real code and real locations, which is what separates them from Resolved.
	BelowThreshold []JSONFinding `json:"belowThreshold,omitempty"`
}

type JSONSnapshotEntry struct {
	Hash        string         `json:"hash"`
	Occurrences int            `json:"occurrences"`
	Files       map[string]int `json:"files"`
	Label       string         `json:"label,omitempty"`
}

type JSONWas struct {
	Occurrences int            `json:"occurrences"`
	Files       map[string]int `json:"files"`
}

type JSONFileChange struct {
	Path string `json:"path"`
	Was  int    `json:"was"`
	Now  int    `json:"now"`
}

// JSONSettings records everything that decided which findings are here, so a finding one run
// reports and another does not can be attributed to a setting. The names match the config file
// and the flags one for one.
type JSONSettings struct {
	// Blinding duplicates the three booleans below, so a consumer can group or display runs without
	// recomposing the name.
	Blinding         string `json:"blinding"`
	BlindIdentifiers bool   `json:"blindIdentifiers"`
	BlindStrings     bool   `json:"blindStrings"`
	BlindNumbers     bool   `json:"blindNumbers"`

	MinTokens     int  `json:"minTokens"`
	MinLines      int  `json:"minLines"`
	MinDepth      int  `json:"minDepth"`
	MinStatements int  `json:"minStatements"`
	MinDuplicates int  `json:"minDuplicates"`
	SkipObjects   bool `json:"skipObjects"`

	IgnoreFiles         []string `json:"ignoreFiles,omitempty"`
	ProcessIgnoredFiles []string `json:"processIgnoredFiles,omitempty"`
	// ConfigLevel is reported separately rather than merged into the lists above: its patterns are
	// relative to the config, so the same string means different files read from the two places.
	ConfigLevel *JSONConfigLevel `json:"configLevel,omitempty"`
}

type JSONConfigLevel struct {
	ConfigPath          string   `json:"configPath"`
	IgnoreFiles         []string `json:"ignoreFiles,omitempty"`
	ProcessIgnoredFiles []string `json:"processIgnoredFiles,omitempty"`
}

type JSONSummary struct {
	Findings          int `json:"findings"`
	Occurrences       int `json:"occurrences"`
	FilesWithFindings int `json:"filesWithFindings"`
	FilesScanned      int `json:"filesScanned"`
	FilesIgnored      int `json:"filesIgnored"`
	StatementBlocks   int `json:"statementBlocks"`
	ObjectLiterals    int `json:"objectLiterals"`
	JSXBlocks         int `json:"jsxBlocks"`
	Other             int `json:"other"`
}

type JSONFinding struct {
	// Hash is comparable only across runs with the same blinding.
	Hash string `json:"hash"`
	// Role refines a code block into what its braces actually are.
	Kind string `json:"kind"`
	Role string `json:"role"`
	// The metrics the filters act on, so a finding another tool reports and this one does not can
	// be explained by a threshold rather than a detection gap. CanonicalChars is informational.
	Tokens         int              `json:"tokens"`
	Lines          int              `json:"lines"`
	Depth          int              `json:"depth"`
	Statements     int              `json:"statements"`
	CanonicalChars int              `json:"canonicalChars"`
	Occurrences    []JSONOccurrence `json:"occurrences"`
	Snippet        string           `json:"snippet,omitempty"`
	// Status is "unchanged", "new", "spread", "moved", "fewer" or "under-min", absent when the run
	// had no snapshot. Filtering on it is how a consumer gets the delta.
	Status      string           `json:"status,omitempty"`
	Was         *JSONWas         `json:"was,omitempty"`
	FileChanges []JSONFileChange `json:"fileChanges,omitempty"`
}

// JSONOccurrence: byte offsets are exact; line and column are 1-based and inclusive at both
// ends, which makes a range comparable with a tool that reports clones as line spans.
type JSONOccurrence struct {
	File      string `json:"file"`
	StartByte uint32 `json:"startByte"`
	EndByte   uint32 `json:"endByte"`
	StartLine int    `json:"startLine"`
	StartCol  int    `json:"startCol"`
	EndLine   int    `json:"endLine"`
	EndCol    int    `json:"endCol"`
}

type findingStatus struct {
	status      string
	was         *JSONWas
	fileChanges []JSONFileChange
}

// findingStatuses indexes the delta by hash, so labelling is a lookup rather than a search
// through six slices per finding.
func findingStatuses(delta *Delta) map[string]findingStatus {
	if delta == nil {
		return nil
	}
	out := map[string]findingStatus{}
	for _, e := range delta.New {
		out[e.Hash] = findingStatus{status: kindNew}
	}
	for kind, changes := range map[string][]EntryChange{
		kindSpread:   delta.Spread,
		kindMoved:    delta.Moved,
		kindFewer:    delta.Shrunk,
		kindUnderMin: delta.Dropped,
	} {
		for _, c := range changes {
			out[c.Now.Hash] = findingStatus{
				status:      kind,
				was:         &JSONWas{Occurrences: c.Was.Occurrences, Files: c.Was.Files},
				fileChanges: jsonFileChanges(c.FileChanges),
			}
		}
	}
	return out
}

func jsonFileChanges(changes []FileChange) []JSONFileChange {
	if len(changes) == 0 {
		return nil
	}
	out := make([]JSONFileChange, 0, len(changes))
	for _, c := range changes {
		out = append(out, JSONFileChange{Path: c.Path, Was: c.Was, Now: c.Now})
	}
	return out
}

func jsonFinding(d Duplication, includeSnippets bool) JSONFinding {
	f := JSONFinding{
		Hash:           d.Hash,
		Kind:           jsonKind(d),
		Role:           jsonRole(d),
		Tokens:         d.Tokens,
		Lines:          d.Lines,
		Depth:          d.Depth,
		Statements:     d.Statements,
		CanonicalChars: d.NormLen,
		Occurrences:    make([]JSONOccurrence, 0, len(d.Occurrences)),
	}
	if includeSnippets {
		f.Snippet = d.Snippet
	}
	for _, o := range d.Occurrences {
		f.Occurrences = append(f.Occurrences, JSONOccurrence{
			File:      o.File,
			StartByte: o.Start,
			EndByte:   o.End,
			StartLine: o.Line,
			StartCol:  o.Col,
			EndLine:   o.EndLine,
			EndCol:    o.EndCol,
		})
	}
	return f
}

// jsonConfigLevel is nil for every run started from the command line without a snapshot.
func jsonConfigLevel(opts Options) *JSONConfigLevel {
	if !opts.ConfigLevel.IsSet() {
		return nil
	}
	return &JSONConfigLevel{
		ConfigPath:          opts.ConfigLevel.ConfigPath,
		IgnoreFiles:         opts.ConfigLevel.IgnoreFiles,
		ProcessIgnoredFiles: opts.ConfigLevel.ProcessIgnoredFiles,
	}
}

// WriteJSON: includeSnippets is off by default because snippets dominate the file size and a
// comparison keyed on hashes and ranges does not need them.
func WriteJSON(w io.Writer, dups []Duplication, stats Stats, opts Options, includeSnippets bool) error {
	return writeJSON(w, dups, nil, nil, stats, opts, "", includeSnippets)
}

// WriteJSONWithSnapshot labels every finding by how it relates to the baseline and adds the
// acknowledged entries the run has no code for. The findings are NOT filtered down to the
// change - see JSONReport.Snapshot.
func WriteJSONWithSnapshot(
	w io.Writer,
	dups, below []Duplication,
	delta *Delta,
	stats Stats,
	opts Options,
	snapshotPath string,
	includeSnippets bool,
) error {
	return writeJSON(w, dups, below, delta, stats, opts, snapshotPath, includeSnippets)
}

func writeJSON(
	w io.Writer,
	dups, below []Duplication,
	delta *Delta,
	stats Stats,
	opts Options,
	snapshotPath string,
	includeSnippets bool,
) error {
	opts = opts.Normalized()
	status := findingStatuses(delta)

	report := JSONReport{
		Version: JSONOutputVersion,
		Tool:    "rev-dep duplicated-code",
		Cwd:     opts.Cwd,
		Settings: JSONSettings{
			Blinding:            opts.Blinding.String(),
			BlindIdentifiers:    opts.Blinding.Identifiers,
			BlindStrings:        opts.Blinding.Strings,
			BlindNumbers:        opts.Blinding.Numbers,
			MinTokens:           opts.MinTokens,
			MinLines:            opts.MinLines,
			MinDepth:            opts.MinDepth,
			MinStatements:       opts.MinStatements,
			MinDuplicates:       opts.MinDuplicates,
			SkipObjects:         opts.SkipObjects,
			IgnoreFiles:         opts.IgnoreFiles,
			ProcessIgnoredFiles: opts.ProcessIgnoredFiles,
			ConfigLevel:         jsonConfigLevel(opts),
		},
		Summary: JSONSummary{
			Findings:        len(dups),
			FilesScanned:    stats.Files,
			FilesIgnored:    stats.IgnoredFiles,
			StatementBlocks: stats.StatementFindings,
			ObjectLiterals:  stats.ObjectFindings,
			JSXBlocks:       stats.JSXFindings,
			Other:           stats.OtherFindings,
		},
		Findings: make([]JSONFinding, 0, len(dups)),
	}

	if delta != nil {
		report.Snapshot = &JSONSnapshot{
			Path:   snapshotPath,
			Clean:  delta.IsClean(),
			Counts: CountsOf(delta),
		}
		for _, e := range delta.Resolved {
			report.Snapshot.Resolved = append(report.Snapshot.Resolved, JSONSnapshotEntry{
				Hash: e.Hash, Occurrences: e.Occurrences, Files: e.Files, Label: e.Label,
			})
		}
		for _, d := range below {
			f := jsonFinding(d, includeSnippets)
			if st, ok := status[d.Hash]; ok {
				f.Status, f.Was, f.FileChanges = st.status, st.was, st.fileChanges
			}
			report.Snapshot.BelowThreshold = append(report.Snapshot.BelowThreshold, f)
		}
	}

	affected := map[string]struct{}{}
	for _, d := range dups {
		report.Summary.Occurrences += len(d.Occurrences)

		f := jsonFinding(d, includeSnippets)
		if delta != nil {
			// Every finding is either something the snapshot knew about or something it did not; saying
			// which is what makes the delta recoverable without throwing the rest away.
			f.Status = "unchanged"
			if st, ok := status[d.Hash]; ok {
				f.Status, f.Was, f.FileChanges = st.status, st.was, st.fileChanges
			}
		}
		for _, o := range d.Occurrences {
			affected[o.File] = struct{}{}
		}
		report.Findings = append(report.Findings, f)
	}
	report.Summary.FilesWithFindings = len(affected)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func jsonKind(d Duplication) string {
	if d.Kind == parser.BlockJSX {
		return "jsx-block"
	}
	return "code-block"
}

func jsonRole(d Duplication) string {
	switch {
	case d.Kind == parser.BlockJSX:
		return "jsx"
	case d.IsObjectLiteral:
		return "object-literal"
	case d.IsStatementBlock:
		return "statement-block"
	default:
		return "declaration-body"
	}
}
