package globutil

import (
	"strings"

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

func ShouldTraverseDir(dirPath string, excludePatterns []GlobMatcher, includePatterns []GlobMatcher, includePrefixes []string) bool {
	if !IsExcludedByPatterns(dirPath, excludePatterns, includePatterns) {
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
