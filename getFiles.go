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
			existingFiles = append(existingFiles, entryFilePath)
		}
	}

	return existingFiles
}

func GetMissingFile(modulePath string) string {
	// modulePath can point to directory -> we have to look for index file
	// or to file without extension -> we have to check all files in directory
	// dirName := filepath.Dir(modulePath)

	// First we check for file
	for ext := range allowedExts {
		filePath := modulePath + ext
		_, err := os.Stat(filePath)
		if err == nil {
			return filePath
		}
	}

	// Then we check for directory with index.ts file, it has lower precedence if both exists
	for ext := range allowedExts {
		filePath := modulePath + "/index" + ext
		_, err := os.Stat(filePath)
		if err == nil {
			return filePath
		}
	}

	return ""
}
