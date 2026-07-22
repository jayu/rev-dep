package node

import "testing"

// TestBinaryWordMatcher pins which occurrences of a binary name count as a real usage.
// A false match here keeps an unused package forever; a missed match reports a used
// package as unused.
func TestBinaryWordMatcher(t *testing.T) {
	cases := []struct {
		binary  string
		content string
		want    bool
	}{
		// Real usages.
		{"biome", "biome check .", true},
		{"biome", "pnpm biome check", true},
		{"biome", "./node_modules/.bin/biome --write", true},
		{"biome", "npx biome", true},
		{"biome", "run(biome)", true},
		{"biome", "lint && biome", true},
		{"tsc", "tsc --noEmit && jest", true},
		{"c8", "c8 --reporter=text jest", true},
		// The Bug 2 false matches.
		{"pack", "webpack --mode production", false},
		{"rm", "prettier --write && format", false},
		{"ts", "ts-node script.ts", false},
		{"next", "nextra build", false},
		// "-" is not a boundary, so hyphenated tools stay distinct.
		{"npm", "npm-run-all lint test", false},
		{"run-all", "npm-run-all lint test", false},
		{"npm-run-all", "npm-run-all lint test", true},
		// "." is never a boundary: a filename containing the name is not an invocation.
		{"ts", "tsc -p . && node build.ts", false},
		{"js", "esbuild src/index.js", false},
		{"pack", "node scripts/pack.js", false},
		{"biome", "cat biome.json", false},
		{"eslint", "eslint.config.js", false},
		// A trailing "." is still a boundary when the name ends the command.
		{"tsc", "tsc -p .", true},
		// "_" is a word character.
		{"tsc", "my_tsc_wrapper", false},
		// Names with regexp metacharacters must be quoted, not interpreted.
		{"a.c", "abc", false},
		{"a.c", "a.c --flag", true},
		{"@scope/foo", "@scope/foo --flag", true},
		// Empty content.
		{"biome", "", false},
	}

	for _, c := range cases {
		if got := binaryWordMatcher(c.binary).MatchString(c.content); got != c.want {
			t.Errorf("binary %q in %q = %v, want %v", c.binary, c.content, got, c.want)
		}
	}
}

// TestBinaryWordMatcherIsCached pins that the same name returns the same compiled regexp,
// so the hot path never recompiles.
func TestBinaryWordMatcherIsCached(t *testing.T) {
	if binaryWordMatcher("cached-binary") != binaryWordMatcher("cached-binary") {
		t.Error("binaryWordMatcher recompiled a name it had already seen")
	}
}
