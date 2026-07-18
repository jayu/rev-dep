package globutil

import "testing"

// TestShouldTraverseDir_PrunesFullyExcludedRecursiveDirs verifies that a directory whose
// entire subtree is excluded by a `<dir>/**` pattern is pruned, even though the bare
// directory path itself does not match that pattern. Non-recursive patterns and unrelated
// directories must still be traversed so no in-scope file is dropped.
func TestShouldTraverseDir_PrunesFullyExcludedRecursiveDirs(t *testing.T) {
	root := "/repo/"
	cases := []struct {
		name         string
		pattern      string
		dir          string
		wantTraverse bool
	}{
		{"recursive-dir-pruned", "dist/**", "/repo/dist", false},
		{"nested-under-recursive-pruned", "dist/**", "/repo/dist/sub", false},
		{"sibling-not-pruned", "dist/**", "/repo/src", true},
		{"prefix-lookalike-not-pruned", "dist/**", "/repo/dist-tools", true},
		{"non-recursive-glob-not-pruned", "dist/*.js", "/repo/dist", true},
		{"bare-name-still-pruned", "node_modules", "/repo/node_modules", false},
		{"match-all-prunes", "**", "/repo/anything", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			matchers := CreateGlobMatchers([]string{c.pattern}, root)
			got := ShouldTraverseDir(c.dir, matchers, nil, nil)
			if got != c.wantTraverse {
				t.Errorf("ShouldTraverseDir(%q, [%q]) = %v, want %v", c.dir, c.pattern, got, c.wantTraverse)
			}
		})
	}
}

// TestShouldTraverseDir_NegationVetoesStructuralPrune ensures a re-inclusion (`!keep`)
// anywhere in the set disables the structural whole-subtree prune, so the walk still
// descends and the file-by-file matcher can honour the exception.
func TestShouldTraverseDir_NegationVetoesStructuralPrune(t *testing.T) {
	matchers := CreateGlobMatchers([]string{"dist/**", "!dist/keep.ts"}, "/repo/")
	if !ShouldTraverseDir("/repo/dist", matchers, nil, nil) {
		t.Fatalf("expected /repo/dist to be traversed when a negation could re-include a descendant")
	}
}

// TestShouldTraverseDir_IncludePrefixRescuesPrunedDir ensures an include prefix (e.g. a
// processIgnoredFiles re-inclusion) under a structurally-excluded directory forces
// traversal.
func TestShouldTraverseDir_IncludePrefixRescuesPrunedDir(t *testing.T) {
	excludes := CreateGlobMatchers([]string{"dist/**"}, "/repo/")
	includes := CreateGlobMatchers([]string{"dist/keep/**"}, "/repo/")
	prefixes := BuildIncludePrefixes(includes)
	if !ShouldTraverseDir("/repo/dist", excludes, includes, prefixes) {
		t.Fatalf("expected /repo/dist to be traversed when an include prefix lives under it")
	}
}
