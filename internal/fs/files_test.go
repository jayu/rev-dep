package fs

import (
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

		// negative paths - should NOT be ignored
		{"manifest re-included by negation", "/repo/dist/manifest.json", false},
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
