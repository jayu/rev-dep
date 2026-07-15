package cli

import (
	"fmt"
	"os"
	"path/filepath"
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

		exitNonZero := false
		if lintConfigFix && len(result.DeadPatterns) > 0 {
			fixResult, err := config.ApplyLintFix(result)
			if err != nil {
				return fmt.Errorf("Error applying fixes: %v", err)
			}
			printConfigLintFixSummary(fixResult)
			exitNonZero = fixResult.ReportOnlyKept > 0
		} else {
			exitNonZero = len(result.DeadPatterns) > 0
		}

		fmt.Printf("\n✨  Done in %dms.\n", time.Since(startTime).Milliseconds())

		if exitNonZero {
			os.Exit(1)
		}
		return nil
	},
}

// deadPatternLabel builds the human-readable option path for a dead pattern.
func deadPatternLabel(dp config.DeadPattern) string {
	label := dp.OptionKey
	if dp.DetectorType != "" {
		if dp.DetectorType == "moduleBoundaries" {
			label = fmt.Sprintf("moduleBoundaries[%d].%s", dp.BoundaryIndex, dp.OptionKey)
		} else {
			label = fmt.Sprintf("%s.%s", dp.DetectorType, dp.OptionKey)
		}
	}
	return label
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

	if len(result.DeadPatterns) == 0 {
		fmt.Printf("\n✅ No dead patterns found — every glob and path matches something.\n")
		return
	}

	// Group by rule, then by option label, preserving the sorted order from LintConfig.
	type header struct {
		ruleIndex int
		rulePath  string
	}
	printedHeader := header{ruleIndex: -2}
	lastLabel := ""

	for _, dp := range result.DeadPatterns {
		h := header{ruleIndex: dp.RuleIndex, rulePath: dp.RulePath}
		if h != printedHeader {
			if dp.RuleIndex < 0 {
				fmt.Printf("\n📄 Top-level\n")
			} else {
				fmt.Printf("\n📁 Rule: %s\n", dp.RulePath)
			}
			printedHeader = h
			lastLabel = ""
		}

		label := deadPatternLabel(dp)
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
	configLintCmd.Flags().StringSliceVar(&lintConfigRules, "rules", nil, "Lint rules to run (comma-separated): orphan-file-globs, orphan-module-globs. Default: all. Note: orphan-file-globs runs from file discovery alone (fast); orphan-module-globs parses the dependency tree.")

	configCmd.AddCommand(configLintCmd)
}
