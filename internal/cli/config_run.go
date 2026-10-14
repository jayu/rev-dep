package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/dupcode"
	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/perf"
	"rev-dep-go/internal/telemetry"
)

// ---------------- config ----------------
var (
	configCwd string
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Create and execute rev-dep configuration files",
	Long:  `Commands for creating and executing rev-dep configuration files.`,
}

// ---------------- config run ----------------
var (
	runConfigCwd            string
	runConfigListAll        bool
	runConfigFix            bool
	runConfigUpdateSnapshot bool
	runConfigRecheck        bool
	runConfigRules          []string
	runConfigFormat         string
	runConfigLint           bool
	runConfigLintRules      []string
)

var configRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute all checks defined in (.)rev-dep.config.json(c)",
	Long:  `Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, import conventions, node modules, unused exports, unresolved imports, restricted imports and restricted dev deps usage) per workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		if err := validateConfigRunFlags(); err != nil {
			return err
		}
		cwd := pathutil.ResolveAbsoluteCwd(runConfigCwd)

		if legacy, version, ok := config.IsLegacyV1Config(cwd); ok && legacy {
			return legacyConfigError(version)
		}

		// Auto-discover config in current working directory
		cfg, err := config.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("Could not load configuration from %s:\n%v", filepath.Join(cwd, config.ConfigFileName()), err)
		}

		switch runConfigFormat {
		case configRunFormatJSON:
			return runConfigWithJSONOutput(cfg, cwd, runConfigFix, runConfigRecheck)
		case configRunFormatIssuesList:
			return runConfigWithIssuesListOutput(cfg, cwd, runConfigFix, runConfigRecheck)
		}

		lintWanted := configRunLintWanted()

		if err := filterRunConfigRules(&cfg, runConfigRules); err != nil {
			return err
		}

		result, err := processConfigRun(&cfg, cwd, runConfigFix, runConfigRecheck, false)
		if err != nil {
			return fmt.Errorf("Error processing config: %v", err)
		}

		// Format and print results
		formatAndPrintConfigResults(result, cwd, runConfigListAll)

		// Optionally lint the config after running. Only the error/warning counts are
		// printed here - use `rev-dep config lint` for per-finding detail and `--fix`.
		lintHasErrors := false
		if lintWanted {
			counts, err := lintConfigAfterRun(cwd, result)
			if err != nil {
				return err
			}
			printConfigLintSummary(counts)
			lintHasErrors = counts.Errors > 0
		}

		executionTime := time.Since(startTime)
		fmt.Printf("\n%s  Done in %dms.\n", emoji.Done, executionTime.Milliseconds())
		perf.Report(executionTime)

		if shouldConfigRunExitNonZero(result, runConfigFix) || lintHasErrors {
			os.Exit(1)
		}

		return nil
	},
}

const (
	configRunFormatJSON       = "json"
	configRunFormatIssuesList = "issues-list"
)

// validateConfigRunFlags rejects bad flag values before any work starts, so a typo fails in
// milliseconds with a clear message instead of being silently ignored or reported after a full run.
func validateConfigRunFlags() error {
	switch runConfigFormat {
	case "", configRunFormatJSON, configRunFormatIssuesList:
	default:
		return fmt.Errorf("invalid --format %q: expected %q or %q (omit it for the default output)",
			runConfigFormat, configRunFormatJSON, configRunFormatIssuesList)
	}
	if _, err := config.ParseLintRules(runConfigLintRules); err != nil {
		return fmt.Errorf("invalid --lint-config-rules: %v", err)
	}
	return nil
}

// configRunLintWanted reports whether config run should lint the config after running.
// --lint-config-rules implies --lint-config.
func configRunLintWanted() bool {
	return runConfigLint || len(runConfigLintRules) > 0
}

// configLintCounts is the outcome of linting the config after a run.
type configLintCounts struct {
	Errors   int
	Warnings int
}

// lintConfigAfterRun runs the config linter over the full config (independent of the run's
// --workspaces filter) using the selected --lint-config-rules. Fixing is intentionally not offered
// here; users run `rev-dep config lint --fix` for that.
//
// For an unfiltered run the discovery and dependency tree the run already built are reused, so
// linting adds almost no cost: the top-level ignoreFiles dead-check reuses the run's own discovery
// byproducts (the files it saw and the directories it pruned). A --workspaces-filtered run builds a
// narrower graph (fewer registered packages), which could make the module universe incomplete for
// the full config, so it is not reused then.
func lintConfigAfterRun(cwd string, runResult *config.ConfigProcessingResult) (configLintCounts, error) {
	lintRules, err := config.ParseLintRules(runConfigLintRules)
	if err != nil {
		return configLintCounts{}, err
	}

	// Load the config fresh so its rule order matches the raw file (the run may have
	// filtered cfg.Rules via --workspaces, which would misalign the linter's presence checks).
	lintCfg, err := config.LoadConfig(cwd)
	if err != nil {
		return configLintCounts{}, fmt.Errorf("Could not load configuration for lint: %v", err)
	}

	var graph *config.LintGraph
	if len(runConfigRules) == 0 && runResult != nil {
		graph = &config.LintGraph{
			AllFiles:            runResult.DiscoveredFiles,
			FullTree:            runResult.FullTree,
			ResolverManager:     runResult.ResolverManager,
			IgnoreScopeFiles:    runResult.IgnoreScopeFiles,
			IgnorePrunedDirs:    runResult.IgnorePrunedDirs,
			IgnoreScopeComputed: true,
		}
	}

	lintResult, err := config.LintConfigWithGraph(&lintCfg, cwd, lintRules, graph)
	if err != nil {
		return configLintCounts{}, fmt.Errorf("Error linting config: %v", err)
	}

	errors, warnings := countLintFindings(lintResult, false)
	return configLintCounts{Errors: errors, Warnings: warnings}, nil
}

// printConfigLintSummary prints only the lint error/warning counts; `rev-dep config lint` has the
// per-finding detail.
func printConfigLintSummary(counts configLintCounts) {
	switch {
	case counts.Errors > 0:
		fmt.Printf("\n%s  Config lint: %d error(s), %d warning(s) - run `rev-dep config lint` for details (or --fix to apply).\n", emoji.Error, counts.Errors, counts.Warnings)
	case counts.Warnings > 0:
		fmt.Printf("\n%s  Config lint: 0 errors, %d warning(s) - run `rev-dep config lint` for details (or --fix to apply).\n", emoji.Warning, counts.Warnings)
	default:
		fmt.Printf("\n%s  Config lint: no issues.\n", emoji.Success)
	}
}

const maxIssuesToList = 5

func workspaceLabel(path string) string {
	if path == "." {
		return ". (root)"
	}
	return path
}

func shouldConfigRunExitNonZero(result *config.ConfigProcessingResult, fix bool) bool {
	if result.Baseline != nil || !fix {
		return result.HasFailures
	}

	return hasUnfixableConfigRunIssues(result)
}

func hasUnfixableConfigRunIssues(result *config.ConfigProcessingResult) bool {
	totalIssues := 0
	fixableIssues := 0

	for _, ruleResult := range result.RuleResults {
		totalIssues += len(ruleResult.CircularDependencies)
		totalIssues += len(ruleResult.OrphanFiles)
		totalIssues += len(ruleResult.ModuleBoundaryViolations)
		totalIssues += len(ruleResult.UnusedNodeModules)
		totalIssues += len(ruleResult.MissingNodeModules)
		totalIssues += len(ruleResult.ImportConventionViolations)
		totalIssues += len(ruleResult.UnusedExports)
		totalIssues += len(ruleResult.UnresolvedImports)
		totalIssues += len(ruleResult.RestrictedDevDependenciesUsageViolations)
		totalIssues += len(ruleResult.RestrictedImportsViolations)
		totalIssues += len(ruleResult.RestrictedImportersViolations)
		totalIssues += len(ruleResult.RestrictedDirectImportersViolations)
		// Without a snapshot every duplicated snippet is an issue; with one, only what changed.
		for _, d := range ruleResult.DuplicatedCode {
			dup := d.Result
			switch {
			case d.Err != nil:
				totalIssues++
			case dup.Delta != nil:
				// Only what changed counts as an issue; acknowledged duplication does not.
				// Changed() adds up every category, so a count cannot be left out of the
				// total by being left out of this expression - which is what happened to the
				// below-threshold one.
				totalIssues += dupcode.CountsOf(dup.Delta).Changed()
			case dup.SnapshotMissing:
				totalIssues++
			default:
				totalIssues += dup.Snippets
			}
		}

		// A workspace with no files cannot be fixed by --fix.
		if ruleResult.EmptyWorkspaceFailed() {
			totalIssues++
		}

		fixableIssues += len(ruleResult.OrphanFilesAutofixable)

		for _, violation := range ruleResult.ImportConventionViolations {
			if violation.Fix != nil {
				fixableIssues++
			}
		}

		for _, unusedExport := range ruleResult.UnusedExports {
			if unusedExport.Fix != nil {
				fixableIssues++
			}
		}
	}

	return totalIssues > fixableIssues
}

// processConfigRun runs every check in the config.
func processConfigRun(
	cfg *config.RevDepConfig,
	cwd string,
	fix bool,
	recheck bool,
	forceDetailed bool,
) (*config.ConfigProcessingResult, error) {
	baselineEnabled := cfg.BaselineSnapshotPath != ""
	var previous *baseline.Snapshot
	var selected []string
	if baselineEnabled {
		if runConfigUpdateSnapshot && len(runConfigRules) > 0 {
			return nil, fmt.Errorf("--update-snapshot cannot be combined with --workspaces when baselineSnapshotPath is configured")
		}
		selected = runConfigRules
		filename, err := config.BaselineSnapshotFile(cfg, cwd)
		if err != nil {
			return nil, err
		}
		if runConfigUpdateSnapshot {
			if err := baseline.CheckDestination(filename); err != nil {
				return nil, err
			}
		} else {
			previous, err = baseline.Load(filename)
			if err != nil {
				return nil, err
			}
		}
	}
	// Baselines always observe final disk state. Defer snapshot updates until the
	// recheck when fixing, so no store captures pre-fix findings.
	updateFirstPass := runConfigUpdateSnapshot && !(baselineEnabled && fix)
	result, err := config.ProcessConfigWithOptions(cfg, cwd, fix, forceDetailed, updateFirstPass, selected...)
	if err != nil {
		return nil, err
	}
	telemetry.Dispatch(cwd, cfg, len(result.FullTree))
	if fix && (recheck || baselineEnabled) {
		updateRecheck := baselineEnabled && runConfigUpdateSnapshot
		final, err := config.ProcessConfigWithOptions(cfg, cwd, false, forceDetailed, updateRecheck, selected...)
		if err != nil {
			return nil, err
		}
		final.FixedFilesCount = result.FixedFilesCount
		final.FixedImportsCount = result.FixedImportsCount
		final.DeletedFilesCount = result.DeletedFilesCount
		final.UnfixableAliasingCount = result.UnfixableAliasingCount
		result = final
	}
	if baselineEnabled {
		if err := config.ApplyFindingsBaseline(cfg, cwd, result, previous, runConfigUpdateSnapshot, selected); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// formatAndPrintConfigResults formats and prints the config processing results
func formatAndPrintConfigResults(result *config.ConfigProcessingResult, cwd string, listAll bool) {
	fmt.Print(formatConfigResults(result, cwd, listAll))
}

func init() {
	// config command
	configCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// config run command. It does not use addSharedFlags: those resolution flags
	// (--condition-names, --follow-monorepo-packages, --package-json, --tsconfig-json) are
	// configured per workspace in the config file instead. Passing them is now an unknown-flag
	// error, which is what we want.
	configRunCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "Show warnings and verbose output")
	configRunCmd.Flags().StringVarP(&runConfigCwd, "cwd", "c", currentDir, "Working directory")
	configRunCmd.Flags().BoolVar(&runConfigListAll, "list-all-issues", false, "List all issues instead of limiting output")
	configRunCmd.Flags().BoolVar(&runConfigFix, "fix", false, "Automatically fix fixable issues")
	configRunCmd.Flags().BoolVar(&runConfigUpdateSnapshot, "update-snapshot", false,
		"Rewrite the configured findings baseline and duplicated-code snapshots, acknowledging current findings.")
	configRunCmd.Flags().BoolVar(&runConfigRecheck, "recheck", false, "Run all checks again after '--fix' to validate the final state")
	configRunCmd.Flags().StringVar(&runConfigFormat, "format", "", "Output format (json, issues-list)")
	configRunCmd.Flags().StringSliceVar(&runConfigRules, "workspaces", []string{}, "Subset of workspaces to run (comma-separated list of workspace paths)")
	configRunCmd.Flags().BoolVar(&runConfigLint, "lint-config", false, "Also lint the config after running; prints only error/warning counts and fails (non-zero exit) on any lint error. Use 'config lint' for details and --fix")
	configRunCmd.Flags().StringSliceVar(&runConfigLintRules, "lint-config-rules", nil, "Which lint rules to run with --lint-config (comma-separated). Default: all. Implies --lint-config")

	// config init command
	configInitCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// Add subcommands to config
	configCmd.AddCommand(configRunCmd, configInitCmd)
}
