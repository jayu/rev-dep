package main

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
)

type GlobMatcher struct {
	globPattern                        glob.Glob
	inputString                        string
	shouldMatchAnyFileOrDirWithPattern bool
	patternRoot                        string
	isAdditional                       bool
}

func CreateGlobMatchers(patterns []string, patternsRoot string) []GlobMatcher {
	globMatchers := []GlobMatcher{}
	// normalize pattern root to internal form and ensure trailing '/'
	patternRootNorm := NormalizePathForInternal(patternsRoot)
	if patternRootNorm != "" && !strings.HasSuffix(patternRootNorm, "/") {
		patternRootNorm = patternRootNorm + "/"
	}

	for _, excludePattern := range patterns {
		// .gitignore for entries without `/` or `*` - so effectively plain text names, matches directories of files with that exact name. We want to align with .gitignore behavior
		shouldMatchAnyFileOrDirWithPattern := !strings.Contains(excludePattern, "/") && !strings.Contains(excludePattern, "*")

		if strings.HasSuffix(excludePattern, "/") && !strings.Contains(excludePattern, "*") {
			// in gitignore entry with `/` suffix matches whole directory recursively
			excludePattern = "**" + excludePattern + "**"

		}

		// normalize pattern separators (globs and gitignore entries use forward slashes)
		patternNorm := NormalizeGlobPattern(excludePattern)

		item := GlobMatcher{
			globPattern:                        glob.MustCompile(patternNorm),
			inputString:                        patternNorm,
			patternRoot:                        patternRootNorm,
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
				globPattern:                        glob.MustCompile(additionalPattern),
				inputString:                        additionalPattern,
				patternRoot:                        patternRootNorm,
				shouldMatchAnyFileOrDirWithPattern: false,
				isAdditional:                       true,
			}
			globMatchers = append(globMatchers, additionalItem)
		}
	}
	return globMatchers
}

func MatchesAnyGlobMatcher(filePath string, matchers []GlobMatcher, debug bool) bool {
	for _, matcher := range matchers {
		// convert candidate path to internal form (forward slashes)
		fileInternal := NormalizePathForInternal(filePath)
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
		if matcher.shouldMatchAnyFileOrDirWithPattern && strings.HasSuffix(fileWithoutPrefix, "/"+matcher.inputString) {
			// matches file with name exactly as the pattern
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches file name exactly", matcher.inputString)
			}
			return true
		}
		if matcher.shouldMatchAnyFileOrDirWithPattern && (strings.Contains(fileWithoutPrefix, "/"+matcher.inputString+"/") || strings.HasPrefix(fileWithoutPrefix, matcher.inputString+"/")) {
			// matches directory with name exactly as the pattern
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches directory", matcher.inputString)
			}

			return true
		}
	}
	return false
}
