package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/tidwall/jsonc"
)

func GetNodeModulesFromPkgJson(packageJsonContent []byte) map[string]bool {
	packageJsonContent = jsonc.ToJSON(packageJsonContent)

	var rawPackageJson map[string]map[string]string

	err := json.Unmarshal(packageJsonContent, &rawPackageJson)

	if err != nil {
		// fmt.Printf("Failed to parse package json : %s\n", err)
	}

	modules := map[string]bool{}

	dependencies, ok := rawPackageJson["dependencies"]

	if ok {
		for dep := range dependencies {
			modules[dep] = true
		}
	}
	devDependencies, ok2 := rawPackageJson["devDependencies"]

	if ok2 {
		for dep := range devDependencies {
			modules[dep] = true
		}
	}

	return modules
}

func GetNodeModuleName(request string) string {
	splitCount := 2
	if strings.HasPrefix(request, "@") {
		splitCount = 3
	}
	parts := strings.SplitN(request, "/", splitCount)
	return strings.Join(parts[:splitCount-1], "/")
}

func FindNodeModuleBinaries(nodeModules map[string]bool, cwd string) map[string][]string {
	nodeModuleDirs := []string{}
	// Walk up the directory tree from cwd and collect any "node_modules" dirs
	cur := filepath.Clean(cwd)
	result := make(map[string][]string, len(nodeModules))

	for moduleName := range nodeModules {
		result[moduleName] = []string{}
	}

	for {
		nmPath := filepath.Join(cur, "node_modules")
		fileInfo, fileInfoErr := os.Stat(nmPath)
		if fileInfoErr == nil && fileInfo.IsDir() {
			nodeModuleDirs = append(nodeModuleDirs, nmPath)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	for nodeModule := range nodeModules {
		for _, nodeModuleDir := range nodeModuleDirs {
			path := filepath.Join(nodeModuleDir, nodeModule, "package.json")
			fileInfo, fileInfoErr := os.Stat(path)
			if fileInfoErr == nil && !fileInfo.IsDir() {
				fileContent, _ := os.ReadFile(path)
				var pkgJsonSingleBinary map[string]string

				err := json.Unmarshal(fileContent, &pkgJsonSingleBinary)

				if err != nil {
					// fmt.Printf("Failed to parse tsConfig paths : %s\n", err)
				}

				singleBinary, hasSingleBinary := pkgJsonSingleBinary["bin"]
				if hasSingleBinary && len(singleBinary) > 0 {
					result[nodeModule] = append(result[nodeModule], nodeModule)
				}

				var pkgJsonMultipleBinaries map[string]map[string]string

				err = json.Unmarshal(fileContent, &pkgJsonMultipleBinaries)

				if err != nil {
					// fmt.Printf("Failed to parse tsConfig paths : %s\n", err)
				}

				multipleBinaries, hasMultipleBinaries := pkgJsonMultipleBinaries["bin"]

				if hasMultipleBinaries && len(multipleBinaries) > 0 {
					for binaryName := range multipleBinaries {
						result[nodeModule] = append(result[nodeModule], binaryName)
					}
				}

				if (hasSingleBinary && len(singleBinary) > 0) || hasMultipleBinaries && len(multipleBinaries) > 0 {
					// If directory closer to cwd has given node module with binaries, it will be resolved by node, hence we should not lookup in upper directories
					break
				}
			}
		}
	}

	// debug lines removed
	return result
}

func NodeModulesCmd(
	inputCwd string,
	ignoreType bool,
	entryPoints []string,
	countFlag bool,
	listUnused bool,
	listMissing bool,
	groupByModule bool,
	groupByFile bool,
	groupByModuleFilesCount bool,
	groupByEntryPoint bool,
	groupByEntryPointModulesCount bool,
	groupByModuleShowEntryPoints bool,
	groupByModuleEntryPointsCount bool,
	pkgJsonFieldsWithBinaries []string,
	filesWithBinaries []string,
	filesWithModules []string,
	modulesToInclude []string,
	modulesToExclude []string,
	packageJson string,
	tsconfigJson string,
	conditionNames []string,
	followMonorepoPackages FollowMonorepoPackagesValue,
) (string, int) {
	cwd := StandardiseDirPath(inputCwd)
	excludeFiles := []string{}
	absolutePathToEntryPoints, discoveredFiles := ResolveEntryPointsFromPatterns(cwd, entryPoints, excludeFiles)

	shouldIncludeModule := createShouldModuleByIncluded(modulesToInclude, modulesToExclude)

	upfrontFilesList := absolutePathToEntryPoints
	if len(discoveredFiles) > 0 {
		upfrontFilesList = discoveredFiles
	}

	minimalTree, _, resolverManager := GetMinimalDepsTreeForCwd(cwd, ignoreType, excludeFiles, upfrontFilesList, packageJson, tsconfigJson, conditionNames, followMonorepoPackages)

	if len(absolutePathToEntryPoints) == 0 && (groupByEntryPoint || groupByEntryPointModulesCount || groupByModuleShowEntryPoints || groupByModuleEntryPointsCount) {
		absolutePathToEntryPoints = GetEntryPoints(minimalTree, []string{}, []string{}, cwd)
	}

	resolverForCwd := resolverManager.GetResolverForFile(cwd)

	cwdNodeModules := make(map[string]bool, 0)

	if resolverForCwd != nil {
		cwdNodeModules = resolverForCwd.nodeModules
	}

	if listMissing {
		missingResults := GetMissingNodeModulesFromTree(minimalTree, modulesToInclude, modulesToExclude, cwdNodeModules)
		return formatMissingNodeModulesResults(missingResults, cwd, countFlag, groupByModule, groupByFile, groupByModuleFilesCount)
	}

	if listUnused {
		unusedModules := GetUnusedNodeModulesFromTree(
			minimalTree,
			cwdNodeModules,
			cwd,
			pkgJsonFieldsWithBinaries,
			filesWithBinaries,
			filesWithModules,
			packageJson,
			tsconfigJson,
			modulesToInclude,
			modulesToExclude,
		)
		return formatUnusedNodeModulesResults(unusedModules, countFlag)
	}

	usedNodeModules := GetUsedNodeModulesFromTree(minimalTree, cwdNodeModules, cwd, pkgJsonFieldsWithBinaries, filesWithBinaries, filesWithModules, packageJson, tsconfigJson)
	return formatUsedNodeModulesResult(
		usedNodeModules,
		cwd,
		countFlag,
		groupByModule,
		groupByFile,
		groupByModuleFilesCount,
		groupByEntryPoint,
		groupByEntryPointModulesCount,
		groupByModuleShowEntryPoints,
		groupByModuleEntryPointsCount,
		absolutePathToEntryPoints,
		minimalTree,
		shouldIncludeModule,
	)
}

type MissingNodeModuleResult struct {
	ModuleName   string
	ImportedFrom []string
}

func isValidNodeModuleName(name string) bool {
	// There are more restrictions on node module name than starting with dot, but for now we just check against that
	return !strings.HasPrefix(name, ".")
}

func GetUsedNodeModulesFromTree(
	minimalTree MinimalDependencyTree,
	cwdNodeModules map[string]bool,
	cwd string,
	pkgJsonFieldsWithBinaries []string,
	filesWithBinaries []string,
	filesWithModules []string,
	packageJson string,
	tsconfigJson string,
) map[string]map[string]bool {

	usedNodeModules := map[string]map[string]bool{}

	nodeModulesBinariesMap := FindNodeModuleBinaries(cwdNodeModules, cwd)

	for filePath, fileDeps := range minimalTree {
		for _, dependency := range fileDeps {
			if dependency.ResolvedType == NodeModule {
				depId := GetNodeModuleName(dependency.Request)
				setFilePathInNodeModuleFilesMap(&usedNodeModules, depId, filePath)
			}

			if dependency.ResolvedType == MonorepoModule {
				depId := GetNodeModuleName(dependency.Request)
				setFilePathInNodeModuleFilesMap(&usedNodeModules, depId, filePath)
			}

			if dependency.ResolvedType == NotResolvedModule {
				depId := GetNodeModuleName(dependency.Request)
				setFilePathInNodeModuleFilesMap(&usedNodeModules, depId, filePath)
			}
		}
	}

	pkgJsonPath := JoinWithCwd(cwd, "package.json")
	if packageJson != "" {
		pkgJsonPath = JoinWithCwd(cwd, packageJson)
	}
	pkgJsonContent, _ := os.ReadFile(pkgJsonPath)

	var pkgJson map[string]any

	json.Unmarshal(pkgJsonContent, &pkgJson)

	additionalContentToLookUpForBinaries := map[string]string{}

	additionalContentToLookUpForBinaries[pkgJsonPath] = ""

	for _, pkgJsonField := range pkgJsonFieldsWithBinaries {
		// Field can contain name of the binary too
		additionalContentToLookUpForBinaries[pkgJsonPath] += pkgJsonField

		fieldContent, has := pkgJson[pkgJsonField]
		if has {
			// TODO find a better way to stringify this
			fieldContentAsString := fmt.Sprintf("%v", fieldContent)
			if len(fieldContentAsString) > 0 {
				additionalContentToLookUpForBinaries[pkgJsonPath] += " " + fieldContentAsString
			}
		}
	}

	for _, filePath := range filesWithBinaries {
		absoluteFilePath := JoinWithCwd(cwd, filePath)

		fileContent, err := os.ReadFile(absoluteFilePath)
		if err == nil && len(fileContent) > 0 {
			additionalContentToLookUpForBinaries[filePath] = string(fileContent)
		}
	}

	var pkgJsonScripts map[string]map[string]string

	err := json.Unmarshal(pkgJsonContent, &pkgJsonScripts)

	if err != nil {
		// fmt.Printf("Failed to parse tsConfig paths : %s\n", err)
	}

	scripts, hasScripts := pkgJsonScripts["scripts"]

	if (hasScripts && len(scripts) > 0) || len(additionalContentToLookUpForBinaries) > 0 {
		for nodeModule, binaries := range nodeModulesBinariesMap {
			if len(binaries) > 0 {
				isUsed := false
				for ib := 0; ib < len(binaries) && !isUsed; ib++ {
					binary := binaries[ib]

					if hasScripts && len(scripts) > 0 {
						for _, script := range scripts {
							if strings.Contains(script, binary) {
								setFilePathInNodeModuleFilesMap(&usedNodeModules, nodeModule, pkgJsonPath)
								isUsed = true
								break
							}
						}
					}
					if !isUsed {
						for filePath, content := range additionalContentToLookUpForBinaries {
							if strings.Contains(content, binary) {
								setFilePathInNodeModuleFilesMap(&usedNodeModules, nodeModule, filePath)
								isUsed = true
								break
							}
						}
					}
				}
			}
		}
	}

	tsconfigPath := JoinWithCwd(cwd, "tsconfig.json")
	if tsconfigJson != "" {
		tsconfigPath = JoinWithCwd(cwd, tsconfigJson)
	}

	// Use ParseTsConfig which reads and resolves "extends" chains. If parsing
	// fails, treat as absent tsconfig (don't mark any types).
	if merged, parseErr := ParseTsConfig(tsconfigPath); parseErr == nil {
		// First parse as generic interface to handle the mixed structure
		var tsconfigGeneric map[string]interface{}
		if unmarshalErr := json.Unmarshal(merged, &tsconfigGeneric); unmarshalErr == nil && tsconfigGeneric != nil {
			if co, ok := tsconfigGeneric["compilerOptions"]; ok {
				if compilerOptions, ok2 := co.(map[string]interface{}); ok2 {
					if typesArr, ok3 := compilerOptions["types"]; ok3 {
						// typesArr should be []interface{}, convert to []string
						if typesSlice, ok4 := typesArr.([]interface{}); ok4 {
							for _, typesModule := range typesSlice {
								if typesModuleStr, ok5 := typesModule.(string); ok5 {
									nodeModuleName := "@types/" + typesModuleStr
									setFilePathInNodeModuleFilesMap(&usedNodeModules, nodeModuleName, tsconfigPath)
								}
							}
						}
					}
				}
			}
		}
	}

	additionalContentToLookUpForNodeModules := map[string]string{}
	// Do NOT include full package.json content for module name lookups —
	// package.json lists dependencies which would incorrectly mark packages as "used".
	// Only include additional files explicitly requested via `filesWithModules`.

	for _, filePath := range filesWithModules {
		absoluteFilePath := JoinWithCwd(cwd, filePath)

		fileContent, err := os.ReadFile(absoluteFilePath)
		if err == nil && len(fileContent) > 0 {
			additionalContentToLookUpForNodeModules[filePath] = string(fileContent)
		}
	}

	for moduleName := range cwdNodeModules {
		for filePath, additionalContent := range additionalContentToLookUpForNodeModules {
			if strings.Contains(additionalContent, moduleName) {
				setFilePathInNodeModuleFilesMap(&usedNodeModules, moduleName, filePath)
			}
		}
	}

	return usedNodeModules
}

// GetUnusedNodeModulesFromTree returns a list of unused node modules from a pre-built dependency tree
func GetUnusedNodeModulesFromTree(
	minimalTree MinimalDependencyTree,
	cwdNodeModules map[string]bool,
	cwd string,
	pkgJsonFieldsWithBinaries []string,
	filesWithBinaries []string,
	filesWithModules []string,
	packageJson string,
	tsconfigJson string,
	modulesToInclude []string,
	modulesToExclude []string,
) []string {
	shouldIncludeModule := createShouldModuleByIncluded(modulesToInclude, modulesToExclude)

	usedNodeModules := GetUsedNodeModulesFromTree(
		minimalTree,
		cwdNodeModules,
		cwd,
		pkgJsonFieldsWithBinaries,
		filesWithBinaries,
		filesWithModules,
		packageJson,
		tsconfigJson,
	)

	unused := []string{}

	for moduleName := range cwdNodeModules {
		_, has := usedNodeModules[moduleName]
		_, hasTypes := usedNodeModules[strings.Replace(moduleName, "@types/", "", 1)]

		if !has && !hasTypes && shouldIncludeModule(moduleName) {
			unused = append(unused, GetNodeModuleName(moduleName))
		}
	}

	slices.Sort(unused)
	return unused
}

// GetMissingNodeModulesFromTree returns structured results for missing node modules from a pre-built dependency tree
func GetMissingNodeModulesFromTree(
	minimalTree MinimalDependencyTree,
	modulesToInclude []string,
	modulesToExclude []string,
	workingDirNodeModules map[string]bool,
) []MissingNodeModuleResult {
	shouldIncludeModule := createShouldModuleByIncluded(modulesToInclude, modulesToExclude)
	unresolved := map[string]map[string]bool{}

	for filePath, fileDeps := range minimalTree {
		for _, dependency := range fileDeps {
			// If following monorepo packages is enabled, files in minimal tree might not belong to the cwd.
			// During resolution, node modules are looked up by package.json that belongs to the file location
			// To capture missing modules correctly (let's say for `app` that imports `shared` package), meaning
			// Capture modules declared in `shared` package.json, used by files from `shared`, but bundled by app
			//
			if dependency.ResolvedType == NotResolvedModule || dependency.ResolvedType == NodeModule {
				moduleName := GetNodeModuleName(dependency.Request)
				if _, exists := workingDirNodeModules[moduleName]; !exists {
					setFilePathInNodeModuleFilesMap(&unresolved, moduleName, filePath)
				}
			}
		}
	}

	results := []MissingNodeModuleResult{}

	for nodeModule, importedFromFiles := range unresolved {
		if shouldIncludeModule(nodeModule) && isValidNodeModuleName(nodeModule) {
			importedFrom := make([]string, 0, len(importedFromFiles))
			for file := range importedFromFiles {
				importedFrom = append(importedFrom, file)
			}
			slices.Sort(importedFrom)

			results = append(results, MissingNodeModuleResult{
				ModuleName:   nodeModule,
				ImportedFrom: importedFrom,
			})
		}
	}

	slices.SortFunc(results, func(a, b MissingNodeModuleResult) int {
		return strings.Compare(a.ModuleName, b.ModuleName)
	})

	return results
}

func setFilePathInNodeModuleFilesMap(nodeModuleFilesMap *map[string]map[string]bool, moduleName string, filePath string) {
	// normalize stored file path to internal forward-slash form
	key := NormalizePathForInternal(filePath)
	_, has := (*nodeModuleFilesMap)[moduleName]
	if has {
		(*nodeModuleFilesMap)[moduleName][key] = true
	} else {
		(*nodeModuleFilesMap)[moduleName] = map[string]bool{key: true}
	}
}

func createShouldModuleByIncluded(modulesToInclude []string, modulesToExclude []string) func(moduleName string) bool {
	// Pre-compile include patterns
	includeGlobs := make([]glob.Glob, len(modulesToInclude))
	for i, pattern := range modulesToInclude {
		includeGlobs[i] = glob.MustCompile(pattern)
	}

	// Pre-compile exclude patterns
	excludeGlobs := make([]glob.Glob, len(modulesToExclude))
	for i, pattern := range modulesToExclude {
		excludeGlobs[i] = glob.MustCompile(pattern)
	}

	return func(moduleName string) bool {
		// Check exclusions first
		for _, g := range excludeGlobs {
			if g.Match(moduleName) {
				return false
			}
		}

		// If no include patterns specified, include everything that's not excluded
		if len(includeGlobs) == 0 {
			return true
		}

		// Check inclusions
		for _, g := range includeGlobs {
			if g.Match(moduleName) {
				return true
			}
		}

		return false
	}
}

type PackageInfo struct {
	Name     string
	FilePath string
	Version  string
}

func ParsePackageJson(filePath string, cwd string, ch chan PackageInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	content, err := os.ReadFile(filePath)
	if err == nil {
		var pkgJson map[string]string

		err = json.Unmarshal(content, &pkgJson)

		name, hasName := pkgJson["name"]
		version, hasVersion := pkgJson["version"]

		if hasName && hasVersion {
			// filePath and cwd are OS-native here
			ch <- PackageInfo{
				Name:     name,
				Version:  version,
				FilePath: strings.Replace(filePath, cwd, "", 1),
			}
		}
	}
}

func formatUsedNodeModulesResult(
	usedNodeModules map[string]map[string]bool,
	cwd string,
	countFlag bool,
	groupByModule bool,
	groupByFile bool,
	groupByModuleFilesCount bool,
	groupByEntryPoint bool,
	groupByEntryPointModulesCount bool,
	groupByModuleShowEntryPoints bool,
	groupByModuleEntryPointsCount bool,
	absolutePathToEntryPoints []string,
	minimalTree MinimalDependencyTree,
	shouldIncludeModule func(moduleName string) bool,
) (string, int) {
	usedNodeModulesArr := make([]string, 0, len(usedNodeModules))

	for depName := range usedNodeModules {
		if shouldIncludeModule(depName) && isValidNodeModuleName(depName) {
			usedNodeModulesArr = append(usedNodeModulesArr, GetNodeModuleName(depName))
		}
	}

	usedNodeModulesCount := len(usedNodeModulesArr)
	result := ""

	if countFlag {
		result += fmt.Sprintln(len(usedNodeModulesArr))
		return result, usedNodeModulesCount
	}

	slices.Sort(usedNodeModulesArr)

	if groupByModule {
		result += getGroupByModuleResult(usedNodeModulesArr, usedNodeModules, cwd)
	} else if groupByFile {
		result += getGroupByFileResult(usedNodeModulesArr, usedNodeModules, cwd)
	} else if groupByModuleFilesCount {
		result += getGroupByModuleFilesCountResult(usedNodeModulesArr, usedNodeModules)
	} else if groupByEntryPoint {
		result += getGroupByEntryPointResult(minimalTree, absolutePathToEntryPoints, usedNodeModules, cwd, shouldIncludeModule)
	} else if groupByEntryPointModulesCount {
		result += getGroupByEntryPointModulesCountResult(minimalTree, absolutePathToEntryPoints, usedNodeModules, cwd, shouldIncludeModule)
	} else if groupByModuleShowEntryPoints {
		result += getGroupByModuleEntryPointsResult(minimalTree, absolutePathToEntryPoints, usedNodeModules, cwd, shouldIncludeModule)
	} else if groupByModuleEntryPointsCount {
		result += getGroupByModuleEntryPointsCountResult(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	} else {
		result += fmt.Sprintln(strings.Join(usedNodeModulesArr, "\n"))
	}

	return result, usedNodeModulesCount
}

// formatMissingNodeModulesResults formats MissingNodeModuleResult into the existing string output format
func formatMissingNodeModulesResults(results []MissingNodeModuleResult, cwd string, countFlag bool, groupByModule bool, groupByFile bool, groupByModuleFilesCount bool) (string, int) {
	if countFlag {
		return fmt.Sprintln(len(results)), len(results)
	}

	// normalize cwd to internal form (analysis uses internal forward-slash paths)
	cwdInternal := NormalizePathForInternal(cwd)

	if groupByModule {
		result := ""
		for _, missing := range results {
			result += fmt.Sprintln("\n", missing.ModuleName)
			slices.Sort(missing.ImportedFrom)
			for _, file := range missing.ImportedFrom {
				cleaned := strings.Replace(file, cwdInternal, "", 1)
				cleaned = strings.TrimPrefix(cleaned, "/")
				result += fmt.Sprintln("    ➞", cleaned)
			}
			result += fmt.Sprintln()
		}
		return result, len(results)
	}

	if groupByFile {
		fileToModules := map[string][]string{}
		for _, missing := range results {
			for _, file := range missing.ImportedFrom {
				fileToModules[file] = append(fileToModules[file], missing.ModuleName)
			}
		}

		result := ""
		// Sort files for consistent output
		sortedFiles := make([]string, 0, len(fileToModules))
		for file := range fileToModules {
			sortedFiles = append(sortedFiles, file)
		}
		slices.Sort(sortedFiles)

		for _, file := range sortedFiles {
			modules := fileToModules[file]
			cleaned := strings.Replace(file, cwdInternal, "", 1)
			cleaned = strings.TrimPrefix(cleaned, "/")
			result += fmt.Sprintln("\n", cleaned)
			slices.Sort(modules)
			for _, module := range modules {
				result += fmt.Sprintln("    ➞", module)
			}
			result += fmt.Sprintln()
		}
		return result, len(results)
	}

	if groupByModuleFilesCount {
		result := ""
		// Sort by module name for consistent output
		slices.SortFunc(results, func(a, b MissingNodeModuleResult) int {
			return strings.Compare(a.ModuleName, b.ModuleName)
		})
		for _, missing := range results {
			result += fmt.Sprintf("%s (%d files)\n", missing.ModuleName, len(missing.ImportedFrom))
		}
		return result, len(results)
	}

	// Default format: just module names
	moduleNames := make([]string, 0, len(results))
	for _, missing := range results {
		moduleNames = append(moduleNames, missing.ModuleName)
	}
	return fmt.Sprintln(strings.Join(moduleNames, "\n")), len(results)
}

// formatUnusedNodeModulesResults formats unused modules into the existing string output format
func formatUnusedNodeModulesResults(unusedModules []string, countFlag bool) (string, int) {
	if countFlag {
		return fmt.Sprintln(len(unusedModules)), len(unusedModules)
	}
	return fmt.Sprintln(strings.Join(unusedModules, "\n")), len(unusedModules)
}

func getGroupByFileResult(modulesArr []string, modulesFilesMap map[string]map[string]bool, cwd string) string {
	result := ""
	moduleByFile := map[string][]string{}
	for _, moduleName := range modulesArr {
		for filePath := range modulesFilesMap[moduleName] {
			current, isInitialized := moduleByFile[filePath]
			if isInitialized {
				moduleByFile[filePath] = append(current, moduleName)
			} else {
				moduleByFile[filePath] = []string{moduleName}
			}
		}
	}

	usedByFileSorted := GetSortedMap(moduleByFile)

	// normalize cwd to internal form (analysis uses internal forward-slash paths)
	cwdInternal := NormalizePathForInternal(cwd)

	for _, kv := range usedByFileSorted {
		filePath := kv.k
		moduleNames := kv.v
		cleaned := strings.Replace(filePath, cwdInternal, "", 1)
		cleaned = strings.TrimPrefix(cleaned, "/")
		result += fmt.Sprintln("\n", cleaned)
		slices.Sort(moduleNames)
		for _, moduleName := range moduleNames {
			result += fmt.Sprintln("    ➞", moduleName)
		}
		result += fmt.Sprintln()
	}
	return result
}

func getGroupByModuleResult(modulesArr []string, modulesFilesMap map[string]map[string]bool, cwd string) string {
	result := ""
	for _, moduleName := range modulesArr {
		result += fmt.Sprintln("\n", moduleName)

		filesPaths := make([]string, 0, len(modulesFilesMap[moduleName]))

		for filePath := range modulesFilesMap[moduleName] {
			filesPaths = append(filesPaths, filePath)
		}

		slices.Sort(filesPaths)

		// normalize cwd to internal form
		cwdInternal := NormalizePathForInternal(cwd)
		for _, filePath := range filesPaths {
			cleaned := strings.Replace(filePath, cwdInternal, "", 1)
			cleaned = strings.TrimPrefix(cleaned, "/")
			result += fmt.Sprintln("    ➞", cleaned)
		}
		result += fmt.Sprintln()
	}
	return result
}

func getGroupByModuleFilesCountResult(modulesArr []string, modulesFilesMap map[string]map[string]bool) string {
	result := ""
	for _, moduleName := range modulesArr {
		filesCount := len(modulesFilesMap[moduleName])
		result += fmt.Sprintf("%s (%d files)\n", moduleName, filesCount)
	}
	return result
}

func getNodeModulesByEntryPoint(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	shouldIncludeModule func(moduleName string) bool,
) map[string]map[string]bool {
	grouped := map[string]map[string]bool{}

	// Group-by-entry-point output should be based on explicit entry points only.
	if len(absolutePathToEntryPoints) == 0 {
		return grouped
	}

	entryPoints := make([]string, 0, len(absolutePathToEntryPoints))
	entryPointsSet := map[string]bool{}
	for _, entryPointRaw := range absolutePathToEntryPoints {
		entryPoint := NormalizePathForInternal(entryPointRaw)
		if _, has := minimalTree[entryPoint]; has {
			entryPoints = append(entryPoints, entryPoint)
			entryPointsSet[entryPoint] = true
			grouped[entryPoint] = map[string]bool{}
		}
	}

	if len(entryPoints) == 0 {
		return grouped
	}

	graph := buildDepsGraphForMultiple(minimalTree, entryPoints, nil, false)

	moduleToFiles := map[string]map[string]bool{}
	uniqueFiles := map[string]bool{}
	for moduleName, filesSet := range usedNodeModules {
		if !shouldIncludeModule(moduleName) || !isValidNodeModuleName(moduleName) {
			continue
		}
		moduleToFiles[moduleName] = filesSet
		for filePath := range filesSet {
			uniqueFiles[filePath] = true
		}
	}

	allFiles := make([]string, 0, len(uniqueFiles))
	for filePath := range uniqueFiles {
		allFiles = append(allFiles, filePath)
	}

	fileToEntryPoints := map[string]map[string]bool{}
	if len(allFiles) > 0 {
		workers := runtime.NumCPU()
		if workers < 1 {
			workers = 1
		}
		if workers > len(allFiles) {
			workers = len(allFiles)
		}

		chFiles := make(chan string, len(allFiles))
		var wg sync.WaitGroup
		var mu sync.Mutex

		resolveEntryPointsForFile := func(filePath string) map[string]bool {
			res := map[string]bool{}
			start := graph.Vertices[filePath]
			if start == nil {
				return res
			}

			visited := map[string]bool{}
			stack := []string{filePath}

			for len(stack) > 0 {
				last := len(stack) - 1
				nodePath := stack[last]
				stack = stack[:last]

				if visited[nodePath] {
					continue
				}
				visited[nodePath] = true

				vertex := graph.Vertices[nodePath]
				if vertex == nil {
					continue
				}

				if len(vertex.Parents) == 0 {
					if entryPointsSet[nodePath] {
						res[nodePath] = true
					}
					continue
				}

				for _, parent := range vertex.Parents {
					stack = append(stack, parent)
				}
			}

			return res
		}

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for filePath := range chFiles {
					entryPointsForFile := resolveEntryPointsForFile(filePath)
					mu.Lock()
					fileToEntryPoints[filePath] = entryPointsForFile
					mu.Unlock()
				}
			}()
		}

		for _, filePath := range allFiles {
			chFiles <- filePath
		}
		close(chFiles)
		wg.Wait()
	}

	for moduleName, filesSet := range moduleToFiles {
		for filePath := range filesSet {
			entryPointsForFile := fileToEntryPoints[filePath]
			for entryPoint := range entryPointsForFile {
				if grouped[entryPoint] == nil {
					grouped[entryPoint] = map[string]bool{}
				}
				grouped[entryPoint][moduleName] = true
			}
		}
	}

	return grouped
}

func getGroupByEntryPointResult(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	cwd string,
	shouldIncludeModule func(moduleName string) bool,
) string {
	usedByEntryPoint := getNodeModulesByEntryPoint(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	var b strings.Builder

	entryPoints := make([]string, 0, len(usedByEntryPoint))
	for entryPoint := range usedByEntryPoint {
		entryPoints = append(entryPoints, entryPoint)
	}
	slices.Sort(entryPoints)

	cwdInternal := NormalizePathForInternal(cwd)

	for _, entryPoint := range entryPoints {
		modulesSet := usedByEntryPoint[entryPoint]
		modules := make([]string, 0, len(modulesSet))
		for moduleName := range modulesSet {
			modules = append(modules, moduleName)
		}
		slices.Sort(modules)

		cleanedEntryPoint := strings.Replace(entryPoint, cwdInternal, "", 1)
		cleanedEntryPoint = strings.TrimPrefix(cleanedEntryPoint, "/")
		b.WriteString("\n ")
		b.WriteString(cleanedEntryPoint)
		b.WriteString("\n")

		for _, moduleName := range modules {
			b.WriteString("    ➞ ")
			b.WriteString(moduleName)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func getGroupByEntryPointModulesCountResult(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	cwd string,
	shouldIncludeModule func(moduleName string) bool,
) string {
	usedByEntryPoint := getNodeModulesByEntryPoint(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	var b strings.Builder

	entryPoints := make([]string, 0, len(usedByEntryPoint))
	for entryPoint := range usedByEntryPoint {
		entryPoints = append(entryPoints, entryPoint)
	}
	slices.Sort(entryPoints)

	maxPathLen := 0
	cleanedPaths := make(map[string]string, len(entryPoints))
	cwdInternal := NormalizePathForInternal(cwd)
	for _, entryPoint := range entryPoints {
		cleanedEntryPoint := strings.Replace(entryPoint, cwdInternal, "", 1)
		cleanedEntryPoint = strings.TrimPrefix(cleanedEntryPoint, "/")
		cleanedPaths[entryPoint] = cleanedEntryPoint
		if len(cleanedEntryPoint) > maxPathLen {
			maxPathLen = len(cleanedEntryPoint)
		}
	}

	for _, entryPoint := range entryPoints {
		b.WriteString(fmt.Sprintf("%-*s %d\n", maxPathLen, cleanedPaths[entryPoint], len(usedByEntryPoint[entryPoint])))
	}

	return b.String()
}

func getEntryPointsByNodeModules(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	shouldIncludeModule func(moduleName string) bool,
) map[string]map[string]bool {
	modulesByEntryPoint := getNodeModulesByEntryPoint(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	entryPointsByModule := map[string]map[string]bool{}

	for entryPoint, modulesSet := range modulesByEntryPoint {
		for moduleName := range modulesSet {
			if entryPointsByModule[moduleName] == nil {
				entryPointsByModule[moduleName] = map[string]bool{}
			}
			entryPointsByModule[moduleName][entryPoint] = true
		}
	}

	return entryPointsByModule
}

func getGroupByModuleEntryPointsResult(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	cwd string,
	shouldIncludeModule func(moduleName string) bool,
) string {
	entryPointsByModule := getEntryPointsByNodeModules(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	modules := make([]string, 0, len(entryPointsByModule))
	for moduleName := range entryPointsByModule {
		modules = append(modules, moduleName)
	}
	slices.Sort(modules)

	cwdInternal := NormalizePathForInternal(cwd)
	var b strings.Builder

	for _, moduleName := range modules {
		b.WriteString("\n ")
		b.WriteString(moduleName)
		b.WriteString("\n")

		entryPoints := make([]string, 0, len(entryPointsByModule[moduleName]))
		for entryPoint := range entryPointsByModule[moduleName] {
			cleanedEntryPoint := strings.Replace(entryPoint, cwdInternal, "", 1)
			cleanedEntryPoint = strings.TrimPrefix(cleanedEntryPoint, "/")
			entryPoints = append(entryPoints, cleanedEntryPoint)
		}
		slices.Sort(entryPoints)

		for _, entryPoint := range entryPoints {
			b.WriteString("    ➞ ")
			b.WriteString(entryPoint)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func getGroupByModuleEntryPointsCountResult(
	minimalTree MinimalDependencyTree,
	absolutePathToEntryPoints []string,
	usedNodeModules map[string]map[string]bool,
	shouldIncludeModule func(moduleName string) bool,
) string {
	entryPointsByModule := getEntryPointsByNodeModules(minimalTree, absolutePathToEntryPoints, usedNodeModules, shouldIncludeModule)
	modules := make([]string, 0, len(entryPointsByModule))
	for moduleName := range entryPointsByModule {
		modules = append(modules, moduleName)
	}
	slices.Sort(modules)

	var b strings.Builder
	maxModuleLen := 0
	for _, moduleName := range modules {
		if len(moduleName) > maxModuleLen {
			maxModuleLen = len(moduleName)
		}
	}

	for _, moduleName := range modules {
		b.WriteString(fmt.Sprintf("%-*s %d\n", maxModuleLen, moduleName, len(entryPointsByModule[moduleName])))
	}
	return b.String()
}
