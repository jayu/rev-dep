package dupcode

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rev-dep-go/internal/parser"
)

// A Snapshot is the acknowledged state of a project's duplication, committed alongside the
// code so a run reports what CHANGED rather than everything that exists.
//
// It records hashes, never code. Identity is the digest of the canonical form, so an entry
// survives reformatting - and under structural blinding, renaming too.
//
// The parameters are recorded but take no part in that identity: tightening a threshold must
// not invalidate the file. Hashing them in would turn every knob turn into a full re-baseline.
type Snapshot struct {
	// SchemaVersion guards the shape of this file.
	SchemaVersion string `json:"schemaVersion"`
	// CanonicalFormVersion is advisory: a normaliser fix usually moves a handful of hashes rather
	// than all of them, so a mismatch adds context to the delta instead of rejecting the file.
	CanonicalFormVersion string             `json:"canonicalFormVersion"`
	Parameters           SnapshotParameters `json:"parameters"`
	// Sorted by hash so the file diffs cleanly in review.
	Entries []SnapshotEntry `json:"duplications"`
}

// it resolves findings nobody fixed. Neither is a change in the code, but both read as one.
//
// There is deliberately no file-size ceiling: it was a build constant applied by one entry
// point and not the other, and produced baselines only one of them could reproduce.
type SnapshotParameters struct {
	// Blinding is the human-readable form of the three flags below, which are what is read back.
	Blinding            string   `json:"blinding"`
	BlindIdentifiers    bool     `json:"blindIdentifiers"`
	BlindStrings        bool     `json:"blindStrings"`
	BlindNumbers        bool     `json:"blindNumbers"`
	MinTokens           int      `json:"minTokens"`
	MinLines            int      `json:"minLines"`
	MinDepth            int      `json:"minDepth"`
	MinStatements       int      `json:"minStatements"`
	MinDuplicates       int      `json:"minDuplicates"`
	SkipObjects         bool     `json:"skipObjects"`
	IgnoreFiles         []string `json:"ignoreFiles,omitempty"`
	ProcessIgnoredFiles []string `json:"processIgnoredFiles,omitempty"`

	// ConfigLevelSettings shapes which files existed for the run at all, so it is as much a part
	// of this baseline as any threshold.
	ConfigLevelSettings *SnapshotConfigLevelSettings `json:"configLevelSettings,omitempty"`
}

// SnapshotConfigLevelSettings is a config file's top-level file selection.
//
// The patterns are stored exactly as the config writes them, never rewritten to another root -
// see ConfigLevelFilters for why. ConfigPath is the only value expressed differently here, and
// it is relative to this snapshot file, so moving the directory the two live in together
// leaves the snapshot valid.
type SnapshotConfigLevelSettings struct {
	// ConfigPath locates the config from the directory this snapshot is saved in.
	ConfigPath string `json:"configPath"`
	// Verbatim, resolved against the directory ConfigPath points into.
	IgnoreFiles         []string `json:"ignoreFiles,omitempty"`
	ProcessIgnoredFiles []string `json:"processIgnoredFiles,omitempty"`
}

// SnapshotParametersFrom records the settings a run ACTUALLY used, defaults filled in: an
// unset field and one set to its default are the same run, and recording them differently
// would make one look like a changed setting.
func SnapshotParametersFrom(opts Options) SnapshotParameters {
	opts = opts.Normalized()
	return SnapshotParameters{
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
		IgnoreFiles:         append([]string{}, opts.IgnoreFiles...),
		ProcessIgnoredFiles: append([]string{}, opts.ProcessIgnoredFiles...),
		ConfigLevelSettings: configLevelFor(opts),
	}
}

// configLevelFor locates the config relative to the snapshot, so the pair stays valid wherever
// the repository is checked out.
func configLevelFor(opts Options) *SnapshotConfigLevelSettings {
	if !opts.ConfigLevel.IsSet() {
		return nil
	}
	return &SnapshotConfigLevelSettings{
		ConfigPath:          relativeToSnapshot(opts.SnapshotPath, opts.ConfigLevel.ConfigPath),
		IgnoreFiles:         append([]string{}, opts.ConfigLevel.IgnoreFiles...),
		ProcessIgnoredFiles: append([]string{}, opts.ConfigLevel.ProcessIgnoredFiles...),
	}
}

// relativeToSnapshot writes forward slashes whatever the host is - the file is committed and
// read on every platform the project is developed on.
func relativeToSnapshot(snapshotPath, target string) string {
	if snapshotPath == "" || target == "" {
		return target
	}
	rel, err := filepath.Rel(filepath.Dir(snapshotPath), target)
	if err != nil {
		// Different volumes on Windows. The absolute path at least names the file that was used.
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func resolveFromSnapshot(snapshotPath, recorded string) string {
	if recorded == "" {
		return ""
	}
	if filepath.IsAbs(recorded) || snapshotPath == "" {
		return recorded
	}
	return filepath.Join(filepath.Dir(snapshotPath), recorded)
}

// Apply takes every detection setting from the snapshot, leaving what steers the run - the
// directory, the file list, the output switches - as the caller set it.
func (p SnapshotParameters) Apply(opts Options) Options {
	opts.Blinding = Blinding{
		Identifiers: p.BlindIdentifiers,
		Strings:     p.BlindStrings,
		Numbers:     p.BlindNumbers,
	}
	opts.MinTokens = p.MinTokens
	opts.MinLines = p.MinLines
	opts.MinDepth = p.MinDepth
	opts.MinStatements = p.MinStatements
	opts.MinDuplicates = p.MinDuplicates
	opts.SkipObjects = p.SkipObjects
	opts.IgnoreFiles = append([]string{}, p.IgnoreFiles...)
	opts.ProcessIgnoredFiles = append([]string{}, p.ProcessIgnoredFiles...)
	opts.ConfigLevel = ConfigLevelFilters{}
	if p.ConfigLevelSettings != nil {
		opts.ConfigLevel = ConfigLevelFilters{
			ConfigPath:          resolveFromSnapshot(opts.SnapshotPath, p.ConfigLevelSettings.ConfigPath),
			IgnoreFiles:         append([]string{}, p.ConfigLevelSettings.IgnoreFiles...),
			ProcessIgnoredFiles: append([]string{}, p.ConfigLevelSettings.ProcessIgnoredFiles...),
		}
	}
	return opts
}

// SettingDifference is one setting a caller asked for that the snapshot was not taken with.
type SettingDifference struct {
	// Setting is the neutral name, so the same difference reads the same whether it came from a
	// flag or a config file.
	Setting  string
	Snapshot string
	Wanted   string
}

// Settings pairs every setting with its value, which makes comparing two sets a loop rather
// than a dozen hand-written branches that can each be forgotten when a setting is added.
func (p SnapshotParameters) Settings() []struct {
	Name  string
	Value string
} {
	return []struct {
		Name  string
		Value string
	}{
		{"blindIdentifiers", fmt.Sprint(p.BlindIdentifiers)},
		{"blindStrings", fmt.Sprint(p.BlindStrings)},
		{"blindNumbers", fmt.Sprint(p.BlindNumbers)},
		{"minTokens", fmt.Sprint(p.MinTokens)},
		{"minLines", fmt.Sprint(p.MinLines)},
		{"minDepth", fmt.Sprint(p.MinDepth)},
		{"minStatements", fmt.Sprint(p.MinStatements)},
		{"minDuplicates", fmt.Sprint(p.MinDuplicates)},
		{"skipObjects", fmt.Sprint(p.SkipObjects)},
		{"ignoreFiles", strings.Join(p.IgnoreFiles, ",")},
		{"processIgnoredFiles", strings.Join(p.ProcessIgnoredFiles, ",")},
		{"configPath", configLevelField(p, func(c SnapshotConfigLevelSettings) string { return c.ConfigPath })},
		{"configIgnoreFiles", configLevelField(p, func(c SnapshotConfigLevelSettings) string {
			return strings.Join(c.IgnoreFiles, ",")
		})},
		{"configProcessIgnoredFiles", configLevelField(p, func(c SnapshotConfigLevelSettings) string {
			return strings.Join(c.ProcessIgnoredFiles, ",")
		})},
	}
}

func configLevelField(p SnapshotParameters, get func(SnapshotConfigLevelSettings) string) string {
	if p.ConfigLevelSettings == nil {
		return ""
	}
	return get(*p.ConfigLevelSettings)
}

// DiffSettings lists the settings wanted differs from p on.
//
// only restricts the comparison to those names, so the command line reports a conflict for the
// flags someone actually typed. NIL means compare everything; an EMPTY map means compare
// nothing, and the difference is the point: a command line with no flags has stated nothing.
func (p SnapshotParameters) DiffSettings(wanted SnapshotParameters, only map[string]bool) []SettingDifference {
	mine, theirs := p.Settings(), wanted.Settings()
	var out []SettingDifference
	for i := range mine {
		if only != nil && !only[mine[i].Name] {
			continue
		}
		if mine[i].Value != theirs[i].Value {
			out = append(out, SettingDifference{
				Setting: mine[i].Name, Snapshot: mine[i].Value, Wanted: theirs[i].Value,
			})
		}
	}
	return out
}

// SnapshotEntry is one acknowledged duplication.
type SnapshotEntry struct {
	// Hash identifies the code.
	Hash string `json:"hash"`
	// Occurrences is how many copies were acknowledged. It is the sum of Files.
	Occurrences int `json:"occurrences"`
	// Files maps each path to how many copies live in it.
	//
	// Counts rather than a bare path list: a second copy appearing in a file already listed moves
	// the total and nothing else, leaving a report to say "2 -> 3 copies" beside three locations
	// and let the reader work out which one is new.
	//
	// Line numbers are deliberately absent. They move whenever anything above them is edited, so
	// tracking them would break the snapshot on nearly every commit while saying nothing about
	// duplication - and they cannot be recovered anyway, since the copies are identical by
	// construction and no one of them is "the new one".
	Files map[string]int `json:"files"`
	// Label is for a human reading the diff. It takes no part in comparison.
	Label string `json:"label,omitempty"`
}

const (
	// Both versions are major.minor strings, like the config and the JSON output: a bare counter
	// only ever grows, and says nothing about whether a change was breaking.
	//
	// SnapshotSchemaVersion is bumped when the file layout changes. A file at any other
	// version is rejected rather than half-trusted: the settings recorded here are applied to
	// the run, so a file that does not carry all of them cannot be the authority it is being
	// treated as. Re-baselining takes one command.
	SnapshotSchemaVersion = "1.0"
	// CanonicalFormVersion is bumped whenever the normaliser's output changes for code that did
	// not change. That breaks every committed snapshot: see canonical_form_golden_test.go, which
	// exists so it never happens by accident.
	CanonicalFormVersion = "1.0"

	labelMaxLen = 72
)

// CanonicalHash is the identity written to disk. Deliberately not the 64-bit polynomial the
// matcher uses internally: that one is tuned for bucketing and free to change with it.
func CanonicalHash(code []byte, blinding Blinding) string {
	// Streamed rather than normalised into a buffer: the canonical form is wanted only to be
	// digested.
	digest := sha256.New()
	it := parser.NewNormIterator(code, blinding)
	var chunk [512]byte
	n := 0
	for {
		c, _, ok := it.Next()
		if !ok {
			break
		}
		chunk[n] = c
		n++
		if n == len(chunk) {
			digest.Write(chunk[:n])
			n = 0
		}
	}
	if n > 0 {
		digest.Write(chunk[:n])
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

func BuildSnapshot(dups []Duplication, opts Options) *Snapshot {
	opts = opts.Normalized()
	// A caller that forgot NeedHashes would otherwise write a file of empty identities that
	// compares equal to nothing.
	if len(dups) > 0 && dups[0].Hash == "" {
		panic("dupcode: BuildSnapshot needs Options.NeedHashes set on the run that produced these duplications")
	}
	snap := &Snapshot{
		SchemaVersion:        SnapshotSchemaVersion,
		CanonicalFormVersion: CanonicalFormVersion,
		Parameters:           SnapshotParametersFrom(opts),
		Entries:              make([]SnapshotEntry, 0, len(dups)),
	}
	for i := range dups {
		snap.Entries = append(snap.Entries, entryForFinding(dups[i].Hash, &dups[i]))
	}
	sort.Slice(snap.Entries, func(a, b int) bool { return snap.Entries[a].Hash < snap.Entries[b].Hash })
	return snap
}

// entryForFinding is used by BuildSnapshot and by the delta, so the two cannot disagree about
// what an entry for a given finding looks like.
func entryForFinding(hash string, d *Duplication) SnapshotEntry {
	return SnapshotEntry{
		Hash:        hash,
		Occurrences: len(d.Occurrences),
		Files:       copiesPerFile(*d),
		Label:       d.Label,
	}
}

func copiesPerFile(d Duplication) map[string]int {
	out := make(map[string]int, len(d.Occurrences))
	for _, o := range d.Occurrences {
		out[o.File]++
	}
	return out
}

func (e SnapshotEntry) SortedFiles() []string {
	out := make([]string, 0, len(e.Files))
	for path := range e.Files {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// SettingsConflictError reports a run whose settings do not match its snapshot.
type SettingsConflictError struct {
	Path        string
	Differences []SettingDifference
	// Source is "flag" or "config", so the message can say which side to change.
	Source string
}

func (e *SettingsConflictError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "these settings do not match the snapshot %s:\n", e.Path)
	for _, d := range e.Differences {
		fmt.Fprintf(&b, "  %s: snapshot has %s, this run asks for %s\n",
			d.Setting, quoteSetting(d.Snapshot), quoteSetting(d.Wanted))
	}
	b.WriteString("\nThe snapshot records the settings its baseline was produced under, and a\n")
	b.WriteString("comparison is only meaningful against a baseline that means the same thing -\n")
	b.WriteString("lowering a threshold invents findings the snapshot never had the chance to\n")
	b.WriteString("acknowledge, and raising one resolves findings nobody fixed.\n\n")
	if e.Source == sourceConfig {
		b.WriteString("Either change the config to match, or re-baseline with: rev-dep config run --update-snapshot")
	} else {
		b.WriteString("Either drop the conflicting flags, or re-baseline with --update-snapshot")
	}
	return b.String()
}

const (
	sourceFlag   = "flag"
	sourceConfig = "config"
)

func quoteSetting(v string) string {
	if v == "" {
		return `""`
	}
	return v
}

// ReconcileWithSnapshot settles which settings a run against a snapshot uses.
//
// The snapshot wins, always. asked names the settings the caller stated explicitly; those are
// checked and a conflict is an error, because someone who typed --min-tokens 30 against a
// snapshot taken at 50 has a wrong belief about what is about to happen. Settings the caller
// did not state are taken from the snapshot. Pass nil to check everything, which is what a
// config file wants: every value there is stated.
func ReconcileWithSnapshot(opts Options, snap *Snapshot, path string, asked map[string]bool, source string) (Options, error) {
	if snap == nil {
		return opts, nil
	}
	wanted := SnapshotParametersFrom(opts)
	if diffs := snap.Parameters.DiffSettings(wanted, asked); len(diffs) > 0 {
		return opts, &SettingsConflictError{Path: path, Differences: diffs, Source: source}
	}
	return snap.Parameters.Apply(opts), nil
}

// ReconcileFlagsWithSnapshot checks only the flags actually typed.
func ReconcileFlagsWithSnapshot(opts Options, snap *Snapshot, path string, asked map[string]bool) (Options, error) {
	return ReconcileWithSnapshot(opts, snap, path, asked, sourceFlag)
}

// ReconcileConfigWithSnapshot checks every setting: in a config file every value is stated,
// by being written down or by being left to a documented default.
func ReconcileConfigWithSnapshot(opts Options, snap *Snapshot, path string) (Options, error) {
	return ReconcileWithSnapshot(opts, snap, path, nil, sourceConfig)
}

// LoadSnapshot reports a missing file distinctly, so a caller can tell "no baseline yet" from
// "the baseline is broken".
func LoadSnapshot(path string) (*Snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &SnapshotMissingError{Path: path}
		}
		return nil, fmt.Errorf("read duplicated code snapshot %s: %w", path, err)
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("parse duplicated code snapshot %s: %w", path, err)
	}
	if snap.SchemaVersion != SnapshotSchemaVersion {
		return nil, fmt.Errorf(
			"duplicated code snapshot %s was written with schema version %s, this build expects %s",
			path, snap.SchemaVersion, SnapshotSchemaVersion)
	}
	return &snap, nil
}

type SnapshotMissingError struct{ Path string }

func (e *SnapshotMissingError) Error() string {
	return fmt.Sprintf("duplicated code snapshot %s does not exist", e.Path)
}

func SaveSnapshot(path string, snap *Snapshot) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", path, err)
		}
	}
	// The default marshaller escapes <, > and & for HTML safety, which turns a JSX label into
	// \u003c soup. This file is read in pull requests, so legibility wins.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// ---------------------------------------------------------------------------
// delta
// ---------------------------------------------------------------------------

type Delta struct {
	New    []NewEntry
	Spread []EntryChange
	Moved  []EntryChange
	Shrunk []EntryChange
	// Dropped are still duplicated, but below MinDuplicates, so the report no longer lists them.
	//
	// Kept apart from Resolved because they are a different fact about the code: at
	// --min-duplicates 3, deleting one of three copies leaves two behind. Calling that "no longer
	// duplicated" would be wrong, and would hide the copies still there from the reader most
	// likely to want to finish the job.
	Dropped []EntryChange
	// Resolved were acknowledged and are genuinely gone: not under the threshold, not anywhere.
	Resolved  []SnapshotEntry
	Unchanged int

	// ParameterChanges is context only; it does not affect the comparison.
	ParameterChanges []string
	// CanonicalFormChanged means part of the delta may be re-canonicalisation rather than change.
	CanonicalFormChanged bool
	SnapshotFormVersion  string
	// MinDuplicates lets a report name the number the reader configured.
	MinDuplicates int
}

// NewEntry is a duplication the snapshot did not know about.
type NewEntry struct {
	SnapshotEntry
	// Finding is the run's own record: the snippet, and where every copy is. A snapshot stores a
	// hash, a file list and a label - enough to detect a change and nowhere near enough to show
	// one - so everything a reader sees is rendered from here.
	Finding *Duplication
	// RevealedBy names a resolved entry this one sits inside the files of.
	//
	// Nested duplicates are collapsed into their parent when reported, so fixing a large duplicate
	// can expose smaller ones that were always there. Without this the report would say "you fixed
	// one and created two", which is both alarming and wrong.
	RevealedBy string
}

type EntryChange struct {
	Was SnapshotEntry
	Now SnapshotEntry
	// FileChanges makes a copy added to a file already on the list visible, rather than folded
	// into the total.
	FileChanges []FileChange
	Finding     *Duplication
}

func (d *Delta) HasRegressions() bool {
	return d.HasMoreDuplication() || len(d.Moved) > 0
}

// HasMoreDuplication is strictly more duplication, as opposed to the same amount somewhere
// else. A move fails the check - a baseline naming the wrong files is one nobody can review -
// but it is not an increase, and reporting it as one describes something that did not happen.
func (d *Delta) HasMoreDuplication() bool {
	return len(d.New) > 0 || len(d.Spread) > 0
}

// HasImprovements: better, but the snapshot is still out of date.
func (d *Delta) HasImprovements() bool {
	return len(d.Resolved) > 0 || len(d.Shrunk) > 0 || len(d.Dropped) > 0
}

func (d *Delta) IsClean() bool { return !d.HasRegressions() && !d.HasImprovements() }

// CompareToSnapshot diffs a run against a snapshot.
//
// below are the sub-threshold chunks from DetectWithBelowThreshold. They are what lets an
// acknowledged pattern that lost a copy read as having fallen under the threshold rather than
// as resolved. Passing nil gives up that distinction.
func CompareToSnapshot(dups, below []Duplication, snap *Snapshot, opts Options) *Delta {
	current := BuildSnapshot(dups, opts)

	// Keyed by hash: that is the identity a snapshot records, so the only thing an entry can be
	// joined back to the run by.
	findings := make(map[string]*Duplication, len(dups))
	for i := range dups {
		if dups[i].Hash != "" {
			findings[dups[i].Hash] = &dups[i]
		}
	}
	underThreshold := make(map[string]*Duplication, len(below))
	for i := range below {
		if below[i].Hash != "" {
			underThreshold[below[i].Hash] = &below[i]
		}
	}

	delta := &Delta{
		MinDuplicates:        opts.Normalized().MinDuplicates,
		SnapshotFormVersion:  snap.CanonicalFormVersion,
		CanonicalFormChanged: snap.CanonicalFormVersion != CanonicalFormVersion,
		ParameterChanges:     diffParameters(snap.Parameters, current.Parameters),
	}

	was := make(map[string]SnapshotEntry, len(snap.Entries))
	for _, e := range snap.Entries {
		was[e.Hash] = e
	}

	for _, now := range current.Entries {
		before, known := was[now.Hash]
		if !known {
			delta.New = append(delta.New, NewEntry{SnapshotEntry: now, Finding: findings[now.Hash]})
			continue
		}
		delete(was, now.Hash)

		moved := diffFiles(before.Files, now.Files)
		change := EntryChange{
			Was: before, Now: now,
			FileChanges: moved,
			Finding:     findings[now.Hash],
		}
		switch {
		case now.Occurrences > before.Occurrences:
			delta.Spread = append(delta.Spread, change)
		case now.Occurrences < before.Occurrences:
			delta.Shrunk = append(delta.Shrunk, change)
		case len(moved) > 0:
			// Same count, different files.
			delta.Moved = append(delta.Moved, change)
		default:
			delta.Unchanged++
		}
	}

	// What is left of `was` was acknowledged and is not in the report. Some is gone; some is still
	// there with too few copies to qualify, and saying so is the difference between "you fixed it"
	// and "one more to go".
	for _, e := range was {
		still, under := underThreshold[e.Hash]
		if !under {
			delta.Resolved = append(delta.Resolved, e)
			continue
		}
		now := entryForFinding(e.Hash, still)
		delta.Dropped = append(delta.Dropped, EntryChange{
			Was: e, Now: now,
			FileChanges: diffFiles(e.Files, now.Files),
			Finding:     still,
		})
	}
	sort.Slice(delta.Resolved, func(a, b int) bool { return delta.Resolved[a].Hash < delta.Resolved[b].Hash })
	sort.Slice(delta.Dropped, func(a, b int) bool { return delta.Dropped[a].Now.Hash < delta.Dropped[b].Now.Hash })

	markRevealed(delta)
	return delta
}

type FileChange struct {
	Path string
	Was  int
	Now  int
}

// IsNew: the file held none of these copies before, the only case where every occurrence in it
// can be pointed at as new.
func (c FileChange) IsNew() bool { return c.Was == 0 }

func (c FileChange) IsGone() bool { return c.Now == 0 }

// diffFiles includes a file that gained or lost copies without appearing or disappearing,
// which is the whole reason the counts are stored.
func diffFiles(before, now map[string]int) []FileChange {
	seen := map[string]struct{}{}
	for path := range before {
		seen[path] = struct{}{}
	}
	for path := range now {
		seen[path] = struct{}{}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var out []FileChange
	for _, path := range paths {
		was, isNow := before[path], now[path]
		if was != isNow {
			out = append(out, FileChange{Path: path, Was: was, Now: isNow})
		}
	}
	return out
}

func diffParameters(was, now SnapshotParameters) []string {
	var out []string
	add := func(name string, a, b any) {
		if fmt.Sprint(a) != fmt.Sprint(b) {
			out = append(out, fmt.Sprintf("%s %v -> %v", name, a, b))
		}
	}
	add("blinding", was.Blinding, now.Blinding)
	add("minTokens", was.MinTokens, now.MinTokens)
	add("minLines", was.MinLines, now.MinLines)
	add("minDepth", was.MinDepth, now.MinDepth)
	add("minStatements", was.MinStatements, now.MinStatements)
	add("minDuplicates", was.MinDuplicates, now.MinDuplicates)
	add("ignoreFiles", strings.Join(was.IgnoreFiles, ","), strings.Join(now.IgnoreFiles, ","))
	return out
}

// markRevealed attributes a new entry to a resolved one when it lives entirely in the same
// files - the signature of a nested duplicate surfacing because its container stopped matching.
//
// An attribution, not a proof: the snapshot holds no code, so containment cannot be checked.
// Sharing a file set with something that just disappeared is enough to phrase the report
// carefully, and the entry is still reported.
func markRevealed(delta *Delta) {
	if len(delta.New) == 0 || len(delta.Resolved) == 0 {
		return
	}
	for i := range delta.New {
		for _, gone := range delta.Resolved {
			if isFileSubset(delta.New[i].SortedFiles(), gone.Files) {
				delta.New[i].RevealedBy = gone.Hash
				break
			}
		}
	}
}

func isFileSubset(inner []string, outer map[string]int) bool {
	if len(inner) == 0 || len(inner) > len(outer) {
		return false
	}
	for _, f := range inner {
		if _, ok := outer[f]; !ok {
			return false
		}
	}
	return true
}
