package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// validateRulePathPackageJson checks if package.json exists in the rule path directory
// and returns true if package.json is missing
func validateRulePathPackageJson(rulePath, cwd string) bool {
	// Construct the full path to the rule directory
	fullRulePath := filepath.Join(cwd, rulePath)

	// Check if the directory exists
	if _, err := os.Stat(fullRulePath); os.IsNotExist(err) {
		// Directory doesn't exist, no need to check for package.json
		return false
	}

	// Check for package.json file in the rule directory
	packageJsonPath := filepath.Join(fullRulePath, "package.json")
	if _, err := os.Stat(packageJsonPath); os.IsNotExist(err) {
		// package.json doesn't exist
		return true
	}

	return false
}

// RuleResult contains the results for a single rule in the config
type RuleResult struct {
	RulePath                                        string
	FileCount                                       int
	EnabledChecks                                   []string
	DependencyTree                                  MinimalDependencyTree
	ModuleBoundaryViolations                        []ModuleBoundaryViolation
	CircularDependencies                            [][]string
	OrphanFiles                                     []string
	MissingNodeModules                              []MissingNodeModuleResult
	MissingNodeModulesOutputType                    string
	UnusedNodeModules                               []string
	UnusedNodeModulesOutputType                     string
	ImportConventionViolations                      []ImportConventionViolation
	UnusedExports                                   []UnusedExport
	UnresolvedImports                               []UnresolvedImport
	MissingPackageJson                              bool
	ShouldWarnAboutImportConventionWithPJsonImports bool
}

// ConfigProcessingResult contains the results for processing an entire config
type ConfigProcessingResult struct {
	RuleResults            []RuleResult
	HasFailures            bool
	FixedFilesCount        int
	FixedImportsCount      int
	DeletedFilesCount      int
	UnfixableAliasingCount int
	FixableIssuesCount     int
}

// discoverAllFilesForConfig discovers all files for config processing
func discoverAllFilesForConfig(
	cwd string,
	ignoreFiles []string,
) ([]string, []GlobMatcher, error) {
	// Create glob matchers for ignore files
	ignoreMatchers := CreateGlobMatchers(ignoreFiles, cwd)

	// Always include gitignore patterns
	gitignoreMatchers := FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd)
	combinedMatchers := append(ignoreMatchers, gitignoreMatchers...)

	// Get all files using the existing GetFiles function
	files := GetFiles(cwd, []string{}, combinedMatchers)

	return files, combinedMatchers, nil
}

func anyRuleChecksForUnusedExports(config *RevDepConfig) bool {
	for _, rule := range config.Rules {
		if rule.UnusedExportsDetection != nil && rule.UnusedExportsDetection.Enabled {
			return true
		}
	}
	return false
}

// buildDependencyTreeForConfig builds dependency tree for config processing
func buildDependencyTreeForConfig(
	allFiles []string,
	excludePatterns []GlobMatcher,
	conditionNames []string,
	cwd string,
	packageJson string,
	tsconfigJson string,
	parseMode ParseMode,
) (MinimalDependencyTree, *ResolverManager, error) {
	// For config processing, we always resolve type imports (we filter later per-check)
	ignoreTypeImports := false

	// We always follow monorepo packages for comprehensive analysis
	followMonorepoPackages := FollowMonorepoPackagesValue{FollowAll: true}

	// Skip resolving missing files for performance
	skipResolveMissing := false

	// Parse imports from all files
	fileImportsArr, _ := ParseImportsFromFiles(allFiles, ignoreTypeImports, parseMode)

	slices.Sort(allFiles)

	// Resolve imports using the existing resolver
	fileImportsArr, _, resolverManager := ResolveImports(
		fileImportsArr,
		allFiles,
		cwd,
		ignoreTypeImports,
		skipResolveMissing,
		packageJson,
		tsconfigJson,
		excludePatterns,
		conditionNames,
		followMonorepoPackages,
		parseMode,
		NodeModulesMatchingStrategySelfResolver,
	)

	// Transform to minimal dependency tree
	minimalTree := TransformToMinimalDependencyTreeCustomParser(fileImportsArr)

	return minimalTree, resolverManager, nil
}

func filterFilesForRule(
	fullTree MinimalDependencyTree,
	rulePath string,
	cwd string,
	followMonorepoPackages FollowMonorepoPackagesValue,
	resolverManager *ResolverManager,
) ([]string, MinimalDependencyTree) {
	normalizedRulePath := NormalizePathForInternal(filepath.Clean(JoinWithCwd(cwd, rulePath)))
	normalizedRulePathWithSlash := StandardiseDirPathInternal(normalizedRulePath)
	isRuleFile := func(filePath string) bool {
		normalizedFilePath := NormalizePathForInternal(filePath)
		return strings.HasPrefix(normalizedFilePath, normalizedRulePathWithSlash)
	}

	filesWithinCwd := []string{}
	subTree := MinimalDependencyTree{}

	for file := range fullTree {
		if isRuleFile(file) {
			filesWithinCwd = append(filesWithinCwd, file)
		}
	}

	if !followMonorepoPackages.IsEnabled() {
		for _, file := range filesWithinCwd {
			subTree[file] = fullTree[file]
		}

		return filesWithinCwd, subTree
	}

	// Build graph to trace dependencies from other packages
	graph := buildDepsGraphForMultiple(fullTree, filesWithinCwd, nil, false)

	allowedPackagePathPrefixes := map[string]bool{}
	if !followMonorepoPackages.ShouldFollowAll() && resolverManager != nil && resolverManager.monorepoContext != nil {
		for packageName, packagePath := range resolverManager.monorepoContext.PackageToPath {
			if !followMonorepoPackages.ShouldFollowPackage(packageName) {
				continue
			}

			normalizedPackagePath := NormalizePathForInternal(packagePath)
			allowedPackagePathPrefixes[StandardiseDirPathInternal(normalizedPackagePath)] = true
		}
	}

	filteredFiles := make([]string, 0, len(graph.Vertices))

	for vertex := range graph.Vertices {
		if !followMonorepoPackages.ShouldFollowAll() && !isRuleFile(vertex) {
			isInAllowedWorkspacePackage := false
			for packagePathPrefix := range allowedPackagePathPrefixes {
				if strings.HasPrefix(vertex, packagePathPrefix) {
					isInAllowedWorkspacePackage = true
					break
				}
			}
			if !isInAllowedWorkspacePackage {
				continue
			}
		}
		filteredFiles = append(filteredFiles, vertex)
		subTree[vertex] = fullTree[vertex]
	}

	return filteredFiles, subTree
}

// processRuleChecks runs all enabled checks for a rule in parallel
func processRuleChecks(
	rule Rule,
	ruleFiles []string,
	ruleTree MinimalDependencyTree,
	fullTree MinimalDependencyTree,
	resolverManager *ResolverManager,
	cwd string,
	fix bool,
) RuleResult {
	// Track enabled checks
	enabledChecks := []string{}

	// Check which detections are enabled
	if rule.CircularImportsDetection != nil && rule.CircularImportsDetection.Enabled {
		enabledChecks = append(enabledChecks, "circular-imports")
	}
	if rule.OrphanFilesDetection != nil && rule.OrphanFilesDetection.Enabled {
		enabledChecks = append(enabledChecks, "orphan-files")
	}
	if len(rule.ModuleBoundaries) > 0 {
		enabledChecks = append(enabledChecks, "module-boundaries")
	}
	if rule.UnusedNodeModulesDetection != nil && rule.UnusedNodeModulesDetection.Enabled {
		enabledChecks = append(enabledChecks, "unused-node-modules")
	}
	if rule.MissingNodeModulesDetection != nil && rule.MissingNodeModulesDetection.Enabled {
		enabledChecks = append(enabledChecks, "missing-node-modules")
	}
	if rule.UnusedExportsDetection != nil && rule.UnusedExportsDetection.Enabled {
		enabledChecks = append(enabledChecks, "unused-exports")
	}
	if rule.UnresolvedImportsDetection != nil && rule.UnresolvedImportsDetection.Enabled {
		enabledChecks = append(enabledChecks, "unresolved-imports")
	}
	if len(rule.ImportConventions) > 0 {
		enabledChecks = append(enabledChecks, "import-conventions")
	}

	ruleResult := RuleResult{
		RulePath:       rule.Path,
		FileCount:      len(ruleFiles),
		EnabledChecks:  enabledChecks,
		DependencyTree: fullTree, // Include the full dependency tree for circular dependency formatting
	}

	fullRulePath := StandardiseDirPath(filepath.Join(cwd, rule.Path))

	rulePathResolver := resolverManager.GetResolverForFile(fullRulePath)
	rulePathNodeModules := make(map[string]bool, 0)

	if rulePathResolver != nil {
		rulePathNodeModules = rulePathResolver.nodeModules
	}

	// Detect module-suffix variants to exclude from orphan/unused-exports detection.
	// Uses per-file resolver lookup so monorepos with package-level moduleSuffixes work.
	moduleSuffixVariants := DetectModuleSuffixVariants(ruleFiles, resolverManager)

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Circular Dependencies
	if rule.CircularImportsDetection != nil && rule.CircularImportsDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// For circular dependencies, use the full tree since we need complete graph
			// Sort rule files as required by FindCircularDependencies
			sortedRuleFiles := make([]string, len(ruleFiles))
			copy(sortedRuleFiles, ruleFiles)
			slices.Sort(sortedRuleFiles)

			circularDeps := FindCircularDependencies(
				ruleTree,
				sortedRuleFiles,
				rule.CircularImportsDetection.IgnoreTypeImports,
			)

			mu.Lock()
			ruleResult.CircularDependencies = circularDeps
			mu.Unlock()
		}()
	}

	// Orphan Files
	if rule.OrphanFilesDetection != nil && rule.OrphanFilesDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orphanFiles := FindOrphanFiles(
				ruleTree,
				rule.OrphanFilesDetection.ValidEntryPoints,
				rule.OrphanFilesDetection.GraphExclude,
				rule.OrphanFilesDetection.IgnoreTypeImports,
				fullRulePath,
				moduleSuffixVariants,
			)

			mu.Lock()
			ruleResult.OrphanFiles = orphanFiles
			mu.Unlock()
		}()
	}

	// Module Boundaries
	if len(rule.ModuleBoundaries) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			violations := CheckModuleBoundariesFromTree(
				ruleTree,
				ruleFiles,
				rule.ModuleBoundaries,
				fullRulePath,
			)

			mu.Lock()
			ruleResult.ModuleBoundaryViolations = violations
			mu.Unlock()
		}()
	}

	// Unused Node Modules
	if rule.UnusedNodeModulesDetection != nil && rule.UnusedNodeModulesDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unusedModules := GetUnusedNodeModulesFromTree(
				ruleTree,
				rulePathNodeModules,
				// likely should be rulePath
				fullRulePath,
				rule.UnusedNodeModulesDetection.PkgJsonFieldsWithBinaries,
				rule.UnusedNodeModulesDetection.FilesWithBinaries,
				rule.UnusedNodeModulesDetection.FilesWithModules,
				"", // use empty path so it is discovered in fullRulePath
				"", // use empty path so it is discovered in fullRulePath
				rule.UnusedNodeModulesDetection.IncludeModules,
				rule.UnusedNodeModulesDetection.ExcludeModules,
			)

			mu.Lock()
			ruleResult.UnusedNodeModules = unusedModules
			if rule.UnusedNodeModulesDetection.OutputType != "" {
				ruleResult.UnusedNodeModulesOutputType = rule.UnusedNodeModulesDetection.OutputType
			}
			mu.Unlock()
		}()
	}

	// Missing Node Modules
	if rule.MissingNodeModulesDetection != nil && rule.MissingNodeModulesDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			missingModules := GetMissingNodeModulesFromTree(
				ruleTree,
				rule.MissingNodeModulesDetection.IncludeModules,
				rule.MissingNodeModulesDetection.ExcludeModules,
				rulePathNodeModules,
			)

			mu.Lock()
			ruleResult.MissingNodeModules = missingModules
			if rule.MissingNodeModulesDetection.OutputType != "" {
				ruleResult.MissingNodeModulesOutputType = rule.MissingNodeModulesDetection.OutputType
			}
			mu.Unlock()
		}()
	}

	// Import Conventions
	if len(rule.ImportConventions) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			violations, shouldWarnAboutImportConventionWithPJsonImports := CheckImportConventionsFromTree(
				ruleTree,
				ruleFiles,
				rule.ImportConventions,
				rulePathResolver,
				fullRulePath, // Use rule path instead of current working directory
				fix,
			)

			mu.Lock()
			ruleResult.ImportConventionViolations = violations
			ruleResult.ShouldWarnAboutImportConventionWithPJsonImports = shouldWarnAboutImportConventionWithPJsonImports
			mu.Unlock()
		}()
	}

	// Unused Exports
	if rule.UnusedExportsDetection != nil && rule.UnusedExportsDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unusedExports := FindUnusedExports(
				ruleFiles,
				ruleTree,
				rule.UnusedExportsDetection.ValidEntryPoints,
				rule.UnusedExportsDetection.GraphExclude,
				rule.UnusedExportsDetection.IgnoreTypeExports,
				rule.UnusedExportsDetection.Autofix,
				fullRulePath,
				moduleSuffixVariants,
			)

			mu.Lock()
			ruleResult.UnusedExports = unusedExports
			mu.Unlock()
		}()
	}

	// Unresolved Imports
	if rule.UnresolvedImportsDetection != nil && rule.UnresolvedImportsDetection.Enabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// NotResolvedModule might be actually a node module, but defined rule path package (eg apps/main-app) not in just in time package (pacakges/shared)
			// We are not able to detect that during module resolution for config file, becasue we resolve all modules without knowing which workspace contains app and which contains shared code
			// During rule evaluation we can assume that package.json in rule path is the one that contains node modules for app build from that rule path
			unresolved := DetectUnresolvedImports(ruleTree, rulePathNodeModules)
			unresolved = FilterUnresolvedImports(unresolved, rule.UnresolvedImportsDetection, fullRulePath)

			mu.Lock()
			ruleResult.UnresolvedImports = unresolved
			mu.Unlock()
		}()
	}

	wg.Wait()
	return ruleResult
}

// ProcessConfig processes a rev-dep configuration with parallel rule and check execution
func ProcessConfig(
	config *RevDepConfig,
	cwd string,
	packageJson string,
	tsconfigJson string,
	fix bool,
) (*ConfigProcessingResult, error) {
	// Step 1: Discover all files
	allFiles, excludePatterns, err := discoverAllFilesForConfig(cwd, config.IgnoreFiles)
	if err != nil {
		return nil, err
	}

	// Step 2: Build dependency tree for config
	parseMode := ParseModeBasic
	if anyRuleChecksForUnusedExports(config) {
		parseMode = ParseModeDetailed
	}

	fullTree, resolverManager, err := buildDependencyTreeForConfig(
		allFiles,
		excludePatterns,
		config.ConditionNames,
		cwd,
		packageJson,
		tsconfigJson,
		parseMode,
	)
	if err != nil {
		return nil, err
	}

	// Step 3: Process each rule in parallel
	result := &ConfigProcessingResult{
		RuleResults: make([]RuleResult, len(config.Rules)),
		HasFailures: false,
	}

	// Validate package.json exists for all rule paths before parallel processing
	missingPackageJsonResults := make([]bool, len(config.Rules))
	for i, rule := range config.Rules {
		missingPackageJsonResults[i] = validateRulePathPackageJson(rule.Path, cwd)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, rule := range config.Rules {
		wg.Add(1)
		go func(ruleIndex int, currentRule Rule) {
			defer wg.Done()

			// Step 3a: Filter files for this rule
			ruleFiles, ruleTree := filterFilesForRule(fullTree, currentRule.Path, cwd, currentRule.FollowMonorepoPackages, resolverManager)

			// Step 3b: Execute enabled checks in parallel
			ruleResult := processRuleChecks(
				currentRule,
				ruleFiles,
				ruleTree,
				fullTree,
				resolverManager,
				cwd,
				fix,
			)

			// Set the missing package.json flag
			ruleResult.MissingPackageJson = missingPackageJsonResults[ruleIndex]

			// Check for failures and update result
			hasFailures := len(ruleResult.CircularDependencies) > 0 ||
				len(ruleResult.OrphanFiles) > 0 ||
				len(ruleResult.ModuleBoundaryViolations) > 0 ||
				len(ruleResult.UnusedNodeModules) > 0 ||
				len(ruleResult.MissingNodeModules) > 0 ||
				len(ruleResult.ImportConventionViolations) > 0 ||
				len(ruleResult.UnusedExports) > 0 ||
				len(ruleResult.UnresolvedImports) > 0

			mu.Lock()
			result.RuleResults[ruleIndex] = ruleResult
			if hasFailures {
				result.HasFailures = true
			}
			mu.Unlock()
		}(i, rule)
	}

	wg.Wait()

	// Step 4: Apply fixes if requested
	if fix {
		changesByFile := make(map[string][]Change)

		for i, ruleResult := range result.RuleResults {
			ruleCfg := config.Rules[i]
			of := ruleCfg.OrphanFilesDetection
			isOrphanFixEnabled := of != nil && of.Enabled && of.Autofix

			// Create a set of orphan files to be deleted by this rule to avoid content fixes on them
			orphanFilesToDelete := make(map[string]bool)
			if isOrphanFixEnabled {
				for _, orphan := range ruleResult.OrphanFiles {
					orphanFilesToDelete[orphan] = true
				}
			}

			// Handle import convention and unused exports fixes as before
			for _, v := range ruleResult.ImportConventionViolations {
				if orphanFilesToDelete[v.FilePath] {
					continue
				}
				if v.Fix != nil {
					changesByFile[v.FilePath] = append(changesByFile[v.FilePath], *v.Fix)
					result.FixedImportsCount++
				} else if v.ViolationType == "should-be-aliased" {
					result.UnfixableAliasingCount++
				}
			}
			for _, v := range ruleResult.UnusedExports {
				if orphanFilesToDelete[v.FilePath] {
					continue
				}
				if v.Fix != nil {
					changesByFile[v.FilePath] = append(changesByFile[v.FilePath], *v.Fix)
					result.FixedImportsCount++
				}
			}

			// Handle orphan files autofix: delete files when configured
			if isOrphanFixEnabled {
				for _, orphan := range ruleResult.OrphanFiles {
					osPath := DenormalizePathForOS(orphan)
					if !filepath.IsAbs(osPath) {
						osPath = filepath.Join(cwd, osPath)
					}
					if err := os.Remove(osPath); err != nil {
						return result, fmt.Errorf("failed to remove orphan file '%s': %w", osPath, err)
					}
					result.DeletedFilesCount++
				}
			}
		}

		if len(changesByFile) > 0 {
			if err := ApplyFileChanges(changesByFile); err != nil {
				return result, fmt.Errorf("failed to apply autofixes: %w", err)
			}
			result.FixedFilesCount += len(changesByFile)
		}
	} else {
		fixableIssuesCount := 0
		for i, ruleResult := range result.RuleResults {
			for _, v := range ruleResult.ImportConventionViolations {
				if v.Fix != nil {
					fixableIssuesCount++
				}
			}
			for _, v := range ruleResult.UnusedExports {
				if v.Fix != nil {
					fixableIssuesCount++
				}
			}

			// Add orphan files to fixable count if autofix is enabled for this rule
			rule := config.Rules[i]
			if rule.OrphanFilesDetection != nil && rule.OrphanFilesDetection.Enabled && rule.OrphanFilesDetection.Autofix {
				fixableIssuesCount += len(ruleResult.OrphanFiles)
			}
		}
		result.FixableIssuesCount = fixableIssuesCount
	}

	return result, nil
}
