package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	aliases        map[string]string
	aliasesRegexps []RegExpArrItem
}

type PackageJsonImports struct {
	imports                       map[string]interface{}
	simpleImportTargetsByKey      map[string]string
	conditionalImportTargetsByKey map[string]map[string]*regexp.Regexp
	importsRegexps                []RegExpArrItem
	conditionNames                []string
	parsedImportTargets           map[string]*ImportTargetTreeNode // parsed tree structure for targets
}

type PackageJsonExports struct {
	exports        map[string]interface{}
	exportsRegexps []RegExpArrItem
	parsedTargets  map[string]*ImportTargetTreeNode // parsed tree structure for targets
	hasDotPrefix   bool                             // cached check if any key starts with "."
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

var extensionRegExp = regexp.MustCompile(`(?:/index)?\.(?:js|jsx|ts|tsx|mjs|mjsx|cjs|d\.ts)$`)
var tsSupportedExtensionRegExp = regexp.MustCompile(`\.(?:js|jsx|ts|tsx|d\.ts)$`)

var extensionToOrder = map[string]int{
	".d.ts": 5,
	".ts":   4,
	".tsx":  3,
	".js":   2,
	".jsx":  1,
}

func stringifyParsedTsConfig(tsConfigParsed *TsConfigParsed) string {
	result := ""

	for key, val := range tsConfigParsed.aliases {
		result += key + ":" + val + "\n"
	}

	result += "\n___________\n"

	result += "\n___________\n"

	for _, val := range tsConfigParsed.aliasesRegexps {
		result += fmt.Sprintf("%v", val) + "\n"
	}

	return result
}

type ResolverManager struct {
	monorepoContext        *MonorepoContext
	subpackageResolvers    map[string]*ModuleResolver
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
		subpackageResolvers:    make(map[string]*ModuleResolver),
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
			rm.subpackageResolvers[pkgPath] = createResolverForDir(pkgPath, rm)
		}
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
	for pkgPath, resolver := range rm.subpackageResolvers {
		if strings.HasPrefix(filePath, pkgPath) {
			return resolver
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
	for _, resolver := range rm.subpackageResolvers {
		for module := range resolver.nodeModules {
			allNodeModules[module] = true
		}
	}

	return allNodeModules
}

func (rm *ResolverManager) GetNodeModulesForFile(filePath string) map[string]bool {
	resolver := rm.GetResolverForFile(filePath)
	if resolver != nil {
		return resolver.nodeModules
	}
	return map[string]bool{}
}

func isValidTsAliasTargetPath(path string) bool {
	return strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}

func NewImportsResolver(dirPath string, tsconfigContent []byte, packageJsonContent []byte, conditionNames []string, allFilePaths []string, manager *ResolverManager) *ModuleResolver {
	debug := false
	tsconfigContent = jsonc.ToJSON(tsconfigContent)

	if debug {
		fmt.Println("tsconfigContent", string(tsconfigContent))
	}

	var rawConfigForPaths map[string]map[string]map[string][]string

	err := json.Unmarshal(tsconfigContent, &rawConfigForPaths)

	if err != nil && debug {
		fmt.Printf("Failed to parse tsConfig paths : %s\n", err)
	}

	paths, ok := rawConfigForPaths["compilerOptions"]["paths"]

	if !ok && debug {
		fmt.Printf("Paths not found in tsConfig from\n")
	}

	if debug {
		fmt.Printf("Paths: %v\n", paths)
	}

	var rawConfigForBaseUrl map[string]map[string]string

	// TODO figure out if we can use just one unmarshaling
	err = json.Unmarshal(tsconfigContent, &rawConfigForBaseUrl)

	if err != nil && debug {
		fmt.Printf("Failed to parse tsConfig baseUrl from %s\n", err)
	}

	baseUrl, hasBaseUrl := rawConfigForBaseUrl["compilerOptions"]["baseUrl"]

	if !hasBaseUrl && debug {
		fmt.Printf("BaseUrl not found in tsConfig from \n")
	}

	tsConfigParsed := &TsConfigParsed{
		aliases:        map[string]string{},
		aliasesRegexps: []RegExpArrItem{},
	}

	if debug {
		fmt.Printf("tsConfigParsed: %v\n", tsConfigParsed)
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
		escapedBaseUrlAliasKey := escapeRegexPattern(baseUrlAliasKey)
		regExp := regexp.MustCompile(strings.Replace(escapedBaseUrlAliasKey, "*", ".+?", 1))

		tsConfigParsed.aliasesRegexps = append(tsConfigParsed.aliasesRegexps, RegExpArrItem{
			regExp:   regExp,
			aliasKey: baseUrlAliasKey,
		})
	}

	packageJsonImports := &PackageJsonImports{
		imports:             map[string]interface{}{},
		importsRegexps:      []RegExpArrItem{},
		conditionNames:      conditionNames,
		parsedImportTargets: map[string]*ImportTargetTreeNode{},
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

				// Parse the target into tree structure
				parsedTarget := parseImportTarget(target, conditionNames)
				if parsedTarget == nil {
					// Skip this key entirely if its target is invalid
					continue
				}

				// Only add valid parsed targets
				packageJsonImports.parsedImportTargets[key] = parsedTarget

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

func (f *ResolverManager) getModulePathWithExtension(modulePath string) (path string, err *ResolutionError) {
	match := extensionRegExp.FindString(modulePath)

	if match != "" {
		tsSupportedExtension := tsSupportedExtensionRegExp.FindString(match)

		// TS has this weird feature that file extension in import actually does not matter until it is js|jsx|ts|tsx
		// You can import 'file.ts' by importing 'file.jsx'
		// Hence we modify the modulePath by removing the extension so the file can be picked up with other extension
		if tsSupportedExtension == "" {
			return modulePath, nil
		}
		modulePath = strings.Replace(modulePath, tsSupportedExtension, "", 1)
	}

	extension, has := (*f.filesAndExtensions)[modulePath]

	if has {

		return modulePath + extension, nil
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

	for _, importRegex := range f.packageJsonImports.importsRegexps {
		if importRegex.regExp.MatchString(request) {
			key := importRegex.aliasKey
			keyMatchRegexp := importRegex.regExp

			var localResolvedTarget string
			if parsedTarget, ok := f.packageJsonImports.parsedImportTargets[key]; ok {
				localResolvedTarget = f.resolveParsedImportTarget(parsedTarget)
			}

			if localResolvedTarget != "" {
				// Replace * if present
				if strings.Contains(key, "*") {
					matches := keyMatchRegexp.FindStringSubmatch(request)
					if len(matches) > 1 {
						wildcardValue := matches[1]
						localResolvedTarget = strings.Replace(localResolvedTarget, "*", wildcardValue, 1)
					}
				}

				// If result starts with ./, it is relative to package.json (root)
				if strings.HasPrefix(localResolvedTarget, "./") {
					resolvedTarget = filepath.Join(root, localResolvedTarget)
					break
				}
				// External package or monorepo workspace package
				resolvedTarget = localResolvedTarget
				break
			}
		}
	}

	if resolvedTarget == "" {
		return false, NotResolvedPath, NotResolvedModule, nil
	}

	modulePath := NormalizePathForInternal(resolvedTarget)
	actualFilePath, e := f.manager.getModulePathWithExtension(modulePath)

	if e == nil {
		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}
		return true, actualFilePath, UserModule, nil
	}

	// Return modulePath becasue user can alias node-module or other external module
	return true, modulePath, NotResolvedModule, e
}

func (f *ModuleResolver) tryResolveTsAlias(request string) (requestMatched bool, resolvedPath string, rtype ResolvedImportType, err *ResolutionError) {
	root := f.resolverRoot
	aliasKey := ""

	for _, aliasRegex := range f.tsConfigParsed.aliasesRegexps {
		if aliasRegex.regExp.MatchString(request) {
			aliasKey = aliasRegex.aliasKey
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

		if strings.HasSuffix(aliasKey, "*") {
			aliasKeyPrefix := strings.TrimSuffix(aliasKey, "*")
			resolvedTarget = strings.Replace(alias, "*", strings.Replace(request, aliasKeyPrefix, "", 1), 1)
		}

		if resolvedTarget == "" {
			fmt.Println("Alias resolved to empty string for request", request)
			return true, "", NotResolvedModule, nil
		}

		modulePath := filepath.Join(root, resolvedTarget)
		modulePath = NormalizePathForInternal(modulePath)

		actualFilePath, e := f.manager.getModulePathWithExtension(modulePath)

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
	actualFilePath, resolveErr := f.manager.getModulePathWithExtension(modulePath)
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
	actualFilePath, resolveErr := f.manager.getModulePathWithExtension(modulePath)
	return actualFilePath, resolveErr
}

func (f *ModuleResolver) resolveExports(exports map[string]interface{}, subpath string) string {
	// 1. Check exact match
	if target, ok := exports[subpath]; ok {
		return f.resolveCondition(target)
	}
	// 2. Check wildcards
	// Iterate exports keys, find wildcard matches.
	// Spec says longest specific key match?
	// Sort keys?
	// Doing simple scan for now.

	// Optimisation: if exports is just strings/nested conditions (no "." content), it treats as "." export?
	// Actually exports can be just the condition map for "." export.
	// e.g. "exports": { "import": "..." } -> equivalent to "exports": { ".": { "import": "..." } }
	// Check if keys start with "."

	hasDot := false
	for k := range exports {
		if strings.HasPrefix(k, ".") {
			hasDot = true
			break
		}
	}

	if !hasDot {
		// Sugar for "." export
		if subpath == "." {
			return f.resolveCondition(exports)
		}
		return "" // Subpaths not allowed if only root export defined in sugar form
	}

	// Sort keys by length desc
	var keys []string
	for k := range exports {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b string) int {
		return len(b) - len(a)
	})

	// TODO: should we cache regexps like we do for ts aliases? Do we need regexps at all - they are slow?
	for _, key := range keys {
		if strings.Contains(key, "*") {
			escapedKey := escapeRegexPattern(key)
			regexKey := "^" + strings.Replace(escapedKey, "*", "(.*)", 1) + "$"
			re := regexp.MustCompile(regexKey) // TODO: we should cache regexps
			matches := re.FindStringSubmatch(subpath)
			if len(matches) > 1 {
				target := exports[key]
				resolved := f.resolveCondition(target)
				if resolved != "" {
					return strings.Replace(resolved, "*", matches[1], 1)
				}
			}
		}
	}

	return ""
}

// resolveExportsCached resolves exports using pre-compiled regex patterns and cached data
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

	// 3. Check wildcard matches using cached regex patterns
	for _, regexItem := range exports.exportsRegexps {
		if regexItem.regExp.MatchString(subpath) {
			key := regexItem.aliasKey
			matches := regexItem.regExp.FindStringSubmatch(subpath)
			if len(matches) > 1 {
				target := exports.exports[key]
				resolved := f.resolveCondition(target)
				if resolved != "" {
					return strings.Replace(resolved, "*", matches[1], 1)
				}
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

		p, e := f.manager.getModulePathWithExtension(modulePathInternal)
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

	if requestMatched, resolvedPath, rtype, err := f.tryResolveTsAlias(request); requestMatched {
		if err != nil && aliasMatchedButFileNotFound == "" {
			// Alias was matched, but path was not resolved
			aliasMatchedButFileNotFound = resolvedPath
		}

		if err == nil {
			return resolvedPath, rtype, err
		}
	}

	requestForWorkspacePackageImportResolution := request

	if aliasMatchedButFileNotFound != "" {
		requestForWorkspacePackageImportResolution = aliasMatchedButFileNotFound
	}

	if requestMatched, resolvedPath, rtype, err := f.tryResolveWorkspacePackageImport(requestForWorkspacePackageImportResolution, root); requestMatched {
		return resolvedPath, rtype, err
	}

	if aliasMatchedButFileNotFound != "" {
		e := FileNotFound
		return aliasMatchedButFileNotFound, UserModule, &e
	}

	e := AliasNotResolved
	return "", NotResolvedModule, &e
}

func ResolveImports(fileImportsArr []FileImports, sortedFiles []string, cwd string, ignoreTypeImports bool, skipResolveMissing bool, packageJson string, tsconfigJson string, excludeFilePatterns []GlobMatcher, conditionNames []string, followMonorepoPackages bool) (fileImports []FileImports, adjustedSortedFiles []string) {
	tsConfigPath := JoinWithCwd(cwd, tsconfigJson)
	pkgJsonPath := JoinWithCwd(cwd, packageJson)

	if tsconfigJson == "" {
		tsConfigPath = filepath.Join(cwd, "tsconfig.json")
	}

	tsConfigDir := filepath.Dir(tsConfigPath)

	if packageJson == "" {
		pkgJsonPath = JoinWithCwd(cwd, "package.json")
	}

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

	resolverManager := NewResolverManager(followMonorepoPackages, conditionNames, RootParams{
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

	go func() {
		for idx := range ch_idx {
			go resolveSingleFileImports(
				resolverManager,
				&missingResolutionFailedAttempts,
				&discoveredFiles,
				&fileImportsArr,
				&sortedFiles,
				tsConfigDir,
				ignoreTypeImports,
				skipResolveMissing,
				idx,
				&wg,
				&mu,
				ch_idx,
				BuiltInModules,
				excludeFilePatterns,
			)
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

	return filteredFileImportsArr, filteredFiles
}

func resolveSingleFileImports(resolverManager *ResolverManager, missingResolutionFailedAttempts *map[string]bool, discoveredFiles *map[string]bool, fileImportsArr *[]FileImports, sortedFiles *[]string, tsConfigDirOrCwd string, ignoreTypeImports bool, skipResolveMissing bool, idx int, wg *sync.WaitGroup, mu *sync.Mutex, ch_idx chan int, builtInModules map[string]bool, excludeFilePatterns []GlobMatcher) {
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

		importPath, resolvedType, resolutionErr := importsResolver.ResolveModule(imp.Request, filePath)

		if resolutionErr != nil && importPath != imp.Request {
			// Some alias matched, but file was not resolved to project file or workspace package file. The resolution might be to some node module sub path eg `lodash/files/utils`
			localModuleName := GetNodeModuleName(importPath)
			if _, isNodeModule2 := importsResolver.nodeModules[localModuleName]; isNodeModule2 {
				moduleName = localModuleName
			}
		}

		_, isNodeModule := importsResolver.nodeModules[moduleName]

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

					missingFilePath := GetMissingFile(modulePath)

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

								missingFileImports := ParseImportsByte(missingFileContent, ignoreTypeImports)

								mu.Lock()
								*fileImportsArr = append(*fileImportsArr, FileImports{
									FilePath: missingFilePath,
									Imports:  missingFileImports,
								})
								wg.Add(1)
								ch_idx <- len(*fileImportsArr) - 1

								*sortedFiles = append(*sortedFiles, missingFilePath)
								importsResolver.manager.AddFilePathToFilesAndExtensions(missingFilePath)
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
				mu.Unlock()
				if !hasFileInDiscoveredFiles {
					missingFileContent, err := os.ReadFile(DenormalizePathForOS(importPath))
					if err == nil {

						missingFileImports := ParseImportsByte(missingFileContent, ignoreTypeImports)
						mu.Lock()
						*fileImportsArr = append(*fileImportsArr, FileImports{
							FilePath: importPath,
							Imports:  missingFileImports,
						})
						wg.Add(1)
						ch_idx <- len(*fileImportsArr) - 1

						*sortedFiles = append(*sortedFiles, importPath)
						(*discoveredFiles)[importPath] = true
						importsResolver.manager.AddFilePathToFilesAndExtensions(importPath)
						mu.Unlock()
					}
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

func isAssetPath(filePath string) bool {
	for _, ext := range assetExtensions {
		if strings.HasSuffix(filePath, "."+ext) {
			return true
		}
	}
	return false
}
