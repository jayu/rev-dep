package telemetry

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"rev-dep-go/internal/config"
)

func TestBuildMetrics(t *testing.T) {
	configJSON := `{
		"configVersion": "1.10",
		"ignoreFiles": ["**/*.spec.ts"],
		"nodeModulesResolution": { "resolutionType": "nearest-package", "includeDevDepsFromRoot": true },
		"workspaces": [
			{
				"path": "packages/a",
				"circularImportsDetection": { "enabled": true },
				"restrictedImportersDetection": [
					{ "enabled": true, "files": ["legacy/**"], "allowedEntryPoints": ["src/admin/**"] },
					{ "enabled": true, "files": ["old/**"], "allowedEntryPoints": ["src/admin/**"] },
					{ "enabled": false, "files": ["dead/**"] }
				],
				"unusedExportsDetection": { "enabled": false }
			},
			{
				"path": "packages/b",
				"circularImportsDetection": { "enabled": true },
				"restrictedImportersDetection": { "enabled": true, "files": ["legacy/**"], "allowedEntryPoints": ["src/admin/**"] },
				"restrictedDirectImportersDetection": { "enabled": true, "files": ["config/**"], "denyImporters": ["src/public/**"] }
			}
		]
	}`

	cfg, err := config.ParseConfig([]byte(configJSON))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	m := BuildMetrics(&cfg, 42)

	checks := map[string]struct{ got, want int }{
		"workspaceCount":  {m.WorkspaceCount, 2},
		"fileCount":       {m.FileCount, 42},
		"circularImports": {m.CircularImports, 1},
		// max across workspaces: package a has 2 enabled (the 3rd is disabled), package b has 1.
		"restrictedImporters":          {m.RestrictedImporters, 2},
		"restrictedDirectImporters":    {m.RestrictedDirectImporters, 1}, // only package b
		"unusedExports":                {m.UnusedExports, 0},             // disabled everywhere -> 0
		"usesNearestPackageResolution": {m.UsesNearestPackageResolution, 1},
		"usesIncludeDevDepsFromRoot":   {m.UsesIncludeDevDepsFromRoot, 1},
		"usesIgnoreFiles":              {m.UsesIgnoreFiles, 1},
		"usesProcessIgnoredFiles":      {m.UsesProcessIgnoredFiles, 0},
		"usesConditionNames":           {m.UsesConditionNames, 0},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", name, c.got, c.want)
		}
	}
}

func TestBuildMetrics_DefaultsAreZero(t *testing.T) {
	cfg, err := config.ParseConfig([]byte(`{"configVersion":"1.10","workspaces":[{"path":"."}]}`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	m := BuildMetrics(&cfg, 0)
	if m.UsesNearestPackageResolution != 0 || m.UsesIncludeDevDepsFromRoot != 0 {
		t.Errorf("default resolution flags should be 0, got nearest=%d includeDev=%d", m.UsesNearestPackageResolution, m.UsesIncludeDevDepsFromRoot)
	}
	if m.RestrictedImporters != 0 || m.CircularImports != 0 {
		t.Errorf("no detectors should yield 0 counts, got %+v", m)
	}
}

func TestNormalizeRepoURL(t *testing.T) {
	cases := map[string]string{
		"git+https://github.com/jayu/rev-dep.git":                        "github.com/jayu/rev-dep",
		"https://github.com/jayu/rev-dep":                                "github.com/jayu/rev-dep",
		"git@github.com:jayu/rev-dep.git":                                "github.com/jayu/rev-dep",
		"ssh://git@github.com/jayu/rev-dep.git/":                         "github.com/jayu/rev-dep",
		"https://user:token@GitHub.com/Jayu/rev-dep.git?ref=main#readme": "github.com/Jayu/rev-dep",
		"file:///work/rev-dep":                                           "",
		"../local-rev-dep":                                               "",
		"":                                                               "",
	}
	for in, want := range cases {
		if got := normalizeRepoURL(in); got != want {
			t.Errorf("normalizeRepoURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGitRemoteURLs(t *testing.T) {
	got := gitRemoteURLs([]byte(`
[core]
	repositoryformatversion = 0
[remote "origin"]
	url = "git@github.com:acme/project.git"
	fetch = +refs/heads/*:refs/remotes/origin/*
[remote "upstream"]
	url = https://github.com/acme/project.git
[branch "main"]
	remote = origin
`))
	want := []gitRemote{
		{name: "origin", url: "git@github.com:acme/project.git"},
		{name: "upstream", url: "https://github.com/acme/project.git"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("gitRemoteURLs() = %#v, want %#v", got, want)
	}
}

func TestRepoIDFindsNearestGitRepository(t *testing.T) {
	root := t.TempDir()
	outerGitDir := filepath.Join(root, ".git")
	makeGitDir(t, outerGitDir, `[remote "origin"]
	url = https://github.com/acme/outer.git
`)

	nestedRoot := filepath.Join(root, "packages", "nested")
	nestedGitDir := filepath.Join(nestedRoot, ".git")
	makeGitDir(t, nestedGitDir, `[remote "origin"]
	url = git@github.com:acme/nested.git
`)

	tests := []struct {
		name       string
		cwd        string
		wantGitDir string
		wantRemote string
	}{
		{
			name:       "ancestor repository",
			cwd:        filepath.Join(root, "apps", "cli", "src"),
			wantGitDir: outerGitDir,
			wantRemote: "github.com/acme/outer",
		},
		{
			name:       "nested repository",
			cwd:        filepath.Join(nestedRoot, "src"),
			wantGitDir: nestedGitDir,
			wantRemote: "github.com/acme/nested",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(tt.cwd, 0o755); err != nil {
				t.Fatalf("MkdirAll(%q): %v", tt.cwd, err)
			}
			if got := closestGitDir(tt.cwd); got != tt.wantGitDir {
				t.Errorf("closestGitDir(%q) = %q, want %q", tt.cwd, got, tt.wantGitDir)
			}
			if got, want := repoID(tt.cwd), sha256Hex(tt.wantRemote); got != want {
				t.Errorf("repoID(%q) = %q, want %q", tt.cwd, got, want)
			}
		})
	}
}

func TestRepoIDReadsLinkedWorktreeConfig(t *testing.T) {
	root := t.TempDir()
	commonGitDir := filepath.Join(root, "repository", ".git")
	makeGitDir(t, commonGitDir, `[remote "origin"]
	url = https://github.com/acme/project.git
`)

	linkedGitDir := filepath.Join(commonGitDir, "worktrees", "feature")
	if err := os.MkdirAll(linkedGitDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(linked git dir): %v", err)
	}
	writeTelemetryTestFile(t, filepath.Join(linkedGitDir, "HEAD"), "ref: refs/heads/feature\n")
	commonDir, err := filepath.Rel(linkedGitDir, commonGitDir)
	if err != nil {
		t.Fatalf("Rel(common git dir): %v", err)
	}
	writeTelemetryTestFile(t, filepath.Join(linkedGitDir, "commondir"), commonDir+"\n")

	worktree := filepath.Join(root, "feature-worktree")
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o755); err != nil {
		t.Fatalf("MkdirAll(worktree): %v", err)
	}
	relativeGitDir, err := filepath.Rel(worktree, linkedGitDir)
	if err != nil {
		t.Fatalf("Rel(linked git dir): %v", err)
	}
	writeTelemetryTestFile(t, filepath.Join(worktree, ".git"), "gitdir: "+relativeGitDir+"\n")

	if got := closestGitDir(filepath.Join(worktree, "src")); got != linkedGitDir {
		t.Errorf("closestGitDir(linked worktree) = %q, want %q", got, linkedGitDir)
	}
	if got, want := repoID(filepath.Join(worktree, "src")), sha256Hex("github.com/acme/project"); got != want {
		t.Errorf("repoID(linked worktree) = %q, want %q", got, want)
	}
}

func TestRepoIDReturnsEmptyWithoutNetworkRemote(t *testing.T) {
	root := t.TempDir()
	makeGitDir(t, filepath.Join(root, ".git"), `[remote "origin"]
	url = ../local-project
`)

	if got := repoID(root); got != "" {
		t.Errorf("repoID() = %q, want empty for a local remote", got)
	}
}

func makeGitDir(t *testing.T, gitDir, config string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", gitDir, err)
	}
	writeTelemetryTestFile(t, filepath.Join(gitDir, "HEAD"), "ref: refs/heads/main\n")
	writeTelemetryTestFile(t, filepath.Join(gitDir, "config"), config)
}

func writeTelemetryTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func TestParseConnectionString(t *testing.T) {
	iKey, endpoint := parseConnectionString("InstrumentationKey=abc-123;IngestionEndpoint=https://westeurope.in.applicationinsights.azure.com/")
	if iKey != "abc-123" {
		t.Errorf("iKey = %q, want abc-123", iKey)
	}
	if endpoint != "https://westeurope.in.applicationinsights.azure.com" {
		t.Errorf("endpoint = %q (trailing slash should be trimmed)", endpoint)
	}

	iKey2, endpoint2 := parseConnectionString("InstrumentationKey=only-key")
	if iKey2 != "only-key" || endpoint2 != "https://dc.services.visualstudio.com" {
		t.Errorf("defaults wrong: iKey=%q endpoint=%q", iKey2, endpoint2)
	}
}

func TestEnabled_OffUnderTests(t *testing.T) {
	// testing.Testing() is true here, so telemetry must be disabled regardless of other state.
	if enabled() {
		t.Errorf("telemetry must be disabled under go test")
	}
}
