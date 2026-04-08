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
	t.Run("Relative pattern ../node_modules should match parent directory contents", func(t *testing.T) {
		root := "/fs/root/dir"
		pattern := "../node_modules"
		filePath := "/fs/root/node_modules/pkg/index.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" not matching path "%s"`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../node_modules should not match nested directory", func(t *testing.T) {
		root := "/fs/root/dir"
		pattern := "../node_modules"
		filePath := "/fs/root/sub/node_modules/pkg/index.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../config should not match nested child directory inside parent", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "../config"
		filePath := "/fs/root/js/projects/app/config/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ./same-dir/config/file.js should match correctly", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "./same-dir/config/file.js"
		filePath := "/fs/root/project/same-dir/config/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ./same-dir/config/file.js should not match when root points to different directory", func(t *testing.T) {
		root := "/fs/root/"
		pattern := "./same-dir/config/file.js"
		filePath := "/fs/root/project/same-dir/config/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../../ should match second level parent directory", func(t *testing.T) {
		root := "/fs/root/project/same-dir/"
		pattern := "../../"
		filePath := "/fs/root/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../../project should not match file in second level parent directory", func(t *testing.T) {
		root := "/fs/root/project/same-dir/"
		pattern := "../../project/"
		filePath := "/fs/root/project/same-dir/config/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../../project should match file in second level parent directory", func(t *testing.T) {
		root := "/fs/root/project/same-dir/"
		pattern := "../../project/"
		filePath := "/fs/root/project/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../../project/**/file.* should match file", func(t *testing.T) {
		root := "/fs/root/project/same-dir/"
		pattern := "../../project/**/file.*"
		filePath := "/fs/root/project/same-dir/config/file.js"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ./config/ should match files under current root config dir", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "./config/"
		filePath := "/fs/root/project/config/settings.json"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ./config/ should not match sibling config dir", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "./config/"
		filePath := "/fs/root/config/settings.json"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if matches {
			t.Errorf(`Pattern "%s" is matching path "%s" but it should not`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../config/ should match files under parent config dir", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "../config/"
		filePath := "/fs/root/config/settings.json"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ./ should match files under current root", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "./"
		filePath := "/fs/root/project/readme.md"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../ should match files under parent root", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := "../"
		filePath := "/fs/root/readme.md"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern .. should match files under parent root", func(t *testing.T) {
		root := "/fs/root/project/"
		pattern := ".."
		filePath := "/fs/root/readme.md"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})

	t.Run("Relative pattern ../.. should match files under grandparent root", func(t *testing.T) {
		root := "/fs/root/project/same-dir/"
		pattern := "../.."
		filePath := "/fs/root/readme.md"
		globMatchers := CreateGlobMatchers([]string{pattern}, root)

		matches := MatchesAnyGlobMatcher(filePath, globMatchers, debug)

		if !matches {
			t.Errorf(`Pattern "%s" is not matching path "%s" but it should`, pattern, filePath)
		}
	})
}
