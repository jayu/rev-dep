package module

import (
	"strings"
	"testing"
)

func TestMatchesAnyModulePattern(t *testing.T) {
	cases := []struct {
		pattern string
		match   []string
		noMatch []string
	}{
		{
			pattern: "fs",
			match:   []string{"fs", "fs/promises"},
			noMatch: []string{"node:fs", "node:fs/promises", "fs-extra", "path"},
		},
		{
			pattern: "node:fs",
			match:   []string{"node:fs", "node:fs/promises"},
			noMatch: []string{"fs", "fs/promises", "node:fs-extra", "node:path"},
		},
		{
			pattern: "fs/*",
			match:   []string{"fs/promises"},
			noMatch: []string{"fs", "node:fs/promises"},
		},
		{
			pattern: "node:fs/promises",
			match:   []string{"node:fs/promises"},
			noMatch: []string{"node:fs", "fs/promises"},
		},
		{
			pattern: "node:*",
			match:   []string{"node:fs", "node:fs/promises", "node:test", "node:test/reporters", "node:sqlite", "node:future_module"},
			noMatch: []string{"fs", "path", "test", "lodash", "@scope/pkg", "bun:sqlite", "bun"},
		},
		{
			pattern: "bun:*",
			match:   []string{"bun:sqlite", "bun:test", "bun:future_module"},
			noMatch: []string{"bun", "node:fs", "bunyan"},
		},
		{
			pattern: "builtin:*",
			match: []string{
				"fs", "node:fs", "fs/promises", "node:fs/promises", "path", "punycode", "_stream_wrap", "freelist",
				"node:test", "node:test/reporters", "node:sqlite", "node:future_module",
				"bun", "bun:sqlite", "bun:test", "bun:future_module",
			},
			noMatch: []string{
				"lodash", "lodash/fp", "@scope/pkg", "fs-extra", "buffer/", "punycode/",
				"ws", "undici", "test", "sqlite", "ffi",
			},
		},
		{
			pattern: "*",
			match:   []string{"lodash", "@scope/pkg", "fs", "node:fs", "bun:sqlite"},
		},
		{
			pattern: "lodash",
			match:   []string{"lodash", "lodash/fp"},
			noMatch: []string{"lodash-es"},
		},
		{
			pattern: "@scope/*",
			match:   []string{"@scope/pkg", "@scope/pkg/sub"},
			noMatch: []string{"@other/pkg"},
		},
	}

	for _, c := range cases {
		patterns := CompileModulePatterns([]string{c.pattern})
		for _, request := range c.match {
			if !MatchesAnyModulePattern(patterns, request) {
				t.Errorf("pattern %q should match %q", c.pattern, request)
			}
		}
		for _, request := range c.noMatch {
			if MatchesAnyModulePattern(patterns, request) {
				t.Errorf("pattern %q should not match %q", c.pattern, request)
			}
		}
	}
}

func TestCompileModulePatterns_SkipsBlankAndInvalid(t *testing.T) {
	patterns := CompileModulePatterns([]string{"", "   ", "[", " fs "})
	if len(patterns) != 1 {
		t.Fatalf("expected 1 compiled pattern, got %d", len(patterns))
	}
	if !MatchesAnyModulePattern(patterns, "fs") {
		t.Errorf("trimmed pattern %q should match %q", " fs ", "fs")
	}
}

func TestValidateModulePattern(t *testing.T) {
	for _, pattern := range []string{"fs", "node:fs", "node:*", "bun:*", "@scope/*", "builtin:*", " builtin:* "} {
		if err := ValidateModulePattern(pattern); err != nil {
			t.Errorf("%q should be valid, got %v", pattern, err)
		}
	}

	invalid := map[string]string{
		"builtin:fs":  "'builtin:fs' is not supported: use 'builtin:*' for every built-in module",
		"builtin:":    "'builtin:' is not supported",
		"builtin:**":  "'builtin:**' is not supported",
		"[":           "invalid glob pattern '['",
		"builtin:*/x": "'builtin:*/x' is not supported",
	}
	for pattern, want := range invalid {
		err := ValidateModulePattern(pattern)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got error %v, want it to contain %q", pattern, err, want)
		}
	}
}

func TestCompileModulePatterns_AllBuiltInsKeyword(t *testing.T) {
	patterns := CompileModulePatterns([]string{"builtin:fs", "builtin:*", "lodash"})
	if len(patterns) != 2 {
		t.Fatalf("expected 2 compiled patterns, got %d", len(patterns))
	}
	for _, request := range []string{"fs", "bun:sqlite", "lodash/fp"} {
		if !MatchesAnyModulePattern(patterns, request) {
			t.Errorf("%q should match", request)
		}
	}
	if MatchesAnyModulePattern(patterns, "axios") {
		t.Errorf("%q should not match", "axios")
	}
	if MatchesAnyModulePattern(CompileModulePatterns([]string{"builtin:fs"}), "fs") {
		t.Errorf("the unsupported pattern %q should match nothing", "builtin:fs")
	}
}
