package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigProcessor_RestrictedChecksMatchBuiltInModules(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("package.json", `{"name":"builtin-fixture","version":"1.0.0","private":true}`)
	mustWrite("src/main.ts", "import { load } from './domain/repo'\nimport { lookup } from './domain/resolver'\nexport const run = () => load() + lookup()\n")
	mustWrite("src/domain/repo.ts", "import { readFileSync } from 'fs'\nimport { readFile } from 'node:fs/promises'\nimport { join } from 'node:path'\nexport const load = () => String(readFileSync(join('a'))) + readFile\n")
	mustWrite("src/domain/resolver.ts", "import dns from 'dns'\nimport { Database } from 'bun:sqlite'\nexport const lookup = () => String(dns) + Database\n")

	configJSON := `{
		"configVersion": "2.0",
		"workspaces": [{
			"path": ".",
			"prodEntryPoints": ["src/main.ts"],
			"restrictedImportsDetection": {
				"enabled": true,
				"entryPoints": ["src/main.ts"],
				"denyModules": ["node:*"],
				"ignoreMatches": ["node:path"]
			},
			"restrictedImportersDetection": {
				"enabled": true,
				"modules": ["fs", "node:fs"],
				"allowedEntryPoints": ["src/allowed.ts"]
			},
			"restrictedDirectImportersDetection": {
				"enabled": true,
				"modules": ["fs", "dns", "net"],
				"denyImporters": ["src/domain/**"]
			}
		}]
	}`
	configPath := filepath.Join(dir, "rev-dep.config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	result, err := ProcessConfig(&cfg, dir, false, false)
	if err != nil {
		t.Fatalf("process config: %v", err)
	}
	if len(result.RuleResults) != 1 {
		t.Fatalf("expected 1 rule result, got %d", len(result.RuleResults))
	}
	rr := result.RuleResults[0]
	repo := filepath.ToSlash(filepath.Join(dir, "src/domain/repo.ts"))
	resolver := filepath.ToSlash(filepath.Join(dir, "src/domain/resolver.ts"))

	gotImports := []string{}
	for _, v := range rr.RestrictedImportsViolations {
		gotImports = append(gotImports, filepath.ToSlash(v.ImporterFile)+" "+v.ImportRequest)
	}
	wantImports := []string{repo + " node:fs/promises"}
	if !reflect.DeepEqual(gotImports, wantImports) {
		t.Errorf("restricted imports: got %v, want %v", gotImports, wantImports)
	}

	gotImporters := []string{}
	for _, v := range rr.RestrictedImportersViolations {
		gotImporters = append(gotImporters, v.Module)
	}
	if want := []string{"fs", "node:fs"}; !reflect.DeepEqual(gotImporters, want) {
		t.Errorf("restricted importers: got %v, want %v", gotImporters, want)
	}

	gotDirect := []string{}
	for _, v := range rr.RestrictedDirectImportersViolations {
		gotDirect = append(gotDirect, filepath.ToSlash(v.ImporterFile)+" "+v.ImportRequest)
	}
	wantDirect := []string{repo + " fs", resolver + " dns"}
	if !reflect.DeepEqual(gotDirect, wantDirect) {
		t.Errorf("restricted direct importers: got %v, want %v", gotDirect, wantDirect)
	}
}

func TestConfigProcessor_FrontendEntryPointMustNotReachBuiltIns(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("package.json", `{"name":"frontend-fixture","version":"1.0.0","private":true}`)
	mustWrite("src/frontend/app.tsx", "import { formatDate } from '../shared/format'\nexport const App = () => formatDate()\n")
	mustWrite("src/shared/format.ts", "import { basename } from 'node:path'\nimport { config } from '../backend/config'\nexport const formatDate = () => basename(config.root)\n")
	mustWrite("src/backend/config.ts", "import { readFileSync } from 'fs'\nimport { Database } from 'bun:sqlite'\nexport const config = { root: String(readFileSync('x')), db: Database }\n")
	mustWrite("src/server/main.ts", "import { config } from '../backend/config'\nexport const start = () => config\n")

	configJSON := `{
		"configVersion": "2.0",
		"workspaces": [{
			"path": ".",
			"prodEntryPoints": ["src/frontend/app.tsx", "src/server/main.ts"],
			"restrictedImportsDetection": {
				"enabled": true,
				"entryPoints": ["src/frontend/**/*.tsx"],
				"denyModules": ["builtin:*"]
			},
			"restrictedImportersDetection": {
				"enabled": true,
				"modules": ["builtin:*"],
				"allowedEntryPoints": ["src/server/**"]
			},
			"restrictedDirectImportersDetection": {
				"enabled": true,
				"modules": ["builtin:*"],
				"allowImporters": ["src/backend/**", "src/server/**"]
			}
		}]
	}`
	configPath := filepath.Join(dir, "rev-dep.config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	result, err := ProcessConfig(&cfg, dir, false, false)
	if err != nil {
		t.Fatalf("process config: %v", err)
	}
	rr := result.RuleResults[0]
	rel := func(p string) string {
		r, err := filepath.Rel(dir, p)
		if err != nil {
			t.Fatalf("rel %s: %v", p, err)
		}
		return filepath.ToSlash(r)
	}

	gotImports := []string{}
	for _, v := range rr.RestrictedImportsViolations {
		gotImports = append(gotImports, rel(v.EntryPoint)+" "+rel(v.ImporterFile)+" "+v.ImportRequest)
	}
	wantImports := []string{
		"src/frontend/app.tsx src/backend/config.ts bun:sqlite",
		"src/frontend/app.tsx src/backend/config.ts fs",
		"src/frontend/app.tsx src/shared/format.ts node:path",
	}
	if !reflect.DeepEqual(gotImports, wantImports) {
		t.Errorf("restricted imports: got %v, want %v", gotImports, wantImports)
	}

	gotImporters := []string{}
	for _, v := range rr.RestrictedImportersViolations {
		gotImporters = append(gotImporters, rel(v.EntryPoint)+" "+v.Module)
	}
	wantImporters := []string{
		"src/frontend/app.tsx bun:sqlite",
		"src/frontend/app.tsx fs",
		"src/frontend/app.tsx node:path",
	}
	if !reflect.DeepEqual(gotImporters, wantImporters) {
		t.Errorf("restricted importers: got %v, want %v", gotImporters, wantImporters)
	}

	gotDirect := []string{}
	for _, v := range rr.RestrictedDirectImportersViolations {
		gotDirect = append(gotDirect, rel(v.ImporterFile)+" "+v.ImportRequest)
	}
	if want := []string{"src/shared/format.ts node:path"}; !reflect.DeepEqual(gotDirect, want) {
		t.Errorf("restricted direct importers: got %v, want %v", gotDirect, want)
	}
}

func TestParseConfig_RejectsUnsupportedBuiltInKeyword(t *testing.T) {
	detectors := map[string]string{
		"restrictedImportsDetection.denyModules":     `"restrictedImportsDetection": {"enabled": true, "entryPoints": ["src/index.ts"], "denyModules": ["fs", "builtin:fs"]}`,
		"restrictedImportersDetection.modules":       `"restrictedImportersDetection": {"enabled": true, "modules": ["fs", "builtin:fs"], "allowedEntryPoints": ["src/index.ts"]}`,
		"restrictedDirectImportersDetection.modules": `"restrictedDirectImportersDetection": {"enabled": true, "modules": ["fs", "builtin:fs"], "denyImporters": ["src/**"]}`,
	}
	for option, detector := range detectors {
		configJSON := `{"configVersion": "2.0", "workspaces": [{"path": ".", ` + detector + `}]}`
		_, err := ParseConfig([]byte(configJSON))
		if err == nil {
			t.Errorf("%s: expected an error for 'builtin:fs'", option)
			continue
		}
		want := option + "[1]: 'builtin:fs' is not supported: use 'builtin:*' for every built-in module"
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %q, want it to contain %q", option, err.Error(), want)
		}
	}

	valid := `{"configVersion": "2.0", "workspaces": [{"path": ".", "restrictedImportsDetection": {"enabled": true, "entryPoints": ["src/index.ts"], "denyModules": ["builtin:*"]}}]}`
	if _, err := ParseConfig([]byte(valid)); err != nil {
		t.Errorf("'builtin:*' should be valid, got %v", err)
	}
}

func TestConfigProcessor_UnlistedSchemeModulesAreBuiltIn(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("package.json", `{"name":"future-fixture","version":"1.0.0","private":true}`)
	mustWrite("src/pages/index.tsx", "import { run } from '../backend/job'\nexport const Page = () => run()\n")
	mustWrite("src/backend/job.ts", "import { open } from 'node:future_module'\nimport { tick } from 'bun:future_module/sub'\nexport const run = () => [open, tick]\n")

	configJSON := `{
		"configVersion": "2.0",
		"workspaces": [{
			"path": ".",
			"prodEntryPoints": ["src/pages/index.tsx"],
			"restrictedImportsDetection": {
				"enabled": true,
				"entryPoints": ["src/pages/**/*.tsx"],
				"denyModules": ["builtin:*"]
			},
			"missingNodeModulesDetection": {"enabled": true}
		}]
	}`
	configPath := filepath.Join(dir, "rev-dep.config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	result, err := ProcessConfig(&cfg, dir, false, false)
	if err != nil {
		t.Fatalf("process config: %v", err)
	}
	rr := result.RuleResults[0]

	got := []string{}
	for _, v := range rr.RestrictedImportsViolations {
		got = append(got, v.DeniedModule+"|"+v.ImportRequest)
	}
	want := []string{"bun:future_module|bun:future_module/sub", "node:future_module|node:future_module"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("restricted imports: got %v, want %v", got, want)
	}

	if len(rr.MissingNodeModules) != 0 {
		t.Errorf("built-in modules must not be reported as missing, got %+v", rr.MissingNodeModules)
	}
}
