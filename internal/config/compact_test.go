package config

import (
	"strings"
	"testing"
)

func TestCompactConfigText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "fold enabled true to boolean",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","circularImportsDetection":{"enabled":true}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","circularImportsDetection":true}]}`,
		},
		{
			name: "fold enabled false to boolean",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","orphanFilesDetection":{"enabled":false}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","orphanFilesDetection":false}]}`,
		},
		{
			name: "empty object folds to true",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","unresolvedImportsDetection":{}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","unresolvedImportsDetection":true}]}`,
		},
		{
			name: "drop redundant enabled true when leading option",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"enabled":true,"entryPoints":["a.ts"]}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "drop redundant enabled true when trailing option",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"],"enabled":true}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "keep disabled object with options",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","orphanFilesDetection":{"enabled":false,"validEntryPoints":["a.ts"]}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","orphanFilesDetection":{"enabled":false,"validEntryPoints":["a.ts"]}}]}`,
		},
		{
			name: "already compact object without enabled is unchanged",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "already boolean is unchanged",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","circularImportsDetection":true}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","circularImportsDetection":true}]}`,
		},
		{
			name: "array elements: drop enabled but do not fold pure enabled element",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":[{"enabled":true},{"enabled":true,"excludeModules":["ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":[{"enabled":true},{"excludeModules":["ts"]}]}]}`,
		},
		{
			name: "unwrap single-element array with options",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[{"entryPoints":["a.ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "unwrap single-element array and drop redundant enabled",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[{"enabled":true,"entryPoints":["a.ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "unwrap single-element array to boolean shorthand",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":[{"enabled":true}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":true}]}`,
		},
		{
			name: "unwrap single-element boolean array",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":[false]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","unusedNodeModulesDetection":false}]}`,
		},
		{
			name: "unwrap keeps disabled element with options as object",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[{"enabled":false,"entryPoints":["a.ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":{"enabled":false,"entryPoints":["a.ts"]}}]}`,
		},
		{
			name: "multi-element array is not unwrapped",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[{"entryPoints":["a.ts"]},{"entryPoints":["b.ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[{"entryPoints":["a.ts"]},{"entryPoints":["b.ts"]}]}]}`,
		},
		{
			name: "single-element array with inner comment is left as array",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[/* keep */{"enabled":true,"entryPoints":["a.ts"]}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","restrictedImportsDetection":[/* keep */{"entryPoints":["a.ts"]}]}]}`,
		},
		{
			name: "non-detector fields are untouched",
			in:   `{"configVersion":"1.11","workspaces":[{"path":".","moduleBoundaries":[{"name":"src","pattern":"src/**/*","enabled":true}]}]}`,
			want: `{"configVersion":"1.11","workspaces":[{"path":".","moduleBoundaries":[{"name":"src","pattern":"src/**/*","enabled":true}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompactConfigText([]byte(tt.in))
			if err != nil {
				t.Fatalf("CompactConfigText error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("mismatch\n in:   %s\n got:  %s\n want: %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestCompactConfigText_PreservesCommentsAndFormatting(t *testing.T) {
	in := `{
  // top-level config for the app
  "configVersion": "1.11",
  "workspaces": [
    {
      "path": ".",
      // enable cycle detection
      "circularImportsDetection": { "enabled": true },
      "restrictedImportsDetection": {
        "enabled": true, // turn the check on
        "entryPoints": ["src/index.ts"] // app entry
      },
      "orphanFilesDetection": { "enabled": false }
    }
  ]
}`
	want := `{
  // top-level config for the app
  "configVersion": "1.11",
  "workspaces": [
    {
      "path": ".",
      // enable cycle detection
      "circularImportsDetection": true,
      "restrictedImportsDetection": {
        "entryPoints": ["src/index.ts"] // app entry
      },
      "orphanFilesDetection": false
    }
  ]
}`
	got, err := CompactConfigText([]byte(in))
	if err != nil {
		t.Fatalf("CompactConfigText error: %v", err)
	}
	if string(got) != want {
		t.Errorf("mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// The surviving structural comments must still be present.
	for _, c := range []string{"top-level config for the app", "enable cycle detection", "app entry"} {
		if !strings.Contains(string(got), c) {
			t.Errorf("expected comment %q to be preserved", c)
		}
	}
}

// TestCompactConfigText_UnwrapReindents verifies that unwrapping a nested, multi-line single-element
// detector array corrects the indentation that the removed array level left behind, honoring the
// file's own indentation width.
func TestCompactConfigText_UnwrapReindents(t *testing.T) {
	t.Run("two-space indentation", func(t *testing.T) {
		in := `{
  "configVersion": "1.11",
  "workspaces": [
    {
      "path": ".",
      "restrictedImportsDetection": [
        {
          "enabled": true,
          "entryPoints": ["a.ts"]
        }
      ]
    }
  ]
}`
		want := `{
  "configVersion": "1.11",
  "workspaces": [
    {
      "path": ".",
      "restrictedImportsDetection": {
        "entryPoints": ["a.ts"]
      }
    }
  ]
}`
		got, err := CompactConfigText([]byte(in))
		if err != nil {
			t.Fatalf("CompactConfigText error: %v", err)
		}
		if string(got) != want {
			t.Errorf("mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
		}
	})

	t.Run("four-space indentation", func(t *testing.T) {
		in := `{
    "configVersion": "1.11",
    "workspaces": [
        {
            "path": ".",
            "restrictedImportsDetection": [
                {
                    "entryPoints": ["a.ts"]
                }
            ]
        }
    ]
}`
		want := `{
    "configVersion": "1.11",
    "workspaces": [
        {
            "path": ".",
            "restrictedImportsDetection": {
                "entryPoints": ["a.ts"]
            }
        }
    ]
}`
		got, err := CompactConfigText([]byte(in))
		if err != nil {
			t.Fatalf("CompactConfigText error: %v", err)
		}
		if string(got) != want {
			t.Errorf("mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
		}
	})
}

// TestCompactConfigText_SemanticEquivalence verifies that compacting does not change how a config
// parses, and that compaction is idempotent.
func TestCompactConfigText_SemanticEquivalence(t *testing.T) {
	verbose := `{
  "configVersion": "1.11",
  "workspaces": [
    {
      "path": ".",
      "circularImportsDetection": { "enabled": true },
      "orphanFilesDetection": { "enabled": false },
      "unresolvedImportsDetection": { "enabled": true },
      "restrictedImportsDetection": { "enabled": true, "entryPoints": ["src/index.ts"], "denyFiles": ["src/secret.ts"] },
      "unusedExportsDetection": { "enabled": false, "validEntryPoints": ["src/index.ts"] }
    }
  ]
}`
	compacted, err := CompactConfigText([]byte(verbose))
	if err != nil {
		t.Fatalf("CompactConfigText error: %v", err)
	}

	before, err := ParseConfig([]byte(verbose))
	if err != nil {
		t.Fatalf("parse verbose: %v", err)
	}
	after, err := ParseConfig(compacted)
	if err != nil {
		t.Fatalf("parse compacted: %v\n%s", err, compacted)
	}

	rb, ra := before.Rules[0], after.Rules[0]
	assertSameEnabled(t, "circular", rb.CircularImportsDetections[0].IsEnabled(), ra.CircularImportsDetections[0].IsEnabled())
	assertSameEnabled(t, "orphan", rb.OrphanFilesDetections[0].IsEnabled(), ra.OrphanFilesDetections[0].IsEnabled())
	assertSameEnabled(t, "unresolved", rb.UnresolvedImportsDetections[0].IsEnabled(), ra.UnresolvedImportsDetections[0].IsEnabled())
	assertSameEnabled(t, "restrictedImports", rb.RestrictedImportsDetections[0].IsEnabled(), ra.RestrictedImportsDetections[0].IsEnabled())
	assertSameEnabled(t, "unusedExports", rb.UnusedExportsDetections[0].IsEnabled(), ra.UnusedExportsDetections[0].IsEnabled())
	if got := ra.RestrictedImportsDetections[0]; len(got.EntryPoints) != 1 || len(got.DenyFiles) != 1 {
		t.Errorf("restrictedImports options lost after compaction: %+v", got)
	}
	if got := ra.UnusedExportsDetections[0]; len(got.ValidEntryPoints) != 1 {
		t.Errorf("unusedExports options lost after compaction: %+v", got)
	}

	// Idempotency: compacting the compacted output changes nothing.
	again, err := CompactConfigText(compacted)
	if err != nil {
		t.Fatalf("second CompactConfigText error: %v", err)
	}
	if string(again) != string(compacted) {
		t.Errorf("compaction not idempotent\nfirst:  %s\nsecond: %s", compacted, again)
	}
}

func assertSameEnabled(t *testing.T, name string, before, after bool) {
	t.Helper()
	if before != after {
		t.Errorf("%s enabled changed: before=%v after=%v", name, before, after)
	}
}
