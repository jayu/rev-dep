package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/dupcode"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/plural"
)

var (
	duplicatedCodeCwd            string
	duplicatedCodeBlindIdents    bool
	duplicatedCodeBlindStrings   bool
	duplicatedCodeBlindNumbers   bool
	duplicatedCodeMinTokens      int
	duplicatedCodeMinLines       int
	duplicatedCodeMinDepth       int
	duplicatedCodeMinStatements  int
	duplicatedCodeMinDuplicates  int
	duplicatedCodeSkipObjects    bool
	duplicatedCodeIgnoreFiles    []string
	duplicatedCodeProcessIgnored []string
	duplicatedCodeFormat         string
	duplicatedCodeJSONSnippets   bool
	duplicatedCodeSnapshot       string
	duplicatedCodeUpdateSnapshot bool
)

// askedFlags names the settings the caller stated, under the names the snapshot records them
// by. Only these are checked against a snapshot: a flag someone did not type is not a belief
// they hold about the run.
func askedFlags(cmd *cobra.Command) map[string]bool {
	byFlag := map[string]string{
		"blind-identifiers": "blindIdentifiers",
		"blind-strings":     "blindStrings",
		"blind-numbers":     "blindNumbers",
		"min-tokens":        "minTokens",
		"min-lines":         "minLines",
		"min-depth":         "minDepth",
		"min-statements":    "minStatements",
		"min-duplicates":    "minDuplicates",
		"skip-objects":      "skipObjects",
		"ignore-files":      "ignoreFiles",
		// --process-ignored-files decides which files exist at all, so it is a detection setting even
		// though it is not a threshold.
		"process-ignored-files": "processIgnoredFiles",
	}
	asked := map[string]bool{}
	for flag, setting := range byFlag {
		if cmd.Flags().Changed(flag) {
			asked[setting] = true
		}
	}
	return asked
}

func duplicatedCodeCmdFn(cwd string, blinding dupcode.Blinding, minTokens, minLines, minDepth, minStatements, minDuplicates int, skipObjects bool, ignoreFiles, processIgnoredFiles []string,
	snapshotPath string, updateSnapshot bool, format string, jsonSnippets bool, asked map[string]bool) error {
	if format != "human" && format != "json" {
		return fmt.Errorf("invalid --format %q: expected \"human\" or \"json\"", format)
	}
	// Empty, never nil: nil would mean "check every setting", and a run with no flags has stated
	// nothing.
	if asked == nil {
		asked = map[string]bool{}
	}

	// A relative --snapshot is relative to --cwd like everything else. Left alone it resolved
	// against the directory the process started in, which put the file somewhere else entirely
	// whenever --cwd pointed at another tree - and --update-snapshot then wrote it there.
	if snapshotPath != "" {
		snapshotPath = pathutil.JoinWithCwd(cwd, snapshotPath)
	}

	opts := duplicatedCodeOptions(cwd, blinding, minTokens, minLines, minDepth, minStatements,
		minDuplicates, skipObjects, ignoreFiles, processIgnoredFiles, snapshotPath, format)

	// The snapshot is loaded BEFORE the run because it decides how the run is configured.
	// --update-snapshot is the exception: it is writing a new baseline, so the flags are the
	// authority and there is nothing to reconcile with.
	var previous *dupcode.Snapshot
	snapshotMissing := false
	if snapshotPath != "" && !updateSnapshot {
		loaded, err := dupcode.LoadSnapshot(snapshotPath)
		switch {
		case err == nil:
			previous = loaded
			opts, err = dupcode.ReconcileFlagsWithSnapshot(opts, previous, snapshotPath, asked)
			if err != nil {
				return err
			}
		case isSnapshotMissing(err):
			// Reported after the run, so the reader can see what they would be acknowledging.
			snapshotMissing = true
		default:
			return err
		}
	}

	// The sub-threshold chunks only explain a snapshot delta.
	var (
		dups, below []dupcode.Duplication
		stats       dupcode.Stats
		err         error
	)
	if previous != nil {
		dups, below, stats, err = dupcode.DetectWithBelowThreshold(opts)
	} else {
		dups, stats, err = dupcode.Detect(opts)
	}
	if err != nil {
		return err
	}

	// What happens to the baseline is decided BEFORE the output format, not inside one of them.
	// Deciding it per format is how --format json came to silently skip writing a snapshot with
	// --update-snapshot, and to exit 0 when the configured snapshot was missing.
	switch {
	case snapshotPath != "" && updateSnapshot:
		// Writing the baseline is explicit, so it needs no existing file and never fails on contents.
		snap := dupcode.BuildSnapshot(dups, opts)
		if err := dupcode.SaveSnapshot(snapshotPath, snap); err != nil {
			return err
		}
		return reportSnapshotWritten(snap, dups, stats, opts, snapshotPath, format, jsonSnippets)

	case snapshotMissing:
		// A configured baseline that is not there is a broken setup, not an empty one: passing here
		// would let a deleted file turn the check green.
		if err := writeFullReport(dups, stats, opts, format, jsonSnippets); err != nil {
			return err
		}
		return fmt.Errorf("duplicated code snapshot %s does not exist"+
			"\n\nCreate it with: rev-dep duplicated-code --snapshot %s --update-snapshot",
			snapshotPath, snapshotPath)

	case previous != nil:
		return reportAgainstSnapshot(dups, below, stats, opts, previous, snapshotPath,
			format, jsonSnippets)

	default:
		return writeFullReport(dups, stats, opts, format, jsonSnippets)
	}
}

func writeFullReport(
	dups []dupcode.Duplication,
	stats dupcode.Stats,
	opts dupcode.Options,
	format string,
	jsonSnippets bool,
) error {
	if format == "json" {
		return dupcode.WriteJSON(os.Stdout, dups, stats, opts, jsonSnippets)
	}
	dupcode.PrintReport(os.Stdout, dups, stats, opts.Blinding)
	return nil
}

// reportSnapshotWritten reports a fresh baseline in JSON as a comparison against itself, which
// is what it is - so a consumer stays on one shape instead of needing a second one for the
// case where the snapshot was being written.
func reportSnapshotWritten(
	snap *dupcode.Snapshot,
	dups []dupcode.Duplication,
	stats dupcode.Stats,
	opts dupcode.Options,
	snapshotPath string,
	format string,
	jsonSnippets bool,
) error {
	if format == "json" {
		written := &dupcode.Delta{Unchanged: len(snap.Entries), MinDuplicates: opts.MinDuplicates}
		return dupcode.WriteJSONWithSnapshot(os.Stdout, dups, nil, written, stats, opts,
			snapshotPath, jsonSnippets)
	}
	fmt.Printf("Wrote %s: %s acknowledged.\n",
		snapshotPath, plural.Count(len(snap.Entries), "duplicated pattern", "duplicated patterns"))
	return nil
}

func isSnapshotMissing(err error) bool {
	var missing *dupcode.SnapshotMissingError
	return errors.As(err, &missing)
}

// reportAgainstSnapshot reports the change rather than the whole result.
func reportAgainstSnapshot(
	dups []dupcode.Duplication,
	below []dupcode.Duplication,
	stats dupcode.Stats,
	opts dupcode.Options,
	previous *dupcode.Snapshot,
	snapshotPath string,
	format string,
	jsonSnippets bool,
) error {
	delta := dupcode.CompareToSnapshot(dups, below, previous, opts)

	if format == "json" {
		// The findings are still all here: filtering them down to the change would take the total away
		// from anything tracking it.
		if err := dupcode.WriteJSONWithSnapshot(os.Stdout, dups, below, delta, stats, opts,
			snapshotPath, jsonSnippets); err != nil {
			return err
		}
	} else {
		dupcode.PrintDelta(os.Stdout, delta, stats, opts.Blinding, snapshotPath)
	}

	if !delta.IsClean() {
		// Exit directly rather than returning an error, as `config run`, `circular` and `node-modules
		// unused` all do. A failing check is not a failing COMMAND: the report has already been printed,
		// and returning an error would make cobra add a bare `Error:` line on stderr, which the npm
		// wrapper forwards whenever the exit code is non-zero. Returning an error stays for real command
		// errors: a bad --format, an unreadable snapshot.
		//
		// --format json exits the same way: a machine is at least as entitled to a failing check.
		os.Exit(1)
	}
	return nil
}

// duplicatedCodeOptions is a function of its arguments and nothing else, so a test can check
// every flag reaches the detector. Not hypothetical: --skip-objects was registered, parsed and
// threaded through as a parameter while the line assigning it here was missing.
func duplicatedCodeOptions(
	cwd string,
	blinding dupcode.Blinding,
	minTokens, minLines, minDepth, minStatements, minDuplicates int,
	skipObjects bool,
	ignoreFiles, processIgnoredFiles []string,
	snapshotPath string,
	format string,
) dupcode.Options {
	return dupcode.Options{
		Cwd:                 cwd,
		Blinding:            blinding,
		MinTokens:           minTokens,
		MinLines:            minLines,
		MinDepth:            minDepth,
		MinStatements:       minStatements,
		MinDuplicates:       minDuplicates,
		SkipObjects:         skipObjects,
		IgnoreFiles:         ignoreFiles,
		ProcessIgnoredFiles: processIgnoredFiles,
		// Not a threshold, but it is the directory the snapshot records the config file relative to.
		SnapshotPath: snapshotPath,
		// The digest costs a second normalisation per reported block, so it is computed only where
		// something reads it.
		NeedHashes: snapshotPath != "" || format == "json",
	}
}

var duplicatedCodeCmd = &cobra.Command{
	Use:   "duplicated-code",
	Short: "Find code duplicated across (and within) the files of the project",
	Long: `Scans every source file for copy-pasteable chunks - brace blocks and JSX elements,
at every level of nesting - and reports the ones that appear more than once.

Comparison ignores formatting and comments entirely, so re-indented copies still match.

Four filters decide what counts as worth reporting. --min-tokens and --min-lines
measure size; --min-depth and --min-statements measure complexity, which is what
separates a duplicated three-key config object from duplicated logic - no size
floor can, because the object's keys and string values may be long. --min-duplicates
sets how many copies it takes to qualify.

By default the code must match as written. Each --blind-* flag drops one category
of token out of the comparison, and they combine freely:

  --blind-identifiers   names are wildcards, so a copy whose variables, functions
                        or components were renamed is still reported
  --blind-strings       string and template contents are wildcards
  --blind-numbers       numeric literals are wildcards
`,
	Example: "rev-dep duplicated-code --cwd ./src --blind-identifiers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return duplicatedCodeCmdFn(
			pathutil.ResolveAbsoluteCwd(duplicatedCodeCwd),
			dupcode.Blinding{
				Identifiers: duplicatedCodeBlindIdents,
				Strings:     duplicatedCodeBlindStrings,
				Numbers:     duplicatedCodeBlindNumbers,
			},
			duplicatedCodeMinTokens,
			duplicatedCodeMinLines,
			duplicatedCodeMinDepth,
			duplicatedCodeMinStatements,
			duplicatedCodeMinDuplicates,
			duplicatedCodeSkipObjects,
			duplicatedCodeIgnoreFiles,
			duplicatedCodeProcessIgnored,
			duplicatedCodeSnapshot,
			duplicatedCodeUpdateSnapshot,
			duplicatedCodeFormat,
			duplicatedCodeJSONSnippets,
			askedFlags(cmd),
		)
	},
}

func addDuplicatedCodeFlags(currentDir string) {
	duplicatedCodeCmd.Flags().StringVarP(&duplicatedCodeCwd, "cwd", "c", currentDir,
		"Working directory for the command")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeBlindIdents, "blind-identifiers", false,
		"Ignore the spelling of names, so a copy whose variables, functions or components were renamed still counts as duplication.")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeBlindStrings, "blind-strings", false,
		"Ignore the text of string and template literals, so a copy with different messages or keys still counts")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeBlindNumbers, "blind-numbers", false,
		"Ignore the value of numeric literals, so a copy with different constants still counts")
	duplicatedCodeCmd.Flags().IntVar(&duplicatedCodeMinTokens, "min-tokens", dupcode.DefaultMinTokens,
		"Smallest duplication to report, in tokens. Tokens rather than characters because the count does not change when a --blind-* flag is applied, so one number means the same amount of code whatever is being ignored")
	duplicatedCodeCmd.Flags().IntVar(&duplicatedCodeMinLines, "min-lines", 3,
		"Smallest duplication to report, in lines of the first occurrence")
	duplicatedCodeCmd.Flags().IntVar(&duplicatedCodeMinDepth, "min-depth", 0,
		"Smallest duplication to report, in nesting levels counting the block itself. 1 admits everything; 2 requires at least one nested level, which is what filters out flat objects and single JSX elements however long their keys or strings are")
	duplicatedCodeCmd.Flags().IntVar(&duplicatedCodeMinStatements, "min-statements", 0,
		"Smallest duplication to report, in statements directly inside the block. Applies only to statement blocks (function and control-flow bodies); object literals and JSX elements are expressions and are not filtered by it - use --min-depth for those")
	duplicatedCodeCmd.Flags().IntVar(&duplicatedCodeMinDuplicates, "min-duplicates", 2,
		"How many copies a chunk needs before it is reported. Raise to 3 to ignore code that has only been copied once")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeSkipObjects, "skip-objects", false,
		"Do not report duplications that are only object literals. ")
	duplicatedCodeCmd.Flags().StringSliceVar(&duplicatedCodeIgnoreFiles, "ignore-files", []string{},
		"Glob patterns of files to leave out of the analysis.")
	duplicatedCodeCmd.Flags().StringSliceVar(&duplicatedCodeProcessIgnored, "process-ignored-files", []string{},
		"Glob patterns to analyse even when gitignore excludes them.")
	duplicatedCodeCmd.Flags().StringVarP(&duplicatedCodeFormat, "format", "f", "human",
		"Output format: \"human\" or \"json\". JSON reports every finding with its canonical hash and the byte and line range of each occurrence, for comparing against another run or another tool")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeJSONSnippets, "json-snippets", false,
		"Include the source of each finding in JSON output. Off by default because snippets dominate the file size and a comparison keyed on ranges does not need them")
	duplicatedCodeCmd.Flags().StringVar(&duplicatedCodeSnapshot, "snapshot", "",
		"Path to a JSON snapshot of acknowledged duplications. With it, the command reports what changed since the snapshot instead of everything that exists, and exits non-zero on any difference")
	duplicatedCodeCmd.Flags().BoolVar(&duplicatedCodeUpdateSnapshot, "update-snapshot", false,
		"Rewrite the --snapshot file from this run, acknowledging everything it found. Always explicit: nothing updates a snapshot on its own")
}
