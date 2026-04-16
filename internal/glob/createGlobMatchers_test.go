package globutil

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

func TestGlobMatchingWithRootAnchoredPattern(t *testing.T) {
	t.Run("Root-anchored /node_modules should match root directory contents", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "/node_modules"
		filePath := "/fs/root/node_modules/pkg/index.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Root-anchored /node_modules should not match nested directory", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "/node_modules"
		filePath := "/fs/root/sub/node_modules/pkg/index.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Root-anchored /config should not match nested directory", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "/config"
		filePath := "/fs/root/js/projects/app/config"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})
}

func TestGlobMatchingWithRelativePattern(t *testing.T) {
	testCases := []struct {
		name    string
		root    string
		pattern string
		filePath string
		match   bool
	}{
		{"Relative pattern ../node_modules should match parent directory contents", "/fs/root/dir", "../node_modules", "/fs/root/node_modules/pkg/index.js", true},
		{"Relative pattern ../node_modules should not match nested directory", "/fs/root/dir", "../node_modules", "/fs/root/sub/node_modules/pkg/index.js", false},
		{"Relative pattern ../config should not match nested child directory inside parent", "/fs/root/", "../config", "/fs/root/js/projects/app/config/file.js", false},
		{"Relative pattern ./same-dir/config/file.js should match correctly", "/fs/root/project/", "./same-dir/config/file.js", "/fs/root/project/same-dir/config/file.js", true},
		{"Relative pattern ./same-dir/config/file.js should not match when root points to different directory", "/fs/root/", "./same-dir/config/file.js", "/fs/root/project/same-dir/config/file.js", false},
		{"Relative pattern ../../ should match second level parent directory", "/fs/root/project/same-dir/", "../../", "/fs/root/file.js", true},
		{"Relative pattern ../../project should not match file in second level parent directory", "/fs/root/project/same-dir/", "../../project/", "/fs/root/project/same-dir/config/file.js", true},
		{"Relative pattern ../../project should match file in second level parent directory", "/fs/root/project/same-dir/", "../../project/", "/fs/root/project/file.js", true},
		{"Relative pattern ../../project/**/file.* should match file", "/fs/root/project/same-dir/", "../../project/**/file.*", "/fs/root/project/same-dir/config/file.js", true},
		{"Relative pattern ./config/ should match files under current root config dir", "/fs/root/project/", "./config/", "/fs/root/project/config/settings.json", true},
		{"Relative pattern ./config/ should not match sibling config dir", "/fs/root/project/", "./config/", "/fs/root/config/settings.json", false},
		{"Relative pattern ../config/ should match files under parent config dir", "/fs/root/project/", "../config/", "/fs/root/config/settings.json", true},
		{"Relative pattern ./ should match files under current root", "/fs/root/project/", "./", "/fs/root/project/readme.md", true},
		{"Relative pattern ../ should match files under parent root", "/fs/root/project/", "../", "/fs/root/readme.md", true},
		{"Relative pattern .. should match files under parent root", "/fs/root/project/", "..", "/fs/root/readme.md", true},
		{"Relative pattern ../.. should match files under grandparent root", "/fs/root/project/same-dir/", "../..", "/fs/root/readme.md", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globMatchers := CreateGlobMatchers([]string{tc.pattern}, tc.root)
			matches := MatchesAnyGlobMatcher(tc.filePath, globMatchers, debug)
			if matches != tc.match {
				if tc.match {
					t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, tc.pattern, tc.filePath)
				} else {
					t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, tc.pattern, tc.filePath)
				}
			}
		})
	}
}
