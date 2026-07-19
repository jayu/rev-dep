package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/node"
	"rev-dep-go/internal/pathutil"
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
	runConfigCwd       string
	runConfigListAll   bool
	runConfigFix       bool
	runConfigRecheck   bool
	runConfigRules     []string
	runConfigFormat    string
	runConfigLint      bool
	runConfigLintRules []string
)

var configRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute all checks defined in (.)rev-dep.config.json(c)",
	Long:  `Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, import conventions, node modules, unused exports, unresolved imports, restricted imports and restricted dev deps usage) per workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		cwd := pathutil.ResolveAbsoluteCwd(runConfigCwd)

		// Auto-discover config in current working directory
		cfg, err := config.LoadConfig(cwd)
		if err != nil {
			return fmt.Errorf("Could not load configuration from %s:\n%v", filepath.Join(cwd, config.ConfigFileName()), err)
		}

		if runConfigFormat == "json" {
			return runConfigWithJSONOutput(cfg, cwd, packageJsonPath, tsconfigJsonPath, runConfigFix, runConfigRecheck)
		}
		if runConfigFormat == "issues-list" {
			return runConfigWithIssuesListOutput(cfg, cwd, packageJsonPath, tsconfigJsonPath, runConfigFix, runConfigRecheck)
		}

		// When linting after the run and reusing the run's graph, the top-level ignoreFiles
		// dead-check reuses the run's own discovery byproducts (the files it saw and the
		// directories it pruned), so no extra walk is needed. Reuse is only safe for an
		// unfiltered run (see runConfigLintSummary).
		lintWanted := runConfigLint || len(runConfigLintRules) > 0
		reuseGraphForLint := lintWanted && len(runConfigRules) == 0

		if err := filterRunConfigRules(&cfg, runConfigRules); err != nil {
			return err
		}

		result, err := processConfigRun(&cfg, cwd, packageJsonPath, tsconfigJsonPath, runConfigFix, runConfigRecheck, false)
		if err != nil {
			return fmt.Errorf("Error processing config: %v", err)
		}

		// Format and print results
		formatAndPrintConfigResults(result, cwd, runConfigListAll)

		// Optionally lint the config after running. Only the error/warning counts are
		// printed here — use `rev-dep config lint` for per-finding detail and `--fix`.
		lintHasErrors := false
		if lintWanted {
			failed, err := runConfigLintSummary(cwd, result, reuseGraphForLint)
			if err != nil {
				return err
			}
			lintHasErrors = failed
		}

		executionTime := time.Since(startTime)
		fmt.Printf("\n%s  Done in %dms.\n", emoji.Done, executionTime.Milliseconds())

		if shouldConfigRunExitNonZero(result, runConfigFix) || lintHasErrors {
			os.Exit(1)
		}

		return nil
	},
}

// runConfigLintSummary runs the config linter over the full config (independent of the
// run's --workspaces filter) using the selected --lint-config-rules, prints only the error/warning
// counts, and reports whether any lint ERROR was found. Fixing is intentionally not
// offered here; users run `rev-dep config lint --fix` for that.
//
// When reuseGraph is true, the discovery + dependency tree the run already built are
// reused, so linting adds almost no cost. It is only safe to reuse when the run was NOT
// rule-filtered: a filtered run builds a narrower graph (fewer registered packages),
// which could make the module universe incomplete for the full config.
func runConfigLintSummary(cwd string, runResult *config.ConfigProcessingResult, reuseGraph bool) (hasErrors bool, err error) {
	lintRules, err := config.ParseLintRules(runConfigLintRules)
	if err != nil {
		return false, err
	}

	// Load the config fresh so its rule order matches the raw file (the run may have
	// filtered cfg.Rules via --workspaces, which would misalign the linter's presence checks).
	lintCfg, err := config.LoadConfig(cwd)
	if err != nil {
		return false, fmt.Errorf("Could not load configuration for lint: %v", err)
	}

	var graph *config.LintGraph
	if reuseGraph && runResult != nil {
		graph = &config.LintGraph{
			AllFiles:            runResult.DiscoveredFiles,
			FullTree:            runResult.FullTree,
			ResolverManager:     runResult.ResolverManager,
			IgnoreScopeFiles:    runResult.IgnoreScopeFiles,
			IgnorePrunedDirs:    runResult.IgnorePrunedDirs,
			IgnoreScopeComputed: true,
		}
	}

	lintResult, err := config.LintConfigWithGraph(&lintCfg, cwd, packageJsonPath, tsconfigJsonPath, lintRules, graph)
	if err != nil {
		return false, fmt.Errorf("Error linting config: %v", err)
	}

	errors, warnings := countLintFindings(lintResult, false)
	switch {
	case errors > 0:
		fmt.Printf("\n%s  Config lint: %d error(s), %d warning(s) — run `rev-dep config lint` for details (or --fix to apply).\n", emoji.Error, errors, warnings)
	case warnings > 0:
		fmt.Printf("\n%s  Config lint: 0 errors, %d warning(s) — run `rev-dep config lint` for details (or --fix to apply).\n", emoji.Warning, warnings)
	default:
		fmt.Printf("\n%s  Config lint: no issues.\n", emoji.Success)
	}
	return errors > 0, nil
}

const maxIssuesToList = 5

func shouldConfigRunExitNonZero(result *config.ConfigProcessingResult, fix bool) bool {
	if !fix {
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

func processConfigRun(
	cfg *config.RevDepConfig,
	cwd string,
	packageJsonPath string,
	tsconfigJsonPath string,
	fix bool,
	recheck bool,
	forceDetailed bool,
) (*config.ConfigProcessingResult, error) {
	result, err := config.ProcessConfig(cfg, cwd, packageJsonPath, tsconfigJsonPath, fix, forceDetailed)
	if err != nil {
		return nil, err
	}

	// Fire anonymous telemetry for this config run. This spawns a detached reporter and returns
	// immediately; it is a no-op under tests, when opted out, or in builds without a baked-in key.
	telemetry.Dispatch(cwd, cfg, len(result.FullTree))

	if !fix || !recheck {
		return result, nil
	}

	recheckedResult, err := config.ProcessConfig(cfg, cwd, packageJsonPath, tsconfigJsonPath, false, forceDetailed)
	if err != nil {
		return nil, err
	}

	recheckedResult.FixedFilesCount = result.FixedFilesCount
	recheckedResult.FixedImportsCount = result.FixedImportsCount
	recheckedResult.DeletedFilesCount = result.DeletedFilesCount
	recheckedResult.UnfixableAliasingCount = result.UnfixableAliasingCount

	return recheckedResult, nil
}

// formatAndPrintConfigResults formats and prints the config processing results
func formatAndPrintConfigResults(result *config.ConfigProcessingResult, cwd string, listAll bool) {
	// Helper function to convert absolute paths to relative paths
	getRelativePath := func(absolutePath string) string {
		if absolutePath == "" {
			return absolutePath
		}
		relPath, err := filepath.Rel(cwd, absolutePath)
		if err != nil {
			// Fallback to absolute path if relative conversion fails
			return absolutePath
		}
		// Convert forward slashes for consistency
		return filepath.ToSlash(relPath)
	}

	shouldWarnAboutImportConventionWithPJsonImports := false

	for _, ruleResult := range result.RuleResults {
		shouldWarnAboutImportConventionWithPJsonImports = shouldWarnAboutImportConventionWithPJsonImports || ruleResult.ShouldWarnAboutImportConventionWithPJsonImports

		if ruleResult.RulePath != "" {
			fmt.Printf("\n%s Rule: %s (%d files)\n", emoji.Rule, ruleResult.RulePath, ruleResult.FileCount)
		}

		// Show enabled checks and their status with indentation
		for _, check := range ruleResult.EnabledChecks {
			switch check {
			case "circular-imports":
				if len(ruleResult.CircularDependencies) > 0 {
					fmt.Printf("  %s Circular Dependencies Issues (%d):\n\n", emoji.Error, len(ruleResult.CircularDependencies))

					circularDepsToDisplay := ruleResult.CircularDependencies
					remaining := 0
					if !listAll && len(circularDepsToDisplay) > maxIssuesToList {
						remaining = len(circularDepsToDisplay) - maxIssuesToList
						circularDepsToDisplay = circularDepsToDisplay[:maxIssuesToList]
					}

					formattedOutput := checks.FormatCircularDependenciesWithoutHeader(circularDepsToDisplay, cwd, ruleResult.DependencyTree, 2)
					fmt.Printf("%s", formattedOutput)

					if remaining > 0 {
						fmt.Printf("    ... and %d more circular dependency issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Circular Dependencies\n", emoji.Success)
				}
			case "orphan-files":
				if len(ruleResult.OrphanFiles) > 0 {
					fmt.Printf("  %s  Orphan Files Issues (%d):\n", emoji.Error, len(ruleResult.OrphanFiles))

					orphanFilesToDisplay := ruleResult.OrphanFiles
					remaining := 0
					if !listAll && len(orphanFilesToDisplay) > maxIssuesToList {
						remaining = len(orphanFilesToDisplay) - maxIssuesToList
						orphanFilesToDisplay = orphanFilesToDisplay[:maxIssuesToList]
					}

					for _, file := range orphanFilesToDisplay {
						fmt.Printf("    - %s\n", getRelativePath(file))
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more orphan file issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Orphan Files\n", emoji.Success)
				}
			case "module-boundaries":
				if len(ruleResult.ModuleBoundaryViolations) > 0 {
					fmt.Printf("  %s Module Boundary Issues (%d):\n", emoji.Error, len(ruleResult.ModuleBoundaryViolations))

					violationsToDisplay := ruleResult.ModuleBoundaryViolations
					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					for _, violation := range violationsToDisplay {
						violationType := "NOT ALLOWED"
						if violation.ViolationType == "denied" {
							violationType = "DENIED"
						}
						fmt.Printf("    - [%s] %s -> %s (%s)\n",
							violation.RuleName,
							getRelativePath(violation.FilePath),
							getRelativePath(violation.ImportPath),
							violationType)
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more module boundary issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Module Boundaries\n", emoji.Success)
				}
			case "unused-node-modules":
				if len(ruleResult.UnusedNodeModules) > 0 {
					fmt.Printf("  %s Unused Node Modules Issues (%d):\n", emoji.Error, len(ruleResult.UnusedNodeModules))

					modulesToDisplay := ruleResult.UnusedNodeModules
					remaining := 0
					if !listAll && len(modulesToDisplay) > maxIssuesToList {
						remaining = len(modulesToDisplay) - maxIssuesToList
						modulesToDisplay = modulesToDisplay[:maxIssuesToList]
					}

					for _, module := range modulesToDisplay {
						packageJsonPath := getRelativePath(module.PackageJsonPath)
						if packageJsonPath != "" {
							fmt.Printf("    - %s (%s)\n", module.ModuleName, packageJsonPath)
						} else {
							fmt.Printf("    - %s\n", module.ModuleName)
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more unused node module issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Unused Node Modules\n", emoji.Success)
				}
			case "missing-node-modules":
				if len(ruleResult.MissingNodeModules) > 0 {
					fmt.Printf("  %s Missing Node Modules Issues (%d):\n", emoji.Error, len(ruleResult.MissingNodeModules))

					missingToDisplay := ruleResult.MissingNodeModules
					remaining := 0
					if !listAll && len(missingToDisplay) > maxIssuesToList {
						remaining = len(missingToDisplay) - maxIssuesToList
						missingToDisplay = missingToDisplay[:maxIssuesToList]
					}

					if ruleResult.MissingNodeModulesOutputType != "" {
						groupByModule := ruleResult.MissingNodeModulesOutputType == "groupByModule"
						groupByFile := ruleResult.MissingNodeModulesOutputType == "groupByFile"
						groupByModuleFilesCount := ruleResult.MissingNodeModulesOutputType == "groupByModuleFilesCount"

						formatted, _ := node.FormatMissingNodeModulesResults(missingToDisplay, cwd, false, groupByModule, groupByFile, groupByModuleFilesCount)
						lines := strings.Split(strings.TrimSpace(formatted), "\n")
						for _, line := range lines {
							if line == "" {
								continue
							}
							if groupByModuleFilesCount || ruleResult.MissingNodeModulesOutputType == "list" {
								fmt.Printf("    - %s\n", line)
							} else {
								fmt.Printf("    %s\n", line)
							}
						}
					} else {
						for _, missing := range missingToDisplay {
							// Convert imported from paths to relative paths
							relativeImportedFrom := make([]string, len(missing.ImportedFrom))
							for j, path := range missing.ImportedFrom {
								relativeImportedFrom[j] = getRelativePath(path)
							}

							if len(relativeImportedFrom) == 1 {
								fmt.Printf("    - %s (imported from: %s)\n", missing.ModuleName, relativeImportedFrom[0])
							} else if len(relativeImportedFrom) > 1 {
								fmt.Printf("    - %s (imported from: %s and %d more files)\n", missing.ModuleName, relativeImportedFrom[0], len(relativeImportedFrom)-1)
							}
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more missing node module issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Missing Node Modules\n", emoji.Success)
				}
			case "import-conventions":
				if len(ruleResult.ImportConventionViolations) > 0 {
					fmt.Printf("  %s Import Convention Issues (%d):\n", emoji.Error, len(ruleResult.ImportConventionViolations))

					violationsToDisplay := ruleResult.ImportConventionViolations

					// Sort violations by file path and then by import index
					slices.SortFunc(violationsToDisplay, func(a, b checks.ImportConventionViolation) int {
						if a.FilePath != b.FilePath {
							return strings.Compare(a.FilePath, b.FilePath)
						}
						return a.ImportIndex - b.ImportIndex
					})

					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					// Group violations by file path
					violationsByFile := make(map[string][]checks.ImportConventionViolation)
					for _, violation := range violationsToDisplay {
						violationsByFile[violation.FilePath] = append(violationsByFile[violation.FilePath], violation)
					}

					// Sort file paths for consistent output
					var sortedFilePaths []string
					for filePath := range violationsByFile {
						sortedFilePaths = append(sortedFilePaths, filePath)
					}
					slices.Sort(sortedFilePaths)

					for _, filePath := range sortedFilePaths {
						fmt.Printf("    %s\n", getRelativePath(filePath))
						fileViolations := violationsByFile[filePath]
						for _, violation := range fileViolations {
							fmt.Printf("     - [%s] : \"%s\"\n", violation.ViolationType, violation.ImportRequest)
						}
						fmt.Printf("\n")
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more import convention issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Import Conventions\n", emoji.Success)
				}
			case "unresolved-imports":
				if len(ruleResult.UnresolvedImports) > 0 {
					fmt.Printf("  %s Unresolved Imports (%d):\n", emoji.Error, len(ruleResult.UnresolvedImports))

					// Sort all results before limiting
					unresolvedToDisplay := ruleResult.UnresolvedImports
					slices.SortFunc(unresolvedToDisplay, func(a, b checks.UnresolvedImport) int {
						if a.FilePath != b.FilePath {
							return strings.Compare(a.FilePath, b.FilePath)
						}
						return strings.Compare(a.Request, b.Request)
					})

					remaining := 0
					if !listAll && len(unresolvedToDisplay) > maxIssuesToList {
						remaining = len(unresolvedToDisplay) - maxIssuesToList
						unresolvedToDisplay = unresolvedToDisplay[:maxIssuesToList]
					}

					// Group by file
					unresolvedByFile := make(map[string][]string)
					for _, u := range unresolvedToDisplay {
						unresolvedByFile[u.FilePath] = append(unresolvedByFile[u.FilePath], u.Request)
					}

					var sortedFilePaths []string
					for fp := range unresolvedByFile {
						sortedFilePaths = append(sortedFilePaths, fp)
					}
					slices.Sort(sortedFilePaths)

					for _, filePath := range sortedFilePaths {
						fmt.Printf("    %s\n", getRelativePath(filePath))
						for _, req := range unresolvedByFile[filePath] {
							fmt.Printf("     - %s\n", req)
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more unresolved import issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Unresolved Imports\n", emoji.Success)
				}
			case "unused-exports":
				if len(ruleResult.UnusedExports) > 0 {
					fmt.Printf("  %s Unused Exports Issues (%d):\n", emoji.Error, len(ruleResult.UnusedExports))

					exportsToDisplay := ruleResult.UnusedExports

					// Sort exports by file path and then by export name
					slices.SortFunc(exportsToDisplay, func(a, b checks.UnusedExport) int {
						if a.FilePath != b.FilePath {
							return strings.Compare(a.FilePath, b.FilePath)
						}
						return strings.Compare(a.ExportName, b.ExportName)
					})

					remaining := 0
					if !listAll && len(exportsToDisplay) > maxIssuesToList {
						remaining = len(exportsToDisplay) - maxIssuesToList
						exportsToDisplay = exportsToDisplay[:maxIssuesToList]
					}

					// Group by file path
					exportsByFile := make(map[string][]checks.UnusedExport)
					for _, ue := range exportsToDisplay {
						exportsByFile[ue.FilePath] = append(exportsByFile[ue.FilePath], ue)
					}

					var sortedFilePaths []string
					for filePath := range exportsByFile {
						sortedFilePaths = append(sortedFilePaths, filePath)
					}
					slices.Sort(sortedFilePaths)

					for _, filePath := range sortedFilePaths {
						fmt.Printf("    %s\n", getRelativePath(filePath))
						for _, ue := range exportsByFile[filePath] {
							if ue.IsType {
								fmt.Printf("     - %s (type)\n", ue.ExportName)
							} else {
								fmt.Printf("     - %s\n", ue.ExportName)
							}
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more unused export issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Unused Exports\n", emoji.Success)
				}
			case "dev-deps-usage-on-prod":
				if len(ruleResult.RestrictedDevDependenciesUsageViolations) > 0 {
					fmt.Printf("  %s Dev Deps Usage On Prod Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedDevDependenciesUsageViolations))

					violationsToDisplay := ruleResult.RestrictedDevDependenciesUsageViolations
					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					// Group by dev dependency
					violationsByDep := make(map[string][]checks.RestrictedDevDependenciesUsageViolation)
					for _, violation := range violationsToDisplay {
						violationsByDep[violation.DevDependency] = append(violationsByDep[violation.DevDependency], violation)
					}

					// Sort dev dependencies for consistent output
					var sortedDeps []string
					for dep := range violationsByDep {
						sortedDeps = append(sortedDeps, dep)
					}
					slices.Sort(sortedDeps)

					for _, dep := range sortedDeps {
						fmt.Printf("    %s\n", dep)
						depViolations := violationsByDep[dep]
						for _, violation := range depViolations {
							fmt.Printf("     - %s (from entry point: %s)\n", getRelativePath(violation.FilePath), getRelativePath(violation.EntryPoint))
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more Dev Deps Usage On Prod Issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Dev Deps Usage On Prod\n", emoji.Success)
				}
			case "restricted-imports":
				if len(ruleResult.RestrictedImportsViolations) > 0 {
					fmt.Printf("  %s Restricted Imports Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedImportsViolations))

					violationsToDisplay := ruleResult.RestrictedImportsViolations
					slices.SortFunc(violationsToDisplay, func(a, b checks.RestrictedImportViolation) int {
						if a.EntryPoint != b.EntryPoint {
							return strings.Compare(a.EntryPoint, b.EntryPoint)
						}
						if a.ViolationType != b.ViolationType {
							return strings.Compare(a.ViolationType, b.ViolationType)
						}

						aItem := a.DeniedFile
						if a.ViolationType == "module" {
							if a.ImportRequest != "" {
								aItem = a.ImportRequest
							} else {
								aItem = a.DeniedModule
							}
						}
						bItem := b.DeniedFile
						if b.ViolationType == "module" {
							if b.ImportRequest != "" {
								bItem = b.ImportRequest
							} else {
								bItem = b.DeniedModule
							}
						}
						if aItem != bItem {
							return strings.Compare(aItem, bItem)
						}

						return strings.Compare(a.ImporterFile, b.ImporterFile)
					})

					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					violationsByEntryPoint := make(map[string][]string)
					seenByEntryPoint := make(map[string]map[string]bool)

					for _, violation := range violationsToDisplay {
						entryPoint := getRelativePath(violation.EntryPoint)
						if _, ok := seenByEntryPoint[entryPoint]; !ok {
							seenByEntryPoint[entryPoint] = make(map[string]bool)
						}

						item := ""
						if violation.ViolationType == "file" {
							item = getRelativePath(violation.DeniedFile)
						} else if violation.ImportRequest != "" {
							item = violation.ImportRequest
						} else {
							item = violation.DeniedModule
						}

						if !seenByEntryPoint[entryPoint][item] {
							violationsByEntryPoint[entryPoint] = append(violationsByEntryPoint[entryPoint], item)
							seenByEntryPoint[entryPoint][item] = true
						}
					}

					var sortedEntryPoints []string
					for entryPoint := range violationsByEntryPoint {
						sortedEntryPoints = append(sortedEntryPoints, entryPoint)
					}
					slices.Sort(sortedEntryPoints)

					for _, entryPoint := range sortedEntryPoints {
						fmt.Printf("    %s\n", entryPoint)
						items := violationsByEntryPoint[entryPoint]
						slices.Sort(items)
						for _, item := range items {
							fmt.Printf("     ➞ %s\n", item)
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more restricted import issues\n", remaining)
					}

					printRestrictedImportsResolveHint(ruleResult, cwd)
				} else {
					fmt.Printf("  %s Restricted Imports\n", emoji.Success)
				}
			case "restricted-importers":
				if len(ruleResult.RestrictedImportersViolations) > 0 {
					fmt.Printf("  %s Restricted Importers Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedImportersViolations))

					violationsToDisplay := ruleResult.RestrictedImportersViolations
					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					violationsByEntryPoint := make(map[string][]string)
					seenByEntryPoint := make(map[string]map[string]bool)
					var sortedEntryPoints []string

					for _, violation := range violationsToDisplay {
						entryPoint := getRelativePath(violation.EntryPoint)
						if _, ok := seenByEntryPoint[entryPoint]; !ok {
							seenByEntryPoint[entryPoint] = make(map[string]bool)
							sortedEntryPoints = append(sortedEntryPoints, entryPoint)
						}

						item := violation.Module
						if item == "" {
							item = getRelativePath(violation.File)
						}

						if !seenByEntryPoint[entryPoint][item] {
							violationsByEntryPoint[entryPoint] = append(violationsByEntryPoint[entryPoint], item)
							seenByEntryPoint[entryPoint][item] = true
						}
					}

					slices.Sort(sortedEntryPoints)
					for _, entryPoint := range sortedEntryPoints {
						fmt.Printf("    %s\n", entryPoint)
						items := violationsByEntryPoint[entryPoint]
						slices.Sort(items)
						for _, item := range items {
							fmt.Printf("     ➞ %s\n", item)
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more restricted importer issues\n", remaining)
					}

					printRestrictedImportersResolveHint(ruleResult, cwd)
				} else {
					fmt.Printf("  %s Restricted Importers\n", emoji.Success)
				}
			case "restricted-direct-importers":
				if len(ruleResult.RestrictedDirectImportersViolations) > 0 {
					fmt.Printf("  %s Restricted Direct Importers Issues (%d):\n", emoji.Error, len(ruleResult.RestrictedDirectImportersViolations))

					violationsToDisplay := ruleResult.RestrictedDirectImportersViolations
					remaining := 0
					if !listAll && len(violationsToDisplay) > maxIssuesToList {
						remaining = len(violationsToDisplay) - maxIssuesToList
						violationsToDisplay = violationsToDisplay[:maxIssuesToList]
					}

					// Group by importer file (the actionable location), listing the targets it imports.
					targetsByImporter := make(map[string][]string)
					seenByImporter := make(map[string]map[string]bool)
					var sortedImporters []string

					for _, violation := range violationsToDisplay {
						importer := getRelativePath(violation.ImporterFile)
						if _, ok := seenByImporter[importer]; !ok {
							seenByImporter[importer] = make(map[string]bool)
							sortedImporters = append(sortedImporters, importer)
						}

						item := violation.Module
						if item == "" {
							item = getRelativePath(violation.File)
						}

						if !seenByImporter[importer][item] {
							targetsByImporter[importer] = append(targetsByImporter[importer], item)
							seenByImporter[importer][item] = true
						}
					}

					slices.Sort(sortedImporters)
					for _, importer := range sortedImporters {
						fmt.Printf("    %s\n", importer)
						items := targetsByImporter[importer]
						slices.Sort(items)
						for _, item := range items {
							fmt.Printf("     ➞ %s\n", item)
						}
					}

					if remaining > 0 {
						fmt.Printf("    ... and %d more restricted direct importer issues\n", remaining)
					}
				} else {
					fmt.Printf("  %s Restricted Direct Importers\n", emoji.Success)
				}
			}
		}

		// Show warning if no files found for this rule
		if ruleResult.FileCount == 0 {
			fmt.Printf("  %s  No files found for this rule - check if the path is correct\n", emoji.Warning)
		}

		// Show warning if package.json is missing in the rule path directory
		if ruleResult.MissingPackageJson {
			packageJsonPath := filepath.Join(cwd, ruleResult.RulePath, "package.json")
			fmt.Printf("  %s  Warning: Rule path missing package.json - some features may not work (missing: %s)\n", emoji.Warning, packageJsonPath)
		}
	}

	// Print final verdict
	if !result.HasFailures {
		fmt.Printf("\n%s All checks passed!\n", emoji.Success)
	} else {
		fmt.Printf("\n%s Checks failed! See details above.\n", emoji.Error)
	}

	// Print autofix summary if any fixes were applied or unfixable issues found
	if result.FixedFilesCount > 0 || result.FixedImportsCount > 0 || result.DeletedFilesCount > 0 {
		var summary []string
		if result.FixedImportsCount > 0 || result.FixedFilesCount > 0 {
			summary = append(summary, fmt.Sprintf("fixed %d imports in %d files", result.FixedImportsCount, result.FixedFilesCount))
		}
		if result.DeletedFilesCount > 0 {
			summary = append(summary, fmt.Sprintf("removed %d orphan files", result.DeletedFilesCount))
		}

		if len(summary) > 0 {
			// Capitalize first letter of first summary part
			summary[0] = strings.ToUpper(summary[0][:1]) + summary[0][1:]
			fmt.Printf("%s %s\n", emoji.Fix, strings.Join(summary, ", "))
		}
	}

	if result.FixableIssuesCount > 0 {
		fmt.Printf("%s Fixable issues: %d. Use '--fix' flag to autofix.\n", emoji.Tip, result.FixableIssuesCount)
	}

	if result.UnfixableAliasingCount > 0 {
		fmt.Printf("%s Warning: %d inter-domain relative imports could not be automatically fixed because target domains lack aliases or are not defined in config.\n", emoji.Warning, result.UnfixableAliasingCount)
	}
	if shouldWarnAboutImportConventionWithPJsonImports {
		fmt.Println(emoji.Warning + " Warning: Support for package.json imports map aliases is not yet implemented for import conventions checks")
	}
}

func init() {
	// config command
	configCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// config run command
	addSharedFlags(configRunCmd)
	configRunCmd.Flags().StringVarP(&runConfigCwd, "cwd", "c", currentDir, "Working directory")
	configRunCmd.Flags().BoolVar(&runConfigListAll, "list-all-issues", false, "List all issues instead of limiting output")
	configRunCmd.Flags().BoolVar(&runConfigFix, "fix", false, "Automatically fix fixable issues")
	configRunCmd.Flags().BoolVar(&runConfigRecheck, "recheck", false, "Run all checks again after '--fix' to validate the final state")
	configRunCmd.Flags().StringVar(&runConfigFormat, "format", "", "Output format (json, issues-list)")
	configRunCmd.Flags().StringSliceVar(&runConfigRules, "workspaces", []string{}, "Subset of workspaces to run (comma-separated list of workspace paths)")
	configRunCmd.Flags().BoolVar(&runConfigLint, "lint-config", false, "Also lint the config after running; prints only error/warning counts and fails (non-zero exit) on any lint error. Use `config lint` for details and --fix")
	configRunCmd.Flags().StringSliceVar(&runConfigLintRules, "lint-config-rules", nil, "Which lint rules to run with --lint-config (comma-separated). Default: all. Implies --lint-config")

	// config init command
	configInitCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// Add subcommands to config
	configCmd.AddCommand(configRunCmd, configInitCmd)
}
