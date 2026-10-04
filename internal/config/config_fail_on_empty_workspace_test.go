package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig_FailOnEmptyWorkspace(t *testing.T) {
	cases := []struct {
		name      string
		root      string // raw JSON for the root field; "" omits it
		workspace string // raw JSON for the workspace field; "" omits it
		want      bool   // effective value for the workspace
		wantError string
	}{
		{name: "defaults to false", want: false},
		{name: "root true applies to workspace", root: "true", want: true},
		{name: "workspace false overrides root true", root: "true", workspace: "false", want: false},
		{name: "workspace true without root", workspace: "true", want: true},
		{name: "root wrong type", root: `"yes"`, wantError: "failOnEmptyWorkspace must be a boolean"},
		{name: "workspace wrong type", workspace: `1`, wantError: "workspaces[0].failOnEmptyWorkspace must be a boolean"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rootField, workspaceField := "", ""
			if c.root != "" {
				rootField = `"failOnEmptyWorkspace": ` + c.root + `,`
			}
			if c.workspace != "" {
				workspaceField = `, "failOnEmptyWorkspace": ` + c.workspace
			}
			cfg := `{
				"configVersion": "2.1",
				` + rootField + `
				"workspaces": [{ "path": "."` + workspaceField + ` }]
			}`

			parsed, err := ParseConfig([]byte(cfg))
			if c.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantError) {
					t.Fatalf("ParseConfig() error = %v, want it to contain %q", err, c.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseConfig() error = %v", err)
			}
			if got := parsed.Rules[0].FailOnEmptyWorkspace; got != c.want {
				t.Errorf("Rules[0].FailOnEmptyWorkspace = %v, want %v", got, c.want)
			}
		})
	}
}

// TestProcessConfig_FailOnEmptyWorkspace checks that an empty workspace fails the run only when
// failOnEmptyWorkspace applies to it, and that a workspace with files is never affected.
func TestProcessConfig_FailOnEmptyWorkspace(t *testing.T) {
	cwd := t.TempDir()
	for path, content := range map[string]string{
		"package.json":                     `{"name": "root"}`,
		"packages/full/package.json":       `{"name": "full"}`,
		"packages/full/src/index.ts":       "export const a = 1;\n",
		"packages/empty/package.json":      `{"name": "empty"}`,
		"packages/empty/README.md":         "no source files here\n",
		"packages/full/src/other/index.ts": "export const b = 2;\n",
	} {
		full := filepath.Join(cwd, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	cases := []struct {
		name            string
		root            string
		emptyOverride   string
		wantHasFailures bool
	}{
		{name: "default warns only", wantHasFailures: false},
		{name: "root true fails", root: `"failOnEmptyWorkspace": true,`, wantHasFailures: true},
		{name: "workspace opts out", root: `"failOnEmptyWorkspace": true,`, emptyOverride: `, "failOnEmptyWorkspace": false`, wantHasFailures: false},
		{name: "workspace opts in", emptyOverride: `, "failOnEmptyWorkspace": true`, wantHasFailures: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := ParseConfig([]byte(`{
				"configVersion": "2.1",
				` + c.root + `
				"workspaces": [
					{ "path": "packages/full" },
					{ "path": "packages/empty"` + c.emptyOverride + ` }
				]
			}`))
			if err != nil {
				t.Fatalf("ParseConfig() error = %v", err)
			}

			result, err := ProcessConfig(&cfg, cwd, false, false)
			if err != nil {
				t.Fatalf("ProcessConfig() error = %v", err)
			}

			full, empty := result.RuleResults[0], result.RuleResults[1]
			if full.IsEmpty() || full.EmptyWorkspaceFailed() {
				t.Errorf("packages/full: FileCount = %d, EmptyWorkspaceFailed = %v; want files and no failure", full.FileCount, full.EmptyWorkspaceFailed())
			}
			if !empty.IsEmpty() {
				t.Fatalf("packages/empty: FileCount = %d, want 0", empty.FileCount)
			}
			if empty.EmptyWorkspaceFailed() != c.wantHasFailures {
				t.Errorf("packages/empty: EmptyWorkspaceFailed = %v, want %v", empty.EmptyWorkspaceFailed(), c.wantHasFailures)
			}
			if result.HasFailures != c.wantHasFailures {
				t.Errorf("HasFailures = %v, want %v", result.HasFailures, c.wantHasFailures)
			}
		})
	}
}
