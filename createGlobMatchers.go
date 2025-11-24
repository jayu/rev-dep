package main

import (
	"fmt"
	"os"
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

	for _, excludePattern := range patterns {
		// .gitignore for entries without `/` or `*` - so effectively plain text names, matches directories of files with that exact name. We want to align with .gitignore behavior
		shouldMatchAnyFileOrDirWithPattern := !strings.Contains(excludePattern, "/") && !strings.Contains(excludePattern, "*")

		if strings.HasSuffix(excludePattern, "/") && !strings.Contains(excludePattern, "*") {
			// in gitignore entry with `/` suffix matches whole directory recursively
			excludePattern = "**" + excludePattern + "**"

		}

		item := GlobMatcher{
			globPattern:                        glob.MustCompile(excludePattern),
			inputString:                        excludePattern,
			patternRoot:                        patternsRoot,
			shouldMatchAnyFileOrDirWithPattern: shouldMatchAnyFileOrDirWithPattern,
			isAdditional:                       false,
		}
		globMatchers = append(globMatchers, item)
		// !!! This glob library does not match files in using directory wildcard (**/) if file is in root directory. eg `**/*.log`` will not match against `file.log`, but will match against `dir/file.log`
		// This is not aligned with TS rev-dep implementation and not aligned with .gitignore behavior
		// So we add additional pattern to patch the discrepancy
		if strings.HasPrefix(excludePattern, "**/") {
			additionalPattern := strings.Replace(excludePattern, "**/", "", 1)
			additionalItem := GlobMatcher{
				globPattern:                        glob.MustCompile(additionalPattern),
				inputString:                        additionalPattern,
				patternRoot:                        patternsRoot,
				shouldMatchAnyFileOrDirWithPattern: false,
				isAdditional:                       true,
			}
			globMatchers = append(globMatchers, additionalItem)
		}
	}
	return globMatchers
}

var osSeparator = string(os.PathSeparator)

func MatchesAnyGlobMatcher(filePath string, matchers []GlobMatcher, debug bool) bool {

	for _, matcher := range matchers {
		fileWithoutPrefix := strings.TrimPrefix(filePath, matcher.patternRoot)
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
		if matcher.shouldMatchAnyFileOrDirWithPattern && strings.HasSuffix(fileWithoutPrefix, osSeparator+matcher.inputString) {
			// matches file with name exactly as the pattern
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches file name exactly", matcher.inputString)
			}
			return true
		}
		if matcher.shouldMatchAnyFileOrDirWithPattern && (strings.Contains(fileWithoutPrefix, osSeparator+matcher.inputString+osSeparator) || strings.HasPrefix(fileWithoutPrefix, matcher.inputString+osSeparator)) {
			// matches directory with name exactly as the pattern
			if debug {
				fmt.Println(fileWithoutPrefix, "return matches directory", matcher.inputString)
			}

			return true
		}
	}
	return false
}
