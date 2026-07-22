package node

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestParseBinField covers every shape package.json "bin" can take. The names it returns are
// substring-matched against npm scripts to decide whether a dependency is used, so a wrong
// or missing name surfaces as a dependency wrongly reported unused.
func TestParseBinField(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		moduleName string
		want       []string
	}{
		{
			name:       "string form is named after the package",
			raw:        `"./cli.js"`,
			moduleName: "eslint",
			want:       []string{"eslint"},
		},
		{
			name:       "string form drops the scope",
			raw:        `"./bin/biome"`,
			moduleName: "@biomejs/biome",
			want:       []string{"biome"},
		},
		{
			name:       "object form uses its keys, sorted",
			raw:        `{"tsc":"./bin/tsc","tsserver":"./bin/tsserver"}`,
			moduleName: "typescript",
			want:       []string{"tsc", "tsserver"},
		},
		{
			name:       "object keys are sorted regardless of source order",
			raw:        `{"zzz":"./z.js","aaa":"./a.js","mmm":"./m.js"}`,
			moduleName: "pkg",
			want:       []string{"aaa", "mmm", "zzz"},
		},
		{
			name:       "object form keys are used verbatim for scoped packages",
			raw:        `{"biome":"./bin/biome"}`,
			moduleName: "@biomejs/biome",
			want:       []string{"biome"},
		},
		{
			name:       "absent bin declares nothing",
			raw:        ``,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "null bin declares nothing",
			raw:        `null`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "empty string bin declares nothing",
			raw:        `""`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "empty object bin declares nothing",
			raw:        `{}`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "array bin is malformed and declares nothing",
			raw:        `["./cli.js"]`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "number bin is malformed and declares nothing",
			raw:        `123`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "object with non-string values declares nothing",
			raw:        `{"cli":{"nested":"./x.js"}}`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "truncated json declares nothing",
			raw:        `{"cli":`,
			moduleName: "lodash",
			want:       nil,
		},
		{
			name:       "leading whitespace is tolerated",
			raw:        "  \n\t" + `{"cli":"./cli.js"}`,
			moduleName: "pkg",
			want:       []string{"cli"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseBinField(json.RawMessage(c.raw), c.moduleName)
			if !slices.Equal(got, c.want) {
				t.Errorf("parseBinField(%q, %q) = %v, want %v", c.raw, c.moduleName, got, c.want)
			}
		})
	}
}

// TestParseBinFieldIsDeterministic pins the sorting of object-form keys. Before it was
// added the names came out in Go's randomised map iteration order.
func TestParseBinFieldIsDeterministic(t *testing.T) {
	raw := json.RawMessage(`{"e":"./e.js","d":"./d.js","c":"./c.js","b":"./b.js","a":"./a.js"}`)
	want := []string{"a", "b", "c", "d", "e"}

	for run := 0; run < 50; run++ {
		if got := parseBinField(raw, "pkg"); !slices.Equal(got, want) {
			t.Fatalf("run %d: got %v, want %v", run, got, want)
		}
	}
}

// TestBinaryNameForModule pins the scope-stripping rule on its own, away from JSON parsing.
func TestBinaryNameForModule(t *testing.T) {
	cases := map[string]string{
		"@biomejs/biome": "biome",
		"@scope/foo":     "foo",
		"eslint":         "eslint",
		// Not a scope, so the name stays whole.
		"foo/bar": "foo/bar",
		// Malformed, but must not panic or slice out of range.
		"@":      "@",
		"@scope": "@scope",
		"":       "",
	}

	for moduleName, want := range cases {
		if got := binaryNameForModule(moduleName); got != want {
			t.Errorf("binaryNameForModule(%q) = %q, want %q", moduleName, got, want)
		}
	}
}
