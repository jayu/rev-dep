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
		name     string
		root     string
		pattern  string
		filePath string
		match    bool
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

// TestGlobSeparatorSemantics locks in the '/'-separator behavior: a single '*'
// (and '?') matches within a single path segment only, while '**', plain
// directory names and trailing-slash directories still cross '/'. Each case
// mirrors the behavior matrix documented for the separator fix. Paths are
// analogous to (not copied from) real-world configs.
func TestGlobSeparatorSemantics(t *testing.T) {
	root := "/fs/root/"
	testCases := []struct {
		name     string
		pattern  string
		filePath string
		match    bool
	}{
		// Single '*' must not cross '/' (the reported deny-files bug class).
		{"'**/use*.ts' does not match a file nested under a use* directory", "**/use*.ts", "/fs/root/src/hooks/useThing/common.ts", false},
		{"'**/use*/*.ts' does match a file nested under a use* directory", "**/use*/*.ts", "/fs/root/src/hooks/useThing/common.ts", true},
		{"'**/use*.ts' still matches a real use*.ts file", "**/use*.ts", "/fs/root/src/hooks/useThing.ts", true},

		// '*' is exactly one segment, never empty-across-slash and never multi-segment.
		{"'src/*/index.ts' does not match a file directly in src", "src/*/index.ts", "/fs/root/src/index.ts", false},
		{"'src/*/index.ts' matches a file one directory deep", "src/*/index.ts", "/fs/root/src/components/index.ts", true},
		{"'a/*/b' does not match across two directories", "a/*/b", "/fs/root/a/x/y/b", false},
		{"'a/*/b' matches across exactly one directory", "a/*/b", "/fs/root/a/x/b", true},

		// '?' is a single non-separator character.
		{"'foo?bar.ts' does not let '?' match '/'", "foo?bar.ts", "/fs/root/foo/bar.ts", false},
		{"'foo?bar.ts' matches a single character", "foo?bar.ts", "/fs/root/fooXbar.ts", true},

		// Bare wildcard patterns are segment-anchored; use '**/' for any depth.
		{"bare '*.ts' does not match a nested file", "*.ts", "/fs/root/src/a/b.ts", false},
		{"bare '*.ts' matches a root-level file", "*.ts", "/fs/root/index.ts", true},
		{"'**/*.ts' matches a root-level file", "**/*.ts", "/fs/root/index.ts", true},
		{"'**/*.ts' matches a nested file", "**/*.ts", "/fs/root/src/a/b.ts", true},

		// Idiomatic ways to express a recursive directory still work.
		{"'dist*' does not cross '/' into directory contents", "dist*", "/fs/root/dist/bundle.js", false},
		{"'dist/**' matches directory contents recursively", "dist/**", "/fs/root/dist/bundle.js", true},
		{"plain 'dist' name matches directory contents", "dist", "/fs/root/dist/bundle.js", true},
		{"trailing-slash 'dist/' matches directory contents recursively", "dist/", "/fs/root/dist/sub/x.js", true},
		{"'src/*' matches a direct child", "src/*", "/fs/root/src/a.ts", true},
		{"'src/*' does not match a nested file", "src/*", "/fs/root/src/a/b.ts", false},
		{"'src/**' matches a nested file", "src/**", "/fs/root/src/a/b.ts", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globMatchers := CreateGlobMatchers([]string{tc.pattern}, root)
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

// A pattern is interpreted relative to its root (the workspace). An unanchored
// wildcard like "**/api/**" must only match files INSIDE that root - not files in
// sibling workspaces. Escaping the workspace must be explicit via "../".
// The `root` is always /repo/apps/web; files under /repo/apps/mobile are foreign.
func TestGlobMatchingScopedToWorkspaceRoot(t *testing.T) {
	root := "/repo/apps/web/"
	testCases := []struct {
		name     string
		pattern  string
		filePath string
		match    bool
	}{
		// leading "**/" wildcard
		{"leading-** in-workspace matches", "**/api/**", "/repo/apps/web/src/api/file.ts", true},
		{"leading-** other-workspace must NOT match", "**/api/**", "/repo/apps/mobile/path/api/file.ts", false},

		// "**/*.ext" (devEntryPoints style)
		{"**/*.test.* in-workspace matches", "**/*.test.*", "/repo/apps/web/x.test.ts", true},
		{"**/*.test.* other-workspace must NOT match", "**/*.test.*", "/repo/apps/mobile/x.test.ts", false},

		// bare directory name (gitignore-style)
		{"bare name in-workspace matches", "api", "/repo/apps/web/api/file.ts", true},
		{"bare name other-workspace must NOT match", "api", "/repo/apps/mobile/api/file.ts", false},

		// patterns that are already scoped today (regression guards)
		{"src/** is already scoped (other-workspace no match)", "src/**", "/repo/apps/mobile/src/file.ts", false},
		{"root-anchored is already scoped (other-workspace no match)", "/src/api/**", "/repo/apps/mobile/src/api/file.ts", false},

		// explicit escape must still work
		{"explicit ../ escape into sibling workspace matches", "../mobile/**", "/repo/apps/mobile/file.ts", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globMatchers := CreateGlobMatchers([]string{tc.pattern}, root)
			matches := MatchesAnyGlobMatcher(tc.filePath, globMatchers, debug)
			if matches != tc.match {
				if tc.match {
					t.Errorf(`Pattern "%s" (root %s) is not matching "%s" but it should`, tc.pattern, root, tc.filePath)
				} else {
					t.Errorf(`Pattern "%s" (root %s) is matching "%s" but it should not (cross-workspace leak)`, tc.pattern, root, tc.filePath)
				}
			}
		})
	}
}

// TestGlobMatchingRelativeEscapePatterns gives full coverage to the documented
// escape hatch: to match files in another workspace you must use an explicit
// relative ("../") pattern. Each of these four pattern shapes is valid and must
// match files according to its semantics (and must NOT match outside its rebased
// root). Root is always /repo/apps/web; "../" -> /repo/apps, "../../" -> /repo.
func TestGlobMatchingRelativeEscapePatterns(t *testing.T) {
	root := "/repo/apps/web/"
	testCases := []struct {
		name     string
		pattern  string
		filePath string
		match    bool
	}{
		// "../../**/api/code" -> root /repo, glob "**/api/code": api/code at any depth under repo.
		{"../../**/api/code matches sibling-workspace api/code", "../../**/api/code", "/repo/apps/mobile/api/code", true},
		{"../../**/api/code matches deeply-nested api/code", "../../**/api/code", "/repo/services/billing/api/code", true},
		{"../../**/api/code does not match outside repo root", "../../**/api/code", "/elsewhere/api/code", false},

		// "../../api/**/code" -> root /repo, glob "api/**/code": api directly under repo, code below.
		{"../../api/**/code matches api/<nested>/code under repo", "../../api/**/code", "/repo/api/v1/code", true},
		{"../../api/**/code matches deeper nesting", "../../api/**/code", "/repo/api/v1/handlers/code", true},
		{"../../api/**/code does not match when api is not directly under repo", "../../api/**/code", "/repo/apps/api/v1/code", false},

		// "../../**" -> root /repo, glob "**": everything under repo.
		{"../../** matches any file under repo root", "../../**", "/repo/anything/deep/file.ts", true},
		{"../../** matches a repo-root-level file", "../../**", "/repo/file.ts", true},
		{"../../** does not match outside repo root", "../../**", "/elsewhere/file.ts", false},

		// "../api/**" -> root /repo/apps, glob "api/**": the apps-level api dir only.
		{"../api/** matches /repo/apps/api contents", "../api/**", "/repo/apps/api/client.ts", true},
		{"../api/** matches nested /repo/apps/api contents", "../api/**", "/repo/apps/api/v2/client.ts", true},
		{"../api/** does not match this workspace's own api dir", "../api/**", "/repo/apps/web/api/client.ts", false},
		{"../api/** does not match api under a different parent", "../api/**", "/repo/services/api/client.ts", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globMatchers := CreateGlobMatchers([]string{tc.pattern}, root)
			matches := MatchesAnyGlobMatcher(tc.filePath, globMatchers, debug)
			if matches != tc.match {
				if tc.match {
					t.Errorf(`Pattern "%s" (root %s) is not matching "%s" but it should`, tc.pattern, root, tc.filePath)
				} else {
					t.Errorf(`Pattern "%s" (root %s) is matching "%s" but it should not`, tc.pattern, root, tc.filePath)
				}
			}
		})
	}
}

func TestGlobNegation(t *testing.T) {
	root := "/fs/root/"
	testCases := []struct {
		name     string
		patterns []string
		filePath string
		match    bool
	}{
		{"positive matches, no exception", []string{"src/**", "!src/vendor/**"}, "/fs/root/src/app/x.ts", true},
		{"exception cancels a positive", []string{"src/**", "!src/vendor/**"}, "/fs/root/src/vendor/y.ts", false},

		// gitignore-style directory pattern re-included by an exception
		{"dir pattern matches", []string{"build/", "!build/keep/**"}, "/fs/root/build/foo.ts", true},
		{"dir pattern re-included by exception", []string{"build/", "!build/keep/**"}, "/fs/root/build/keep/bar.ts", false},

		// a negation only cancels the target it names, not other positives
		{"negation is scoped to its own subtree", []string{"a/**", "b/**", "!a/skip/**"}, "/fs/root/b/skip/x.ts", true},
		{"negation cancels its own subtree", []string{"a/**", "b/**", "!a/skip/**"}, "/fs/root/a/skip/x.ts", false},

		// anchored negation
		{"anchored positive matches", []string{"/dist/**", "!/dist/public/**"}, "/fs/root/dist/app.ts", true},
		{"anchored exception cancels", []string{"/dist/**", "!/dist/public/**"}, "/fs/root/dist/public/app.ts", false},

		// a set with only negations matches nothing (no positive to cancel)
		{"only negations match nothing", []string{"!src/**"}, "/fs/root/src/app/x.ts", false},

		// order independence: exception declared before the positive still applies
		{"exception before positive still cancels", []string{"!src/vendor/**", "src/**"}, "/fs/root/src/vendor/y.ts", false},

		// a lone "!" is skipped and does not disable the sibling positive
		{"lone bang is ignored", []string{"file.ts", "!"}, "/fs/root/file.ts", true},

		// escaped leading bang is a literal '!', not a negation
		{"escaped bang is literal, not negation", []string{"\\!keep.ts"}, "/fs/root/!keep.ts", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			globMatchers := CreateGlobMatchers(tc.patterns, root)
			matches := MatchesAnyGlobMatcher(tc.filePath, globMatchers, debug)
			if matches != tc.match {
				t.Errorf(`Patterns %v against path "%s": got match=%v, want %v`, tc.patterns, tc.filePath, matches, tc.match)
			}
		})
	}
}
