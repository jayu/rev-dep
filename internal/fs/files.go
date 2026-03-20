package fs

import (
	"os"
	"path/filepath"
	"strings"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/pathutil"
)

var allowedExts = map[string]struct{}{
	".ts":     {},
	".tsx":    {},
	".mts":    {},
	".js":     {},
	".jsx":    {},
	".cjs":    {},
	".mjs":    {},
	".mjsx":   {},
	".vue":    {},
	".svelte": {},
}

func hasCorrectExtension(name string) bool {
	ext := filepath.Ext(name)
	_, ok := allowedExts[ext]
	return ok
}

func ParseGitIgnore(fileContent string, dirPath string) []globutil.GlobMatcher {
	lines := strings.Split(fileContent, "\n")

	sanitizedLines := []string{}

	for _, line := range lines {
		trimmedLined := strings.TrimSpace(line)
		if len(trimmedLined) > 0 && !strings.HasPrefix(trimmedLined, "#") {
			sanitizedLines = append(sanitizedLines, line)
		}

	}

	return globutil.CreateGlobMatchers(sanitizedLines, dirPath)
}

func FindAndProcessGitIgnoreFilesUpToRepoRoot(dirPath string) []globutil.GlobMatcher {
	return findAndProcessGitIgnoreFilesUpToRepoRoot(dirPath, []globutil.GlobMatcher{})
}

func findAndProcessGitIgnoreFilesUpToRepoRoot(dirPath string, globMatchers []globutil.GlobMatcher) []globutil.GlobMatcher {
	gitIgnoreFilePath := filepath.Join(dirPath, ".gitignore")
	gitignoreFile, gitignoreError := os.ReadFile(gitIgnoreFilePath)

	if gitignoreError == nil {
		globMatchers = append(globMatchers, ParseGitIgnore(string(gitignoreFile), dirPath)...)
	}

	gitDir, gitDirReadErr := os.Stat(filepath.Join(dirPath, ".git"))

	if gitDirReadErr == nil && gitDir.IsDir() {
		// found git root
		return globMatchers
	}

	parent := pathutil.StandardiseDirPath(filepath.Join(dirPath, "../"))
	if parent == dirPath {
		return globMatchers
	}

	return findAndProcessGitIgnoreFilesUpToRepoRoot(parent, globMatchers)
}

func GetFiles(directory string, existingFiles []string, parentGlobMatchers []globutil.GlobMatcher) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return existingFiles
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(directory, entryName)

		if entry.IsDir() {
			debug := false
			if !globutil.MatchesAnyGlobMatcher(entryFilePath, parentGlobMatchers, debug) {
				// We parse gitignore here to avoid duplicated processing of gitignore from cwd - it will be captured by FindAndProcessGitIgnoreFilesUpToRepoRoot which result should be passed as parentGlobMatchers to root invocation of getFiles

				gitignoreFile, gitignoreError := os.ReadFile(filepath.Join(entryFilePath, ".gitignore"))

				ignoreGlobs := []globutil.GlobMatcher{}
				if gitignoreError == nil {
					ignoreGlobs = ParseGitIgnore(string(gitignoreFile), entryFilePath)
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

		if hasCorrectExtension(entryName) && !globutil.MatchesAnyGlobMatcher(entryFilePath, parentGlobMatchers, false) {
			// store internal normalized path (forward slashes) for analysis and tests
			existingFiles = append(existingFiles, pathutil.NormalizePathForInternal(entryFilePath))
		}
	}

	return existingFiles
}

func GetMissingFile(modulePath string, moduleSuffixes []string) string {
	if len(moduleSuffixes) == 0 {
		moduleSuffixes = []string{""}
	}

	for _, suffix := range moduleSuffixes {
		// First we check for file with possible extensions and this suffix
		for ext := range allowedExts {
			filePath := modulePath + suffix

			// filePath might be the exact path already
			if !strings.HasSuffix(filePath, ext) {
				filePath = filePath + ext
			}

			// modulePath may be internal (forward slashes) or OS-native; try denormalized form for FS checks
			filePathOs := pathutil.DenormalizePathForOS(filePath)
			info, err := os.Stat(filePathOs)
			if err == nil && !info.IsDir() {
				return pathutil.NormalizePathForInternal(filePath)
			}
		}

		// Then we check for directory with index file and this suffix
		for ext := range allowedExts {
			// check directory index; normalize to OS path for Stat
			filePath := modulePath + "/index" + suffix + ext
			filePathOs := pathutil.DenormalizePathForOS(filePath)
			info, err := os.Stat(filePathOs)
			if err == nil && !info.IsDir() {
				return pathutil.NormalizePathForInternal(filePath)
			}
		}
	}

	return ""
}
