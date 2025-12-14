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

type ModuleResolver struct {
	tsConfigParsed     *TsConfigParsed
	aliasesCache       map[string]string
	filesAndExtensions *map[string]string
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

func NewImportsResolver(tsconfigContent []byte, allFilePaths []string) *ModuleResolver {
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

	// Sort regexps as they are matched starting from longest matching prefix
	slices.SortFunc(tsConfigParsed.aliasesRegexps, func(itemA RegExpArrItem, itemB RegExpArrItem) int {
		keyAMatchingPrefix := strings.Replace(itemA.aliasKey, "*", "", 1)
		keyBMatchingPrefix := strings.Replace(itemB.aliasKey, "*", "", 1)

		return len(keyBMatchingPrefix) - len(keyAMatchingPrefix)
	})

	filesAndExtensions := &map[string]string{}

	for _, filePath := range allFilePaths {
		addFilePathToFilesAndExtensions(NormalizePathForInternal(filePath), filesAndExtensions)
	}

	factory := &ModuleResolver{
		tsConfigParsed:     tsConfigParsed,
		aliasesCache:       map[string]string{},
		filesAndExtensions: filesAndExtensions,
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

	extension, has := (*f.filesAndExtensions)[modulePath]

	if has {

		return modulePath + extension, nil
	}

	e := FileNotFound

	return modulePath, &e
}

func (f *ModuleResolver) ResolveModule(request string, filePath string, root string) (path string, err *ResolutionError) {
	// fmt.Println("Resolve module")
	// fmt.Println("Request", request)
	// fmt.Println("FilePath", filePath)
	// fmt.Println("Root", root)
	// fmt.Printf("module resolver filesAndExtensions %v\n", f.filesAndExtensions)
	// fmt.Printf("module resolver tsconfig parsed %v \n", f.tsConfigParsed)
	cached, ok := f.aliasesCache[request]

	if ok {
		return cached, nil
	}

	var modulePath string
	relativeFileName, _ := filepath.Rel(root, filePath)

	// Relative path
	if strings.HasPrefix(request, ".") {
		modulePath = filepath.Join(root, relativeFileName, "../"+request)

		cleanedModulePath := filepath.Clean(modulePath)
		// Normalize to internal forward-slash form for matching against files map
		modulePathInternal := NormalizePathForInternal(cleanedModulePath)

		return f.getModulePathWithExtension(modulePathInternal)
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
			return actualFilePath, e
		}

		f.aliasesCache[request] = actualFilePath

		return actualFilePath, nil

	}

	// Could not resolve alias
	e := AliasNotResolved
	return "", &e
}

func ResolveImports(fileImportsArr []FileImports, sortedFiles []string, cwd string, ignoreTypeImports bool, skipResolveMissing bool, packageJson string, tsconfigJson string, excludeFilePatterns []GlobMatcher) (fileImports []FileImports, adjustedSortedFiles []string, nodeModules map[string]bool) {
	tsConfigPath := filepath.Join(cwd, tsconfigJson)
	pkgJsonPath := filepath.Join(cwd, packageJson)

	if tsconfigJson == "" {
		tsConfigPath = filepath.Join(cwd, "tsconfig.json")
	}

	tsConfigDir := filepath.Dir(tsConfigPath)

	if packageJson == "" {
		pkgJsonPath = filepath.Join(cwd, "package.json")
	}

	// Let ParseTsConfig read and resolve the tsconfig file. If user provided
	// an explicit tsconfig path and parsing fails, exit with error to match
	// previous behaviour. Otherwise continue with empty tsconfig content.
	tsconfigContent := []byte("")
	if merged, err := ParseTsConfig(tsConfigPath); err == nil {
		tsconfigContent = merged
	} else {
		fmt.Fprintf(os.Stderr, "⚠️  Error when parsing tsconfig: %v\n", err)
		if tsconfigJson != "" {
			os.Exit(1)
		}
	}

	pkgJsonContent, err := os.ReadFile(pkgJsonPath)

	if err != nil {
		pkgJsonContent = []byte("")
	}

	nodeModules = GetNodeModulesFromPkgJson(jsonc.ToJSON(pkgJsonContent))

	importsResolver := NewImportsResolver(tsconfigContent, sortedFiles)

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
				importsResolver,
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

func resolveSingleFileImports(importsResolver *ModuleResolver, missingResolutionFailedAttempts *map[string]bool, discoveredFiles *map[string]bool, fileImportsArr *[]FileImports, sortedFiles *[]string, tsConfigDirOrCwd string, ignoreTypeImports bool, skipResolveMissing bool, idx int, wg *sync.WaitGroup, mu *sync.Mutex, ch_idx chan int, nodeModules map[string]bool, builtInModules map[string]bool) {
	mu.Lock()
	fileImports := (*fileImportsArr)[idx]
	mu.Unlock()
	imports := fileImports.Imports
	filePath := fileImports.FilePath
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
		importPath, resolutionErr := importsResolver.ResolveModule(imp.Request, filePath, tsConfigDirOrCwd)

		if isNodeModule && resolutionErr != nil {
			fileImports.Imports[impIdx].PathOrName = moduleName
			fileImports.Imports[impIdx].ResolvedType = NodeModule
			mu.Unlock()
			continue
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
						missingFileContent, err := os.ReadFile(DenormalizePathForOS(missingFilePath))
						if err == nil {
							mu.Lock()
							imports[impIdx].PathOrName = missingFilePath
							imports[impIdx].ResolvedType = UserModule
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
							// In case we encounter missing import, we don't know if it was node_module or path to user file
							// For comparison convenience we store both cases as paths prefixed with cwd
							(*missingResolutionFailedAttempts)[importPath] = true
							mu.Unlock()
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
						}

					} else {
						mu.Lock()
						// In case we encounter missing import, we don't know if it was node_module or path to user file
						// For comparison convenience we store both cases as paths prefixed with cwd
						(*missingResolutionFailedAttempts)[importPath] = true
						mu.Unlock()
					}
				}

			}

			if *resolutionErr == AliasNotResolved {
				// Likely external dependency, TODO handle that later
				// fmt.Printf("Likely external dep '%s' -> '%s' in %s\n", imp.Request, importPath, filePath)
				continue
			}

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
			imports[impIdx].ResolvedType = UserModule
			mu.Unlock()
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
