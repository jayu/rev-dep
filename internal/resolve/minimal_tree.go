package resolve

import (
	"os"
	"slices"
	"strings"

	"rev-dep-go/internal/fs"
	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/parser"
	"rev-dep-go/internal/pathutil"
)

func GetMinimalDepsTreeForCwd(cwd string, ignoreTypeImports bool, excludeFiles []string, upfrontFilesList []string, packageJson string, tsconfigJson string, conditionNames []string, followMonorepoPackages model.FollowMonorepoPackagesValue, customAssetExtensions []string) (model.MinimalDependencyTree, []string, *ResolverManager) {
	var files []string

	excludePatterns := globutil.CreateGlobMatchers(excludeFiles, cwd)

	gitIgnoreExcludePatterns := fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd)

	allExcludePatterns := append(excludePatterns, gitIgnoreExcludePatterns...)

	// Resolver is capable of starting with just one file and discover other files as it iterates.
	// While it's faster than looking up for all files upfront, if the file list for entry point is small, it's slower if file list for entry point is long, as resolver is not concurrent
	// To leverage that we have to make resolver concurrent using channels as queue
	if len(upfrontFilesList) == 0 {
		files = fs.GetFiles(cwd, []string{}, allExcludePatterns)
	} else {
		files = upfrontFilesList
	}

	fileImportsArr, _ := parser.ParseImportsFromFiles(files, ignoreTypeImports, model.ParseModeBasic)

	slices.Sort(files)

	skipResolveMissing := false

	fileImportsArr, sortedFiles, resolverManager := ResolveImports(fileImportsArr, files, cwd, ignoreTypeImports, skipResolveMissing, packageJson, tsconfigJson, allExcludePatterns, conditionNames, followMonorepoPackages, customAssetExtensions, model.ParseModeBasic, model.NodeModulesMatchingStrategyCwdResolver)

	minimalTree := model.TransformToMinimalDependencyTreeCustomParser(fileImportsArr)

	return minimalTree, sortedFiles, resolverManager
}

// ResolveEntryPointsFromPatterns expands entry point globs and filters excluded files.
func ResolveEntryPointsFromPatterns(cwd string, entryPoints []string, excludeFiles []string) ([]string, []string) {
	if len(entryPoints) == 0 {
		return []string{}, []string{}
	}

	excludePatterns := globutil.CreateGlobMatchers(excludeFiles, cwd)
	gitIgnoreExcludePatterns := fs.FindAndProcessGitIgnoreFilesUpToRepoRoot(cwd)
	allExcludePatterns := append(excludePatterns, gitIgnoreExcludePatterns...)

	absolutePathToEntryPoints := make([]string, 0, len(entryPoints))
	entryPointsSet := map[string]bool{}
	hasGlob := false

	for _, entryPoint := range entryPoints {
		abs := pathutil.NormalizePathForInternal(pathutil.JoinWithCwd(cwd, entryPoint))
		if fileInfo, err := os.Stat(pathutil.DenormalizePathForOS(abs)); err == nil && !fileInfo.IsDir() {
			if globutil.MatchesAnyGlobMatcher(abs, allExcludePatterns, false) {
				continue
			}
			if !entryPointsSet[abs] {
				entryPointsSet[abs] = true
				absolutePathToEntryPoints = append(absolutePathToEntryPoints, abs)
			}
			continue
		}

		if strings.ContainsAny(entryPoint, "*?[]{}") {
			hasGlob = true
		} else {
			if globutil.MatchesAnyGlobMatcher(abs, allExcludePatterns, false) {
				continue
			}
			if !entryPointsSet[abs] {
				entryPointsSet[abs] = true
				absolutePathToEntryPoints = append(absolutePathToEntryPoints, abs)
			}
		}
	}

	if !hasGlob {
		slices.Sort(absolutePathToEntryPoints)
		return absolutePathToEntryPoints, []string{}
	}

	matchers := globutil.CreateGlobMatchers(entryPoints, cwd)
	allFiles := fs.GetFiles(cwd, []string{}, allExcludePatterns)
	for _, filePath := range allFiles {
		if globutil.MatchesAnyGlobMatcher(filePath, matchers, false) {
			normalized := pathutil.NormalizePathForInternal(filePath)
			if !entryPointsSet[normalized] {
				entryPointsSet[normalized] = true
				absolutePathToEntryPoints = append(absolutePathToEntryPoints, normalized)
			}
		}
	}

	slices.Sort(absolutePathToEntryPoints)
	return absolutePathToEntryPoints, allFiles
}
