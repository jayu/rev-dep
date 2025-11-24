package main

import (
	"testing"
)

var debug = false

func TestGlobMatchingForDirectoryWithoutWildcard(t *testing.T) {
	t.Run("Directory With Comma and trailing slash in root dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := ".next/"
		filePath := "/fs/root/.next/static/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Directory With Comma without slash in root dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := ".next"
		filePath := "/fs/root/.next/static/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)
		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Directory With Comma without trailing slash in sub dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := ".next"
		filePath := "/fs/root/sub/sub2/.next/static/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)
		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Directory With Comma with trailing slash in sub dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := ".next/"
		filePath := "/fs/root/sub/sub2/.next/static/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)
		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})
}

func TestGlobMatchingForFileNameWithoutWildcard(t *testing.T) {
	t.Run("Filename in root dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "file.js"
		filePath := "/fs/root/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Filename in sub dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "file.js"
		filePath := "/fs/root/sub/sub2/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})
}

func TestGlobMatchingForFileUsingDirectoryWildcard(t *testing.T) {
	t.Run("File in root dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "**/*.log"
		filePath := "/fs/root/data.log"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})
	t.Run("file in sub dir", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "**/*.log"
		filePath := "/fs/root/data/sub/file.log"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})
}

func TestGlobMatchingShouldNotMatch(t *testing.T) {
	t.Run("Should not match nested dir/file pattern without wildcards", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "bin/file"
		filePath := "/fs/root/data/bin/file"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Should not match dir/file by part of the name", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "logs"
		filePath := "/fs/root/data/my-logs"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})
}
