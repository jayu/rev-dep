package globutil

import (
	"fmt"
	"path"
	"strings"

	"github.com/gobwas/glob"

	"rev-dep-go/internal/pathutil"
)

type GlobMatcher struct {
	globPattern                        glob.Glob
	inputString                        string
	shouldMatchAnyFileOrDirWithPattern bool
	patternRoot                        string
	isAnchoredToPatternRoot            bool
	isAdditional                       bool
	isNegated                          bool
}

func rebaseRelativePattern(pattern string, patternRoot string) (string, string, bool, bool) {
	if pattern == "" || strings.HasPrefix(pattern, "/") {
		return patternRoot, pattern, false, false
	}
	if pattern != "." && pattern != ".." && !strings.HasPrefix(pattern, "./") && !strings.HasPrefix(pattern, "../") {
		return patternRoot, pattern, false, false
	}

	trailingSlash := strings.HasSuffix(pattern, "/")
	cleaned := path.Clean(pattern)
	parts := strings.Split(cleaned, "/")
	up := 0
	i := 0
	for i < len(parts) && parts[i] == ".." {
		up++
		i++
	}
	rest := strings.Join(parts[i:], "/")
	if rest == "" {
		rest = "."
	}
	if rest == "." && !trailingSlash {
		trailingSlash = true
	}

	root := strings.TrimSuffix(patternRoot, "/")
	for i := 0; i < up; i++ {
		root = path.Dir(root)
	}
	if root != "" {
		root += "/"
	}

	return root, rest, true, trailingSlash
}

func CreateGlobMatchers(patterns []string, patternsRoot string) []GlobMatcher {
	globMatchers := []GlobMatcher{}
	// normalize pattern root to internal form and ensure trailing '/'
	patternRootNorm := pathutil.NormalizePathForInternal(patternsRoot)
	if patternRootNorm != "" && !strings.HasSuffix(patternRootNorm, "/") {
		patternRootNorm = patternRootNorm + "/"
	}

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)

		// Negation (gitignore-style): a leading `!` marks an exception that re-includes paths a
		// positive pattern would otherwise match. A leading `\!` is an escaped literal `!`.
		isNegated := false
		if strings.HasPrefix(pattern, "!") {
			isNegated = true
			pattern = pattern[1:]
		} else if strings.HasPrefix(pattern, "\\!") {
			pattern = pattern[1:] // drop the escaping backslash, keep the literal '!'
		}
		if isNegated && pattern == "" {
			continue // a lone "!" is not a usable pattern
		}

		pattern = pathutil.NormalizeGlobPattern(pattern)

		patternRootForPattern := patternRootNorm
		isAnchoredToPatternRoot := strings.HasPrefix(pattern, "/")
		if isAnchoredToPatternRoot {
			pattern = strings.TrimPrefix(pattern, "/")
		}

		if !isAnchoredToPatternRoot {
			if newRoot, newPattern, ok, trailingSlash := rebaseRelativePattern(pattern, patternRootNorm); ok {
				patternRootForPattern = newRoot
				pattern = newPattern
				isAnchoredToPatternRoot = true
				if trailingSlash {
					if pattern == "." {
						pattern = ""
					}
					if pattern == "" {
						pattern = "**"
					} else if !strings.HasSuffix(pattern, "/") {
						pattern += "/"
					}
				}
			}
		}

		// .gitignore for entries without `/` or `*` - so effectively plain text names, matches directories of files with that exact name. We want to align with .gitignore behavior
		shouldMatchAnyFileOrDirWithPattern := !strings.Contains(pattern, "/") && !strings.Contains(pattern, "*")

		if strings.HasSuffix(pattern, "/") && !strings.Contains(pattern, "*") {
			// in gitignore entry with `/` suffix matches whole directory recursively
			if isAnchoredToPatternRoot {
				pattern = pattern + "**"
			} else {
				pattern = "**" + pattern + "**"
			}

		}

		// normalize pattern separators (globs and gitignore entries use forward slashes)
		patternNorm := pathutil.NormalizeGlobPattern(pattern)

		item := GlobMatcher{
			globPattern:                        glob.MustCompile(patternNorm),
			inputString:                        patternNorm,
			patternRoot:                        patternRootForPattern,
			isAnchoredToPatternRoot:            isAnchoredToPatternRoot,
			shouldMatchAnyFileOrDirWithPattern: shouldMatchAnyFileOrDirWithPattern,
			isAdditional:                       false,
			isNegated:                          isNegated,
		}
		globMatchers = append(globMatchers, item)
		// !!! This glob library does not match files in using directory wildcard (**/) if file is in root directory. eg `**/*.log`` will not match against `file.log`, but will match against `dir/file.log`
		// This is not aligned with TS rev-dep implementation and not aligned with .gitignore behavior
		// So we add additional pattern to patch the discrepancy
		if strings.HasPrefix(patternNorm, "**/") {
			additionalPattern := strings.Replace(patternNorm, "**/", "", 1)
			additionalItem := GlobMatcher{
				globPattern:                        glob.MustCompile(additionalPattern),
				inputString:                        additionalPattern,
				patternRoot:                        patternRootForPattern,
				isAnchoredToPatternRoot:            isAnchoredToPatternRoot,
				shouldMatchAnyFileOrDirWithPattern: false,
				isAdditional:                       true,
				isNegated:                          isNegated,
			}
			globMatchers = append(globMatchers, additionalItem)
		}
	}
	return globMatchers
}

// matchesOne reports whether a single matcher matches the (already internal-form) path.
func (matcher GlobMatcher) matchesOne(fileInternal string, debug bool) bool {
	fileWithoutPrefix := strings.TrimPrefix(fileInternal, matcher.patternRoot)
	if debug {
		fmt.Println("Matcher", matcher.globPattern, matcher.inputString, matcher.patternRoot, matcher.shouldMatchAnyFileOrDirWithPattern, matcher.isAdditional, "negated:", matcher.isNegated)
		fmt.Println("Input", fileWithoutPrefix, fileInternal)
	}
	if matcher.globPattern.Match(fileWithoutPrefix) {
		return true
	}
	if matcher.shouldMatchAnyFileOrDirWithPattern && !matcher.isAnchoredToPatternRoot && strings.HasSuffix(fileWithoutPrefix, "/"+matcher.inputString) {
		// matches file/dir with name exactly as the pattern (unnanchored only - anchored patterns like /boot must only match at root)
		return true
	}
	if matcher.shouldMatchAnyFileOrDirWithPattern && matcher.isAnchoredToPatternRoot && strings.HasPrefix(fileWithoutPrefix, matcher.inputString+"/") {
		// anchored patterns (e.g. /node_modules) should only match at this matcher root
		return true
	}
	if matcher.shouldMatchAnyFileOrDirWithPattern && !matcher.isAnchoredToPatternRoot && (strings.Contains(fileWithoutPrefix, "/"+matcher.inputString+"/") || strings.HasPrefix(fileWithoutPrefix, matcher.inputString+"/")) {
		// matches directory with name exactly as the pattern
		return true
	}
	return false
}

func MatchesAnyGlobMatcher(filePath string, matchers []GlobMatcher, debug bool) bool {
	fileInternal := pathutil.NormalizePathForInternal(filePath)

	positiveMatched := false
	hasNegated := false
	for i := range matchers {
		if matchers[i].isNegated {
			hasNegated = true
			continue
		}
		if positiveMatched {
			continue // keep scanning cheaply to learn whether any negation exists
		}
		if matchers[i].matchesOne(fileInternal, debug) {
			positiveMatched = true
		}
	}

	if !positiveMatched {
		return false
	}
	if !hasNegated {
		return true
	}

	// A positive matched and the set has exceptions - a negated match cancels the result.
	for i := range matchers {
		if matchers[i].isNegated && matchers[i].matchesOne(fileInternal, debug) {
			if debug {
				fmt.Println(fileInternal, "cancelled by negated matcher", matchers[i].inputString)
			}
			return false
		}
	}
	return true
}
