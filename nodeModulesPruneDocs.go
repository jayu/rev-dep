package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var defaultPruneNodeModulesPatterns = []string{"LICENSE", "README.md", "docs/**"}

func mergePrunePatterns(patterns []string, useDefaults bool) []string {
	merged := make([]string, 0, len(defaultPruneNodeModulesPatterns)+len(patterns))
	seen := make(map[string]bool)

	add := func(pattern string) {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || seen[pattern] {
			return
		}
		seen[pattern] = true
		merged = append(merged, pattern)
	}

	if useDefaults {
		for _, pattern := range defaultPruneNodeModulesPatterns {
			add(pattern)
		}
	}
	for _, pattern := range patterns {
		add(pattern)
	}

	return merged
}

func getInstalledModulePackageDirs(cwd string) []string {
	modules, _ := GetInstalledModules(cwd, []string{}, []string{})
	dirsSet := make(map[string]bool)

	for _, installations := range modules {
		for _, installation := range installations {
			packageFilePath := installation.FilePath
			suffix := string(os.PathSeparator) + "package.json"
			if !strings.HasSuffix(packageFilePath, suffix) {
				continue
			}

			relativePackageDir := strings.TrimSuffix(packageFilePath, suffix)
			relativePackageDir = strings.TrimPrefix(relativePackageDir, string(os.PathSeparator))
			if relativePackageDir == "" {
				continue
			}
			dirsSet[filepath.Join(cwd, relativePackageDir)] = true
		}
	}

	result := make([]string, 0, len(dirsSet))
	for dir := range dirsSet {
		result = append(result, dir)
	}
	slices.Sort(result)

	return result
}

func NodeModulesPruneDocsCmd(cwd string, patterns []string, useDefaults bool) (string, error) {
	patternsToUse := mergePrunePatterns(patterns, useDefaults)
	if len(patternsToUse) == 0 {
		return "", errors.New("no prune patterns specified; use --patterns or --defaults")
	}

	globMatchers := CreateGlobMatchers(patternsToUse, "")
	packageDirs := getInstalledModulePackageDirs(cwd)

	removedCount := 0
	errorsCount := 0

	for _, packageDir := range packageDirs {
		walkErr := filepath.WalkDir(packageDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				errorsCount++
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}

			relativeToPackage, relErr := filepath.Rel(packageDir, path)
			if relErr != nil {
				errorsCount++
				return nil
			}

			if !MatchesAnyGlobMatcher(relativeToPackage, globMatchers, false) {
				return nil
			}

			if removeErr := os.Remove(path); removeErr != nil {
				errorsCount++
				return nil
			}
			removedCount++
			return nil
		})

		if walkErr != nil {
			errorsCount++
		}
	}

	result := fmt.Sprintf(
		"Pruned files in node_modules packages\nPatterns: %s\nPackages scanned: %d\nFiles removed: %d\nErrors: %d\n",
		strings.Join(patternsToUse, ", "),
		len(packageDirs),
		removedCount,
		errorsCount,
	)

	return result, nil
}
