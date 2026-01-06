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

type RegExpArrItem struct {
	aliasKey string
	regExp   *regexp.Regexp
}

type TsConfigParsed struct {
	aliases        map[string]string
	aliasesRegexps []RegExpArrItem
}

type PackageJsonImports struct {
	imports        map[string]interface{}
	importsRegexps []RegExpArrItem
	conditionNames []string
}

type ResolvedModuleInfo struct {
	Path string
	Type ResolvedImportType
}

type ModuleResolver struct {
	tsConfigParsed     *TsConfigParsed
	packageJsonImports *PackageJsonImports
	aliasesCache       map[string]ResolvedModuleInfo
	filesAndExtensions *map[string]string
	packageRoot        string
	manager            *ResolverManager
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
	resolvers              map[string]*ModuleResolver
	mu                     sync.RWMutex
	followMonorepoPackages bool
	conditionNames         []string
	rootParams             RootParams
	filesAndExtensions     map[string]string
	muFiles                sync.Mutex
}

type RootParams struct {
	TsConfigContent []byte
	PkgJsonContent  []byte
	SortedFiles     []string
}

func NewResolverManager(monorepoContext *MonorepoContext, followMonorepoPackages bool, conditionNames []string, rootParams RootParams) *ResolverManager {
	rm := &ResolverManager{
		monorepoContext:        monorepoContext,
		resolvers:              make(map[string]*ModuleResolver),
		followMonorepoPackages: followMonorepoPackages,
		conditionNames:         conditionNames,
		rootParams:             rootParams,
		filesAndExtensions:     make(map[string]string),
	}

	for _, filePath := range rootParams.SortedFiles {
		addFilePathToFilesAndExtensions(NormalizePathForInternal(filePath), &rm.filesAndExtensions)
	}

	return rm
}

/*
	TODO this function is weird, get resolver for file should be simple and straightforward
  We need to have thre resolvers created upfront. If there is only one resolver, use it without any checks.
*/

func (rm *ResolverManager) GetResolverForFile(filePath string) *ModuleResolver {
	// 1. Determine package root for filePath
	// If monorepo support is enabled, traversing up to find nearest package.json
	// If not enabled or no package.json found (except root), use root resolver.

	packageRoot := ""

	if rm.followMonorepoPackages && rm.monorepoContext != nil {
		// Traverse up from filePath to find package.json
		// Only consider packages known in monorepo context to avoid scanning unrelated directories?
		// Or just find nearest package.json.
		// Use known packages map for validation or lookup efficiency.

		dir := NormalizePathForInternal(filepath.Dir(filePath))
		for {
			if dir == "." || dir == "/" || dir == "" || len(dir) < len(rm.monorepoContext.WorkspaceRoot) {
				break
			}
			if _, ok := rm.monorepoContext.PackageToPath[filepath.Base(dir)]; ok { // This check is insufficient, key is name not path
				// We have paths in Packages values.
				// Inverting map or iterating reasonable?
				// Better: check if package.json exists, parse it's name, check if valid.
			}
			// Optimization: MonorepoContext already has packages map [name] -> path.
			// We can check if `dir` is a value in that map.
			// Iterating map is slow?
			// Let's rely on finding package.json and checking if it matches a known workspace package.

			// Simpler: Just look for package.json.
			pkgPath := filepath.Join(dir, "package.json")
			if _, err := os.Stat(pkgPath); err == nil {
				// Found a package root
				packageRoot = dir
				break
			}

			dir = filepath.Dir(dir)
			dir = NormalizePathForInternal(dir)
		}
	}

	// Fallback to root if not found or outside known packages
	if packageRoot == "" || (rm.monorepoContext != nil && packageRoot == rm.monorepoContext.WorkspaceRoot) {
		// Use Root Resolver
		// Root resolver key can be special or just empty string?
		// Let's use root workspace path or "ROOT"
		packageRoot = "ROOT"
	}

	rm.mu.RLock()
	if resolver, ok := rm.resolvers[packageRoot]; ok {
		rm.mu.RUnlock()
		return resolver
	}
	rm.mu.RUnlock()

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Double check
	if resolver, ok := rm.resolvers[packageRoot]; ok {
		return resolver
	}

	// Create new resolver
	var resolver *ModuleResolver

	if packageRoot == "ROOT" {
		resolver = NewImportsResolver(rm.rootParams.TsConfigContent, rm.rootParams.PkgJsonContent, rm.conditionNames, rm.rootParams.SortedFiles, rm)
	} else {
		// Load package.json for this package
		// We need to load tsconfig? Monorepo packages likely have their own tsconfig or extend base.
		// For simplicity V1: try to load tsconfig.json in that dir, or empty if fail.

		_, err := rm.monorepoContext.GetPackageConfig(packageRoot)
		var pkgContent []byte
		if err == nil {
			// Re-marshal to bytes or store bytes in cache?
			// Since NewImportsResolver takes bytes, maybe we should have stored bytes.
			// Or read file again.
			pkgContent, _ = os.ReadFile(filepath.Join(packageRoot, "package.json"))
		}

		tsConfigPath := filepath.Join(packageRoot, "tsconfig.json")
		tsConfigContent, _ := ParseTsConfig(tsConfigPath)
		// if err, tsConfigContent is empty/partial, handled by NewImportsResolver

		// Note: sortedFiles passed here is technically "files known so far".
		// But for a new resolver, the filesAndExtensions map starts fresh?
		// Actually, filesAndExtensions is used to check existence of files.
		// It should probably be shared or populated with all files?
		// NewImportsResolver currently builds a map from allFilePaths.
		// Effectively we want to know legal files relative to this package?
		// Or just all files in project.
		// We can pass the same file list.
		resolver = NewImportsResolver(tsConfigContent, pkgContent, rm.conditionNames, rm.rootParams.SortedFiles, rm)
		resolver.packageRoot = packageRoot // Store root for relative calc
	}

	rm.resolvers[packageRoot] = resolver
	return resolver
}

func NewImportsResolver(tsconfigContent []byte, packageJsonContent []byte, conditionNames []string, allFilePaths []string, manager *ResolverManager) *ModuleResolver {
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
		tsConfigParsed.aliases[aliasKey] = aliasValue
		regExp := regexp.MustCompile("^" + strings.Replace(aliasKey, "*", ".+?", 1) + "$")

		tsConfigParsed.aliasesRegexps = append(tsConfigParsed.aliasesRegexps, RegExpArrItem{
			regExp:   regExp,
			aliasKey: aliasKey,
		})
	}

	if hasBaseUrl {
		baseUrlAliasKey := "*"
		baseUrlAliasValue := strings.TrimSuffix(baseUrl, "/") + "/*"
		tsConfigParsed.aliases[baseUrlAliasKey] = baseUrlAliasValue
		regExp := regexp.MustCompile(strings.Replace(baseUrlAliasKey, "*", ".+?", 1))

		tsConfigParsed.aliasesRegexps = append(tsConfigParsed.aliasesRegexps, RegExpArrItem{
			regExp:   regExp,
			aliasKey: baseUrlAliasKey,
		})
	}

	packageJsonImports := &PackageJsonImports{
		imports:        map[string]interface{}{},
		importsRegexps: []RegExpArrItem{},
		conditionNames: conditionNames,
	}

	var rawPackageJson map[string]interface{}
	json.Unmarshal(packageJsonContent, &rawPackageJson)

	if imports, ok := rawPackageJson["imports"]; ok {
		if importsMap, ok := imports.(map[string]interface{}); ok {
			packageJsonImports.imports = importsMap
			for key := range importsMap {
				// imports keys like "#foo" or "#foo/*"
				// regex should match them.
				// Spec says keys starting with #.
				// If key has *, replace with .+?
				pattern := "^" + strings.Replace(key, "*", ".+?", 1) + "$"
				regExp := regexp.MustCompile(pattern)
				packageJsonImports.importsRegexps = append(packageJsonImports.importsRegexps, RegExpArrItem{
					aliasKey: key,
					regExp:   regExp,
				})
			}
			// Sort regexps longest prefix first
			slices.SortFunc(packageJsonImports.importsRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
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

	// TODO when manager can be nil ? It should always exist
	var filesAndExtensions *map[string]string
	if manager != nil {
		filesAndExtensions = &manager.filesAndExtensions
	} else {
		m := make(map[string]string)
		filesAndExtensions = &m
		for _, filePath := range allFilePaths {
			addFilePathToFilesAndExtensions(NormalizePathForInternal(filePath), filesAndExtensions)
		}
	}

	factory := &ModuleResolver{
		tsConfigParsed:     tsConfigParsed,
		packageJsonImports: packageJsonImports,
		aliasesCache:       map[string]ResolvedModuleInfo{},
		filesAndExtensions: filesAndExtensions,
		manager:            manager,
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

func (f *ModuleResolver) AddFilePathToFilesAndExtensions(filePath string) {
	if f.manager != nil {
		f.manager.muFiles.Lock()
		defer f.manager.muFiles.Unlock()
	}
	addFilePathToFilesAndExtensions(filePath, f.filesAndExtensions)
}

func (f *ModuleResolver) getModulePathWithExtension(modulePath string) (path string, err *ResolutionError) {
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

	if f.manager != nil {
		// TODO why we need this?
		f.manager.muFiles.Lock()
		defer f.manager.muFiles.Unlock()
	}
	extension, has := (*f.filesAndExtensions)[modulePath]

	if has {

		return modulePath + extension, nil
	}

	e := FileNotFound

	return modulePath, &e
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

func (f *ModuleResolver) resolvePackageJsonImport(request string, root string) (path string, found bool) {
	if !strings.HasPrefix(request, "#") {
		return "", false
	}

	for _, importRegex := range f.packageJsonImports.importsRegexps {
		if importRegex.regExp.MatchString(request) {
			key := importRegex.aliasKey
			target := f.packageJsonImports.imports[key]

			resolvedTarget := f.resolveCondition(target)

			if resolvedTarget != "" {
				// Replace * if present
				if strings.Contains(key, "*") {
					regexKey := strings.Replace(key, "*", "(.*)", 1)
					re := regexp.MustCompile("^" + regexKey + "$")
					matches := re.FindStringSubmatch(request)
					if len(matches) > 1 {
						wildcardValue := matches[1]
						resolvedTarget = strings.Replace(resolvedTarget, "*", wildcardValue, 1)
					}
				}

				// If result starts with ./, it is relative to package.json (root)
				if strings.HasPrefix(resolvedTarget, "./") {
					return filepath.Join(root, resolvedTarget), true
				}
				// Otherwise it might be external package reference or other import?
				// Node spec says targets must start with ./ for file paths.
				// Or they can be package names.
				return resolvedTarget, true
			}
		}
	}

	return "", false
}

func (f *ModuleResolver) ResolveModule(request string, filePath string, root string) (path string, rtype ResolvedImportType, err *ResolutionError) {
	// fmt.Println("Resolve module")
	// fmt.Println("Request", request)
	// fmt.Println("FilePath", filePath)
	// fmt.Println("Root", root)
	// fmt.Printf("module resolver filesAndExtensions %v\n", f.filesAndExtensions)
	// fmt.Printf("module resolver tsconfig parsed %v \n", f.tsConfigParsed)
	cached, ok := f.aliasesCache[request]

	if ok {
		return cached.Path, cached.Type, nil
	}

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
		relative := alias

		if strings.HasSuffix(aliasKey, "*") {
			aliasKeyPrefix := strings.TrimSuffix(aliasKey, "*")
			relative = strings.Replace(alias, "*", strings.Replace(request, aliasKeyPrefix, "", 1), 1)
		}

		modulePath = filepath.Join(root, relative)
		modulePath = NormalizePathForInternal(modulePath)

		if modulePath == "" {
			fmt.Println("Alias resolved to empty string for request", request)
		}

		actualFilePath, e := f.getModulePathWithExtension(modulePath)

		if e != nil {
			return actualFilePath, UserModule, e
		}

		f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}

		return actualFilePath, UserModule, nil
	}

	if resolvedImport, found := f.resolvePackageJsonImport(request, root); found {
		modulePath = NormalizePathForInternal(resolvedImport)
		actualFilePath, e := f.getModulePathWithExtension(modulePath)
		if e == nil {
			f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: UserModule}
			return actualFilePath, UserModule, nil
		}
		return actualFilePath, UserModule, e
	}

	// Check if it is a workspace package import (Monorepo support)
	// Only if manager is present and monorepo is enabled
	if f.manager != nil && f.manager.followMonorepoPackages && f.manager.monorepoContext != nil {
		pkgName := GetNodeModuleName(request)
		// Check if pkgName is in our monorepo packages
		if pkgPath, ok := f.manager.monorepoContext.PackageToPath[pkgName]; ok {
			// Found a workspace package!

			// NOTE: Validation logic:
			validDep := false
			if consumerConfig, err := f.manager.monorepoContext.GetPackageConfig(root); err == nil {
				// Check dependencies and devDependencies
				// RELAXED: allow any version if the package name is in workspaces
				if _, ok := consumerConfig.Dependencies[pkgName]; ok {
					validDep = true
				} else if _, ok := consumerConfig.DevDependencies[pkgName]; ok {
					validDep = true
				}
			} else if root == f.manager.monorepoContext.WorkspaceRoot || root == "ROOT" || root == "" {
				// If we are at root (or root resolution), allow it if flag is enabled
				validDep = true
			}

			if validDep {
				// Resolve against exports of target package
				targetConfig, err := f.manager.monorepoContext.GetPackageConfig(pkgPath)
				if err == nil {
					// Get exports from targetConfig
					// request is like "@company/common/utils"
					// pkgName is "@company/common"
					// subpath is "./utils" (or "." if exact match)

					subpath := "."
					if len(request) > len(pkgName) {
						subpath = "." + request[len(pkgName):]
					}

					var exportsMap map[string]interface{}
					if targetConfig.Exports != nil {
						if exportsString, ok := targetConfig.Exports.(string); ok {
							exportsMap = map[string]interface{}{
								".": exportsString,
							}
						} else if m, ok := targetConfig.Exports.(map[string]interface{}); ok {
							exportsMap = m
						}
					}

					if exportsMap != nil {
						// Resolve using exports logic
						resolvedExport := f.resolveExports(exportsMap, subpath)
						if resolvedExport != "" {
							// resolvedExport is relative to target package root
							fullPath := filepath.Join(pkgPath, resolvedExport)
							modulePath = NormalizePathForInternal(fullPath)
							actualFilePath, e := f.getModulePathWithExtension(modulePath)
							if e == nil {
								f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: MonorepoModule}
								return actualFilePath, MonorepoModule, nil
							}
							return actualFilePath, MonorepoModule, e
						}
					} else {
						// No exports? Fallback to main/module or default index
						resolvedSubpath := subpath
						if subpath == "." {
							if targetConfig.Module != "" {
								resolvedSubpath = targetConfig.Module
							} else if targetConfig.Main != "" {
								resolvedSubpath = targetConfig.Main
							}
						}

						fullPath := filepath.Join(pkgPath, resolvedSubpath)
						modulePath = NormalizePathForInternal(fullPath)
						actualFilePath, e := f.getModulePathWithExtension(modulePath)
						if e == nil {
							f.aliasesCache[request] = ResolvedModuleInfo{Path: actualFilePath, Type: MonorepoModule}
							return actualFilePath, MonorepoModule, nil
						}
						return actualFilePath, MonorepoModule, e
					}
				}
			}
		}
	}

	// Could not resolve alias
	e := AliasNotResolved
	return "", NotResolvedModule, &e
}

// TODO compare this code with ts alias resolution, can it be simplified?
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
			regexKey := "^" + strings.Replace(key, "*", "(.*)", 1) + "$"
			re := regexp.MustCompile(regexKey)
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

func ResolveImports(fileImportsArr []FileImports, sortedFiles []string, cwd string, ignoreTypeImports bool, skipResolveMissing bool, packageJson string, tsconfigJson string, excludeFilePatterns []GlobMatcher, conditionNames []string, followMonorepoPackages bool) (fileImports []FileImports, adjustedSortedFiles []string, nodeModules map[string]bool) {
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

	nodeModules = GetNodeModulesFromPkgJson(jsonc.ToJSON(pkgJsonContent))

	var monorepoCtx *MonorepoContext
	if followMonorepoPackages {
		monorepoCtx = DetectMonorepo(cwd)
		if monorepoCtx != nil {
			monorepoCtx.FindWorkspacePackages(cwd)
		}
	}

	resolverManager := NewResolverManager(monorepoCtx, followMonorepoPackages, conditionNames, RootParams{
		TsConfigContent: tsconfigContent,
		PkgJsonContent:  pkgJsonContent,
		SortedFiles:     sortedFiles,
	})

	// TODO all resolvers should be created here, not inside resolveSingleFileImports
	// TODO there shold be simple map between filePath part -> resolver

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
				nodeModules,
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

	return filteredFileImportsArr, filteredFiles, nodeModules
}

func resolveSingleFileImports(resolverManager *ResolverManager, missingResolutionFailedAttempts *map[string]bool, discoveredFiles *map[string]bool, fileImportsArr *[]FileImports, sortedFiles *[]string, tsConfigDirOrCwd string, ignoreTypeImports bool, skipResolveMissing bool, idx int, wg *sync.WaitGroup, mu *sync.Mutex, ch_idx chan int, nodeModules map[string]bool, builtInModules map[string]bool, excludeFilePatterns []GlobMatcher) {
	mu.Lock()
	fileImports := (*fileImportsArr)[idx]
	mu.Unlock()
	imports := fileImports.Imports
	filePath := fileImports.FilePath

	fmt.Println("Resolving imports for", filePath)

	importsResolver := resolverManager.GetResolverForFile(filePath)

	currentRoot := tsConfigDirOrCwd
	// TODO I don't like this ambiguity, we should have a single resolver for the root, what is root btw?
	if importsResolver.packageRoot != "" && importsResolver.packageRoot != "ROOT" {
		currentRoot = importsResolver.packageRoot
	}

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

		_, isNodeModule := nodeModules[moduleName]
		importPath, resolvedType, resolutionErr := importsResolver.ResolveModule(imp.Request, filePath, currentRoot)

		if isNodeModule && resolutionErr != nil {
			// Check if it's a followed workspace package
			isFollowedWorkspace := false
			if importsResolver.manager != nil && importsResolver.manager.followMonorepoPackages && importsResolver.manager.monorepoContext != nil {
				name := GetNodeModuleName(imp.Request)
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
								importsResolver.AddFilePathToFilesAndExtensions(missingFilePath)
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
				// fmt.Printf("Likely external dep '%s' -> '%s' in %s\n", imp.Request, importPath, filePath)
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
						importsResolver.AddFilePathToFilesAndExtensions(importPath)
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
