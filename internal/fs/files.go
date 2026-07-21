package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	globutil "rev-dep-go/internal/glob"
	"rev-dep-go/internal/pathutil"
)

// orderedExts is the single source of truth for which extensions count as source files,
// listed in resolution-precedence order (mirroring resolve.extensionToOrder, with .mjsx
// added - it has no entry there). GetMissingFile probes the filesystem in this order, so a
// module that exists as both foo.ts and foo.js resolves to foo.ts every time. Ranging over
// a map instead made that outcome depend on Go's randomised map iteration.
var orderedExts = []string{".ts", ".tsx", ".mts", ".js", ".jsx", ".mjs", ".mjsx", ".cjs", ".vue", ".svelte"}

// allowedExts is the membership index for orderedExts, derived rather than written out so
// the two cannot drift. Maintaining both by hand would let an extension be added to only
// one: present in allowedExts alone, a file is discovered by the walk but never probed for
// as a missing import; present in orderedExts alone, the reverse. Both fail silently.
var allowedExts = func() map[string]struct{} {
	exts := make(map[string]struct{}, len(orderedExts))
	for _, ext := range orderedExts {
		exts[ext] = struct{}{}
	}
	return exts
}()

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

// hasGitignoreEntry reports whether a directory listing contains a .gitignore file. The
// listing is already in hand, so this answers the question without the openat that probing
// for the file would cost in every directory that has none - which is nearly all of them.
func hasGitignoreEntry(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() == ".gitignore" && !entry.IsDir() {
			return true
		}
	}
	return false
}

// comparePathsDepthFirst orders paths the way the previous depth-first walk emitted them.
//
// That walk relied on os.ReadDir returning entries sorted by name and descended into each
// directory as soon as it saw it, so a directory's contents were emitted before any sibling
// sorting after it. Reproducing that is a matter of ranking the separator below every other
// byte: "a/b.ts" then sorts before "a.ts", exactly as descending into "a" before reaching
// "a.ts" did. A plain byte sort would swap them, since '.' (0x2E) < '/' (0x2F).
//
// Keeping the old order matters because it is user-visible - `list-cwd-files` prints the
// walk's output directly.
func comparePathsDepthFirst(left, right string) int {
	limit := min(len(left), len(right))
	for i := 0; i < limit; i++ {
		if left[i] == right[i] {
			continue
		}
		return pathByteRank(left[i]) - pathByteRank(right[i])
	}
	return len(left) - len(right)
}

// pathByteRank maps the path separator below every other byte; see comparePathsDepthFirst.
func pathByteRank(b byte) int {
	if b == '/' {
		return -1
	}
	return int(b)
}

// walkItem is one directory queued for traversal, carrying the exclude set that applies
// inside it (its ancestors' patterns plus any it contributed itself).
type walkItem struct {
	dirPath      string
	excludeGlobs []globutil.GlobMatcher
	isRoot       bool
}

// getFiles is the shared discovery walk. includePrefixes is computed once by the public
// entry points (it is derived only from includeMatchers, which never change during the
// walk). When rec is non-nil, excluded files and pruned directories are recorded.
//
// The walk is a breadth-first traversal spread over a worker pool: it is dominated by
// ReadDir latency and per-entry glob matching, both of which parallelise cleanly since the
// matchers are immutable once built. Results are collected per directory and merged under a
// mutex, then sorted, so the output does not depend on the order workers happen to finish.
//
// The pool grows on demand rather than starting at full width, so a shallow tree is walked
// by the single goroutine that started it and only genuine fan-out costs goroutines.
func getFiles(directory string, existingFiles []string, parentGlobMatchers []globutil.GlobMatcher, includeMatchers []globutil.GlobMatcher, includePrefixes []string, rec *DiscoveryExclusions) ([]string, *DiscoveryExclusions) {
	workerCount := min(max(runtime.GOMAXPROCS(0), 2), 16)

	var resultMu sync.Mutex
	discovered := make([]string, 0, 1024)

	var queueMu sync.Mutex
	queueCond := sync.NewCond(&queueMu)
	queue := []walkItem{{dirPath: directory, excludeGlobs: parentGlobMatchers, isRoot: true}}
	// head is the index of the next item to pop. Consuming via an index instead of
	// reslicing (queue = queue[1:]) lets the popped element be zeroed, releasing its
	// retained matcher slice; a reslice leaves the whole backing array - and everything it
	// references - alive until the walk ends.
	head := 0
	pending := 1
	workersLive := 1

	var wg sync.WaitGroup
	var worker func()
	startWorker := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}

	worker = func() {
		for {
			queueMu.Lock()
			for head == len(queue) && pending > 0 {
				queueCond.Wait()
			}
			if pending == 0 {
				queueMu.Unlock()
				return
			}
			current := queue[head]
			queue[head] = walkItem{}
			head++
			queueMu.Unlock()

			var localFiles, localExcluded, localPruned []string
			var subDirs []walkItem

			entries, err := os.ReadDir(current.dirPath)
			if err == nil {
				globMatchers := current.excludeGlobs

				// A directory's own .gitignore applies to it and everything below. The root's
				// is deliberately skipped: FindAndProcessGitIgnoreFilesUpToRepoRoot has already
				// folded it, and every ancestor's, into parentGlobMatchers.
				if !current.isRoot && hasGitignoreEntry(entries) {
					gitignoreFile, gitignoreError := os.ReadFile(filepath.Join(current.dirPath, ".gitignore"))
					if gitignoreError == nil {
						nested := ParseGitIgnore(string(gitignoreFile), current.dirPath)
						if len(nested) > 0 {
							// Build a fresh slice instead of appending into the inherited one:
							// sibling directories share its backing array and are walked
							// concurrently, so appending in place would let them overwrite each
							// other's patterns.
							globMatchers = make([]globutil.GlobMatcher, 0, len(current.excludeGlobs)+len(nested))
							globMatchers = append(globMatchers, current.excludeGlobs...)
							globMatchers = append(globMatchers, nested...)
						}
					}
				}

				for _, entry := range entries {
					entryName := entry.Name()
					entryFilePath := filepath.Join(current.dirPath, entryName)

					if entry.IsDir() {
						// .git holds no source files but is large and deeply nested (the 256
						// object fanout dirs alone). The workspace walk skips it too.
						if entryName == ".git" {
							continue
						}
						if globutil.ShouldTraverseDir(entryFilePath, globMatchers, includeMatchers, includePrefixes) {
							subDirs = append(subDirs, walkItem{dirPath: entryFilePath, excludeGlobs: globMatchers})
						} else if rec != nil {
							localPruned = append(localPruned, pathutil.NormalizePathForInternal(entryFilePath))
						}
						continue
					}

					if !hasCorrectExtension(entryName) {
						continue
					}
					if globutil.IsExcludedByPatterns(entryFilePath, globMatchers, includeMatchers) {
						if rec != nil {
							localExcluded = append(localExcluded, pathutil.NormalizePathForInternal(entryFilePath))
						}
						continue
					}
					// store internal normalized path (forward slashes) for analysis and tests
					localFiles = append(localFiles, pathutil.NormalizePathForInternal(entryFilePath))
				}
			}

			if len(localFiles) > 0 || len(localExcluded) > 0 || len(localPruned) > 0 {
				resultMu.Lock()
				discovered = append(discovered, localFiles...)
				if rec != nil {
					rec.ExcludedFiles = append(rec.ExcludedFiles, localExcluded...)
					rec.PrunedDirs = append(rec.PrunedDirs, localPruned...)
				}
				resultMu.Unlock()
			}

			queueMu.Lock()
			pending--
			if len(subDirs) > 0 {
				queue = append(queue, subDirs...)
				pending += len(subDirs)
			}
			// Add workers only for backlog that actually exists, capped at workerCount.
			// wg.Add here cannot race the parent's Wait: this worker has not called Done
			// yet, so the counter is at least 1 for the whole call.
			toStart := 0
			for workersLive < workerCount && workersLive < len(queue)-head {
				workersLive++
				toStart++
			}
			queueCond.Broadcast()
			queueMu.Unlock()

			for i := 0; i < toStart; i++ {
				startWorker()
			}
		}
	}

	startWorker()
	wg.Wait()

	slices.SortFunc(discovered, comparePathsDepthFirst)
	if rec != nil {
		slices.SortFunc(rec.ExcludedFiles, comparePathsDepthFirst)
		slices.SortFunc(rec.PrunedDirs, comparePathsDepthFirst)
	}

	return append(existingFiles, discovered...), rec
}

func GetMissingFile(modulePath string, moduleSuffixes []string) string {
	if len(moduleSuffixes) == 0 {
		moduleSuffixes = []string{""}
	}

	for _, suffix := range moduleSuffixes {
		// First we check for file with possible extensions and this suffix
		for _, ext := range orderedExts {
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
		for _, ext := range orderedExts {
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
