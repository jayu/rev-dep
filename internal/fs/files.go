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

// anyDepthGitIgnorePattern applies the .gitignore rule that a pattern containing no "/"
// matches at every level, not just the .gitignore's own directory. The glob engine treats a
// single "*" as one path segment (it does not cross "/"), so a wildcard pattern like "*.log"
// would otherwise only match at the root. Prefixing "**/" restores the .gitignore meaning.
//
// Only wildcard patterns need this: a plain name ("node_modules") and a trailing-slash
// directory ("dist/") are already matched at any depth by CreateGlobMatchers. Patterns that
// contain a "/" ("doc/frotz", "!dist/manifest.json") are anchored to the .gitignore's
// directory per the spec and must be left alone.
func anyDepthGitIgnorePattern(line string) string {
	pattern := line
	negation := ""
	if strings.HasPrefix(pattern, "!") {
		negation = "!"
		pattern = pattern[1:]
	}

	if strings.Contains(pattern, "/") || !strings.ContainsAny(pattern, "*?") {
		return line
	}

	return negation + "**/" + pattern
}

func ParseGitIgnore(fileContent string, dirPath string) []globutil.GlobMatcher {
	lines := strings.Split(fileContent, "\n")

	sanitizedLines := []string{}

	for _, line := range lines {
		trimmedLined := strings.TrimSpace(line)
		if len(trimmedLined) > 0 && !strings.HasPrefix(trimmedLined, "#") {
			sanitizedLines = append(sanitizedLines, anyDepthGitIgnorePattern(trimmedLined))
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

// DiscoveryExclusions records the paths a discovery walk left out — individual files an
// exclude pattern matched, and whole directories the walk pruned as fully excluded. Files
// inside pruned directories are intentionally NOT enumerated (that is the point of
// pruning); PrunedDirs marks those subtrees so a caller (e.g. the config linter) can reason
// about them without a second, unpruned walk. All paths are internal-normalized.
type DiscoveryExclusions struct {
	ExcludedFiles []string
	PrunedDirs    []string
}

// GetFiles walks directory and returns every source file not excluded by the given
// matchers. It is the plain entry point used when the caller does not need to know what was
// excluded.
func GetFiles(directory string, existingFiles []string, parentGlobMatchers []globutil.GlobMatcher, includeMatchers []globutil.GlobMatcher) []string {
	files, _ := getFiles(directory, existingFiles, parentGlobMatchers, includeMatchers, globutil.BuildIncludePrefixes(includeMatchers), nil)
	return files
}

// GetFilesWithExclusions is GetFiles that also reports what the walk left out (see
// DiscoveryExclusions). The linter uses it to decide which ignore patterns still match
// something from the SAME pruned walk, avoiding a second unpruned traversal of large
// ignored directories.
func GetFilesWithExclusions(directory string, parentGlobMatchers []globutil.GlobMatcher, includeMatchers []globutil.GlobMatcher) ([]string, *DiscoveryExclusions) {
	rec := &DiscoveryExclusions{}
	files, _ := getFiles(directory, nil, parentGlobMatchers, includeMatchers, globutil.BuildIncludePrefixes(includeMatchers), rec)
	return files, rec
}

// getFiles is the shared recursive walk. includePrefixes is computed once by the public
// entry points and threaded down (it is derived only from includeMatchers, which never
// change during the walk). When rec is non-nil, excluded files and pruned directories are
// recorded.
func getFiles(directory string, existingFiles []string, parentGlobMatchers []globutil.GlobMatcher, includeMatchers []globutil.GlobMatcher, includePrefixes []string, rec *DiscoveryExclusions) ([]string, *DiscoveryExclusions) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return existingFiles, rec
	}

	for _, entry := range entries {
		entryName := entry.Name()
		entryFilePath := filepath.Join(directory, entryName)

		if entry.IsDir() {
			if globutil.ShouldTraverseDir(entryFilePath, parentGlobMatchers, includeMatchers, includePrefixes) {
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

				existingFiles, rec = getFiles(entryFilePath, existingFiles, ignoreGlobs, includeMatchers, includePrefixes, rec)
			} else if rec != nil {
				rec.PrunedDirs = append(rec.PrunedDirs, pathutil.NormalizePathForInternal(entryFilePath))
			}
			continue
		}

		if !hasCorrectExtension(entryName) {
			continue
		}
		if globutil.IsExcludedByPatterns(entryFilePath, parentGlobMatchers, includeMatchers) {
			if rec != nil {
				rec.ExcludedFiles = append(rec.ExcludedFiles, pathutil.NormalizePathForInternal(entryFilePath))
			}
			continue
		}
		// store internal normalized path (forward slashes) for analysis and tests
		existingFiles = append(existingFiles, pathutil.NormalizePathForInternal(entryFilePath))
	}

	return existingFiles, rec
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
