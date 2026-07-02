package telemetry

import (
	"testing"

	"rev-dep-go/internal/config"
)

func TestBuildMetrics(t *testing.T) {
	configJSON := `{
		"configVersion": "1.10",
		"ignoreFiles": ["**/*.spec.ts"],
		"nodeModulesResolution": { "resolutionType": "nearest-package", "includeDevDepsFromRoot": true },
		"rules": [
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
	cfg, err := config.ParseConfig([]byte(`{"configVersion":"1.10","rules":[{"path":"."}]}`))
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
		"git+https://github.com/jayu/rev-dep.git": "github.com/jayu/rev-dep",
		"https://github.com/jayu/rev-dep":         "github.com/jayu/rev-dep",
		"git@github.com:jayu/rev-dep.git":         "github.com/jayu/rev-dep",
		"ssh://git@github.com/jayu/rev-dep.git/":  "github.com/jayu/rev-dep",
		"":                                        "",
	}
	for in, want := range cases {
		if got := normalizeRepoURL(in); got != want {
			t.Errorf("normalizeRepoURL(%q) = %q, want %q", in, got, want)
		}
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
