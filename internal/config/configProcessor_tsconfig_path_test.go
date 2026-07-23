package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigProcessor_TsConfigPathOverride proves the per-workspace `tsConfigPath` field
// changes module resolution: the path alias that makes `@app/util` resolvable lives ONLY in
// tsconfig.build.json, so the import is unresolved with the default tsconfig and resolved
// once the workspace points at the custom one.
func TestConfigProcessor_TsConfigPathOverride(t *testing.T) {
	cwd := t.TempDir()

	writeFile(t, cwd, "package.json", `{"name":"ws-root"}`)
	// Default tsconfig: no path aliases, so "@app/*" cannot resolve.
	writeFile(t, cwd, "tsconfig.json", `{"compilerOptions":{}}`)
	// Custom tsconfig: maps "@app/*" to "src/*".
	writeFile(t, cwd, "tsconfig.build.json", `{
		"compilerOptions": { "baseUrl": ".", "paths": { "@app/*": ["src/*"] } }
	}`)
	writeFile(t, cwd, "src/index.ts", `import { x } from "@app/util";\nexport const y = x;\n`)
	writeFile(t, cwd, "src/util.ts", `export const x = 1;\n`)

	countAppUnresolved := func(cfg string) int {
		result := loadAndProcessUnresolvedConfig(t, cwd, cfg)
		if len(result.RuleResults) == 0 {
			t.Fatalf("expected a rule result")
		}
		return countUnresolvedByRequest(result.RuleResults[0].UnresolvedImports, "@app/util")
	}

	t.Run("default tsconfig leaves the aliased import unresolved", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"workspaces": [
				{ "path": ".", "unresolvedImportsDetection": { "enabled": true } }
			]
		}`
		if got := countAppUnresolved(cfg); got == 0 {
			t.Errorf("expected @app/util unresolved with the default tsconfig, got 0")
		}
	})

	t.Run("tsConfigPath override resolves the aliased import", func(t *testing.T) {
		cfg := `{
			"configVersion": "1.3",
			"workspaces": [
				{
					"path": ".",
					"tsConfigPath": "tsconfig.build.json",
					"unresolvedImportsDetection": { "enabled": true }
				}
			]
		}`
		if got := countAppUnresolved(cfg); got != 0 {
			t.Errorf("expected @app/util resolved via tsconfig.build.json, still unresolved (%d)", got)
		}
	})
}

func writeFile(t *testing.T, cwd, rel, content string) {
	t.Helper()
	full := filepath.Join(cwd, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
