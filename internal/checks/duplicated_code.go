package checks

import (
	"errors"

	"rev-dep-go/internal/dupcode"
)

func isSnapshotMissing(err error) bool {
	var missing *dupcode.SnapshotMissingError
	return errors.As(err, &missing)
}

// DuplicatedCodeResult carries counts, not the duplications themselves. A config run happens on
// every commit, and several hundred snippets is not something anyone acts on there - what a
// rule can act on is whether the number moved.
type DuplicatedCodeResult struct {
	Snippets int
	// Occurrences is at least twice Snippets.
	Occurrences int
	Files       int
	Blinding    string
	// Scanned and Ignored let a disagreement with `rev-dep duplicated-code` be traced to the scope.
	Scanned int
	Ignored int

	// With a Delta the counts above are still the totals, but what the rule acts on is the change.
	Delta           *dupcode.Delta
	SnapshotPath    string
	SnapshotWritten bool
	// SnapshotMissing is a broken setup rather than an empty one.
	SnapshotMissing bool
}

type DuplicatedCodeParams struct {
	Cwd   string
	Files []string
	// Collector holds blocks already scanned during the run's single parse pass. Without it the
	// detector reads and scans the files itself, as the standalone command does.
	Collector        *dupcode.Collector
	BlindIdentifiers bool
	BlindStrings     bool
	BlindNumbers     bool
	MinTokens        int
	MinLines         int
	MinDepth         int
	MinStatements    int
	MinDuplicates    int
	SkipObjects      bool
	IgnoreFiles      []string
	// ConfigLevel shaped which files the run saw at all, so a baseline produced without it recorded
	// cannot be reproduced from the command line.
	ConfigLevel dupcode.ConfigLevelFilters
	// SnapshotPath turns the result into a delta; UpdateSnapshot rewrites the baseline instead.
	SnapshotPath   string
	UpdateSnapshot bool
}

func FindDuplicatedCode(params DuplicatedCodeParams) (DuplicatedCodeResult, error) {
	blinding := dupcode.Blinding{
		Identifiers: params.BlindIdentifiers,
		Strings:     params.BlindStrings,
		Numbers:     params.BlindNumbers,
	}

	opts := dupcode.Options{
		Cwd:   params.Cwd,
		Files: params.Files,
		// A rule reports a number, so snippet text and per-occurrence line/column have no consumer.
		CountOnly: true,
		// Only a snapshot reads the digest, and computing it normalises each block again.
		NeedHashes:    params.SnapshotPath != "",
		Blinding:      blinding,
		MinTokens:     params.MinTokens,
		MinLines:      params.MinLines,
		MinDepth:      params.MinDepth,
		MinStatements: params.MinStatements,
		MinDuplicates: params.MinDuplicates,
		SkipObjects:   params.SkipObjects,
		IgnoreFiles:   params.IgnoreFiles,
		ConfigLevel:   params.ConfigLevel,
		SnapshotPath:  params.SnapshotPath,
	}

	// The snapshot is read before the run because it decides how the run is configured. A config run
	// differs from the command line on a mismatch: every value in a config file is stated, by being
	// written or by being left to a documented default, so there is no "the caller did not say" to
	// fall back on. Any difference is a contradiction between two files in the repository.
	var previous *dupcode.Snapshot
	snapshotMissing := false
	if params.SnapshotPath != "" && !params.UpdateSnapshot {
		loaded, err := dupcode.LoadSnapshot(params.SnapshotPath)
		switch {
		case err == nil:
			previous = loaded
			opts, err = dupcode.ReconcileConfigWithSnapshot(opts, previous, params.SnapshotPath)
			if err != nil {
				return DuplicatedCodeResult{}, err
			}
		case isSnapshotMissing(err):
			snapshotMissing = true
		default:
			return DuplicatedCodeResult{}, err
		}
	}

	var (
		dups  []dupcode.Duplication
		below []dupcode.Duplication
		stats dupcode.Stats
		err   error
	)
	// The sub-threshold chunks are only read by a snapshot comparison.
	explain := previous != nil
	switch {
	case params.Collector != nil && explain:
		dups, below, stats, err = params.Collector.DetectWithBelowThreshold(opts, params.Files)
	case params.Collector != nil:
		dups, stats, err = params.Collector.Detect(opts, params.Files)
	case explain:
		dups, below, stats, err = dupcode.DetectWithBelowThreshold(opts)
	default:
		dups, stats, err = dupcode.Detect(opts)
	}
	if err != nil {
		return DuplicatedCodeResult{}, err
	}

	result := DuplicatedCodeResult{SnapshotPath: params.SnapshotPath}

	switch {
	case params.SnapshotPath == "":
	case params.UpdateSnapshot:
		if err := dupcode.SaveSnapshot(params.SnapshotPath, dupcode.BuildSnapshot(dups, opts)); err != nil {
			return DuplicatedCodeResult{}, err
		}
		result.SnapshotWritten = true
	case snapshotMissing:
		result.SnapshotMissing = true
	default:
		result.Delta = dupcode.CompareToSnapshot(dups, below, previous, opts)
	}

	occurrences := 0
	files := map[int32]struct{}{}
	for _, d := range dups {
		occurrences += len(d.Occurrences)
		for _, o := range d.Occurrences {
			files[o.FileID] = struct{}{}
		}
	}

	result.Snippets = len(dups)
	result.Occurrences = occurrences
	result.Files = len(files)
	result.Blinding = blinding.String()
	result.Scanned = stats.Files
	result.Ignored = stats.IgnoredFiles
	return result, nil
}
