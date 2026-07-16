package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/graph"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/node"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/pathutil"
	"rev-dep-go/internal/resolve"
	"rev-dep-go/internal/sourceedit"
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
	DependencyTree                                  model.MinimalDependencyTree
	ModuleBoundaryViolations                        []checks.ModuleBoundaryViolation
	CircularDependencies                            [][]string
	OrphanFiles                                     []string
	OrphanFilesAutofixable                          []string
	MissingNodeModules                              []node.MissingNodeModuleResult
	MissingNodeModulesOutputType                    string
	UnusedNodeModules                               []node.UnusedNodeModuleIssue
	UnusedNodeModulesOutputType                     string
	ImportConventionViolations                      []checks.ImportConventionViolation
	UnusedExports                                   []checks.UnusedExport
	UnresolvedImports                               []checks.UnresolvedImport
	RestrictedDevDependenciesUsageViolations        []checks.RestrictedDevDependenciesUsageViolation
	RestrictedImportsViolations                     []checks.RestrictedImportViolation
	RestrictedImportersViolations                   []checks.RestrictedImporterViolation
	RestrictedDirectImportersViolations             []checks.RestrictedDirectImporterViolation
	RestrictedImportsFollowMonorepoPackages         model.FollowMonorepoPackagesValue
	ProcessIgnoredFiles                             []string
	MissingPackageJson                              bool
	ShouldWarnAboutImportConventionWithPJsonImports bool
	UnmatchedEntryPointPatterns                     UnmatchedEntryPointPatterns
}

// UnmatchedEntryPointPatterns captures, per entry-point bucket, the glob patterns
// that did not match any file in the rule's workspace. This metadata is collected
// and persisted for future use (e.g. surfacing warnings about stale patterns) but
// is not acted upon during processing.
type UnmatchedEntryPointPatterns struct {
	ProdEntryPoints   []string
	DevEntryPoints    []string
	IgnoreEntryPoints []string
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
	FullTree               model.MinimalDependencyTree
	// Discovery/resolver artifacts, exposed so a caller (e.g. `config run --lint-config`) can
	// lint without redoing the expensive discovery + dependency-tree build.
	DiscoveredFiles []string
	ResolverManager *resolve.ResolverManager
}

// discoverAllFilesForConfig discovers all files for config processing
func discoverAllFilesForConfig(
	cwd string,
	ignoreFiles []string,
	processIgnoredFiles []string,
) ([]string, []globutil.GlobMatcher, []globutil.GlobMatcher, error) {
	// Create glob matchers for ignore files
	ignoreMatchers := globutil.CreateGlobMatchers(ignoreFiles, cwd)
	processIgnoredMatchers := globutil.CreateGlobMatchers(processIgnoredFiles, cwd)

	// Always include gitignore patterns
	gitignoreMatchers := fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd)
	combinedMatchers := append(ignoreMatchers, gitignoreMatchers...)

	// Get all files using the existing GetFiles function
	files := fs.GetFiles(cwd, []string{}, combinedMatchers, processIgnoredMatchers)

	return files, combinedMatchers, processIgnoredMatchers, nil
}

func anyRuleChecksForUnusedExports(config *RevDepConfig) bool {
	for _, rule := range config.Rules {
		if anyEnabled(rule.getUnusedExportsDetections()) {
			return true
		}
	}
	return false
}

type enabledOption interface {
	IsEnabled() bool
}

func anyEnabled[T enabledOption](items []T) bool {
	for _, item := range items {
		if item.IsEnabled() {
			return true
		}
	}
	return false
}

// buildDependencyTreeForConfig builds dependency tree for config processing
func buildDependencyTreeForConfig(
	allFiles []string,
	excludePatterns []globutil.GlobMatcher,
	includePatterns []globutil.GlobMatcher,
	conditionNames []string,
	cwd string,
	packageJson string,
	tsconfigJson string,
	customAssetExtensions []string,
	parseMode model.ParseMode,
	explicitPackageDirs []string,
) (model.MinimalDependencyTree, *resolve.ResolverManager, error) {
	// For config processing, we always resolve type imports (we filter later per-check)
	ignoreTypeImports := false

	// We always follow monorepo packages for comprehensive analysis
	followMonorepoPackages := model.FollowMonorepoPackagesValue{FollowAll: true}

	// Skip resolving missing files for performance
	skipResolveMissing := false

	// Parse imports from all files
	fileImportsArr, _ := parser.ParseImportsFromFiles(allFiles, ignoreTypeImports, parseMode)

	slices.Sort(allFiles)

	// Resolve imports using the existing resolver
	fileImportsArr, _, resolverManager := resolve.ResolveImports(
		fileImportsArr,
		allFiles,
		cwd,
		ignoreTypeImports,
		skipResolveMissing,
		packageJson,
		tsconfigJson,
		excludePatterns,
		includePatterns,
		conditionNames,
		followMonorepoPackages,
		explicitPackageDirs,
		customAssetExtensions,
		parseMode,
		model.NodeModulesMatchingStrategySelfResolver,
	)

	// Transform to minimal dependency tree
	minimalTree := model.TransformToMinimalDependencyTreeCustomParser(fileImportsArr)

	return minimalTree, resolverManager, nil
}

func filterFilesForRule(
	fullTree model.MinimalDependencyTree,
	rulePath string,
	cwd string,
	followMonorepoPackages model.FollowMonorepoPackagesValue,
	resolverManager *resolve.ResolverManager,
) ([]string, model.MinimalDependencyTree) {
	normalizedRulePath := pathutil.NormalizePathForInternal(filepath.Clean(pathutil.JoinWithCwd(cwd, rulePath)))
	normalizedRulePathWithSlash := pathutil.StandardiseDirPathInternal(normalizedRulePath)
	isRuleFile := func(filePath string) bool {
		normalizedFilePath := pathutil.NormalizePathForInternal(filePath)
		return strings.HasPrefix(normalizedFilePath, normalizedRulePathWithSlash)
	}

	filesWithinCwd := []string{}
	subTree := model.MinimalDependencyTree{}

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

	graph := graph.BuildDepsGraphForMultiple(fullTree, filesWithinCwd, nil, false, false)

	allowedPackagePathPrefixes := map[string]bool{}
	if !followMonorepoPackages.ShouldFollowAll() && resolverManager != nil && resolverManager.MonorepoContext() != nil {
		for packageName, packagePath := range resolverManager.MonorepoContext().PackageToPath {
			if !followMonorepoPackages.ShouldFollowPackage(packageName) {
				continue
			}

			normalizedPackagePath := pathutil.NormalizePathForInternal(packagePath)
			allowedPackagePathPrefixes[pathutil.StandardiseDirPathInternal(normalizedPackagePath)] = true
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

func FilterFilesForRule(
	fullTree model.MinimalDependencyTree,
	rulePath string,
	cwd string,
	followMonorepoPackages model.FollowMonorepoPackagesValue,
	resolverManager *resolve.ResolverManager,
) ([]string, model.MinimalDependencyTree) {
	return filterFilesForRule(fullTree, rulePath, cwd, followMonorepoPackages, resolverManager)
}

// findUnmatchedEntryPointPatterns returns the subset of patterns that do not match
// any file in ruleFiles. Patterns are matched relative to cwd, consistent with how
// entry-point globs are matched by the reachability-based checks. Empty/whitespace
// patterns are skipped. Returns nil when every pattern matches at least one file.
func findUnmatchedEntryPointPatterns(patterns []string, ruleFiles []string, cwd string) []string {
	var unmatched []string
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		matchers := globutil.CreateGlobMatchers([]string{pattern}, cwd)
		matched := false
		for _, file := range ruleFiles {
			if globutil.MatchesAnyGlobMatcher(file, matchers, false) {
				matched = true
				break
			}
		}
		if !matched {
			unmatched = append(unmatched, pattern)
		}
	}
	return unmatched
}

// processRuleChecks runs all enabled checks for a rule in parallel
func processRuleChecks(
	rule Rule,
	ruleFiles []string,
	ruleTree model.MinimalDependencyTree,
	fullTree model.MinimalDependencyTree,
	resolverManager *resolve.ResolverManager,
	cwd string,
	fix bool,
	nearestPackage bool,
	includeDevDepsFromRoot bool,
) RuleResult {
	// Track enabled checks
	enabledChecks := []string{}

	// Check which detections are enabled
	if anyEnabled(rule.getCircularImportsDetections()) {
		enabledChecks = append(enabledChecks, "circular-imports")
	}
	if anyEnabled(rule.getOrphanFilesDetections()) {
		enabledChecks = append(enabledChecks, "orphan-files")
	}
	if len(rule.ModuleBoundaries) > 0 {
		enabledChecks = append(enabledChecks, "module-boundaries")
	}
	if anyEnabled(rule.getUnusedNodeModulesDetections()) {
		enabledChecks = append(enabledChecks, "unused-node-modules")
	}
	if anyEnabled(rule.getMissingNodeModulesDetections()) {
		enabledChecks = append(enabledChecks, "missing-node-modules")
	}
	if anyEnabled(rule.getUnusedExportsDetections()) {
		enabledChecks = append(enabledChecks, "unused-exports")
	}
	if anyEnabled(rule.getUnresolvedImportsDetections()) {
		enabledChecks = append(enabledChecks, "unresolved-imports")
	}
	if anyEnabled(rule.getDevDepsUsageOnProdDetections()) {
		enabledChecks = append(enabledChecks, "dev-deps-usage-on-prod")
	}
	if anyEnabled(rule.getRestrictedImportsDetections()) {
		enabledChecks = append(enabledChecks, "restricted-imports")
	}
	if anyEnabled(rule.getRestrictedImportersDetections()) {
		enabledChecks = append(enabledChecks, "restricted-importers")
	}
	if anyEnabled(rule.getRestrictedDirectImportersDetections()) {
		enabledChecks = append(enabledChecks, "restricted-direct-importers")
	}
	if len(rule.ImportConventions) > 0 {
		enabledChecks = append(enabledChecks, "import-conventions")
	}

	ruleResult := RuleResult{
		RulePath:                                rule.Path,
		FileCount:                               len(ruleFiles),
		EnabledChecks:                           enabledChecks,
		DependencyTree:                          fullTree, // Include the full dependency tree for circular dependency formatting
		RestrictedImportsFollowMonorepoPackages: rule.FollowMonorepoPackages,
	}

	fullRulePath := pathutil.StandardiseDirPath(filepath.Join(cwd, rule.Path))

	rulePathResolver := resolverManager.GetResolverForFile(fullRulePath)
	rulePathNodeModules := make(map[string]bool, 0)

	if rulePathResolver != nil {
		rulePathNodeModules = rulePathResolver.NodeModules()
	}

	// When includeDevDepsFromRoot is enabled, the monorepo root (cwd) package.json
	// devDependencies are treated as available to package code, so importing a dev dependency
	// declared only at the root is not reported as missing. These are passed to the missing
	// check as extra allowed modules in both resolution modes. They are intentionally NOT added
	// to the unused check's candidate set, since they belong to the root, not the package.
	var rootDevDependencies map[string]bool
	if includeDevDepsFromRoot {
		if rootResolver := resolverManager.RootResolver(); rootResolver != nil {
			rootDevDependencies = rootResolver.DevNodeModules()
		}
	}

	// In nearest-package mode, "unused" (variant A) only counts an entry dependency as used when
	// one of the entry package's OWN files imports it. Build that file set from the rule's files;
	// entryOwnedFiles stays nil in entry-package mode, which means "all files".
	var entryOwnedFiles map[string]bool
	if nearestPackage && rulePathResolver != nil {
		entryOwnedFiles = make(map[string]bool, len(ruleFiles))
		for _, filePath := range ruleFiles {
			if resolverManager.GetResolverForFile(filePath) == rulePathResolver {
				entryOwnedFiles[filePath] = true
			}
		}
	}

	// Detect module-suffix variants to exclude from orphan/unused-exports detection.
	// Uses per-file resolver lookup so monorepos with package-level moduleSuffixes work.
	moduleSuffixVariants := resolve.DetectModuleSuffixVariants(ruleFiles, resolverManager)

	// Capture entry-point glob patterns that do not match any file in the rule's
	// workspace. This is metadata only - it is persisted for future use and does not
	// affect the checks below. Computed here in the calling goroutine before the
	// per-check goroutines start, so no synchronization is required.
	ruleResult.UnmatchedEntryPointPatterns = UnmatchedEntryPointPatterns{
		ProdEntryPoints:   findUnmatchedEntryPointPatterns(rule.ProdEntryPoints, ruleFiles, fullRulePath),
		DevEntryPoints:    findUnmatchedEntryPointPatterns(rule.DevEntryPoints, ruleFiles, fullRulePath),
		IgnoreEntryPoints: findUnmatchedEntryPointPatterns(rule.IgnoreEntryPoints, ruleFiles, fullRulePath),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Circular Dependencies
	if anyEnabled(rule.getCircularImportsDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// For circular dependencies, use the full tree since we need complete graph
			// Sort rule files as required by FindCircularDependencies
			sortedRuleFiles := make([]string, len(ruleFiles))
			copy(sortedRuleFiles, ruleFiles)
			slices.Sort(sortedRuleFiles)

			circularDeps := make([][]string, 0)
			for _, detection := range rule.getCircularImportsDetections() {
				if !detection.Enabled {
					continue
				}
				algo := strings.ToLower(strings.TrimSpace(detection.Algorithm))
				if algo == "" {
					algo = "dfs"
				}
				switch algo {
				case "scc":
					circularDeps = append(circularDeps, checks.FindCircularDependenciesSCC(
						ruleTree,
						sortedRuleFiles,
						detection.IgnoreTypeImports,
					)...)
				default:
					circularDeps = append(circularDeps, checks.FindCircularDependencies(
						ruleTree,
						sortedRuleFiles,
						detection.IgnoreTypeImports,
					)...)
				}
			}

			mu.Lock()
			ruleResult.CircularDependencies = circularDeps
			mu.Unlock()
		}()
	}

	// Orphan Files
	if anyEnabled(rule.getOrphanFilesDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orphanSet := map[string]bool{}
			orphanAutofixSet := map[string]bool{}
			orphanFiles := make([]string, 0)
			orphanFilesAutofixable := make([]string, 0)
			for _, detection := range rule.getOrphanFilesDetections() {
				if !detection.Enabled {
					continue
				}
				found := checks.FindOrphanFiles(
					ruleTree,
					detection.ValidEntryPoints,
					detection.GraphExclude,
					detection.IgnoreTypeImports,
					fullRulePath,
					moduleSuffixVariants,
				)
				for _, file := range found {
					if !orphanSet[file] {
						orphanSet[file] = true
						orphanFiles = append(orphanFiles, file)
					}
					if detection.Autofix && !orphanAutofixSet[file] {
						orphanAutofixSet[file] = true
						orphanFilesAutofixable = append(orphanFilesAutofixable, file)
					}
				}
			}
			slices.Sort(orphanFiles)
			slices.Sort(orphanFilesAutofixable)

			mu.Lock()
			ruleResult.OrphanFiles = orphanFiles
			ruleResult.OrphanFilesAutofixable = orphanFilesAutofixable
			mu.Unlock()
		}()
	}

	// Module Boundaries
	if len(rule.ModuleBoundaries) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			violations := checks.CheckModuleBoundariesFromTree(
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
	if anyEnabled(rule.getUnusedNodeModulesDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unusedSet := map[string]bool{}
			unusedModules := make([]node.UnusedNodeModuleIssue, 0)
			outputType := ""

			for _, detection := range rule.getUnusedNodeModulesDetections() {
				if !detection.Enabled {
					continue
				}
				found := node.GetUnusedNodeModulesFromTree(
					ruleTree,
					rulePathNodeModules,
					fullRulePath,
					detection.PkgJsonFieldsWithBinaries,
					detection.FilesWithBinaries,
					detection.FilesWithModules,
					"", // use empty path so it is discovered in fullRulePath
					"", // use empty path so it is discovered in fullRulePath
					detection.IncludeModules,
					detection.ExcludeModules,
					entryOwnedFiles,
				)
				for _, moduleName := range found {
					if !unusedSet[moduleName] {
						unusedSet[moduleName] = true
						unusedModules = append(unusedModules, node.UnusedNodeModuleIssue{
							ModuleName:      moduleName,
							PackageJsonPath: rulePathResolver.PackageJSONPath(),
						})
					}
				}
				if outputType == "" && detection.OutputType != "" {
					outputType = detection.OutputType
				}
			}
			slices.SortFunc(unusedModules, func(a, b node.UnusedNodeModuleIssue) int {
				return strings.Compare(a.ModuleName, b.ModuleName)
			})

			mu.Lock()
			ruleResult.UnusedNodeModules = unusedModules
			if outputType != "" {
				ruleResult.UnusedNodeModulesOutputType = outputType
			}
			mu.Unlock()
		}()
	}

	// Missing Node Modules
	if anyEnabled(rule.getMissingNodeModulesDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			missingModules := make([]node.MissingNodeModuleResult, 0)
			outputType := ""
			for _, detection := range rule.getMissingNodeModulesDetections() {
				if !detection.Enabled {
					continue
				}

				missingModules = append(missingModules, node.GetMissingNodeModulesFromTree(
					ruleTree,
					detection.IncludeModules,
					detection.ExcludeModules,
					rulePathNodeModules,
					rootDevDependencies,
					nearestPackage,
				)...)

				if outputType == "" && detection.OutputType != "" {
					outputType = detection.OutputType
				}
			}

			mu.Lock()
			ruleResult.MissingNodeModules = missingModules
			if outputType != "" {
				ruleResult.MissingNodeModulesOutputType = outputType
			}
			mu.Unlock()
		}()
	}

	// Import Conventions
	if len(rule.ImportConventions) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			violations, shouldWarnAboutImportConventionWithPJsonImports := checks.CheckImportConventionsFromTree(
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
	if anyEnabled(rule.getUnusedExportsDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unusedExports := make([]checks.UnusedExport, 0)
			for _, detection := range rule.getUnusedExportsDetections() {
				if !detection.Enabled {
					continue
				}
				found := checks.FindUnusedExports(
					ruleFiles,
					ruleTree,
					detection.ValidEntryPoints,
					detection.GraphExclude,
					detection.IgnoreTypeExports,
					detection.Autofix,
					fullRulePath,
					moduleSuffixVariants,
				)
				filterOpts := &checks.UnusedExportsFilterOptions{
					Ignore:        detection.Ignore,
					IgnoreFiles:   detection.IgnoreFiles,
					IgnoreExports: detection.IgnoreExports,
				}
				found = checks.FilterUnusedExports(found, filterOpts, fullRulePath)
				unusedExports = append(unusedExports, found...)
			}

			mu.Lock()
			ruleResult.UnusedExports = unusedExports
			mu.Unlock()
		}()
	}

	// Unresolved Imports
	if anyEnabled(rule.getUnresolvedImportsDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// entry-package mode: a NotResolvedModule might actually be a node module declared in the
			// rule path package (e.g. apps/main-app) but not in the just-in-time package (packages/shared).
			// We cannot detect that during module resolution for the config file, because we resolve all
			// modules without knowing which workspace contains the app and which contains shared code.
			// During rule evaluation we assume the package.json in the rule path is the one that contains
			// node modules for the app built from that rule path, so we suppress those from unresolved.
			//
			// nearest-package mode: each import was resolved against the package.json that owns its file,
			// so any NotResolvedModule is genuinely unresolved. Pass an empty set so nothing is suppressed.
			ignoredNodeModules := rulePathNodeModules
			if nearestPackage {
				ignoredNodeModules = map[string]bool{}
			}
			// includeDevDepsFromRoot: the monorepo root devDependencies are treated as available to
			// package code, so a root-declared dependency is not reported as unresolved (mirrors the
			// missing check). Applied in both modes via a fresh copy so the resolver's own
			// rulePathNodeModules map is never mutated.
			if len(rootDevDependencies) > 0 {
				merged := make(map[string]bool, len(ignoredNodeModules)+len(rootDevDependencies))
				for moduleName := range ignoredNodeModules {
					merged[moduleName] = true
				}
				for moduleName := range rootDevDependencies {
					merged[moduleName] = true
				}
				ignoredNodeModules = merged
			}
			unresolved := make([]checks.UnresolvedImport, 0)
			for _, detection := range rule.getUnresolvedImportsDetections() {
				if !detection.Enabled {
					continue
				}
				found := checks.DetectUnresolvedImports(ruleTree, ignoredNodeModules)
				filterOpts := &checks.UnresolvedFilterOptions{
					Ignore:        detection.Ignore,
					IgnoreFiles:   detection.IgnoreFiles,
					IgnoreImports: detection.IgnoreImports,
				}
				found = checks.FilterUnresolvedImports(found, filterOpts, fullRulePath)
				unresolved = append(unresolved, found...)
			}

			mu.Lock()
			ruleResult.UnresolvedImports = unresolved
			mu.Unlock()
		}()
	}

	// Restricted Dev Dependencies Usage
	if anyEnabled(rule.getDevDepsUsageOnProdDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// The dev dependency set checked against a production import follows nodeModulesResolution,
			// so a dev dependency leaking into production is reported regardless of where it is
			// declared: entry-package uses the entry package's devDependencies for every file;
			// nearest-package uses each file's own nearest package devDependencies; and
			// includeDevDepsFromRoot adds the monorepo root devDependencies on top in either mode.
			// The applicable dev-dependency set is constant within a workspace, so it is computed
			// once per workspace up front rather than recomputed for every file. mergeWithRoot folds
			// in the monorepo root devDependencies (includeDevDepsFromRoot) and returns the base
			// untouched when there are none.
			mergeWithRoot := func(base map[string]bool) map[string]bool {
				if len(rootDevDependencies) == 0 {
					return base
				}
				merged := make(map[string]bool, len(base)+len(rootDevDependencies))
				for moduleName := range base {
					merged[moduleName] = true
				}
				for moduleName := range rootDevDependencies {
					merged[moduleName] = true
				}
				return merged
			}

			var devDepsForFile func(filePath string) map[string]bool
			if nearestPackage {
				// nearest-package: each file is checked against its own nearest package's
				// devDependencies. Precompute one merged set per resolver root (keyed by the
				// resolver root path) and attribute each file to a root via the canonical
				// prefix-matching resolver lookup.
				devDepsByResolverRoot := map[string]map[string]bool{}
				registerResolver := func(resolver *resolve.ModuleResolver) {
					if resolver == nil {
						return
					}
					root := resolver.ResolverRoot()
					if _, exists := devDepsByResolverRoot[root]; exists {
						return
					}
					devDepsByResolverRoot[root] = mergeWithRoot(resolver.DevNodeModules())
				}
				registerResolver(resolverManager.RootResolver())
				registerResolver(resolverManager.CwdResolver())
				for _, subPkg := range resolverManager.SubpackageResolvers() {
					registerResolver(subPkg.Resolver)
				}

				devDepsForFile = func(filePath string) map[string]bool {
					fileResolver := resolverManager.GetResolverForFile(filePath)
					if fileResolver == nil {
						return nil
					}
					return devDepsByResolverRoot[fileResolver.ResolverRoot()]
				}
			} else {
				// entry-package: every file in the rule is checked against the entry package's
				// devDependencies, so the set is identical for all files and built once.
				var entryDevDependencies map[string]bool
				if rulePathResolver != nil {
					entryDevDependencies = rulePathResolver.DevNodeModules()
				}
				entryMerged := mergeWithRoot(entryDevDependencies)
				devDepsForFile = func(filePath string) map[string]bool {
					return entryMerged
				}
			}

			violations := make([]checks.RestrictedDevDependenciesUsageViolation, 0)
			for _, detection := range rule.getDevDepsUsageOnProdDetections() {
				if !detection.Enabled {
					continue
				}
				violations = append(violations, checks.FindDevDependenciesInProduction(
					ruleTree,
					detection.ProdEntryPoints,
					detection.IgnoreTypeImports,
					fullRulePath,
					devDepsForFile,
				)...)
			}

			mu.Lock()
			ruleResult.RestrictedDevDependenciesUsageViolations = violations
			mu.Unlock()
		}()
	}

	// Restricted Imports
	if anyEnabled(rule.getRestrictedImportsDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			violations := make([]checks.RestrictedImportViolation, 0)
			for _, detection := range rule.getRestrictedImportsDetections() {
				if !detection.Enabled {
					continue
				}
				violations = append(violations, checks.FindRestrictedImports(
					ruleTree,
					detection,
					fullRulePath,
				)...)
			}

			mu.Lock()
			ruleResult.RestrictedImportsViolations = violations
			mu.Unlock()
		}()
	}

	if anyEnabled(rule.getRestrictedImportersDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Universe of entry points for the allowlist policy: the rule's prod + dev entry points.
			ruleEntryPoints := make([]string, 0, len(rule.ProdEntryPoints)+len(rule.DevEntryPoints))
			ruleEntryPoints = append(ruleEntryPoints, rule.ProdEntryPoints...)
			ruleEntryPoints = append(ruleEntryPoints, rule.DevEntryPoints...)

			violations := make([]checks.RestrictedImporterViolation, 0)
			for _, detection := range rule.getRestrictedImportersDetections() {
				if !detection.Enabled {
					continue
				}
				violations = append(violations, checks.FindRestrictedImporters(
					ruleTree,
					detection,
					fullRulePath,
					ruleEntryPoints,
				)...)
			}

			mu.Lock()
			ruleResult.RestrictedImportersViolations = violations
			mu.Unlock()
		}()
	}

	if anyEnabled(rule.getRestrictedDirectImportersDetections()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			violations := make([]checks.RestrictedDirectImporterViolation, 0)
			for _, detection := range rule.getRestrictedDirectImportersDetections() {
				if !detection.Enabled {
					continue
				}
				violations = append(violations, checks.FindRestrictedDirectImporters(
					ruleTree,
					detection,
					fullRulePath,
				)...)
			}

			mu.Lock()
			ruleResult.RestrictedDirectImportersViolations = violations
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
	forceDetailed bool,
) (*ConfigProcessingResult, error) {
	// Step 1: Discover all files
	allFiles, excludePatterns, includePatterns, err := discoverAllFilesForConfig(cwd, config.IgnoreFiles, config.ProcessIgnoredFiles)
	if err != nil {
		return nil, err
	}

	// Step 2: Build dependency tree for config
	parseMode := model.ParseModeBasic
	if forceDetailed || anyRuleChecksForUnusedExports(config) {
		parseMode = model.ParseModeDetailed
	}

	// Resolve the config's rule paths (always relative to cwd) to absolute, internal-form
	// package directories. This lets the resolver register each rule directory that has its
	// own package.json as a workspace package even when there is no workspace-aware root
	// package.json, so per-package node_modules dependencies resolve instead of being
	// reported as unresolved.
	rulePackageDirs := make([]string, 0, len(config.Rules))
	for _, rule := range config.Rules {
		if rule.Path == "" {
			continue
		}
		rulePackageDirs = append(rulePackageDirs, pathutil.NormalizePathForInternal(filepath.Clean(pathutil.JoinWithCwd(cwd, rule.Path))))
	}

	fullTree, resolverManager, err := buildDependencyTreeForConfig(
		allFiles,
		excludePatterns,
		includePatterns,
		config.ConditionNames,
		cwd,
		packageJson,
		tsconfigJson,
		config.CustomAssetExtensions,
		parseMode,
		rulePackageDirs,
	)
	if err != nil {
		return nil, err
	}

	// Step 3: Process each rule in parallel
	result := &ConfigProcessingResult{
		RuleResults:     make([]RuleResult, len(config.Rules)),
		HasFailures:     false,
		FullTree:        fullTree,
		DiscoveredFiles: allFiles,
		ResolverManager: resolverManager,
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
				config.UsesNearestPackage(),
				config.IncludeDevDepsFromRoot(),
			)
			ruleResult.ProcessIgnoredFiles = config.ProcessIgnoredFiles

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
				len(ruleResult.UnresolvedImports) > 0 ||
				len(ruleResult.RestrictedDevDependenciesUsageViolations) > 0 ||
				len(ruleResult.RestrictedImportsViolations) > 0 ||
				len(ruleResult.RestrictedImportersViolations) > 0 ||
				len(ruleResult.RestrictedDirectImportersViolations) > 0

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
		changesByFile := make(map[string][]sourceedit.Change)

		for i, ruleResult := range result.RuleResults {
			ruleCfg := config.Rules[i]
			isOrphanFixEnabled := false
			for _, orphanCfg := range ruleCfg.getOrphanFilesDetections() {
				if orphanCfg.Enabled && orphanCfg.Autofix {
					isOrphanFixEnabled = true
					break
				}
			}

			// Create a set of orphan files to be deleted by this rule to avoid content fixes on them
			orphanFilesToDelete := make(map[string]bool)
			if isOrphanFixEnabled {
				for _, orphan := range ruleResult.OrphanFilesAutofixable {
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
				for _, orphan := range ruleResult.OrphanFilesAutofixable {
					osPath := pathutil.DenormalizePathForOS(orphan)
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
			if err := sourceedit.ApplyFileChanges(changesByFile); err != nil {
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
			isOrphanFixEnabled := false
			for _, orphanCfg := range rule.getOrphanFilesDetections() {
				if orphanCfg.Enabled && orphanCfg.Autofix {
					isOrphanFixEnabled = true
					break
				}
			}
			if isOrphanFixEnabled {
				fixableIssuesCount += len(ruleResult.OrphanFilesAutofixable)
			}
		}
		result.FixableIssuesCount = fixableIssuesCount
	}

	return result, nil
}
