package globutil

import (
	"strings"
	"testing"
)

// A path deeper than the fixed slash buffer in anyMatches must still match a pattern that
// only hits a directory near the bottom - the fallback path.
func TestDeepPathBeyondSlashBuffer(t *testing.T) {
	for _, depth := range []int{5, 23, 24, 25, 40, 80} {
		segs := make([]string, depth)
		for i := range segs {
			segs[i] = "d"
		}
		segs[depth-1] = "node_modules"
		path := "/repo/" + strings.Join(segs, "/") + "/pkg/index.js"
		m := CreateGlobMatchers([]string{"node_modules"}, "/repo")
		if !MatchesAnyGlobMatcher(path, m, false) {
			t.Errorf("depth %d: pattern %q did not match %q", depth, "node_modules", path)
		}
		m2 := CreateGlobMatchers([]string{"nomatch-xyz"}, "/repo")
		if MatchesAnyGlobMatcher(path, m2, false) {
			t.Errorf("depth %d: unrelated pattern wrongly matched", depth)
		}
	}
}
