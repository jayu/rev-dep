package globutil

import (
	"runtime"
	"slices"
	"strings"
	"sync"

	"rev-dep-go/internal/pathutil"
)

func containsGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[]{}")
}

func getStaticPrefixBeforeGlob(pattern string) string {
	if pattern == "" {
		return ""
	}
	parts := strings.Split(pattern, "/")
	staticParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || containsGlobMeta(part) {
			break
		}
		staticParts = append(staticParts, part)
	}
	if len(staticParts) == 0 {
		return ""
	}
	return strings.Join(staticParts, "/")
}

func IsExcludedByPatterns(path string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher) bool {
	if len(excludePatterns) == 0 {
		return false
	}
	if !MatchesAnyGlobMatcher(path, excludePatterns, false) {
		return false
	}
	if len(includePatterns) == 0 {
		return true
	}
	return !MatchesAnyGlobMatcher(path, includePatterns, false)
}

// parallelFilterThreshold is the input size below which RejectExcluded runs inline. Below a
// few thousand paths the goroutine setup costs more than the matching it saves.
const parallelFilterThreshold = 2048

// RejectExcluded returns the paths that IsExcludedByPatterns does NOT exclude, preserving
// input order. The result is always a freshly allocated slice, never the input.
//
// Matching is pure — GlobMatcher is read-only once built and gobwas matchers hold no
// per-match state — so for large inputs the scan is sharded across the available cores.
// Each shard writes a disjoint range of a preallocated verdict slice, which is what keeps
// the result deterministic regardless of how the shards interleave.
func RejectExcluded(paths []string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher) []string {
	if len(excludePatterns) == 0 {
		// Without exclude patterns IsExcludedByPatterns excludes nothing, so every path is
		// kept. Still return a copy: callers treat the result as a slice they own, while the
		// input is often one the caller is separately appending to (ResolveImports grows
		// sortedFiles during resolution). Handing back the caller's own backing array would
		// let a later append on either side write through the other.
		return slices.Clone(paths)
	}

	excluded := make([]bool, len(paths))

	if len(paths) < parallelFilterThreshold {
		for i, path := range paths {
			excluded[i] = IsExcludedByPatterns(path, excludePatterns, includePatterns)
		}
	} else {
		shards := runtime.GOMAXPROCS(0)
		if shards > 16 {
			shards = 16
		}
		if shards < 1 {
			shards = 1
		}
		chunk := (len(paths) + shards - 1) / shards

		var wg sync.WaitGroup
		for start := 0; start < len(paths); start += chunk {
			end := start + chunk
			if end > len(paths) {
				end = len(paths)
			}
			wg.Add(1)
			go func(start, end int) {
				defer wg.Done()
				for i := start; i < end; i++ {
					excluded[i] = IsExcludedByPatterns(paths[i], excludePatterns, includePatterns)
				}
			}(start, end)
		}
		wg.Wait()
	}

	kept := make([]string, 0, len(paths))
	for i, path := range paths {
		if !excluded[i] {
			kept = append(kept, path)
		}
	}
	return kept
}

func BuildIncludePrefixes(matchers []GlobMatcher) []string {
	if len(matchers) == 0 {
		return nil
	}

	prefixes := make(map[string]bool)
	for _, matcher := range matchers {
		staticPrefix := getStaticPrefixBeforeGlob(matcher.inputString)
		if staticPrefix == "" {
			prefixes[""] = true
			continue
		}
		fullPrefix := matcher.patternRoot + staticPrefix
		fullPrefix = pathutil.NormalizePathForInternal(strings.TrimSuffix(fullPrefix, "/"))
		fullPrefix = pathutil.StandardiseDirPathInternal(fullPrefix)
		prefixes[fullPrefix] = true
	}

	out := make([]string, 0, len(prefixes))
	for prefix := range prefixes {
		out = append(out, prefix)
	}
	return out
}

// recursiveCoverRoot reports, for a whole-subtree exclude pattern, the internal-form
// directory path at (and below) which every descendant path is matched. It recognises the
// two provable forms — a bare `**` (root = the matcher root) and `<staticPrefix>/**` where
// the prefix has no glob metacharacters — and returns ok=false for anything else. The
// returned root always carries a trailing slash so prefix tests are boundary-safe.
func recursiveCoverRoot(matcher GlobMatcher) (string, bool) {
	pattern := matcher.inputString
	if pattern == "**" {
		return pathutil.StandardiseDirPathInternal(strings.TrimSuffix(matcher.patternRoot, "/")), true
	}
	stem, ok := strings.CutSuffix(pattern, "/**")
	if !ok || stem == "" || containsGlobMeta(stem) {
		return "", false
	}
	full := pathutil.NormalizePathForInternal(matcher.patternRoot + stem)
	return pathutil.StandardiseDirPathInternal(strings.TrimSuffix(full, "/")), true
}

// StaticPrefixPath resolves a pattern's leading glob-free segment to an absolute
// internal-form directory (trailing slash) under root, or returns "" when the pattern
// begins with a glob metacharacter (so it could match at any depth). It lets callers reason
// about which subtree a pattern can possibly touch — e.g. whether it could match inside a
// directory the walk pruned whole.
func StaticPrefixPath(pattern, root string) string {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.TrimPrefix(pattern, "!")
	pattern = strings.TrimPrefix(pattern, "/")
	prefix := getStaticPrefixBeforeGlob(pattern)
	if prefix == "" {
		return ""
	}
	rootNorm := pathutil.NormalizePathForInternal(root)
	if rootNorm != "" && !strings.HasSuffix(rootNorm, "/") {
		rootNorm += "/"
	}
	return pathutil.StandardiseDirPathInternal(strings.TrimSuffix(rootNorm+prefix, "/"))
}

// DirFullyExcluded reports whether EVERY possible path under dirPath is excluded, so the
// walk can prune the directory instead of descending to exclude each file one by one.
// A plain `<dir>/**` pattern does NOT match the bare directory path (only its contents),
// so IsExcludedByPatterns alone never prunes such a directory — this recovers that case.
//
// It is deliberately conservative and only returns true for provable full coverage: if any
// negation is present in the set (a re-inclusion could rescue a descendant) it returns
// false, and it ignores non-recursive patterns. That guarantees pruning never drops a file
// the file-by-file path would have kept — it is a pure optimisation, not a semantic change.
func DirFullyExcluded(dirPath string, excludePatterns []GlobMatcher) bool {
	if len(excludePatterns) == 0 {
		return false
	}
	dirInternal := pathutil.StandardiseDirPathInternal(strings.TrimSuffix(pathutil.NormalizePathForInternal(dirPath), "/"))
	covered := false
	for i := range excludePatterns {
		if excludePatterns[i].isNegated {
			return false
		}
		if covered {
			continue // keep scanning for a negation that would veto pruning
		}
		coverRoot, ok := recursiveCoverRoot(excludePatterns[i])
		if ok && strings.HasPrefix(dirInternal, coverRoot) {
			covered = true
		}
	}
	return covered
}

func ShouldTraverseDir(dirPath string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher, includePrefixes []string) bool {
	if !IsExcludedByPatterns(dirPath, excludePatterns, includePatterns) && !DirFullyExcluded(dirPath, excludePatterns) {
		return true
	}
	if len(includePrefixes) == 0 {
		return false
	}

	dirInternal := pathutil.NormalizePathForInternal(dirPath)
	dirInternal = pathutil.StandardiseDirPathInternal(strings.TrimSuffix(dirInternal, "/"))
	for _, prefix := range includePrefixes {
		if prefix == "" {
			return true
		}
		if strings.HasPrefix(prefix, dirInternal) || strings.HasPrefix(dirInternal, prefix) {
			return true
		}
	}
	return false
}
