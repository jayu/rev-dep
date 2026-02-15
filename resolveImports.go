package main

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
)

// escapeRegexPattern escapes all regex special characters except * which we want to preserve for wildcard replacement
func escapeRegexPattern(pattern string) string {
	escaped := strings.Replace(pattern, "*", "\x00", -1) // Temporarily replace *
	escaped = regexp.QuoteMeta(escaped)
	escaped = strings.Replace(escaped, "\x00", "*", -1) // Restore *
	return escaped
}

type RegExpArrItem struct {
	aliasKey string
	regExp   *regexp.Regexp
}

type NodeType string

const (
	LeafNode NodeType = "leaf"
	MapNode  NodeType = "map"
)

type ImportTargetTreeNode struct {
	nodeType      NodeType                         // "leaf" | "map"
	value         string                           // target value or empty string
	conditionsMap map[string]*ImportTargetTreeNode // conditional targets
}

type TsConfigParsed struct {
	aliases          map[string]string
	aliasesRegexps   []RegExpArrItem // Keep for backward compatibility during transition
	wildcardPatterns []WildcardPattern
	moduleSuffixes   []string
}

type PackageJsonImports struct {
	imports                       map[string]interface{}
	simpleImportTargetsByKey      map[string]string
	conditionalImportTargetsByKey map[string]map[string]*regexp.Regexp
	importsRegexps                []RegExpArrItem
	wildcardPatterns              []WildcardPattern
	conditionNames                []string
	parsedImportTargets           map[string]*ImportTargetTreeNode // parsed tree structure for targets
}

type PackageJsonExports struct {
	exports          map[string]interface{}
	wildcardPatterns []WildcardPattern
	parsedTargets    map[string]*ImportTargetTreeNode // parsed tree structure for targets
	hasDotPrefix     bool                             // cached check if any key starts with "."
}

type WildcardPattern struct {
	key    string
	prefix string
	suffix string
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
}

type ResolutionError int8

const (
	AliasNotResolved ResolutionError = iota
	FileNotFound
)

var SourceExtensions = []string{".d.ts", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}

var extensionRegExp = regexp.MustCompile(`(?:/index)?\.(?:js|jsx|ts|tsx|mjs|mjsx|cjs|d\.ts)$`)
var tsSupportedExtensionRegExp = regexp.MustCompile(`\.(?:js|jsx|ts|tsx|d\.ts)$`)

var extensionToOrder = map[string]int{
	".d.ts": 7,
	".ts":   6,
	".tsx":  5,
	".js":   4,
	".jsx":  3,
	".mjs":  2,
	".cjs":  1,
}

type SubpackageResolver struct {
	PkgPath  string
	Resolver *ModuleResolver
}

type ResolverManager struct {
	monorepoContext        *MonorepoContext
	subpackageResolvers    []SubpackageResolver
	rootResolver           *ModuleResolver
	followMonorepoPackages bool
	conditionNames         []string
	rootParams             RootParams
	filesAndExtensions     *map[string]string
}

type RootParams struct {
	TsConfigContent []byte
	PkgJsonContent  []byte
	SortedFiles     []string
	Cwd             string
}

func NewResolverManager(followMonorepoPackages bool, conditionNames []string, rootParams RootParams, excludeFilePatterns []GlobMatcher) *ResolverManager {
	var monorepoCtx *MonorepoContext
	if followMonorepoPackages {
		monorepoCtx = DetectMonorepo(rootParams.Cwd)
		if monorepoCtx != nil {
			monorepoCtx.FindWorkspacePackages(monorepoCtx.WorkspaceRoot, excludeFilePatterns)
		}
	}

	rm := &ResolverManager{
		monorepoContext:        monorepoCtx,
		subpackageResolvers:    []SubpackageResolver{},
		rootResolver:           nil,
		followMonorepoPackages: followMonorepoPackages,
		conditionNames:         conditionNames,
		rootParams:             rootParams,
		filesAndExtensions:     &map[string]string{},
	}

	for _, filePath := range rootParams.SortedFiles {
		addFilePathToFilesAndExtensions(NormalizePathForInternal(filePath), rm.filesAndExtensions)
	}

	// Create resolvers
	if monorepoCtx != nil {
		rm.rootResolver = createResolverForDir(monorepoCtx.WorkspaceRoot, rm)
		for _, pkgPath := range monorepoCtx.PackageToPath {
			rm.subpackageResolvers = append(rm.subpackageResolvers, SubpackageResolver{
				PkgPath:  pkgPath,
				Resolver: createResolverForDir(pkgPath, rm),
			})
		}
		// Sort by path length descending to ensure most specific paths are checked first
		slices.SortFunc(rm.subpackageResolvers, func(a, b SubpackageResolver) int {
			return len(b.PkgPath) - len(a.PkgPath)
		})
	} else {
		rm.rootResolver = NewImportsResolver(rootParams.Cwd, rootParams.TsConfigContent, rootParams.PkgJsonContent, rm.conditionNames, rm.rootParams.SortedFiles, rm)
	}

	return rm
}

func createResolverForDir(dirPath string, rm *ResolverManager) *ModuleResolver {

	var pkgContent []byte

	pkgContent, _ = os.ReadFile(filepath.Join(dirPath, "package.json"))

	tsConfigPath := filepath.Join(dirPath, "tsconfig.json")
	tsConfigContent, _ := ParseTsConfig(tsConfigPath)

	resolver := NewImportsResolver(dirPath, tsConfigContent, pkgContent, rm.conditionNames, rm.rootParams.SortedFiles, rm)

	return resolver
}

func (rm *ResolverManager) GetResolverForFile(filePath string) *ModuleResolver {
	for _, subPkg := range rm.subpackageResolvers {
		if strings.HasPrefix(filePath, subPkg.PkgPath) {
			return subPkg.Resolver
		}
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
		aliases:          map[string]string{},
		aliasesRegexps:   []RegExpArrItem{},
		wildcardPatterns: []WildcardPattern{},
		moduleSuffixes:   moduleSuffixes,
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

		tsConfigParsed.aliases[aliasKey] = aliasValue

		// Create wildcard pattern if alias contains wildcard
		if strings.Contains(aliasKey, "*") {
			wildcardIndex := strings.Index(aliasKey, "*")
			prefix := aliasKey[:wildcardIndex]
			suffix := aliasKey[wildcardIndex+1:]
			tsConfigParsed.wildcardPatterns = append(tsConfigParsed.wildcardPatterns, WildcardPattern{
				key:    aliasKey,
				prefix: prefix,
				suffix: suffix,
			})
		}

		// Keep regex for backward compatibility during transition
		escapedAliasKey := escapeRegexPattern(aliasKey)
		regExp := regexp.MustCompile("^" + strings.Replace(escapedAliasKey, "*", ".+?", 1) + "$")

		tsConfigParsed.aliasesRegexps = append(tsConfigParsed.aliasesRegexps, RegExpArrItem{
			regExp:   regExp,
			aliasKey: aliasKey,
		})
	}

	if hasBaseUrl {
		baseUrlAliasKey := "*"
		baseUrlAliasValue := strings.TrimSuffix(baseUrl, "/") + "/*"
		tsConfigParsed.aliases[baseUrlAliasKey] = baseUrlAliasValue

		// Create wildcard pattern for baseUrl
		tsConfigParsed.wildcardPatterns = append(tsConfigParsed.wildcardPatterns, WildcardPattern{
			key:    baseUrlAliasKey,
			prefix: "",
			suffix: "",
		})

		// Keep regex for backward compatibility during transition
		escapedBaseUrlAliasKey := escapeRegexPattern(baseUrlAliasKey)
		regExp := regexp.MustCompile(strings.Replace(escapedBaseUrlAliasKey, "*", ".+?", 1))

		tsConfigParsed.aliasesRegexps = append(tsConfigParsed.aliasesRegexps, RegExpArrItem{
			regExp:   regExp,
			aliasKey: baseUrlAliasKey,
		})
	}

	// Sort regexps as they are matched starting from longest matching prefix
	slices.SortFunc(tsConfigParsed.aliasesRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
		keyAMatchingPrefix := strings.Replace(itemA.aliasKey, "*", "", 1)
		keyBMatchingPrefix := strings.Replace(itemB.aliasKey, "*", "", 1)

		return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(tsConfigParsed.wildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.key) - len(patternA.key)
	})

	if debug {
		fmt.Printf("tsConfigParsed: %v\n", tsConfigParsed)
	}

	return tsConfigParsed
}

func NewImportsResolver(dirPath string, tsconfigContent []byte, packageJsonContent []byte, conditionNames []string, allFilePaths []string, manager *ResolverManager) *ModuleResolver {
	tsConfigParsed := ParseTsConfigContent(tsconfigContent)

	packageJsonImports := &PackageJsonImports{
		imports:                  map[string]interface{}{},
		simpleImportTargetsByKey: map[string]string{},
		importsRegexps:           []RegExpArrItem{},
		wildcardPatterns:         []WildcardPattern{},
		conditionNames:           conditionNames,
		parsedImportTargets:      map[string]*ImportTargetTreeNode{},
	}

	var rawPackageJson map[string]interface{}
	json.Unmarshal(jsonc.ToJSON(packageJsonContent), &rawPackageJson)

	if imports, ok := rawPackageJson["imports"]; ok {
		if importsMap, ok := imports.(map[string]interface{}); ok {
			packageJsonImports.imports = importsMap
			for key, target := range importsMap {
				if strings.Count(key, "*") > 1 {
					continue
				}

				// For simple string targets, store them directly (used by import conventions)
				if targetStr, ok := target.(string); ok && !strings.Contains(targetStr, "#") {
					cleanTarget := strings.TrimPrefix(targetStr, "./")
					packageJsonImports.simpleImportTargetsByKey[key] = cleanTarget
				}

				// Parse the target into tree structure
				parsedTarget := parseImportTarget(target, conditionNames)
				if parsedTarget == nil {
					// Skip this key entirely if its target is invalid
					continue
				}

				// Only add valid parsed targets
				packageJsonImports.parsedImportTargets[key] = parsedTarget

				// Create wildcard pattern if key contains wildcard
				if strings.Contains(key, "*") {
					wildcardIndex := strings.Index(key, "*")
					prefix := key[:wildcardIndex]
					suffix := key[wildcardIndex+1:]
					packageJsonImports.wildcardPatterns = append(packageJsonImports.wildcardPatterns, WildcardPattern{
						key:    key,
						prefix: prefix,
						suffix: suffix,
					})
				}

				// pre process and store import targets

				escapedKey := escapeRegexPattern(key)
				pattern := "^" + strings.Replace(escapedKey, "*", "(.*)", 1) + "$" // Since there can be only one wildcard, we could use prefix + suffix instead of regexp. Do it only if there will be perf issues with regexps
				regExp := regexp.MustCompile(pattern)
				packageJsonImports.importsRegexps = append(packageJsonImports.importsRegexps, RegExpArrItem{
					aliasKey: key,
					regExp:   regExp,
				})

			}

			// Sort regexps longest prefix first
			slices.SortFunc(packageJsonImports.importsRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
				aHasWildcard := strings.Contains(itemA.aliasKey, "*")
				bHasWildcard := strings.Contains(itemB.aliasKey, "*")

				if !aHasWildcard && bHasWildcard {
					return -1
				}
				if aHasWildcard && !bHasWildcard {
					return 1

				}
				keyAMatchingPrefix := strings.Replace(itemA.aliasKey, "*", "", 1)
				keyBMatchingPrefix := strings.Replace(itemB.aliasKey, "*", "", 1)

				return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
			})
		}
	}

	// Sort regexps as they are matched starting from longest matching prefix
	slices.SortFunc(tsConfigParsed.aliasesRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
		keyAMatchingPrefix := strings.Replace(itemA.aliasKey, "*", "", 1)
		keyBMatchingPrefix := strings.Replace(itemB.aliasKey, "*", "", 1)

		return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(tsConfigParsed.wildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.key) - len(patternA.key)
	})

	// Sort wildcard patterns by key length descending for specificity
	slices.SortFunc(packageJsonImports.wildcardPatterns, func(patternA, patternB WildcardPattern) int {
		return len(patternB.key) - len(patternA.key)
	})

	factory := &ModuleResolver{
		tsConfigParsed:     tsConfigParsed,
		packageJsonImports: packageJsonImports,
		aliasesCache:       map[string]ResolvedModuleInfo{},
		manager:            manager,
		resolverRoot:       dirPath,
		nodeModules:        GetNodeModulesFromPkgJson(packageJsonContent),
	}

	return factory
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
		base := strings.Replace(filePath, match, "", 1)
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

func (f *ModuleResolver) getModulePathWithExtension(modulePath string) (path string, err *ResolutionError) {
	// Strip existing TS extension from modulePath
	match := extensionRegExp.FindString(modulePath)
	if match != "" {
		tsSupportedExtension := tsSupportedExtensionRegExp.FindString(match)
		if tsSupportedExtension == "" {
			return modulePath, nil
		}
		modulePath = strings.Replace(modulePath, tsSupportedExtension, "", 1)
	}

	suffixes := f.tsConfigParsed.moduleSuffixes
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

func parseImportTarget(target interface{}, conditionNames []string) *ImportTargetTreeNode {
	if targetStr, ok := target.(string); ok {
		// Check if string contains more than one wildcard
		if strings.Count(targetStr, "*") > 1 {
			return nil // Invalid target - too many wildcards
		}
		// Simple string target - leaf node
		return &ImportTargetTreeNode{
			nodeType:      LeafNode,
			value:         targetStr,
			conditionsMap: nil,
		}
	}

	if targetMap, ok := target.(map[string]interface{}); ok {
		// Check if this is a conditional map or nested conditions
		hasConditions := false
		conditionsMap := make(map[string]*ImportTargetTreeNode)

		// First, check for condition names (conditional exports)
		for _, condition := range conditionNames {
			if val, ok := targetMap[condition]; ok {
				hasConditions = true
				parsedChild := parseImportTarget(val, conditionNames)
				if parsedChild != nil {
					conditionsMap[condition] = parsedChild
				}
			}
		}

		// Check for default condition
		if val, ok := targetMap["default"]; ok {
			hasConditions = true
			parsedChild := parseImportTarget(val, conditionNames)
			if parsedChild != nil {
				conditionsMap["default"] = parsedChild
			}
		}

		if hasConditions {
			// This is a conditional map node
			return &ImportTargetTreeNode{
				nodeType:      MapNode,
				value:         "",
				conditionsMap: conditionsMap,
			}
		} else {
			// This might be a nested condition (like import/require within node)
			// Treat all keys as conditions
			for key, val := range targetMap {
				parsedChild := parseImportTarget(val, conditionNames)
				if parsedChild != nil {
					conditionsMap[key] = parsedChild
				}
			}
			return &ImportTargetTreeNode{
				nodeType:      MapNode,
				value:         "",
				conditionsMap: conditionsMap,
			}
		}
	}

	return nil // Invalid target
}

func (f *ModuleResolver) resolveParsedImportTarget(node *ImportTargetTreeNode) string {
	if node == nil {
		return ""
	}

	if node.nodeType == LeafNode {
		return node.value
	}

	if node.nodeType == MapNode {
		// iterate through conditionNames
		for _, condition := range f.packageJsonImports.conditionNames {
			if child, ok := node.conditionsMap[condition]; ok {
				return f.resolveParsedImportTarget(child)
			}
		}
		// Try default
		if child, ok := node.conditionsMap["default"]; ok {
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
		for _, condition := range f.packageJsonImports.conditionNames {
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
	if _, ok := f.packageJsonImports.imports[request]; ok {
		if parsedTarget, ok := f.packageJsonImports.parsedImportTargets[request]; ok {
			resolvedTarget = f.resolveParsedImportTarget(parsedTarget)
		}
	}

	// If no exact match, try wildcard patterns using prefix/suffix matching
	if resolvedTarget == "" {
		for _, pattern := range f.packageJsonImports.wildcardPatterns {
			if strings.HasPrefix(request, pattern.prefix) && strings.HasSuffix(request, pattern.suffix) {
				if parsedTarget, ok := f.packageJsonImports.parsedImportTargets[pattern.key]; ok {
					resolvedTarget = f.resolveParsedImportTarget(parsedTarget)

					// Extract wildcard value using string slicing
					if strings.Contains(pattern.key, "*") {
						wildcardValue := request[len(pattern.prefix) : len(request)-len(pattern.suffix)]
						resolvedTarget = strings.Replace(resolvedTarget, "*", wildcardValue, 1)
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

	modulePath := NormalizePathForInternal(resolvedTarget)
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
	if aliasValue, ok := f.tsConfigParsed.aliases[request]; ok {
		aliasKey = request
		alias := aliasValue

		resolvedTarget := alias
		modulePath := filepath.Join(root, resolvedTarget)
		modulePath = NormalizePathForInternal(modulePath)

		actualFilePath, e := f.getModulePathWithExtension(modulePath)
		if e != nil {
			// alias matched, but file was not resolved
			return true, modulePath, NotResolvedModule, e
		}

		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}
		return true, actualFilePath, UserModule, nil
	}

	// Try wildcard patterns using prefix/suffix matching
	for _, pattern := range f.tsConfigParsed.wildcardPatterns {
		if strings.HasPrefix(request, pattern.prefix) && strings.HasSuffix(request, pattern.suffix) {
			aliasKey = pattern.key
			break
		}
	}

	var alias string
	if aliasKey != "" {
		if aliasValue, ok := f.tsConfigParsed.aliases[aliasKey]; ok {
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
			resolvedTarget = strings.Replace(alias, "*", wildcardValue, 1)
		}

		if resolvedTarget == "" {
			fmt.Println("Alias resolved to empty string for request", request)
			return true, "", NotResolvedModule, nil
		}

		modulePath := filepath.Join(root, resolvedTarget)
		modulePath = NormalizePathForInternal(modulePath)

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

	// Check if we're at root level
	if consumerRoot == f.manager.monorepoContext.WorkspaceRoot {
		return true
	}
	consumerConfig, err := f.manager.monorepoContext.GetPackageConfig(consumerRoot)
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
	if f.manager == nil || !f.manager.followMonorepoPackages || f.manager.monorepoContext == nil {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	pkgName := GetNodeModuleName(request)
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
		if exports != nil && len(exports.exports) > 0 {
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

	if exports == nil || len(exports.exports) == 0 {
		return "", nil // No exports, caller should use fallback
	}

	// Use the cached resolveExports method
	resolvedExport := f.resolveExportsCached(exports, subpath)
	if resolvedExport == "" {
		return "", nil // Export not found, don't try fallback
	}

	// resolvedExport is relative to target package root
	fullPath := filepath.Join(pkgPath, resolvedExport)
	modulePath := NormalizePathForInternal(fullPath)
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
	modulePath := NormalizePathForInternal(fullPath)
	actualFilePath, resolveErr := f.getModulePathWithExtension(modulePath)
	return actualFilePath, resolveErr
}

// resolveExportsCached resolves exports using pre-compiled string patterns and cached data
func (f *ModuleResolver) resolveExportsCached(exports *PackageJsonExports, subpath string) string {
	// 1. Check exact match
	if target, ok := exports.exports[subpath]; ok {
		return f.resolveCondition(target)
	}

	// 2. Handle sugar form (no dot prefix)
	if !exports.hasDotPrefix {
		// Sugar for "." export
		if subpath == "." {
			return f.resolveCondition(exports.exports)
		}
		return "" // Subpaths not allowed if only root export defined in sugar form
	}

	// 3. Check wildcard matches using cached string patterns
	for _, pattern := range exports.wildcardPatterns {
		if strings.HasPrefix(subpath, pattern.prefix) && strings.HasSuffix(subpath, pattern.suffix) {
			// Extract wildcard value
			wildcardValue := subpath[len(pattern.prefix) : len(subpath)-len(pattern.suffix)]
			target := exports.exports[pattern.key]
			resolved := f.resolveCondition(target)
			if resolved != "" {
				return strings.Replace(resolved, "*", wildcardValue, 1)
			}
		}
	}

	return ""
}

func (f *ModuleResolver) ResolveModule(request string, filePath string) (path string, rtype ResolvedImportType, err *ResolutionError) {
	cached, ok := f.aliasesCache[request]

	if ok {
		return cached.Path, cached.Type, nil
	}

	root := f.resolverRoot

	var modulePath string
	relativeFileName, _ := filepath.Rel(root, filePath)

	// Relative path
	if strings.HasPrefix(request, "./") || strings.HasPrefix(request, "../") || request == "." || request == ".." {
		modulePath = filepath.Join(root, relativeFileName, "../"+request)

		cleanedModulePath := filepath.Clean(modulePath)
		modulePathInternal := NormalizePathForInternal(cleanedModulePath)

		p, e := f.getModulePathWithExtension(modulePathInternal)

		return p, UserModule, e
	}

	aliasMatchedButFileNotFound := ""

	if requestMatched, resolvedPath, rtype, err := f.tryResolvePackageJsonImport(request, root); requestMatched {
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
		if requestMatched, resolvedPath, rtype, err := f.tryResolveWorkspacePackageImport(request, root); requestMatched {
			return resolvedPath, rtype, err
		}
	}

	if requestMatched, resolvedPath, rtype, err := f.tryResolveTsAlias(request); requestMatched {
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
	if aliasMatchedButFileNotFound != "" && aliasMatchedButFileNotFound != request {
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

func ResolveImports(fileImportsArr []FileImports, sortedFiles []string, cwd string, ignoreTypeImports bool, skipResolveMissing bool, packageJson string, tsconfigJson string, excludeFilePatterns []GlobMatcher, conditionNames []string, followMonorepoPackages bool, parseMode ParseMode, nodeModulesMatchingStrategy NodeModulesMatchingStrategy) (fileImports []FileImports, adjustedSortedFiles []string, resolverManager *ResolverManager) {
	tsConfigPath := JoinWithCwd(cwd, tsconfigJson)
	pkgJsonPath := JoinWithCwd(cwd, packageJson)

	if tsconfigJson == "" {
		tsConfigPath = filepath.Join(cwd, "tsconfig.json")
	}

	if packageJson == "" {
		pkgJsonPath = JoinWithCwd(cwd, "package.json")
	}

	pkjJsonDir := filepath.Dir(pkgJsonPath)

	// Let ParseTsConfig read and resolve the tsconfig file. If user provided
	// an explicit tsconfig path and parsing fails, exit with error to match
	// previous behaviour. Otherwise continue with empty tsconfig content.
	tsconfigContent := []byte("")
	if merged, err := ParseTsConfig(tsConfigPath); err == nil {
		tsconfigContent = merged
	} else {
		logWarning("Error when parsing tsconfig: %v", err)
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
		SortedFiles:     sortedFiles,
		Cwd:             cwd,
	}, excludeFilePatterns)

	missingResolutionFailedAttempts := map[string]bool{}
	discoveredFiles := map[string]bool{}

	for _, filePath := range sortedFiles {
		discoveredFiles[filePath] = true
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	ch_idx := make(chan int)

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
					pkjJsonDir,
					ignoreTypeImports,
					skipResolveMissing,
					i,
					&wg,
					&mu,
					ch_idx,
					BuiltInModules,
					excludeFilePatterns,
					parseMode,
					nodeModulesMatchingStrategy,
				)
			}(idx)
		}
	}()

	idx := 0
	for idx < len(fileImportsArr) {
		wg.Add(1)
		ch_idx <- idx
		idx++
	}

	wg.Wait()
	close(ch_idx)

	slices.Sort(sortedFiles)

	filteredFiles := make([]string, 0, len(sortedFiles))

	for _, filePath := range sortedFiles {
		if !MatchesAnyGlobMatcher(filePath, excludeFilePatterns, false) {
			filteredFiles = append(filteredFiles, filePath)
		}
	}
	filteredFileImportsArr := make([]FileImports, 0, len(fileImportsArr))

	for _, entry := range fileImportsArr {
		if !MatchesAnyGlobMatcher(entry.FilePath, excludeFilePatterns, false) {
			filteredFileImportsArr = append(filteredFileImportsArr, entry)
		}
	}

	return filteredFileImportsArr, filteredFiles, resolverManager
}

func resolveSingleFileImports(resolverManager *ResolverManager, missingResolutionFailedAttempts *map[string]bool, discoveredFiles *map[string]bool, fileImportsArr *[]FileImports, sortedFiles *[]string, pkgJsonDir string, ignoreTypeImports bool, skipResolveMissing bool, idx int, wg *sync.WaitGroup, mu *sync.Mutex, ch_idx chan int, builtInModules map[string]bool, excludeFilePatterns []GlobMatcher, parseMode ParseMode, nodeModulesMatchingStrategy NodeModulesMatchingStrategy) {
	mu.Lock()
	fileImports := (*fileImportsArr)[idx]
	mu.Unlock()
	imports := fileImports.Imports
	filePath := fileImports.FilePath

	importsResolver := resolverManager.GetResolverForFile(filePath)

	for impIdx, imp := range imports {
		moduleName := GetNodeModuleName(imp.Request)
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

		importPath, resolvedType, resolutionErr := importsResolver.ResolveModule(imp.Request, filePath)

		if resolutionErr != nil && importPath != imp.Request {
			// Some alias matched, but file was not resolved to project file or workspace package file. The resolution might be to some node module sub path eg `lodash/files/utils`
			localModuleName := GetNodeModuleName(importPath)
			if _, isNodeModule2 := nodeModulesList[localModuleName]; isNodeModule2 {
				moduleName = localModuleName
			}
		}

		_, isNodeModule := nodeModulesList[moduleName]

		if isNodeModule && resolutionErr != nil {
			// Check if it's a followed workspace package, only if not, consider package a node module
			isFollowedWorkspace := false
			if importsResolver.manager != nil && importsResolver.manager.followMonorepoPackages && importsResolver.manager.monorepoContext != nil {

				name := GetNodeModuleName(moduleName)
				if _, ok := importsResolver.manager.monorepoContext.PackageToPath[name]; ok {
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

					missingFilePath := GetMissingFile(modulePath, importsResolver.tsConfigParsed.moduleSuffixes)

					if missingFilePath != "" {
						// If file exists on disk but matches exclude patterns, mark it as excluded by user and do not add to discovery lists
						if MatchesAnyGlobMatcher(missingFilePath, excludeFilePatterns, false) {
							mu.Lock()
							imports[impIdx].PathOrName = missingFilePath
							imports[impIdx].ResolvedType = ExcludedByUser
							mu.Unlock()
						} else {
							missingFileContent, err := os.ReadFile(DenormalizePathForOS(missingFilePath))
							if err == nil {
								mu.Lock()
								imports[impIdx].PathOrName = missingFilePath
								imports[impIdx].ResolvedType = resolvedType
								mu.Unlock()

								missingFileImports := ParseImportsByte(missingFileContent, ignoreTypeImports, parseMode)

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
								logWarning("could not read resolved file '%s' imported from '%s'", missingFilePath, filePath)
							}
						}
					} else if isAssetPath(modulePath) {
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
							logWarning("asset import '%s' not found in %s", modulePath, filePath)
						}

					} else {
						mu.Lock()
						// In case we encounter missing import, we don't know if it was node_module or path to user file
						// For comparison convenience we store both cases as paths prefixed with cwd
						(*missingResolutionFailedAttempts)[importPath] = true
						mu.Unlock()
						logWarning("import '%s' in '%s' could not be resolved to a file", imp.Request, filePath)
					}
				}

			}

			if *resolutionErr == AliasNotResolved {
				// Likely external dependency, TODO handle that later
				continue
			}

		} else {
			// resolved to a path; if it's excluded by user, mark and do not add to discovery
			if MatchesAnyGlobMatcher(importPath, excludeFilePatterns, false) {
				mu.Lock()
				fileImports.Imports[impIdx].PathOrName = importPath
				imports[impIdx].ResolvedType = ExcludedByUser
				mu.Unlock()
			} else {
				mu.Lock()
				_, hasFileInDiscoveredFiles := (*discoveredFiles)[importPath]
				if !hasFileInDiscoveredFiles {
					mu.Unlock()
					missingFileContent, err := os.ReadFile(DenormalizePathForOS(importPath))
					if err == nil {

						missingFileImports := ParseImportsByte(missingFileContent, ignoreTypeImports, parseMode)
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

var assetExtensions = []string{"json", "png", "jpeg", "webp", "jpg", "svg", "gif", "ttf", "otf", "woff", "woff2", "css", "scss"}

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
		moduleSuffixes := resolver.tsConfigParsed.moduleSuffixes
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

func isAssetPath(filePath string) bool {
	for _, ext := range assetExtensions {
		if strings.HasSuffix(filePath, "."+ext) {
			return true
		}
	}
	return false
}
