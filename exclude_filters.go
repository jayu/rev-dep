package main

import (
	"strings"
)

func isExcludedByPatterns(path string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher) bool {
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

func buildIncludePrefixes(matchers []GlobMatcher) []string {
	if len(matchers) == 0 {
		return nil
	}
	// Compute static directory prefixes for whitelisted patterns. We use these
	// to decide whether to descend into directories that are otherwise excluded.
	// This avoids missing whitelisted files while still skipping unrelated
	// ignored trees for performance.
	prefixes := make(map[string]bool)
	for _, matcher := range matchers {
		staticPrefix := getStaticPrefixBeforeGlob(matcher.inputString)
		if staticPrefix == "" {
			prefixes[""] = true
			continue
		}
		fullPrefix := matcher.patternRoot + staticPrefix
		fullPrefix = NormalizePathForInternal(strings.TrimSuffix(fullPrefix, "/"))
		fullPrefix = StandardiseDirPathInternal(fullPrefix)
		prefixes[fullPrefix] = true
	}
	out := make([]string, 0, len(prefixes))
	for prefix := range prefixes {
		out = append(out, prefix)
	}
	return out
}

func shouldTraverseDir(dirPath string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher, includePrefixes []string) bool {
	if !isExcludedByPatterns(dirPath, excludePatterns, includePatterns) {
		return true
	}
	if len(includePrefixes) == 0 {
		return false
	}
	dirInternal := NormalizePathForInternal(dirPath)
	dirInternal = StandardiseDirPathInternal(strings.TrimSuffix(dirInternal, "/"))
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
