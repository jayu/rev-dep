package main

import (
	"os"
	"path/filepath"
	"strings"
)

var allowedExts = map[string]struct{}{
	".ts":   {},
	".tsx":  {},
	".js":   {},
	".jsx":  {},
	".cjs":  {},
	".mjs":  {},
	".mjsx": {},
}

func hasCorrectExtension(name string) bool {
	ext := filepath.Ext(name)
	_, ok := allowedExts[ext]
	return ok
}

func parseGitIgnore(fileContent string, dirPath string) []GlobMatcher {
	lines := strings.Split(fileContent, "\n")

	sanitizedLines := []string{}

	for _, line := range lines {
		trimmedLined := strings.TrimSpace(line)
		if len(trimmedLined) > 0 && !strings.HasPrefix(trimmedLined, "#") {
			sanitizedLines = append(sanitizedLines, line)
		}

	}

	return CreateGlobMatchers(sanitizedLines, dirPath)
}

func FindAndProcessGitIgnoreFilesUpToRepoRoot(dirPath string) []GlobMatcher {
	return findAndProcessGitIgnoreFilesUpToRepoRoot(dirPath, []GlobMatcher{})
}

func findAndProcessGitIgnoreFilesUpToRepoRoot(dirPath string, globMatchers []GlobMatcher) []GlobMatcher {
	gitIgnoreFilePath := filepath.Join(dirPath, ".gitignore")
	gitignoreFile, gitignoreError := os.ReadFile(gitIgnoreFilePath)

	if gitignoreError == nil {
		globMatchers = append(globMatchers, parseGitIgnore(string(gitignoreFile), dirPath)...)
	}

	gitDir, gitDirReadErr := os.Stat(filepath.Join(dirPath, ".git"))

	if gitDirReadErr == nil && gitDir.IsDir() {
		// found git root
		return globMatchers
	}

	return findAndProcessGitIgnoreFilesUpToRepoRoot(StandardiseDirPath(filepath.Join(dirPath, "../")), globMatchers)
}

func GetFiles(directory string, existingFiles []string, parentGlobMatchers []GlobMatcher) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return existingFiles
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(directory, entryName)

		if entry.IsDir() {
			debug := false
			if !MatchesAnyGlobMatcher(entryFilePath, parentGlobMatchers, debug) {
				// We parse gitignore here to avoid duplicated processing of gitignore from cwd - it will be captured by FindAndProcessGitIgnoreFilesUpToRepoRoot which result should be passed as parentGlobMatchers to root invocation of getFiles

				gitignoreFile, gitignoreError := os.ReadFile(filepath.Join(entryFilePath, ".gitignore"))

				ignoreGlobs := []GlobMatcher{}
				if gitignoreError == nil {
					ignoreGlobs = parseGitIgnore(string(gitignoreFile), entryFilePath)
				}
				if len(ignoreGlobs) > 0 {
					ignoreGlobs = append(parentGlobMatchers, ignoreGlobs...)
				} else {
					ignoreGlobs = parentGlobMatchers
				}

				existingFiles = GetFiles(entryFilePath, existingFiles, ignoreGlobs)
			}
			continue
		}

		if hasCorrectExtension(entryName) && !MatchesAnyGlobMatcher(entryFilePath, parentGlobMatchers, false) {
			// store internal normalized path (forward slashes) for analysis and tests
			existingFiles = append(existingFiles, NormalizePathForInternal(entryFilePath))
		}
	}

	return existingFiles
}

func GetMissingFile(modulePath string) string {
	// modulePath can point to directory -> we have to look for index file
	// or to file without extension -> we have to check all files in directory
	// dirName := filepath.Dir(modulePath)

	// First we check for file with possible extensions
	for ext := range allowedExts {
		filePath := modulePath

		// filePath might be the exact path already
		if !strings.HasSuffix(modulePath, ext) {
			filePath = modulePath + ext
		}

		// modulePath may be internal (forward slashes) or OS-native; try denormalized form for FS checks
		filePathOs := DenormalizePathForOS(filePath)
		info, err := os.Stat(filePathOs)
		if err == nil && !info.IsDir() {
			return NormalizePathForInternal(filePath)
		}
	}

	// Then we check for directory with index.ts file, it has lower precedence if both exists
	for ext := range allowedExts {
		// check directory index; normalize to OS path for Stat
		filePath := modulePath + "/index" + ext
		filePathOs := DenormalizePathForOS(filePath)
		info, err := os.Stat(filePathOs)
		if err == nil && !info.IsDir() {
			return NormalizePathForInternal(filePath)
		}
	}

	return ""
}
