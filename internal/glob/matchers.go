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
			// compile with '/' as path separator so a single '*' matches within a
			// single path segment only (gitignore-aligned); '**' still crosses '/'.
			// Without a separator gobwas/glob lets '*' span '/', so e.g. "**/use*.ts"
			// would wrongly match "dir/useSearch/common.ts".
			globPattern:                        glob.MustCompile(patternNorm, '/'),
			inputString:                        patternNorm,
			patternRoot:                        patternRootForPattern,
			isAnchoredToPatternRoot:            isAnchoredToPatternRoot,
			shouldMatchAnyFileOrDirWithPattern: shouldMatchAnyFileOrDirWithPattern,
			isAdditional:                       false,
		}
		globMatchers = append(globMatchers, item)
		// !!! This glob library does not match files in using directory wildcard (**/) if file is in root directory. eg `**/*.log`` will not match against `file.log`, but will match against `dir/file.log`
		// This is not aligned with TS rev-dep implementation and not aligned with .gitignore behavior
		// So we add additional pattern to patch the discrepancy
		if strings.HasPrefix(patternNorm, "**/") {
			additionalPattern := strings.Replace(patternNorm, "**/", "", 1)
			additionalItem := GlobMatcher{
				globPattern:                        glob.MustCompile(additionalPattern, '/'),
				inputString:                        additionalPattern,
				patternRoot:                        patternRootForPattern,
				isAnchoredToPatternRoot:            isAnchoredToPatternRoot,
				shouldMatchAnyFileOrDirWithPattern: false,
				isAdditional:                       true,
			}
			globMatchers = append(globMatchers, additionalItem)
		}
	}
	return globMatchers
}

func MatchesAnyGlobMatcher(filePath string, matchers []GlobMatcher, debug bool) bool {
	// convert candidate path to internal form (forward slashes)
	fileInternal := pathutil.NormalizePathForInternal(filePath)
	for _, matcher := range matchers {
		// A pattern is interpreted relative to its root - the workspace, or the
		// directory an explicit "../" climbed up to. An absolute file path outside
		// that root is out of scope for this matcher. Without this guard an
		// unanchored pattern like "**/api/**" leaks across workspaces: TrimPrefix
		// below would be a no-op for a foreign absolute path and the leading "**"
		// would happily swallow the "/repo/apps/other/..." prefix. To match other
		// workspaces users must opt in explicitly with a relative "../" pattern,
		// which has already rebased patternRoot upward so such files pass here.
		//
		// The guard applies only when BOTH the root and the candidate are absolute
		// paths. MatchesAnyGlobMatcher is also used to match non-path strings -
		// notably node-module specifiers like "react-dom/client" (restricted
		// imports' ignoreMatches) - which are not workspace-relative and must not
		// be scoped. An empty root means "no scoping" (callers that pass cwd="").
		if pathutil.IsAbsoluteInternalPath(matcher.patternRoot) && pathutil.IsAbsoluteInternalPath(fileInternal) &&
			!strings.HasPrefix(fileInternal, matcher.patternRoot) {
			continue
		}
		fileWithoutPrefix := strings.TrimPrefix(fileInternal, matcher.patternRoot)
		if debug {
			fmt.Println("Matcher", matcher.globPattern, matcher.inputString, matcher.patternRoot, matcher.shouldMatchAnyFileOrDirWithPattern, matcher.isAdditional)
			fmt.Println("Input", fileWithoutPrefix, filePath)
		}
		if matcher.globPattern.Match(fileWithoutPrefix) {
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches pattern", matcher.globPattern)
			}
			return true
		}
		if matcher.shouldMatchAnyFileOrDirWithPattern && !matcher.isAnchoredToPatternRoot && strings.HasSuffix(fileWithoutPrefix, "/"+matcher.inputString) {
			// matches file/dir with name exactly as the pattern (unnanchored only - anchored patterns like /boot must only match at root)
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches file name exactly", matcher.inputString)
			}
			return true
		}
		if matcher.shouldMatchAnyFileOrDirWithPattern && matcher.isAnchoredToPatternRoot && strings.HasPrefix(fileWithoutPrefix, matcher.inputString+"/") {
			// anchored patterns (e.g. /node_modules) should only match at this matcher root
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches anchored directory", matcher.inputString)
			}

			return true
		}
		if matcher.shouldMatchAnyFileOrDirWithPattern && !matcher.isAnchoredToPatternRoot && (strings.Contains(fileWithoutPrefix, "/"+matcher.inputString+"/") || strings.HasPrefix(fileWithoutPrefix, matcher.inputString+"/")) {
			// matches directory with name exactly as the pattern
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches directory", matcher.inputString)
			}

			return true
		}
	}
	return false
}
