package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/pathutil"
)

// ---------------- config lint ----------------
var (
	lintConfigCwd   string
	lintConfigFix   bool
	lintConfigRules []string
)

var configLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Report (and optionally remove) config glob/path patterns that match nothing",
	Long: `Scan a (.)rev-dep.config.json(c) for "dead" glob and path patterns — ignore
patterns, entry point patterns, rule paths, graph excludes, denied files/modules and
similar — that no longer match any discovered file or module. Over time configs
accumulate patterns for files that were renamed or deleted; this command surfaces them
so the config stays lean.

With --fix, dead patterns are removed in place, preserving all comments and formatting.
Some patterns are reported but never auto-removed because deleting them could change a
check's behavior or make the config invalid — rule paths, required entry points / files
/ modules, and module-boundary selectors. These are marked "not auto-removed"; resolve
them by hand.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		cwd := pathutil.ResolveAbsoluteCwd(lintConfigCwd)

		cfg, err := config.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("Could not load configuration from %s:\n%v", filepath.Join(cwd, config.ConfigFileName()), err)
		}

		selectedRules, err := config.ParseLintRules(lintConfigRules)
		if err != nil {
			return err
		}

		result, err := config.LintConfig(&cfg, cwd, packageJsonPath, tsconfigJsonPath, selectedRules)
		if err != nil {
			return fmt.Errorf("Error linting config: %v", err)
		}

		printConfigLintResults(result, cwd)

		if lintConfigFix && len(result.DeadPatterns) > 0 {
			fixResult, err := config.ApplyLintFix(result)
			if err != nil {
				return fmt.Errorf("Error applying fixes: %v", err)
			}
			printConfigLintFixSummary(fixResult)
		}

		// Exit code is driven by ERRORS only; warnings (dead negation patterns and all
		// overlap findings) are advisory. A removable error is cleared by --fix; a
		// report-only error (required entry points, rule paths, etc.) still fails.
		errorsRemaining := 0
		warnings := 0
		for _, dp := range result.DeadPatterns {
			if dp.Severity == config.SeverityWarning {
				warnings++
				continue
			}
			if lintConfigFix && dp.Removable {
				continue // removed by --fix
			}
			errorsRemaining++
		}
		warnings += len(result.Overlaps)

		printConfigLintStatus(errorsRemaining, warnings, lintConfigFix)
		fmt.Printf("✨  Done in %dms.\n", time.Since(startTime).Milliseconds())

		if errorsRemaining > 0 {
			os.Exit(1)
		}
		return nil
	},
}

// printConfigLintStatus prints the final one-line verdict.
func printConfigLintStatus(errors, warnings int, fixed bool) {
	switch {
	case errors == 0 && warnings == 0:
		// "all clean" already printed by the results section
	case errors == 0:
		fmt.Printf("\n⚠️  %d warning(s), no errors — exit 0.\n", warnings)
	default:
		verb := "found"
		if fixed {
			verb = "remaining after --fix"
		}
		fmt.Printf("\n❌ %d error(s) %s, %d warning(s).\n", errors, verb, warnings)
	}
}

// optionLabel builds the human-readable option path (e.g. "orphanFilesDetection.validEntryPoints").
func optionLabel(detectorType string, boundaryIndex int, optionKey string) string {
	if detectorType == "" {
		return optionKey
	}
	if detectorType == "moduleBoundaries" {
		return fmt.Sprintf("moduleBoundaries[%d].%s", boundaryIndex, optionKey)
	}
	return fmt.Sprintf("%s.%s", detectorType, optionKey)
}

func printConfigLintResults(result *config.LintResult, cwd string) {
	configRel := result.ConfigFilePath
	if rel, err := filepath.Rel(cwd, result.ConfigFilePath); err == nil {
		configRel = filepath.ToSlash(rel)
	}
	ruleNames := make([]string, len(result.RulesRun))
	for i, r := range result.RulesRun {
		ruleNames[i] = string(r)
	}
	fmt.Printf("🔍 Config lint: %s  [rules: %s]\n", configRel, strings.Join(ruleNames, ", "))

	var errorDeads, warningDeads []config.DeadPattern
	for _, dp := range result.DeadPatterns {
		if dp.Severity == config.SeverityWarning {
			warningDeads = append(warningDeads, dp)
		} else {
			errorDeads = append(errorDeads, dp)
		}
	}

	if len(errorDeads) == 0 && len(warningDeads) == 0 && len(result.Overlaps) == 0 {
		fmt.Printf("\n✅ No issues found — every glob matches something and no patterns overlap.\n")
		return
	}

	if len(errorDeads) > 0 {
		printErrorSection(errorDeads)
	}
	if len(warningDeads) > 0 || len(result.Overlaps) > 0 {
		printWarningSection(warningDeads, result.Overlaps)
	}
}

// ruleHeader prints a "📁 Rule:" / "📄 Top-level" header when the rule changes.
type ruleHeaderPrinter struct {
	ruleIndex int
	rulePath  string
	first     bool
}

func newRuleHeaderPrinter() *ruleHeaderPrinter {
	return &ruleHeaderPrinter{ruleIndex: -2, first: true}
}

func (p *ruleHeaderPrinter) print(ruleIndex int, rulePath string) bool {
	if p.ruleIndex == ruleIndex && p.rulePath == rulePath && !p.first {
		return false
	}
	if ruleIndex < 0 {
		fmt.Printf("\n📄 Top-level\n")
	} else {
		fmt.Printf("\n📁 Rule: %s\n", rulePath)
	}
	p.ruleIndex, p.rulePath, p.first = ruleIndex, rulePath, false
	return true
}

// printErrorSection lists dead positive patterns — the findings that fail the lint.
func printErrorSection(deads []config.DeadPattern) {
	fmt.Printf("\n── Errors ──\n")
	hdr := newRuleHeaderPrinter()
	lastLabel := ""
	for _, dp := range deads {
		if hdr.print(dp.RuleIndex, dp.RulePath) {
			lastLabel = ""
		}
		label := optionLabel(dp.DetectorType, dp.BoundaryIndex, dp.OptionKey)
		if label != lastLabel {
			fmt.Printf("  %s\n", label)
			lastLabel = label
		}
		suffix := kindSuffix(dp.Kind)
		if !dp.Removable {
			suffix += " [not auto-removed]"
		}
		fmt.Printf("    ✗ %q%s\n", dp.Value, suffix)
	}
}

// warnLine is a unified warning entry (a dead negation or an overlap) for display.
type warnLine struct {
	ruleIndex     int
	rulePath      string
	detectorType  string
	detectorIndex int
	boundaryIndex int
	optionKey     string
	ord1, ord2    int // stable ordering within an option
	text          string
}

// printWarningSection lists advisory findings — dead negation patterns and overlapping
// patterns — grouped by rule and option so both kinds sit together.
func printWarningSection(warningDeads []config.DeadPattern, overlaps []config.OverlapFinding) {
	lines := make([]warnLine, 0, len(warningDeads)+len(overlaps))

	for _, dp := range warningDeads {
		lines = append(lines, warnLine{
			ruleIndex: dp.RuleIndex, rulePath: dp.RulePath,
			detectorType: dp.DetectorType, detectorIndex: dp.DetectorIndex, boundaryIndex: dp.BoundaryIndex,
			optionKey: dp.OptionKey, ord1: dp.ElementIndex,
			text: fmt.Sprintf("%q  (negation matches nothing)", dp.Value),
		})
	}
	for _, o := range overlaps {
		var text string
		switch o.Kind {
		case config.OverlapDuplicate:
			text = fmt.Sprintf("%q and %q match the same files (possible duplicate)", o.PatternA, o.PatternB)
		case config.OverlapContained:
			text = fmt.Sprintf("%q is redundant — its files are all covered by %q", o.PatternA, o.PatternB)
		case config.OverlapPartial:
			text = fmt.Sprintf("%q and %q partially overlap (%d shared file(s))", o.PatternA, o.PatternB, o.SharedFileCount)
		}
		lines = append(lines, warnLine{
			ruleIndex: o.RuleIndex, rulePath: o.RulePath,
			detectorType: o.DetectorType, detectorIndex: o.DetectorIndex, boundaryIndex: o.BoundaryIndex,
			optionKey: o.OptionKey, ord1: o.ElementIndexA, ord2: o.ElementIndexB,
			text: text,
		})
	}

	sort.SliceStable(lines, func(i, j int) bool {
		a, b := lines[i], lines[j]
		switch {
		case a.ruleIndex != b.ruleIndex:
			return a.ruleIndex < b.ruleIndex
		case a.detectorType != b.detectorType:
			return a.detectorType < b.detectorType
		case a.detectorIndex != b.detectorIndex:
			return a.detectorIndex < b.detectorIndex
		case a.boundaryIndex != b.boundaryIndex:
			return a.boundaryIndex < b.boundaryIndex
		case a.optionKey != b.optionKey:
			return a.optionKey < b.optionKey
		case a.ord1 != b.ord1:
			return a.ord1 < b.ord1
		default:
			return a.ord2 < b.ord2
		}
	})

	fmt.Printf("\n── Warnings ──\n")
	hdr := newRuleHeaderPrinter()
	lastLabel := ""
	for _, l := range lines {
		if hdr.print(l.ruleIndex, l.rulePath) {
			lastLabel = ""
		}
		label := optionLabel(l.detectorType, l.boundaryIndex, l.optionKey)
		if label != lastLabel {
			fmt.Printf("  %s\n", label)
			lastLabel = label
		}
		fmt.Printf("    ⚠ %s\n", l.text)
	}
}

func kindSuffix(kind config.PatternKind) string {
	switch kind {
	case config.KindModule:
		return "  (matches no module)"
	case config.KindMixed:
		return "  (matches no file or module)"
	case config.KindDir:
		return "  (resolves to no files)"
	default:
		return "  (matches no file)"
	}
}

func printConfigLintFixSummary(fix *config.FixResult) {
	if fix.RemovedCount > 0 {
		fmt.Printf("\n✍️  Removed %d dead pattern(s).\n", fix.RemovedCount)
	}
	if fix.ReportOnlyKept > 0 {
		fmt.Printf("⚠️  %d dead pattern(s) not auto-removed (removing them could change a check's behavior or make the config invalid) — review and remove manually.\n", fix.ReportOnlyKept)
	}
	if fix.RemovedCount == 0 && fix.ReportOnlyKept == 0 {
		fmt.Printf("\n✅ Nothing to remove.\n")
	}
}

func init() {
	addSharedFlags(configLintCmd)
	configLintCmd.Flags().StringVarP(&lintConfigCwd, "cwd", "c", currentDir, "Working directory")
	configLintCmd.Flags().BoolVar(&lintConfigFix, "fix", false, "Remove dead patterns from the config file (preserves comments and formatting)")
	configLintCmd.Flags().StringSliceVar(&lintConfigRules, "rules", nil, "Lint rules to run (comma-separated): orphan-file-globs, orphan-module-globs, overlapping-globs. Default: all. orphan-file-globs and overlapping-globs run from file discovery alone (fast); orphan-module-globs parses the dependency tree.")

	configCmd.AddCommand(configLintCmd)
}
