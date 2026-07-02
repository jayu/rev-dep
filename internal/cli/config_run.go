package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/monorepo"
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
	runConfigCwd     string
	runConfigListAll bool
	runConfigFix     bool
	runConfigRecheck bool
	runConfigRules   []string
	runConfigFormat  string
)

var configRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute all checks defined in (.)rev-dep.config.json(c)",
	Long:  `Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, import conventions, node modules, unused exports, unresolved imports, restricted imports and restricted dev deps usage) per rule.`,
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

		if err := filterRunConfigRules(&cfg, runConfigRules); err != nil {
			return err
		}

		result, err := processConfigRun(&cfg, cwd, packageJsonPath, tsconfigJsonPath, runConfigFix, runConfigRecheck, false)
		if err != nil {
			return fmt.Errorf("Error processing config: %v", err)
		}

		// Format and print results
		formatAndPrintConfigResults(result, cwd, runConfigListAll)
		executionTime := time.Since(startTime)
		fmt.Printf("\n✨  Done in %dms.\n", executionTime.Milliseconds())

		if shouldConfigRunExitNonZero(result, runConfigFix) {
			os.Exit(1)
		}

		return nil
	},
}

// ---------------- config init ----------------
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new rev-dep.config.json file",
	Long:  `Create a new rev-dep.config.json configuration file in the current directory with default settings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd := pathutil.ResolveAbsoluteCwd(configCwd)
		configPath, rules, createdForMonorepoSubPackage, err := initConfigFileCore(cwd)
		if err != nil {
			return err
		}
		printInitConfigResults(configPath, rules, createdForMonorepoSubPackage)
		return nil
	},
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
			fmt.Printf("\n📁 Rule: %s (%d files)\n", ruleResult.RulePath, ruleResult.FileCount)
		}

		// Show enabled checks and their status with indentation
		for _, check := range ruleResult.EnabledChecks {
			switch check {
			case "circular-imports":
				if len(ruleResult.CircularDependencies) > 0 {
					fmt.Printf("  ❌ Circular Dependencies Issues (%d):\n\n", len(ruleResult.CircularDependencies))

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
					fmt.Printf("  ✅ Circular Dependencies\n")
				}
			case "orphan-files":
				if len(ruleResult.OrphanFiles) > 0 {
					fmt.Printf("  ❌  Orphan Files Issues (%d):\n", len(ruleResult.OrphanFiles))

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
					fmt.Printf("  ✅ Orphan Files\n")
				}
			case "module-boundaries":
				if len(ruleResult.ModuleBoundaryViolations) > 0 {
					fmt.Printf("  ❌ Module Boundary Issues (%d):\n", len(ruleResult.ModuleBoundaryViolations))

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
					fmt.Printf("  ✅ Module Boundaries\n")
				}
			case "unused-node-modules":
				if len(ruleResult.UnusedNodeModules) > 0 {
					fmt.Printf("  ❌ Unused Node Modules Issues (%d):\n", len(ruleResult.UnusedNodeModules))

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
					fmt.Printf("  ✅ Unused Node Modules\n")
				}
			case "missing-node-modules":
				if len(ruleResult.MissingNodeModules) > 0 {
					fmt.Printf("  ❌ Missing Node Modules Issues (%d):\n", len(ruleResult.MissingNodeModules))

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
					fmt.Printf("  ✅ Missing Node Modules\n")
				}
			case "import-conventions":
				if len(ruleResult.ImportConventionViolations) > 0 {
					fmt.Printf("  ❌ Import Convention Issues (%d):\n", len(ruleResult.ImportConventionViolations))

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
					fmt.Printf("  ✅ Import Conventions\n")
				}
			case "unresolved-imports":
				if len(ruleResult.UnresolvedImports) > 0 {
					fmt.Printf("  ❌ Unresolved Imports (%d):\n", len(ruleResult.UnresolvedImports))

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
					fmt.Printf("  ✅ Unresolved Imports\n")
				}
			case "unused-exports":
				if len(ruleResult.UnusedExports) > 0 {
					fmt.Printf("  ❌ Unused Exports Issues (%d):\n", len(ruleResult.UnusedExports))

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
					fmt.Printf("  ✅ Unused Exports\n")
				}
			case "dev-deps-usage-on-prod":
				if len(ruleResult.RestrictedDevDependenciesUsageViolations) > 0 {
					fmt.Printf("  ❌ Dev Deps Usage On Prod Issues (%d):\n", len(ruleResult.RestrictedDevDependenciesUsageViolations))

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
					fmt.Printf("  ✅ Dev Deps Usage On Prod\n")
				}
			case "restricted-imports":
				if len(ruleResult.RestrictedImportsViolations) > 0 {
					fmt.Printf("  ❌ Restricted Imports Issues (%d):\n", len(ruleResult.RestrictedImportsViolations))

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
					fmt.Printf("  ✅ Restricted Imports\n")
				}
			case "restricted-importers":
				if len(ruleResult.RestrictedImportersViolations) > 0 {
					fmt.Printf("  ❌ Restricted Importers Issues (%d):\n", len(ruleResult.RestrictedImportersViolations))

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
					fmt.Printf("  ✅ Restricted Importers\n")
				}
			case "restricted-direct-importers":
				if len(ruleResult.RestrictedDirectImportersViolations) > 0 {
					fmt.Printf("  ❌ Restricted Direct Importers Issues (%d):\n", len(ruleResult.RestrictedDirectImportersViolations))

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
					fmt.Printf("  ✅ Restricted Direct Importers\n")
				}
			}
		}

		// Show warning if no files found for this rule
		if ruleResult.FileCount == 0 {
			fmt.Printf("  ⚠️  No files found for this rule - check if the path is correct\n")
		}

		// Show warning if package.json is missing in the rule path directory
		if ruleResult.MissingPackageJson {
			packageJsonPath := filepath.Join(cwd, ruleResult.RulePath, "package.json")
			fmt.Printf("  ⚠️  Warning: Rule path missing package.json - some features may not work (missing: %s)\n", packageJsonPath)
		}
	}

	// Print final verdict
	if !result.HasFailures {
		fmt.Printf("\n✅ All checks passed!\n")
	} else {
		fmt.Printf("\n❌ Checks failed! See details above.\n")
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
			fmt.Printf("✍️ %s\n", strings.Join(summary, ", "))
		}
	}

	if result.FixableIssuesCount > 0 {
		fmt.Printf("💡 Fixable issues: %d. Use '--fix' flag to autofix.\n", result.FixableIssuesCount)
	}

	if result.UnfixableAliasingCount > 0 {
		fmt.Printf("⚠️ Warning: %d inter-domain relative imports could not be automatically fixed because target domains lack aliases or are not defined in config.\n", result.UnfixableAliasingCount)
	}
	if shouldWarnAboutImportConventionWithPJsonImports {
		fmt.Println("⚠️ Warning: Support for package.json imports map aliases is not yet implemented for import conventions checks")
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
	configRunCmd.Flags().StringSliceVar(&runConfigRules, "rules", []string{}, "Subset of rules to run (comma-separated list of rule paths)")

	// config init command
	configInitCmd.Flags().StringVarP(&configCwd, "cwd", "c", currentDir, "Working directory")

	// Add subcommands to config
	configCmd.AddCommand(configRunCmd, configInitCmd)
}

// initConfigFileCore creates the config file without printing results
func initConfigFileCore(cwd string) (string, []config.Rule, bool, error) {
	currentConfigVersion := "1.11"

	// Check if any config file already exists
	existingConfig, err := config.FindConfigFile(cwd)
	if err == nil && existingConfig != "" {
		return "", nil, false, fmt.Errorf("config file already exists at %s", existingConfig)
	}

	// Define the path for the new config file (always use the standard name)
	configPath := filepath.Join(cwd, ".rev-dep.config.jsonc")

	// Discover monorepo packages
	var rules []config.Rule
	monorepoCtx := monorepo.DetectMonorepo(cwd)
	createdForMonorepoSubPackage := false

	if monorepoCtx != nil {
		// If invoked from inside a monorepo but not at the workspace root,
		// create a config only for the current sub-package (single rule with Path '.').
		// cwd is in OS form (backslash on Windows, with a trailing separator) while WorkspaceRoot is
		// in internal forward-slash form (no trailing slash). Normalize BOTH the separators
		// (NormalizePathForInternal) and the trailing slash (StandardiseDirPathInternal) before
		// comparing - a plain string compare silently mismatches on Windows and makes the workspace
		// root look like a sub-package (no per-package rules created).
		cwdInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(cwd))
		rootInternal := pathutil.StandardiseDirPathInternal(pathutil.NormalizePathForInternal(monorepoCtx.WorkspaceRoot))
		if cwdInternal != rootInternal {
			packageRule := config.Rule{
				Path: ".",
				CircularImportsDetections: []*config.CircularImportsOptions{{
					Enabled:           true,
					IgnoreTypeImports: false,
				}},
				OrphanFilesDetections: []*config.OrphanFilesOptions{{
					Enabled: false,
				}},
				UnusedNodeModulesDetections: []*config.UnusedNodeModulesOptions{{
					Enabled: false,
				}},
				MissingNodeModulesDetections: []*config.MissingNodeModulesOptions{{
					Enabled: false,
				}},
				UnusedExportsDetections: []*config.UnusedExportsOptions{{
					Enabled: false,
				}},
				UnresolvedImportsDetections: []*config.UnresolvedImportsOptions{{
					Enabled: true,
				}},
				DevDepsUsageOnProdDetections: []*config.RestrictedDevDependenciesUsageOptions{{
					Enabled: false,
				}},
				RestrictedImportsDetections: []*config.RestrictedImportsDetectionOptions{{
					Enabled: false,
				}},
			}
			rules = append(rules, packageRule)
			createdForMonorepoSubPackage = true
		} else {
			// Monorepo root: Root rule only has module boundaries
			rootRule := config.Rule{
				Path: ".",
				ModuleBoundaries: []config.BoundaryRule{
					{
						Name:    "packages",
						Pattern: "packages/**/*",
						Allow:   []string{"packages/**/*"},
					},
				},
				OrphanFilesDetections: []*config.OrphanFilesOptions{{
					Enabled: false,
				}},
				UnusedExportsDetections: []*config.UnusedExportsOptions{{
					Enabled: false,
				}},
			}
			rules = append(rules, rootRule)

			// Find workspace packages
			excludePatterns := globutil.CreateGlobMatchers([]string{}, cwd)
			monorepoCtx.FindWorkspacePackages(excludePatterns, nil)

			// Collect and sort package paths
			var packagePaths []string
			for _, packagePath := range monorepoCtx.PackageToPath {
				// Convert absolute path to relative path from cwd
				relPath, err := filepath.Rel(cwd, packagePath)
				if err != nil {
					continue // Skip if we can't get relative path
				}
				relPath = filepath.ToSlash(relPath)

				// Skip root package (already covered)
				if relPath == "." || relPath == "" {
					continue
				}

				packagePaths = append(packagePaths, relPath)
			}

			// Sort package paths alphabetically
			slices.Sort(packagePaths)

			// Create a rule for each discovered package in sorted order
			for _, relPath := range packagePaths {
				packageRule := config.Rule{
					Path: relPath,
					CircularImportsDetections: []*config.CircularImportsOptions{{
						Enabled:           true,
						IgnoreTypeImports: false,
					}},
					OrphanFilesDetections: []*config.OrphanFilesOptions{{
						Enabled: false,
					}},
					UnusedNodeModulesDetections: []*config.UnusedNodeModulesOptions{{
						Enabled: false,
					}},
					MissingNodeModulesDetections: []*config.MissingNodeModulesOptions{{
						Enabled: false,
					}},
					UnusedExportsDetections: []*config.UnusedExportsOptions{{
						Enabled: false,
					}},
					UnresolvedImportsDetections: []*config.UnresolvedImportsOptions{{
						Enabled: true,
					}},
					DevDepsUsageOnProdDetections: []*config.RestrictedDevDependenciesUsageOptions{{
						Enabled: false,
					}},
					RestrictedImportsDetections: []*config.RestrictedImportsDetectionOptions{{
						Enabled: false,
					}},
				}
				rules = append(rules, packageRule)
			}
		}
	} else {
		// Non-monorepo: Single rule with all checks including module boundaries
		rootRule := config.Rule{
			Path: ".",
			ModuleBoundaries: []config.BoundaryRule{
				{
					Name:    "src",
					Pattern: "src/**/*",
					Allow:   []string{"src/**/*"},
				},
			},
			CircularImportsDetections: []*config.CircularImportsOptions{{
				Enabled:           true,
				IgnoreTypeImports: false,
			}},
			OrphanFilesDetections: []*config.OrphanFilesOptions{{
				Enabled: false,
			}},
			UnusedNodeModulesDetections: []*config.UnusedNodeModulesOptions{{
				Enabled: false,
			}},
			MissingNodeModulesDetections: []*config.MissingNodeModulesOptions{{
				Enabled: false,
			}},
			UnusedExportsDetections: []*config.UnusedExportsOptions{{
				Enabled: false,
			}},
			UnresolvedImportsDetections: []*config.UnresolvedImportsOptions{{
				Enabled: true,
			}},
			DevDepsUsageOnProdDetections: []*config.RestrictedDevDependenciesUsageOptions{{
				Enabled: false,
			}},
			RestrictedImportsDetections: []*config.RestrictedImportsDetectionOptions{{
				Enabled: false,
			}},
		}
		rules = append(rules, rootRule)
	}

	// Create config structure
	config := config.RevDepConfig{
		ConfigVersion: currentConfigVersion,
		Rules:         rules,
		Schema:        "https://github.com/jayu/rev-dep/blob/master/config-schema/" + currentConfigVersion + ".schema.json?raw=true",
		NodeModulesResolution: &config.NodeModulesResolutionConfig{
			ResolutionType:         config.NodeModulesResolutionEntryPackage,
			IncludeDevDepsFromRoot: false,
		},
	}

	// Add schema reference if schema file exists
	schemaPath := filepath.Join(cwd, "config-schema", "1.0.schema.json")
	if _, err := os.Stat(schemaPath); err == nil {
		// We'll add the schema field during JSON marshaling
	}

	// Marshal config to JSON with proper formatting
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, configJSON, 0644); err != nil {
		return "", nil, false, fmt.Errorf("failed to write config file: %v", err)
	}

	return configPath, rules, createdForMonorepoSubPackage, nil
}

// printInitConfigResults prints the results of config initialization
func printInitConfigResults(configPath string, rules []config.Rule, createdForMonorepoSubPackage bool) {
	fmt.Printf("✅ Created .rev-dep.config.jsonc at %s\n", configPath)
	if createdForMonorepoSubPackage {
		fmt.Printf("⚠️  Created config for monorepo sub-package. This file targets the current package only.\n")
	}
	if len(rules) > 1 {
		fmt.Printf("📦 Discovered %d monorepo packages and created rules for each\n", len(rules)-1)
	} else {
		fmt.Printf("📁 Created single rule for root directory\n")
	}

	fmt.Println("Adjust rules to make them relevant to your project setup.\nGenerated module boundaries config is exemplary and does not make much sense.")
	fmt.Println("Hint: feed LLM with config file JSON schema to get started.")
}

// initConfigFile initializes a new rev-dep.config.json file with minimal structure
func initConfigFile(cwd string) error {
	configPath, rules, createdForMonorepoSubPackage, err := initConfigFileCore(cwd)
	if err != nil {
		return err
	}
	printInitConfigResults(configPath, rules, createdForMonorepoSubPackage)
	return nil
}
