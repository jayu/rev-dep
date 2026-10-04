package fs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	globutil "rev-dep-go/internal/glob"
)

// TestParseGitIgnoreMatching exercises .gitignore parsing end-to-end through the shared glob engine,
// covering positive paths (ignored) and negative paths (re-included via a `!` exception), plus
// comment/blank-line handling.
func TestParseGitIgnoreMatching(t *testing.T) {
	dirPath := "/repo/"
	gitignore := "" +
		"# dependencies\n" +
		"node_modules/\n" +
		"\n" + // blank line must be skipped
		"*.log\n" +
		"# build output, but keep the manifest\n" +
		"dist/\n" +
		"!dist/manifest.json\n"

	matchers := ParseGitIgnore(gitignore, dirPath)

	cases := []struct {
		name     string
		filePath string
		ignored  bool // expected result of the exclude match
	}{
		// positive paths - should be ignored
		{"node_modules is ignored", "/repo/node_modules/react/index.js", true},
		{"log file is ignored", "/repo/server.log", true},
		{"nested log file is ignored", "/repo/logs/error.log", true},
		{"dist output is ignored", "/repo/dist/app.js", true},
		{"nested dist output is ignored", "/repo/dist/assets/main.css", true},

		// `dist/` excludes the *directory*, and gitignore cannot re-include a file whose
		// parent directory is excluded - so the `!dist/manifest.json` exception has no
		// effect here. Verified with `git check-ignore` (git 2.39.5). Had the pattern been
		// `dist/**` (which does not match bare `dist`) the exception would apply.
		{"exception cannot re-include under an excluded dir", "/repo/dist/manifest.json", true},

		// negative paths - should NOT be ignored
		{"source file is not ignored", "/repo/src/index.ts", false},
		{"a file named like a comment is not ignored", "/repo/dependencies.ts", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := globutil.MatchesAnyGlobMatcher(tc.filePath, matchers, false)
			if got != tc.ignored {
				t.Errorf("path %q: got ignored=%v, want %v", tc.filePath, got, tc.ignored)
			}
		})
	}
}

// TestParseGitIgnoreSkipsCommentsAndBlanks verifies non-pattern lines never become matchers.
func TestParseGitIgnoreSkipsCommentsAndBlanks(t *testing.T) {
	matchers := ParseGitIgnore("# only a comment\n\n   \n", "/repo/")
	if len(matchers) != 0 {
		t.Fatalf("expected 0 matchers for a comment/blank-only .gitignore, got %d", len(matchers))
	}
	if globutil.MatchesAnyGlobMatcher("/repo/anything.ts", matchers, false) {
		t.Error("comment/blank-only .gitignore must not match anything")
	}
}

// TestGetFilesWithExclusions_PrunesRecursivelyExcludedDir verifies the walk stops at a
// directory fully covered by a `<dir>/**` pattern instead of enumerating it: the directory
// appears in PrunedDirs, none of its files are visited (so they are neither discovered nor
// recorded as individually excluded), while a file individually excluded by a non-recursive
// pattern IS recorded.
func TestGetFilesWithExclusions_PrunesRecursivelyExcludedDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("export const x = 1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("src/index.ts")
	mustWrite("src/used.ts")
	mustWrite("build/a.ts")
	mustWrite("build/nested/b.ts")

	matchers := globutil.CreateGlobMatchers([]string{"build/**", "src/used.ts"}, dir)
	files, exclusions := GetFilesWithExclusions(dir, matchers, nil)

	contains := func(list []string, suffix string) bool {
		for _, item := range list {
			if strings.HasSuffix(item, suffix) {
				return true
			}
		}
		return false
	}

	if !contains(files, "src/index.ts") {
		t.Errorf("expected src/index.ts to be discovered, got %v", files)
	}
	if contains(files, "build/a.ts") || contains(files, "build/nested/b.ts") {
		t.Errorf("files under a pruned dir must not be discovered, got %v", files)
	}
	// build/* were pruned, so they must NOT be enumerated as individually excluded files.
	if contains(exclusions.ExcludedFiles, "build/a.ts") || contains(exclusions.ExcludedFiles, "build/nested/b.ts") {
		t.Errorf("pruned dir contents must not appear in ExcludedFiles (would mean we descended), got %v", exclusions.ExcludedFiles)
	}
	if !contains(exclusions.PrunedDirs, "build") {
		t.Errorf("expected build to be recorded as a pruned dir, got %v", exclusions.PrunedDirs)
	}
	// src/used.ts is excluded by a non-recursive pattern, so it is visited and recorded.
	if !contains(exclusions.ExcludedFiles, "src/used.ts") {
		t.Errorf("expected src/used.ts in ExcludedFiles, got %v", exclusions.ExcludedFiles)
	}
}

// TestGitIgnoreLookupStopsAtGitFile covers issue #46: linked worktrees and submodules mark their
// root with a .git file, and the enclosing checkout's .gitignore must not apply across it.
func TestGitIgnoreLookupStopsAtGitFile(t *testing.T) {
	tests := []struct {
		name      string
		gitEntry  map[string]string
		wantFiles []string
	}{
		{
			name:      "linked worktree .git file",
			gitEntry:  map[string]string{"outer/wt/.git": "gitdir: /elsewhere/.git/worktrees/wt\n"},
			wantFiles: []string{"src/a.ts"},
		},
		{
			name:      ".git directory",
			gitEntry:  map[string]string{"outer/wt/.git/HEAD": "ref: refs/heads/main\n"},
			wantFiles: []string{"src/a.ts"},
		},
		{
			name:      ".git file that is not a gitdir pointer",
			gitEntry:  map[string]string{"outer/wt/.git": "not a checkout\n"},
			wantFiles: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{
				"outer/.git/HEAD":         "ref: refs/heads/main\n",
				"outer/.gitignore":        "wt/\n",
				"outer/wt/.gitignore":     "ignored.ts\n",
				"outer/wt/src/a.ts":       "export const a = 1;\n",
				"outer/wt/src/ignored.ts": "export const b = 1;\n",
			}
			for path, content := range tt.gitEntry {
				files[path] = content
			}
			root := filepath.Join(writeTree(t, files), "outer", "wt")

			var got []string
			for _, file := range GetFiles(root, nil, FindAndProcessGitIgnoreFilesUpToRepoRoot(root), nil) {
				rel, err := filepath.Rel(root, file)
				if err != nil {
					t.Fatalf("Rel(%q): %v", file, err)
				}
				got = append(got, filepath.ToSlash(rel))
			}
			if !slices.Equal(got, tt.wantFiles) {
				t.Errorf("GetFiles() = %v, want %v", got, tt.wantFiles)
			}
		})
	}
}
