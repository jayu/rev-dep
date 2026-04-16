package resolve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/tidwall/jsonc"

	"rev-dep-go/internal/diag"
	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/module"
	"rev-dep-go/internal/monorepo"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/pathutil"
)

// escapeRegexPattern escapes all regex special characters except * which we want to preserve for wildcard replacement
func escapeRegexPattern(pattern string) string {
	escaped := strings.Replace(pattern, "*", "\x00", -1) // Temporarily replace *
	escaped = regexp.QuoteMeta(escaped)
	escaped = strings.Replace(escaped, "\x00", "*", -1) // Restore *
	return escaped
}

type RegExpArrItem struct {
	AliasKey string
	RegExp   *regexp.Regexp
}

type TsConfigParsed struct {
	Aliases          map[string]string
	AliasesRegexps   []RegExpArrItem // Keep for backward compatibility during transition
	WildcardPatterns []WildcardPattern
	ModuleSuffixes   []string
}

type PackageJsonImports struct {
	Imports                       map[string]interface{}
	SimpleImportTargetsByKey      map[string]string
	ConditionalImportTargetsByKey map[string]map[string]*regexp.Regexp
	ImportsRegexps                []RegExpArrItem
	WildcardPatterns              []WildcardPattern
	ConditionNames                []string
	ParsedImportTargets           map[string]*ImportTargetTreeNode // parsed tree structure for targets
}

type ResolvedModuleInfo struct {
	Path string
	Type ResolvedImportType
}

type ModuleResolver struct {
	tsConfigParsed     *TsConfigParsed
	packageJsonImports *PackageJsonImports
	aliasesCache       map[string]ResolvedModuleInfo
	resolverRoot       string
	manager            *ResolverManager
	nodeModules        map[string]bool
	devNodeModules     map[string]bool
	packageJsonPath    string
}

type ResolutionError int8

const (
	AliasNotResolved ResolutionError = iota
	FileNotFound
)

var SourceExtensions = []string{".d.ts", ".ts", ".tsx", ".mts", ".js", ".jsx", ".mjs", ".cjs", ".vue", ".svelte"}

var extensionRegExp = regexp.MustCompile(`(?:/index)?\.(?:js|jsx|ts|tsx|mts|mjs|mjsx|cjs|vue|svelte|d\.ts)$`)
var tsSupportedExtensionRegExp = regexp.MustCompile(`\.(?:js|jsx|ts|tsx|mts|d\.ts)$`)

var extensionToOrder = map[string]int{
	".d.ts":   8,
	".ts":     7,
	".tsx":    6,
	".mts":    5,
	".js":     4,
	".jsx":    3,
	".mjs":    2,
	".cjs":    1,
	".vue":    1,
	".svelte": 1,
}

type SubpackageResolver struct {
	PkgPath  string
	Resolver *ModuleResolver
}

type ResolverManager struct {
	monorepoContext        *MonorepoContext
	subpackageResolvers    []SubpackageResolver
	rootResolver           *ModuleResolver
	cwdResolver            *ModuleResolver
	cwdPackagePath         string
	followMonorepoPackages FollowMonorepoPackagesValue
	conditionNames         []string
	rootParams             RootParams
	filesAndExtensions     *map[string]string
}

type RootParams struct {
	TsConfigContent []byte
	PkgJsonContent  []byte
	PkgJsonPath     string
	SortedFiles     []string
	Cwd             string
}

func NewResolverManager(followMonorepoPackages FollowMonorepoPackagesValue, conditionNames []string, rootParams RootParams, excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher) *ResolverManager {
	monorepoCtx := detectMonorepoContext(rootParams.Cwd, followMonorepoPackages, excludeFilePatterns, includeFilePatterns)

	rm := &ResolverManager{
		monorepoContext:        monorepoCtx,
		subpackageResolvers:    []SubpackageResolver{},
		rootResolver:           nil,
		cwdResolver:            nil,
		cwdPackagePath:         "",
		followMonorepoPackages: followMonorepoPackages,
		conditionNames:         conditionNames,
		rootParams:             rootParams,
		filesAndExtensions:     &map[string]string{},
	}

	for _, filePath := range rootParams.SortedFiles {
		addFilePathToFilesAndExtensions(pathutil.NormalizePathForInternal(filePath), rm.filesAndExtensions)
	}

	if monorepoCtx == nil {
		rm.rootResolver = NewImportsResolver(rootParams.Cwd, rootParams.TsConfigContent, rootParams.PkgJsonContent, rootParams.PkgJsonPath, rm.conditionNames, rm.rootParams.SortedFiles, rm)
		rm.cwdResolver = rm.rootResolver
		return rm
	}

	rm.rootResolver = createResolverForDir(monorepoCtx.WorkspaceRoot, rm)
	rm.cwdPackagePath = findCwdPackagePath(monorepoCtx, rootParams.Cwd)

	packagePathsToCreate := collectPackagePathsToCreate(monorepoCtx, followMonorepoPackages, rm.cwdPackagePath)
	rm.subpackageResolvers = createSubpackageResolvers(monorepoCtx, packagePathsToCreate, rm)

	rm.cwdResolver = findCwdResolver(rm.rootResolver, rm.subpackageResolvers, rm.cwdPackagePath)

	return rm
}

func detectMonorepoContext(cwd string, followMonorepoPackages FollowMonorepoPackagesValue, excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher) *monorepo.MonorepoContext {
	if !followMonorepoPackages.IsEnabled() {
		return nil
	}
	monorepoCtx := monorepo.DetectMonorepo(cwd)
	if monorepoCtx == nil {
		return nil
	}
	monorepoCtx.FindWorkspacePackages(excludeFilePatterns, includeFilePatterns)
	return monorepoCtx
}

func findCwdPackagePath(monorepoCtx *MonorepoContext, cwd string) string {
	if monorepoCtx == nil {
		return ""
	}
	normalizedCwd := pathutil.NormalizePathForInternal(cwd)
	bestMatch := ""

	for _, pkgPath := range monorepoCtx.PackageToPath {
		normalizedPkgPath := pathutil.NormalizePathForInternal(pkgPath)
		pkgPrefix := pathutil.StandardiseDirPathInternal(normalizedPkgPath)
		if normalizedCwd == normalizedPkgPath || strings.HasPrefix(normalizedCwd, pkgPrefix) {
			if bestMatch == "" || len(normalizedPkgPath) > len(bestMatch) {
				bestMatch = normalizedPkgPath
			}
		}
	}

	return bestMatch
}

func collectPackagePathsToCreate(monorepoCtx *MonorepoContext, followMonorepoPackages FollowMonorepoPackagesValue, cwdPackagePath string) map[string]bool {
	packagePathsToCreate := map[string]bool{}

	for pkgName, pkgPath := range monorepoCtx.PackageToPath {
		if !followMonorepoPackages.ShouldFollowPackage(pkgName) {
			continue
		}
		packagePathsToCreate[pathutil.NormalizePathForInternal(pkgPath)] = true
	}

	// Always keep a resolver for the cwd package when cwd belongs to a workspace package.
	// This preserves package-local tsconfig/package.json context for cwd files.
	if cwdPackagePath != "" {
		packagePathsToCreate[cwdPackagePath] = true
	}

	return packagePathsToCreate
}

func createSubpackageResolvers(monorepoCtx *MonorepoContext, packagePathsToCreate map[string]bool, rm *ResolverManager) []SubpackageResolver {
	subpackageResolvers := []SubpackageResolver{}

	for _, pkgPath := range monorepoCtx.PackageToPath {
		normalizedPkgPath := pathutil.NormalizePathForInternal(pkgPath)
		if !packagePathsToCreate[normalizedPkgPath] {
			continue
		}
		subpackageResolvers = append(subpackageResolvers, SubpackageResolver{
			PkgPath:  normalizedPkgPath,
			Resolver: createResolverForDir(pkgPath, rm),
		})
	}

	// Sort by path length descending to ensure most specific paths are checked first.
	slices.SortFunc(subpackageResolvers, func(a, b SubpackageResolver) int {
		return len(b.PkgPath) - len(a.PkgPath)
	})

	return subpackageResolvers
}

func findCwdResolver(rootResolver *ModuleResolver, subpackageResolvers []SubpackageResolver, cwdPackagePath string) *ModuleResolver {
	if cwdPackagePath == "" {
		return rootResolver
	}

	for _, subPkg := range subpackageResolvers {
		if subPkg.PkgPath == cwdPackagePath {
			return subPkg.Resolver
		}
	}

	return rootResolver
}

func createResolverForDir(dirPath string, rm *ResolverManager) *ModuleResolver {

	var pkgContent []byte

	pkgJsonPath := filepath.Join(dirPath, "package.json")
	pkgContent, _ = os.ReadFile(pkgJsonPath)

	tsConfigPath := filepath.Join(dirPath, "tsconfig.json")
	tsConfigContent, _ := ParseTsConfig(tsConfigPath)

	resolver := NewImportsResolver(dirPath, tsConfigContent, pkgContent, pkgJsonPath, rm.conditionNames, rm.rootParams.SortedFiles, rm)

	return resolver
}

func (rm *ResolverManager) GetResolverForFile(filePath string) *ModuleResolver {
	normalizedFilePath := pathutil.NormalizePathForInternal(filePath)
	for _, subPkg := range rm.subpackageResolvers {
		subPkgPrefix := pathutil.StandardiseDirPathInternal(subPkg.PkgPath)
		if normalizedFilePath == subPkg.PkgPath || strings.HasPrefix(normalizedFilePath, subPkgPrefix) {
			return subPkg.Resolver
		}
	}

	if rm.monorepoContext != nil && rm.cwdPackagePath != "" {
		return rm.rootResolver
	}
	if rm.cwdResolver != nil {
		return rm.cwdResolver
	}
	return rm.rootResolver
}

func (rm *ResolverManager) CollectAllNodeModules() map[string]bool {
	allNodeModules := map[string]bool{}

	// Collect from root resolver
	if rm.rootResolver != nil {
		for module := range rm.rootResolver.nodeModules {
			allNodeModules[module] = true
		}
	}

	// Collect from subpackage resolvers
	for _, subPkg := range rm.subpackageResolvers {
		for module := range subPkg.Resolver.nodeModules {
			allNodeModules[module] = true
		}
	}

	return allNodeModules
}

func isValidTsAliasTargetPath(path string) bool {
	// Reject absolute paths (starting with /)
	if strings.HasPrefix(path, "/") {
		return false
	}

	// Reject URLs
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return false
	}

	// Reject node_modules paths
	if strings.HasPrefix(path, "node_modules/") {
		return false
	}

	// Reject paths that look like other aliases (starting with @)
	if strings.HasPrefix(path, "@") {
		return false
	}

	// Allow everything else - this includes:
	// - Relative paths: "./src/*", "../lib/*"
	// - Directory-relative paths: "src/*", "lib/*"
	// - Direct file paths: "index.ts", "lib.js"
	// - Bare module names: "lodash", "react"
	return true
}

// ParseTsConfigContent extracts and processes TypeScript configuration from raw tsconfig content
// Returns a TsConfigParsed struct containing aliases, regex patterns, and wildcard patterns
func ParseTsConfigContent(tsconfigContent []byte) *TsConfigParsed {
	debug := false
	tsconfigContent = jsonc.ToJSON(tsconfigContent)

	if debug {
		fmt.Println("tsconfigContent", string(tsconfigContent))
	}

	var paths map[string][]string
	var baseUrl string
	var hasBaseUrl bool
	var moduleSuffixes []string

	// Only attempt to parse if tsconfig content is not empty
	if len(tsconfigContent) > 0 && string(tsconfigContent) != "" && string(tsconfigContent) != "{}" {
		var rawConfigForPaths map[string]interface{}

		err := json.Unmarshal(tsconfigContent, &rawConfigForPaths)

		if err != nil && debug {
			fmt.Printf("Failed to parse tsConfig paths : %s\n", err)
		}

		if compilerOptions, ok := rawConfigForPaths["compilerOptions"].(map[string]interface{}); ok {
			if pathsRaw, ok := compilerOptions["paths"].(map[string]interface{}); ok {
				paths = make(map[string][]string)
				for key, value := range pathsRaw {
					if valueArray, ok := value.([]interface{}); ok {
						pathArray := make([]string, len(valueArray))
						for i, v := range valueArray {
							if str, ok := v.(string); ok {
								pathArray[i] = str
							}
						}
						paths[key] = pathArray
					}
				}
			}

			if suffixesRaw, ok := compilerOptions["moduleSuffixes"].([]interface{}); ok {
				for _, v := range suffixesRaw {
					if str, ok := v.(string); ok {
						moduleSuffixes = append(moduleSuffixes, str)
					}
				}
			}
		}

		if paths == nil && debug {
			fmt.Printf("Paths not found in tsConfig from\n")
		}

		if debug {
			fmt.Printf("Paths: %v\n", paths)
		}

		var rawConfigForBaseUrl map[string]interface{}

		// TODO figure out if we can use just one unmarshaling
		err = json.Unmarshal(tsconfigContent, &rawConfigForBaseUrl)

		if err != nil && debug {
			fmt.Printf("Failed to parse tsConfig baseUrl from %s\n", err)
		}

		if compilerOptions, ok := rawConfigForBaseUrl["compilerOptions"].(map[string]interface{}); ok {
			if baseUrlRaw, ok := compilerOptions["baseUrl"]; ok {
				if baseUrlStr, ok := baseUrlRaw.(string); ok {
					baseUrl = baseUrlStr
					hasBaseUrl = true
				}
				// Handle cases where baseUrl might be a boolean or other type
				// We only process it if it's a string
			}
		}

		if !hasBaseUrl && debug {
			fmt.Printf("BaseUrl not found in tsConfig from \n")
		}
	} else {
		if debug {
			fmt.Printf("Empty tsconfig content, skipping parsing\n")
		}
		paths = make(map[string][]string)
	}

	tsConfigParsed := &TsConfigParsed{
		Aliases:          map[string]string{},
		AliasesRegexps:   []RegExpArrItem{},
		WildcardPatterns: []WildcardPattern{},
		ModuleSuffixes:   moduleSuffixes,
	}

	for aliasKey, aliasValues := range paths {
		// TODO parse multiple aliases values
		// In order to do it we would have to store aliasValues as array and return possibly multiple paths from ResolveModule and then process all of them in loop in resolveSingleFileImports
		// This is a lot of additional complexity, so it's not supported in initial version.
		aliasValue := aliasValues[0]

		// Validate that alias target is a relative path
		if !isValidTsAliasTargetPath(aliasValue) {
			continue // Skip aliases with non-relative paths
		}

		tsConfigParsed.Aliases[aliasKey] = aliasValue

		// Create wildcard pattern if alias contains wildcard
		if strings.Contains(aliasKey, "*") {
			wildcardIndex := strings.Index(aliasKey, "*")
			prefix := aliasKey[:wildcardIndex]
			suffix := aliasKey[wildcardIndex+1:]
			tsConfigParsed.WildcardPatterns = append(tsConfigParsed.WildcardPatterns, WildcardPattern{
				Key:    aliasKey,
				Prefix: prefix,
				Suffix: suffix,
			})
		}

		// Keep regex for backward compatibility during transition
		escapedAliasKey := escapeRegexPattern(aliasKey)
		regExp := regexp.MustCompile("^" + strings.Replace(escapedAliasKey, "*", ".+?", 1) + "$")

		tsConfigParsed.AliasesRegexps = append(tsConfigParsed.AliasesRegexps, RegExpArrItem{
			RegExp:   regExp,
			AliasKey: aliasKey,
		})
	}

	if hasBaseUrl {
		baseUrlAliasKey := "*"
		baseUrlAliasValue := strings.TrimSuffix(baseUrl, "/") + "/*"
		if _, hasWildcardAlias := tsConfigParsed.Aliases[baseUrlAliasKey]; !hasWildcardAlias {
			tsConfigParsed.Aliases[baseUrlAliasKey] = baseUrlAliasValue

			// Create wildcard pattern for baseUrl
			tsConfigParsed.WildcardPatterns = append(tsConfigParsed.WildcardPatterns, WildcardPattern{
				Key:    baseUrlAliasKey,
				Prefix: "",
				Suffix: "",
			})

			// Keep regex for backward compatibility during transition
			escapedBaseUrlAliasKey := escapeRegexPattern(baseUrlAliasKey)
			regExp := regexp.MustCompile(strings.Replace(escapedBaseUrlAliasKey, "*", ".+?", 1))

			tsConfigParsed.AliasesRegexps = append(tsConfigParsed.AliasesRegexps, RegExpArrItem{
				RegExp:   regExp,
				AliasKey: baseUrlAliasKey,
			})
		}
	}

	// Sort regexps as they are matched starting from longest matching prefix
	slices.SortFunc(tsConfigParsed.AliasesRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
		keyAMatchingPrefix := strings.Replace(itemA.AliasKey, "*", "", 1)
		keyBMatchingPrefix := strings.Replace(itemB.AliasKey, "*", "", 1)

		return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(tsConfigParsed.WildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.Key) - len(patternA.Key)
	})

	if debug {
		fmt.Printf("tsConfigParsed: %v\n", tsConfigParsed)
	}

	return tsConfigParsed
}

func NewImportsResolver(dirPath string, tsconfigContent []byte, packageJsonContent []byte, packageJsonPath string, conditionNames []string, allFilePaths []string, manager *ResolverManager) *ModuleResolver {
	tsConfigParsed := ParseTsConfigContent(tsconfigContent)

	packageJsonImports := &PackageJsonImports{
		Imports:                       map[string]interface{}{},
		SimpleImportTargetsByKey:      map[string]string{},
		ConditionalImportTargetsByKey: map[string]map[string]*regexp.Regexp{},
		ImportsRegexps:                []RegExpArrItem{},
		WildcardPatterns:              []WildcardPattern{},
		ConditionNames:                conditionNames,
		ParsedImportTargets:           map[string]*ImportTargetTreeNode{},
	}

	var rawPackageJson map[string]interface{}
	json.Unmarshal(jsonc.ToJSON(packageJsonContent), &rawPackageJson)

	if imports, ok := rawPackageJson["imports"]; ok {
		if importsMap, ok := imports.(map[string]interface{}); ok {
			packageJsonImports.Imports = importsMap
			for key, target := range importsMap {
				if strings.Count(key, "*") > 1 {
					continue
				}

				// For simple string targets, store them directly (used by import conventions)
				if targetStr, ok := target.(string); ok && !strings.Contains(targetStr, "#") {
					cleanTarget := strings.TrimPrefix(targetStr, "./")
					packageJsonImports.SimpleImportTargetsByKey[key] = cleanTarget
				}

				// Parse the target into tree structure
				parsedTarget := monorepo.ParseImportTarget(target, conditionNames)
				if parsedTarget == nil {
					// Skip this key entirely if its target is invalid
					continue
				}

				// Only add valid parsed targets
				packageJsonImports.ParsedImportTargets[key] = parsedTarget

				// Create wildcard pattern if key contains wildcard
				if strings.Contains(key, "*") {
					wildcardIndex := strings.Index(key, "*")
					prefix := key[:wildcardIndex]
					suffix := key[wildcardIndex+1:]
					packageJsonImports.WildcardPatterns = append(packageJsonImports.WildcardPatterns, WildcardPattern{
						Key:    key,
						Prefix: prefix,
						Suffix: suffix,
					})
				}

				// pre process and store import targets

				escapedKey := escapeRegexPattern(key)
				pattern := "^" + strings.Replace(escapedKey, "*", "(.*)", 1) + "$" // Since there can be only one wildcard, we could use prefix + suffix instead of regexp. Do it only if there will be perf issues with regexps
				regExp := regexp.MustCompile(pattern)
				packageJsonImports.ImportsRegexps = append(packageJsonImports.ImportsRegexps, RegExpArrItem{
					AliasKey: key,
					RegExp:   regExp,
				})

			}

			// Sort regexps longest prefix first
			slices.SortFunc(packageJsonImports.ImportsRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
				aHasWildcard := strings.Contains(itemA.AliasKey, "*")
				bHasWildcard := strings.Contains(itemB.AliasKey, "*")

				if !aHasWildcard && bHasWildcard {
					return -1
				}
				if aHasWildcard && !bHasWildcard {
					return 1

				}
				keyAMatchingPrefix := strings.Replace(itemA.AliasKey, "*", "", 1)
				keyBMatchingPrefix := strings.Replace(itemB.AliasKey, "*", "", 1)

				return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
			})
		}
	}

	// Sort regexps as they are matched starting from longest matching prefix
	slices.SortFunc(tsConfigParsed.AliasesRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
		keyAMatchingPrefix := strings.Replace(itemA.AliasKey, "*", "", 1)
		keyBMatchingPrefix := strings.Replace(itemB.AliasKey, "*", "", 1)

		return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(tsConfigParsed.WildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.Key) - len(patternA.Key)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(packageJsonImports.WildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.Key) - len(patternA.Key)
	})

	deps, devDeps := module.GetNodeModulesFromPkgJson(packageJsonContent)

	factory := &ModuleResolver{
		tsConfigParsed:     tsConfigParsed,
		packageJsonImports: packageJsonImports,
		aliasesCache:       map[string]ResolvedModuleInfo{},
		manager:            manager,
		resolverRoot:       dirPath,
		devNodeModules:     devDeps,
		nodeModules:        nil,
		packageJsonPath:    packageJsonPath,
	}

	factory.nodeModules = mergeNodeModules(deps, devDeps)

	return factory
}

func mergeNodeModules(deps map[string]bool, devDeps map[string]bool) map[string]bool {
	merged := make(map[string]bool, len(deps)+len(devDeps))
	for dep := range deps {
		merged[dep] = true
	}
	for dep := range devDeps {
		merged[dep] = true
	}
	return merged
}

func hasExtensionPrecedence(previousExt string, currentExt string) bool {
	if strings.HasPrefix(previousExt, "/index") {
		return true
	} else if strings.HasPrefix(currentExt, "/index") {
		return false
	}

	previousExtOrder, hasPrevExtOrder := extensionToOrder[previousExt]
	if !hasPrevExtOrder {
		previousExtOrder = 0
	}

	currentExtOrder, hasCurrExtOrder := extensionToOrder[currentExt]
	if !hasCurrExtOrder {
		currentExtOrder = 0
	}

	return currentExtOrder > previousExtOrder
}

func addFilePathToFilesAndExtensions(filePath string, filesAndExtensions *map[string]string) {
	match := extensionRegExp.FindString(filePath)

	if match != "" {
		base := strings.TrimSuffix(filePath, match)
		baseExt, hasBaseExt := (*filesAndExtensions)[base]
		if !hasBaseExt || hasExtensionPrecedence(baseExt, match) {
			// If value is not in the map we have to add it
			// If it stores `index.*`` extension we have to replace with non-index extension (case when there is `file.ext` and `file/index.ext`)
			// Any imports in the codebase will have to refer to index file by explicitly using path with `index` suffix
			// Also in case of multiple extensions for the same file name, they have to be referred explicitly. In that case we keep what was there in the map, it most likely won't be used if user has an app that builds without errors.
			// We can extent our approach by storing array of extensions, but it not needed, unless we want to warn about ambiguous import
			(*filesAndExtensions)[base] = match
		}

		// index files can be either referenced by containing directory name eg `path/to/dir` or by `index` file name without extension `path/to/dir/index`
		// If we have multiple index files, eg `index.ts`, `index.js` only the last one discovered in fs will be in the map
		// Electively users will have to have explicit import in their app, if app is building correctly
		// As a static analysis tool we could warn against ambiguous import
		if strings.HasPrefix(match, "/index") {
			key := base + "/index"
			value := strings.Replace(match, "/index", "", 1)

			(*filesAndExtensions)[key] = value
		}
	}
}

func (f *ResolverManager) AddFilePathToFilesAndExtensions(filePath string) {
	addFilePathToFilesAndExtensions(filePath, f.filesAndExtensions)
}

func applyWildcardValue(target string, wildcardValue string) string {
	prefix, suffix, found := strings.Cut(target, "*")

	if !found {
		return target
	}

	wildcardValue = strings.TrimSuffix(wildcardValue, suffix)

	return prefix + wildcardValue + suffix
}

func (f *ModuleResolver) getModulePathWithExtension(modulePath string) (path string, err *ResolutionError) {
	match := extensionRegExp.FindString(modulePath)
	if match != "" {
		// Explicit extension import, modulePath contains extension
		explicitBase := strings.TrimSuffix(modulePath, match)
		explicitExt, hasExplicitBase := (*f.manager.filesAndExtensions)[explicitBase]
		if hasExplicitBase && explicitExt == match {
			return modulePath, nil
		}

		// Explicit extension import, modulePath contains extension, special-case explicit index imports like ".../dir/index.ts".
		if strings.HasPrefix(match, "/index") {
			indexBase := explicitBase + "/index"
			indexExt, hasIndexBase := (*f.manager.filesAndExtensions)[indexBase]
			expectedIndexExt := strings.TrimPrefix(match, "/index")
			if hasIndexBase && indexExt == expectedIndexExt {
				return modulePath, nil
			}
		}

		// For explicit non-TS substitutions (eg .mjs/.cjs), do not try extension fallback.
		tsSupportedExtension := tsSupportedExtensionRegExp.FindString(match)
		if tsSupportedExtension == "" {
			e := FileNotFound
			return modulePath, &e
		}
		modulePath = strings.Replace(modulePath, tsSupportedExtension, "", 1)
	}

	suffixes := f.tsConfigParsed.ModuleSuffixes
	if len(suffixes) == 0 {
		// No suffixes configured - direct lookup
		extension, has := (*f.manager.filesAndExtensions)[modulePath]
		if has {
			return modulePath + extension, nil
		}
		e := FileNotFound
		return modulePath, &e
	}

	// Try each suffix in order
	for _, suffix := range suffixes {
		suffixedPath := modulePath + suffix
		extension, has := (*f.manager.filesAndExtensions)[suffixedPath]
		if has {
			return suffixedPath + extension, nil
		}

		// For non-empty suffixes, also try index files: basePath + "/index" + suffix
		if suffix != "" {
			indexPath := modulePath + "/index" + suffix
			extension, has := (*f.manager.filesAndExtensions)[indexPath]
			if has {
				return indexPath + extension, nil
			}
		}
	}

	e := FileNotFound
	return modulePath, &e
}

func (f *ModuleResolver) resolveParsedImportTarget(node *ImportTargetTreeNode) string {
	if node == nil {
		return ""
	}

	if node.NodeType == LeafNode {
		return node.Value
	}

	if node.NodeType == MapNode {
		// iterate through conditionNames
		for _, condition := range f.packageJsonImports.ConditionNames {
			if child, ok := node.ConditionsMap[condition]; ok {
				return f.resolveParsedImportTarget(child)
			}
		}
		// Try default
		if child, ok := node.ConditionsMap["default"]; ok {
			return f.resolveParsedImportTarget(child)
		}
	}

	return ""
}

func (f *ModuleResolver) resolveCondition(target interface{}) string {
	if targetStr, ok := target.(string); ok {
		return targetStr
	}

	if targetMap, ok := target.(map[string]interface{}); ok {
		// iterate through conditionNames
		for _, condition := range f.packageJsonImports.ConditionNames {
			if val, ok := targetMap[condition]; ok {
				return f.resolveCondition(val)
			}
		}
		// Try default
		if val, ok := targetMap["default"]; ok {
			return f.resolveCondition(val)
		}
	}

	return ""
}

var NotResolvedPath = "NotResolvedPath"

func (f *ModuleResolver) tryResolvePackageJsonImport(request string, root string) (requestMatched bool, resolvedPath string, rtype ResolvedImportType, err *ResolutionError) {
	if !strings.HasPrefix(request, "#") {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	resolvedTarget := ""

	// First try exact match in imports map (fast path)
	if _, ok := f.packageJsonImports.Imports[request]; ok {
		if parsedTarget, ok := f.packageJsonImports.ParsedImportTargets[request]; ok {
			resolvedTarget = f.resolveParsedImportTarget(parsedTarget)
		}
	}

	// If no exact match, try wildcard patterns using prefix/suffix matching
	if resolvedTarget == "" {
		for _, pattern := range f.packageJsonImports.WildcardPatterns {
			if strings.HasPrefix(request, pattern.Prefix) && strings.HasSuffix(request, pattern.Suffix) {
				if parsedTarget, ok := f.packageJsonImports.ParsedImportTargets[pattern.Key]; ok {
					resolvedTarget = f.resolveParsedImportTarget(parsedTarget)

					// Extract wildcard value using string slicing
					if strings.Contains(pattern.Key, "*") {
						wildcardValue := request[len(pattern.Prefix) : len(request)-len(pattern.Suffix)]
						resolvedTarget = applyWildcardValue(resolvedTarget, wildcardValue)
					}
				}
				break
			}
		}
	}

	if resolvedTarget == "" {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	// If result starts with ./, it is relative to package.json (root)
	if strings.HasPrefix(resolvedTarget, "./") {
		resolvedTarget = filepath.Join(root, resolvedTarget)
	}

	modulePath := pathutil.NormalizePathForInternal(resolvedTarget)
	actualFilePath, e := f.getModulePathWithExtension(modulePath)

	if e == nil {
		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}
		return true, actualFilePath, UserModule, nil
	}

	// Return modulePath because user can alias node-module or other external module
	return true, modulePath, NotResolvedModule, e
}

func (f *ModuleResolver) tryResolveTsAlias(request string) (requestMatched bool, resolvedPath string, rtype ResolvedImportType, err *ResolutionError) {
	root := f.resolverRoot
	aliasKey := ""

	// First try exact match
	if aliasValue, ok := f.tsConfigParsed.Aliases[request]; ok {
		aliasKey = request
		alias := aliasValue

		resolvedTarget := alias
		modulePath := filepath.Join(root, resolvedTarget)
		modulePath = pathutil.NormalizePathForInternal(modulePath)

		actualFilePath, e := f.getModulePathWithExtension(modulePath)
		if e != nil {
			// alias matched, but file was not resolved
			return true, modulePath, NotResolvedModule, e
		}

		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}
		return true, actualFilePath, UserModule, nil
	}

	// Try wildcard patterns using prefix/suffix matching
	for _, pattern := range f.tsConfigParsed.WildcardPatterns {
		if strings.HasPrefix(request, pattern.Prefix) && strings.HasSuffix(request, pattern.Suffix) {
			aliasKey = pattern.Key
			break
		}
	}

	var alias string
	if aliasKey != "" {
		if aliasValue, ok := f.tsConfigParsed.Aliases[aliasKey]; ok {
			alias = aliasValue // TODO: we assume only one aliased path exists
		}
	}

	// AliasedPath
	if alias != "" {
		resolvedTarget := alias

		if strings.Contains(aliasKey, "*") {
			// Extract wildcard value
			wildcardIndex := strings.Index(aliasKey, "*")
			prefix := aliasKey[:wildcardIndex]
			suffix := aliasKey[wildcardIndex+1:]
			wildcardValue := request[len(prefix) : len(request)-len(suffix)]
			resolvedTarget = applyWildcardValue(alias, wildcardValue)
		}

		if resolvedTarget == "" {
			fmt.Println("Alias resolved to empty string for request", request)
			return true, "", NotResolvedModule, nil
		}

		modulePath := filepath.Join(root, resolvedTarget)
		modulePath = pathutil.NormalizePathForInternal(modulePath)

		actualFilePath, e := f.getModulePathWithExtension(modulePath)

		if e != nil {
			// alias matched, but file was not resolved
			return true, modulePath, NotResolvedModule, e
		}

		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}

		return true, actualFilePath, UserModule, nil
	}

	return false, NotResolvedPath, NotResolvedModule, nil
}

// validateWorkspaceDependency checks if the consumer package can depend on the target package
func (f *ModuleResolver) validateWorkspaceDependency(consumerRoot, targetPkgName string) bool {
	if f.manager == nil || f.manager.monorepoContext == nil {
		return false
	}

	normalizedConsumerRoot := pathutil.NormalizePathForInternal(consumerRoot)
	if targetPkgPath, ok := f.manager.monorepoContext.PackageToPath[targetPkgName]; ok {
		// Allow same-package imports by package name (self-reference), even without explicit self-dependency.
		if pathutil.NormalizePathForInternal(targetPkgPath) == normalizedConsumerRoot {
			return true
		}
	}

	// Check if we're at root level
	if normalizedConsumerRoot == pathutil.NormalizePathForInternal(f.manager.monorepoContext.WorkspaceRoot) {
		return true
	}
	consumerConfig, err := f.manager.monorepoContext.GetPackageConfig(normalizedConsumerRoot)
	if err != nil {
		return false
	}

	// Check dependencies and devDependencies
	_, hasDep := consumerConfig.Dependencies[targetPkgName]
	_, hasDevDep := consumerConfig.DevDependencies[targetPkgName]

	return hasDep || hasDevDep
}

func (f *ModuleResolver) tryResolveWorkspacePackageImport(request string, root string) (requestMatched bool, resolvedPath string, rtype ResolvedImportType, err *ResolutionError) {
	// Check if it is a workspace package import (Monorepo support)
	// Only if manager is present and monorepo is enabled
	if f.manager == nil || !f.manager.followMonorepoPackages.IsEnabled() || f.manager.monorepoContext == nil {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	pkgName := module.GetNodeModuleName(request)

	if !f.manager.followMonorepoPackages.ShouldFollowPackage(pkgName) {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	pkgPath, ok := f.manager.monorepoContext.PackageToPath[pkgName]

	if !ok {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	// Validate dependency relationship
	if !f.validateWorkspaceDependency(root, pkgName) {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	// Extract subpath from request
	subpath := "."
	if len(request) > len(pkgName) {
		subpath = "." + request[len(pkgName):]
	}

	// Try to resolve using exports first
	actualFilePath, err := f.resolvePackageExports(pkgPath, subpath)
	if err != nil {
		return true, actualFilePath, MonorepoModule, err
	}

	// If exports resolution returned empty but there are exports, don't try fallback
	if actualFilePath == "" {
		// Check if the package has exports defined
		exports, _ := f.manager.monorepoContext.GetPackageExports(pkgPath, f.manager.conditionNames)
		if exports != nil && len(exports.Exports) > 0 {
			// Package has exports but the subpath wasn't found, so fail
			return false, NotResolvedPath, NotResolvedModule, nil
		}

		// No exports, try fallback
		actualFilePath, err = f.resolvePackageFallback(pkgPath, subpath)
		if err != nil {
			return true, actualFilePath, MonorepoModule, err
		}
	}

	// Cache and return the result
	if actualFilePath != "" {
		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: MonorepoModule}
		return true, actualFilePath, MonorepoModule, nil
	}

	return false, NotResolvedPath, NotResolvedModule, nil
}

// resolvePackageExports resolves the subpath using the package's exports configuration
func (f *ModuleResolver) resolvePackageExports(pkgPath, subpath string) (string, *ResolutionError) {
	exports, err := f.manager.monorepoContext.GetPackageExports(pkgPath, f.manager.conditionNames)
	if err != nil {
		e := FileNotFound
		return "", &e
	}

	if exports == nil || len(exports.Exports) == 0 {
		return "", nil // No exports, caller should use fallback
	}

	// Use the cached resolveExports method
	resolvedExport := f.resolveExportsCached(exports, subpath)
	if resolvedExport == "" {
		return "", nil // Export not found, don't try fallback
	}

	// resolvedExport is relative to target package root
	fullPath := filepath.Join(pkgPath, resolvedExport)
	modulePath := pathutil.NormalizePathForInternal(fullPath)
	actualFilePath, resolveErr := f.getModulePathWithExtension(modulePath)
	return actualFilePath, resolveErr
}

// resolvePackageFallback resolves the subpath using main/module fallback when no exports are defined
func (f *ModuleResolver) resolvePackageFallback(pkgPath, subpath string) (string, *ResolutionError) {
	config, err := f.manager.monorepoContext.GetPackageConfig(pkgPath)
	if err != nil {
		e := FileNotFound
		return "", &e
	}

	resolvedSubpath := subpath
	if subpath == "." {
		if config.Module != "" {
			resolvedSubpath = config.Module
		} else if config.Main != "" {
			resolvedSubpath = config.Main
		}
	}

	fullPath := filepath.Join(pkgPath, resolvedSubpath)
	modulePath := pathutil.NormalizePathForInternal(fullPath)
	actualFilePath, resolveErr := f.getModulePathWithExtension(modulePath)
	return actualFilePath, resolveErr
}

// resolveExportsCached resolves exports using pre-compiled string patterns and cached data
func (f *ModuleResolver) resolveExportsCached(exports *PackageJsonExports, subpath string) string {
	// 1. Check exact match
	if target, ok := exports.Exports[subpath]; ok {
		return f.resolveCondition(target)
	}

	// 2. Handle sugar form (no dot prefix)
	if !exports.HasDotPrefix {
		// Sugar for "." export
		if subpath == "." {
			return f.resolveCondition(exports.Exports)
		}
		return "" // Subpaths not allowed if only root export defined in sugar form
	}

	// 3. Check wildcard matches using cached string patterns
	for _, pattern := range exports.WildcardPatterns {
		if strings.HasPrefix(subpath, pattern.Prefix) && strings.HasSuffix(subpath, pattern.Suffix) {
			// Extract wildcard value
			wildcardValue := subpath[len(pattern.Prefix) : len(subpath)-len(pattern.Suffix)]
			target := exports.Exports[pattern.Key]
			resolved := f.resolveCondition(target)
			if resolved != "" {
				return applyWildcardValue(resolved, wildcardValue)
			}
		}
	}

	return ""
}

func (f *ModuleResolver) ResolveModule(request string, filePath string) (path string, rtype ResolvedImportType, err *ResolutionError) {
	requestWithoutQuery := request
	if idx := strings.Index(requestWithoutQuery, "?"); idx >= 0 {
		requestWithoutQuery = requestWithoutQuery[:idx]
	}

	cached, ok := f.aliasesCache[requestWithoutQuery]

	if ok {
		return cached.Path, cached.Type, nil
	}

	root := f.resolverRoot

	var modulePath string
	relativeFileName, _ := filepath.Rel(root, filePath)

	// Relative path
	if strings.HasPrefix(requestWithoutQuery, "./") || strings.HasPrefix(requestWithoutQuery, "../") || requestWithoutQuery == "." || requestWithoutQuery == ".." {
		modulePath = filepath.Join(root, relativeFileName, "../"+requestWithoutQuery)

		cleanedModulePath := filepath.Clean(modulePath)
		modulePathInternal := pathutil.NormalizePathForInternal(cleanedModulePath)

		p, e := f.getModulePathWithExtension(modulePathInternal)

		return p, UserModule, e
	}

	aliasMatchedButFileNotFound := ""

	if requestMatched, resolvedPath, rtype, err := f.tryResolvePackageJsonImport(requestWithoutQuery, root); requestMatched {
		if err != nil {
			// Alias was matched, but path was not resolved
			aliasMatchedButFileNotFound = resolvedPath
		} else {
			return resolvedPath, rtype, err
		}
	}

	// Try workspace package resolution for the original request before TypeScript alias resolution
	// to ensure monorepo packages take precedence over TypeScript aliases
	// But only if no package.json import was matched
	if aliasMatchedButFileNotFound == "" {
		if requestMatched, resolvedPath, rtype, err := f.tryResolveWorkspacePackageImport(requestWithoutQuery, root); requestMatched {
			return resolvedPath, rtype, err
		}
	}

	if requestMatched, resolvedPath, rtype, err := f.tryResolveTsAlias(requestWithoutQuery); requestMatched {
		if err != nil && aliasMatchedButFileNotFound == "" {
			// Alias was matched, but path was not resolved
			aliasMatchedButFileNotFound = resolvedPath
		}

		if err == nil {
			return resolvedPath, rtype, err
		}
	}

	// Only try workspace package resolution again if we have a different target to resolve
	// This avoids redundant calls when the target is the same as the original request
	if aliasMatchedButFileNotFound != "" && aliasMatchedButFileNotFound != requestWithoutQuery {
		if requestMatched, resolvedPath, rtype, err := f.tryResolveWorkspacePackageImport(aliasMatchedButFileNotFound, root); requestMatched {
			return resolvedPath, rtype, err
		}
	}

	if aliasMatchedButFileNotFound != "" {
		e := FileNotFound
		return aliasMatchedButFileNotFound, UserModule, &e
	}

	e := AliasNotResolved
	return "", NotResolvedModule, &e
}

func ResolveImports(fileImportsArr []FileImports, sortedFiles []string, cwd string, ignoreTypeImports bool, skipResolveMissing bool, packageJson string, tsconfigJson string, excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher, conditionNames []string, followMonorepoPackages FollowMonorepoPackagesValue, customAssetExtensions []string, parseMode ParseMode, nodeModulesMatchingStrategy NodeModulesMatchingStrategy) (fileImports []FileImports, adjustedSortedFiles []string, resolverManager *ResolverManager) {
	tsConfigPath := pathutil.JoinWithCwd(cwd, tsconfigJson)
	pkgJsonPath := pathutil.JoinWithCwd(cwd, packageJson)

	if tsconfigJson == "" {
		tsConfigPath = filepath.Join(cwd, "tsconfig.json")
	}

	if packageJson == "" {
		pkgJsonPath = pathutil.JoinWithCwd(cwd, "package.json")
	}

	// Let ParseTsConfig read and resolve the tsconfig file. If user provided
	// an explicit tsconfig path and parsing fails, exit with error to match
	// previous behaviour. Otherwise continue with empty tsconfig content.
	tsconfigContent := []byte("")
	if merged, err := ParseTsConfig(tsConfigPath); err == nil {
		tsconfigContent = merged
	} else {
		diag.Warnf("Error when parsing tsconfig: %v", err)
		if tsconfigJson != "" {
			os.Exit(1)
		}
	}

	pkgJsonContent, err := os.ReadFile(pkgJsonPath)

	if err != nil {
		pkgJsonContent = []byte("")
	}

	resolverManager = NewResolverManager(followMonorepoPackages, conditionNames, RootParams{
		TsConfigContent: tsconfigContent,
		PkgJsonContent:  pkgJsonContent,
		PkgJsonPath:     pkgJsonPath,
		SortedFiles:     sortedFiles,
		Cwd:             cwd,
	}, excludeFilePatterns, includeFilePatterns)

	missingResolutionFailedAttempts := map[string]bool{}
	discoveredFiles := map[string]bool{}

	for _, filePath := range sortedFiles {
		discoveredFiles[filePath] = true
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	ch_idx := make(chan int)
	assetExtensionsSet := createAssetExtensionsSet(customAssetExtensions)

	// Limit concurrency to avoid memory spikes
	maxConcurrency := runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, maxConcurrency)

	go func() {
		for idx := range ch_idx {
			sem <- struct{}{} // Acquire semaphore
			go func(i int) {
				defer func() { <-sem }() // Release semaphore
				resolveSingleFileImports(
					resolverManager,
					&missingResolutionFailedAttempts,
					&discoveredFiles,
					&fileImportsArr,
					&sortedFiles,
					ignoreTypeImports,
					skipResolveMissing,
					i,
					&wg,
					&mu,
					ch_idx,
					module.BuiltInModules,
					excludeFilePatterns,
					includeFilePatterns,
					assetExtensionsSet,
					parseMode,
					nodeModulesMatchingStrategy,
				)
			}(idx)
		}
	}()

	initialWorkItems := len(fileImportsArr)
	for idx := 0; idx < initialWorkItems; idx++ {
		wg.Add(1)
		ch_idx <- idx
	}

	wg.Wait()
	close(ch_idx)

	slices.Sort(sortedFiles)

	filteredFiles := make([]string, 0, len(sortedFiles))

	for _, filePath := range sortedFiles {
		if !globutil.IsExcludedByPatterns(filePath, excludeFilePatterns, includeFilePatterns) {
			filteredFiles = append(filteredFiles, filePath)
		}
	}
	filteredFileImportsArr := make([]FileImports, 0, len(fileImportsArr))

	for _, entry := range fileImportsArr {
		if !globutil.IsExcludedByPatterns(entry.FilePath, excludeFilePatterns, includeFilePatterns) {
			filteredFileImportsArr = append(filteredFileImportsArr, entry)
		}
	}

	return filteredFileImportsArr, filteredFiles, resolverManager
}

func resolveSingleFileImports(resolverManager *ResolverManager, missingResolutionFailedAttempts *map[string]bool, discoveredFiles *map[string]bool, fileImportsArr *[]FileImports, sortedFiles *[]string, ignoreTypeImports bool, skipResolveMissing bool, idx int, wg *sync.WaitGroup, mu *sync.Mutex, ch_idx chan int, builtInModules map[string]bool, excludeFilePatterns []globutil.GlobMatcher, includeFilePatterns []globutil.GlobMatcher, assetExtensionsSet map[string]bool, parseMode ParseMode, nodeModulesMatchingStrategy NodeModulesMatchingStrategy) {
	mu.Lock()
	fileImports := (*fileImportsArr)[idx]
	mu.Unlock()
	imports := fileImports.Imports
	filePath := fileImports.FilePath

	importsResolver := resolverManager.GetResolverForFile(filePath)

	for impIdx, imp := range imports {

		if imp.ResolvedType == LocalExportDeclaration {
			continue
		}

		moduleName := module.GetNodeModuleName(imp.Request)
		mu.Lock()
		_, isBuiltInModule := builtInModules[moduleName]
		if isBuiltInModule {
			fileImports.Imports[impIdx].PathOrName = moduleName
			fileImports.Imports[impIdx].ResolvedType = BuiltInModule
			mu.Unlock()
			continue
		}

		nodeModulesList := importsResolver.nodeModules

		if nodeModulesMatchingStrategy == NodeModulesMatchingStrategyRootResolver {
			nodeModulesList = resolverManager.rootResolver.nodeModules
		}

		if nodeModulesMatchingStrategy == NodeModulesMatchingStrategyCwdResolver {
			nodeModulesList = resolverManager.cwdResolver.nodeModules
		}

		importPath, resolvedType, resolutionErr := importsResolver.ResolveModule(imp.Request, filePath)

		if resolutionErr != nil && importPath != imp.Request {
			// Some alias matched, but file was not resolved to project file or workspace package file. The resolution might be to some node module sub path eg `lodash/files/utils`
			localModuleName := module.GetNodeModuleName(importPath)
			if _, isNodeModule2 := nodeModulesList[localModuleName]; isNodeModule2 {
				moduleName = localModuleName
			}
		}

		_, isNodeModule := nodeModulesList[moduleName]

		if isNodeModule && resolutionErr != nil {
			// Check if it's a followed workspace package, only if not, consider package a node module
			isFollowedWorkspace := false
			if importsResolver.manager != nil && importsResolver.manager.followMonorepoPackages.IsEnabled() && importsResolver.manager.monorepoContext != nil {

				name := module.GetNodeModuleName(moduleName)
				if _, ok := importsResolver.manager.monorepoContext.PackageToPath[name]; ok && importsResolver.manager.followMonorepoPackages.ShouldFollowPackage(name) {
					isFollowedWorkspace = true
				}
			}

			if !isFollowedWorkspace {
				fileImports.Imports[impIdx].PathOrName = moduleName
				fileImports.Imports[impIdx].ResolvedType = NodeModule
				mu.Unlock()

				continue
			}

		}

		mu.Unlock()

		if resolutionErr != nil {

			if *resolutionErr == FileNotFound && !skipResolveMissing {
				mu.Lock()
				_, checkedAlready := (*missingResolutionFailedAttempts)[importPath]
				mu.Unlock()
				if !checkedAlready {

					// If alias was resolved, importPath contains modulePath
					// We have to look for file in fs under different extensions
					// File is likely outside of cwd or in ignored path
					modulePath := importPath

					missingFilePath := fs.GetMissingFile(modulePath, importsResolver.tsConfigParsed.ModuleSuffixes)

					if missingFilePath != "" {
						// If file exists on disk but matches exclude patterns, mark it as excluded by user and do not add to discovery lists
						if globutil.IsExcludedByPatterns(missingFilePath, excludeFilePatterns, includeFilePatterns) {
							mu.Lock()
							imports[impIdx].PathOrName = missingFilePath
							imports[impIdx].ResolvedType = ExcludedByUser
							mu.Unlock()
						} else {
							missingFileContent, err := os.ReadFile(pathutil.DenormalizePathForOS(missingFilePath))
							if err == nil {
								mu.Lock()
								imports[impIdx].PathOrName = missingFilePath
								imports[impIdx].ResolvedType = resolvedType
								mu.Unlock()

								missingFileImports := parser.ParseImportsByte(missingFileContent, ignoreTypeImports, parseMode)

								mu.Lock()
								// Double-check after acquiring lock in case another goroutine added it
								if _, alreadyAdded := (*discoveredFiles)[missingFilePath]; !alreadyAdded {
									*fileImportsArr = append(*fileImportsArr, FileImports{
										FilePath: missingFilePath,
										Imports:  missingFileImports,
									})
									wg.Add(1)
									// We use a goroutine here to push the new index to the channel.
									// If we pushed directly (ch_idx <- val), it could block if the channel is full (unbuffered)
									// or no worker is ready to receive. Since we are inside a worker, blocking here
									// could lead to a deadlock if all workers are blocked trying to add new work.
									// By spawning a goroutine, we ensure this worker can finish its current job
									// and release the semaphore, allowing another worker (or itself) to pick up this new task.
									go func(val int) {
										ch_idx <- val
									}(len(*fileImportsArr) - 1)

									*sortedFiles = append(*sortedFiles, missingFilePath)
									(*discoveredFiles)[missingFilePath] = true
									importsResolver.manager.AddFilePathToFilesAndExtensions(missingFilePath)
								}
								mu.Unlock()
							} else {
								mu.Lock()
								(*missingResolutionFailedAttempts)[importPath] = true
								mu.Unlock()
								diag.Warnf("could not read resolved file '%s' imported from '%s'", missingFilePath, filePath)
							}
						}
					} else if isAssetPath(modulePath, assetExtensionsSet) {
						_, err := os.Stat(modulePath)
						if err == nil {
							mu.Lock()
							imports[impIdx].PathOrName = modulePath
							imports[impIdx].ResolvedType = AssetModule
							mu.Unlock()
						} else {
							mu.Lock()
							// In case we encounter missing import, we don't know if it was node_module or path to user file
							// For comparison convenience we store both cases as paths prefixed with cwd
							(*missingResolutionFailedAttempts)[importPath] = true
							mu.Unlock()
							diag.Warnf("asset import '%s' not found in %s", modulePath, filePath)
						}

					} else {
						mu.Lock()
						// In case we encounter missing import, we don't know if it was node_module or path to user file
						// For comparison convenience we store both cases as paths prefixed with cwd
						(*missingResolutionFailedAttempts)[importPath] = true
						mu.Unlock()
						diag.Warnf("import '%s' in '%s' could not be resolved to a file", imp.Request, filePath)
					}
				}

			}

			if *resolutionErr == AliasNotResolved {
				// Likely external dependency, TODO handle that later
				continue
			}

		} else {
			// resolved to a path; if it's excluded by user, mark and do not add to discovery
			if globutil.IsExcludedByPatterns(importPath, excludeFilePatterns, includeFilePatterns) {
				mu.Lock()
				fileImports.Imports[impIdx].PathOrName = importPath
				imports[impIdx].ResolvedType = ExcludedByUser
				mu.Unlock()
			} else {
				mu.Lock()
				_, hasFileInDiscoveredFiles := (*discoveredFiles)[importPath]
				if !hasFileInDiscoveredFiles {
					mu.Unlock()
					missingFileContent, err := os.ReadFile(pathutil.DenormalizePathForOS(importPath))
					if err == nil {

						missingFileImports := parser.ParseImportsByte(missingFileContent, ignoreTypeImports, parseMode)
						mu.Lock()
						// Double-check after acquiring lock in case another goroutine added it
						if _, alreadyAdded := (*discoveredFiles)[importPath]; !alreadyAdded {
							*fileImportsArr = append(*fileImportsArr, FileImports{
								FilePath: importPath,
								Imports:  missingFileImports,
							})
							wg.Add(1)
							/*
								We use a goroutine here to push the new index to the channel.
								If we pushed directly (ch_idx <- val), it could block if the channel is full (unbuffered)
								or no worker is ready to receive. Since we are inside a worker, blocking here
								could lead to a deadlock if all workers are blocked trying to add new work.
								By spawning a goroutine, we ensure this worker can finish its current job
								and release the semaphore, allowing another worker (or itself) to pick up this new task.
							*/
							go func(val int) {
								ch_idx <- val
							}(len(*fileImportsArr) - 1)

							*sortedFiles = append(*sortedFiles, importPath)
							(*discoveredFiles)[importPath] = true
							importsResolver.manager.AddFilePathToFilesAndExtensions(importPath)
						}
						mu.Unlock()
					}
				} else {
					mu.Unlock()
				}
				mu.Lock()
				fileImports.Imports[impIdx].PathOrName = importPath
				imports[impIdx].ResolvedType = resolvedType
				mu.Unlock()
			}
		}
	}

	wg.Done()
}

var defaultAssetExtensions = []string{"json", "png", "jpeg", "webp", "jpg", "svg", "gif", "ttf", "otf", "woff", "woff2", "css", "scss", "yml", "yaml"}

// DetectModuleSuffixVariants identifies files that are platform-specific variants
// created by moduleSuffixes (e.g., button.android.tsx when button.ios.tsx is the
// resolved variant). These files should be excluded from orphan/unused-exports
// detection since they are valid platform alternatives, not truly unreferenced.
// Each file is checked against the moduleSuffixes of its own resolver, so
// monorepos where only some packages define moduleSuffixes are handled correctly.
func DetectModuleSuffixVariants(files []string, resolverManager *ResolverManager) map[string]bool {
	variants := map[string]bool{}

	for _, file := range files {
		resolver := resolverManager.GetResolverForFile(file)
		if resolver == nil {
			continue
		}
		moduleSuffixes := resolver.tsConfigParsed.ModuleSuffixes
		if len(moduleSuffixes) == 0 {
			continue
		}

		ext := extensionRegExp.FindString(file)
		if ext == "" {
			continue
		}
		fileWithoutExt := strings.TrimSuffix(file, ext)

		for _, suffix := range moduleSuffixes {
			var base string
			if suffix == "" {
				base = fileWithoutExt
			} else {
				if !strings.HasSuffix(fileWithoutExt, suffix) {
					continue
				}
				base = strings.TrimSuffix(fileWithoutExt, suffix)
			}

			// Check if any OTHER configured suffix + base exists
			for _, otherSuffix := range moduleSuffixes {
				if otherSuffix == suffix {
					continue
				}
				candidate := base + otherSuffix
				if _, exists := (*resolverManager.filesAndExtensions)[candidate]; exists {
					variants[file] = true
					break
				}
			}
			if variants[file] {
				break
			}
		}
	}

	return variants
}

func createAssetExtensionsSet(customAssetExtensions []string) map[string]bool {
	extensions := make(map[string]bool, len(defaultAssetExtensions)+len(customAssetExtensions))
	for _, ext := range defaultAssetExtensions {
		extensions[ext] = true
	}
	for _, ext := range customAssetExtensions {
		extensions[ext] = true
	}
	return extensions
}

func ValidateCustomAssetExtensions(customAssetExtensions []string, prefix string) error {
	for i, extension := range customAssetExtensions {
		if extension == "" {
			return fmt.Errorf("%s[%d] cannot be empty", prefix, i)
		}
		if extension != strings.TrimSpace(extension) {
			return fmt.Errorf("%s[%d] must not have leading or trailing spaces", prefix, i)
		}
		if strings.HasPrefix(extension, ".") {
			return fmt.Errorf("%s[%d] must not start with '.'", prefix, i)
		}
	}
	return nil
}

func isAssetPath(filePath string, assetExtensionsSet map[string]bool) bool {
	if len(assetExtensionsSet) == 0 {
		return false
	}
	lastDot := strings.LastIndex(filePath, ".")
	if lastDot == -1 || lastDot == len(filePath)-1 {
		return false
	}
	ext := strings.ToLower(filePath[lastDot+1:])
	return assetExtensionsSet[ext]
}
