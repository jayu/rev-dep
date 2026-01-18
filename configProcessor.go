package main

import (
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// RuleResult contains the results for a single rule in the config
type RuleResult struct {
	RulePath                 string
	FileCount                int
	EnabledChecks            []string
	DependencyTree           MinimalDependencyTree
	ModuleBoundaryViolations []ModuleBoundaryViolation
	CircularDependencies     [][]string
	OrphanFiles              []string
	UnusedNodeModules        []string
	MissingNodeModules       []MissingNodeModuleResult
}

// ConfigProcessingResult contains the results for processing an entire config
type ConfigProcessingResult struct {
	RuleResults []RuleResult
	HasFailures bool
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

// buildDependencyTreeForConfig builds dependency tree for config processing
func buildDependencyTreeForConfig(
	allFiles []string,
	excludePatterns []GlobMatcher,
	conditionNames []string,
	cwd string,
	packageJson string,
	tsconfigJson string,
) (MinimalDependencyTree, *ResolverManager, error) {
	// For config processing, we always resolve type imports (we filter later per-check)
	ignoreTypeImports := false

	// We always follow monorepo packages for comprehensive analysis
	followMonorepoPackages := true

	// Skip resolving missing files for performance
	skipResolveMissing := false

	// Parse imports from all files
	fileImportsArr, _ := ParseImportsFromFiles(allFiles, ignoreTypeImports)

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
	)

	// Transform to minimal dependency tree
	minimalTree := TransformToMinimalDependencyTreeCustomParser(fileImportsArr)

	return minimalTree, resolverManager, nil
}

func filterFilesForRule(
	fullTree MinimalDependencyTree,
	rulePath string,
	cwd string,
	followMonorepoPackages bool,
) ([]string, MinimalDependencyTree) {
	normalizedRulePath := normalizeRulePath(filepath.Join(cwd, rulePath))

	filesWithinCwd := []string{}
	subTree := MinimalDependencyTree{}

	for file := range fullTree {
		if strings.HasPrefix(file, normalizedRulePath) {
			filesWithinCwd = append(filesWithinCwd, file)
		}
	}

	if !followMonorepoPackages {
		for _, file := range filesWithinCwd {
			subTree[file] = fullTree[file]
		}

		return filesWithinCwd, subTree
	}

	// Build graph to trace dependencies from other packages
	graph := buildDepsGraphForMultiple(fullTree, filesWithinCwd, nil, false)

	filteredFiles := make([]string, 0, len(graph.Vertices))

	for vertex := range graph.Vertices {
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
	packageJson string,
	tsconfigJson string,
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
				fullTree, // Use full tree for circular dependency detection
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
) (*ConfigProcessingResult, error) {
	// Step 1: Discover all files
	allFiles, excludePatterns, err := discoverAllFilesForConfig(cwd, config.IgnoreFiles)
	if err != nil {
		return nil, err
	}

	// Step 2: Build dependency tree for config
	fullTree, resolverManager, err := buildDependencyTreeForConfig(
		allFiles,
		excludePatterns,
		config.ConditionNames,
		cwd,
		packageJson,
		tsconfigJson,
	)
	if err != nil {
		return nil, err
	}

	// Step 3: Process each rule in parallel
	result := &ConfigProcessingResult{
		RuleResults: make([]RuleResult, len(config.Rules)),
		HasFailures: false,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, rule := range config.Rules {
		wg.Add(1)
		go func(ruleIndex int, currentRule Rule) {
			defer wg.Done()

			// Step 3a: Filter files for this rule
			ruleFiles, ruleTree := filterFilesForRule(fullTree, currentRule.Path, cwd, true)

			// Step 3b: Execute enabled checks in parallel
			ruleResult := processRuleChecks(
				currentRule,
				ruleFiles,
				ruleTree,
				fullTree,
				resolverManager,
				cwd,
				packageJson,
				tsconfigJson,
			)

			// Check for failures and update result
			hasFailures := len(ruleResult.CircularDependencies) > 0 ||
				len(ruleResult.OrphanFiles) > 0 ||
				len(ruleResult.ModuleBoundaryViolations) > 0 ||
				len(ruleResult.UnusedNodeModules) > 0 ||
				len(ruleResult.MissingNodeModules) > 0

			mu.Lock()
			result.RuleResults[ruleIndex] = ruleResult
			if hasFailures {
				result.HasFailures = true
			}
			mu.Unlock()
		}(i, rule)
	}

	wg.Wait()
	return result, nil
}
