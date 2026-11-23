package resolve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTsConfig_Simple(t *testing.T) {
	tmp := t.TempDir()

	cfg := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"baseUrl": "./src",
			"paths": map[string]interface{}{
				"@app/*": []interface{}{"src/*"},
			},
			"types": []interface{}{"node", "jest"},
		},
	}

	b, _ := json.Marshal(cfg)

	// write to disk and call ParseTsConfig
	cfgPath := filepath.Join(tmp, "tsconfig.json")
	if err := os.WriteFile(cfgPath, b, 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	merged, err := ParseTsConfig(cfgPath)
	if err != nil {
		t.Fatalf("ParseTsConfig error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(merged, &out); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	co, ok := out["compilerOptions"].(map[string]interface{})
	if !ok {
		t.Fatalf("compilerOptions missing in merged")
	}

	if co["baseUrl"] != "./src" {
		t.Fatalf("expected baseUrl ./src got %v", co["baseUrl"])
	}

	if _, ok := co["paths"].(map[string]interface{}); !ok {
		t.Fatalf("paths missing or wrong type")
	}

	if typesArr, ok := co["types"].([]interface{}); !ok || len(typesArr) != 2 {
		t.Fatalf("types missing or wrong: %v", co["types"])
	}
}

func TestParseTsConfig_Extends(t *testing.T) {
	tmp := t.TempDir()

	base := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"baseUrl": "./base",
			"paths": map[string]interface{}{
				"a/*":         []interface{}{"base/a/*"},
				"base-only/*": []interface{}{"base/only/*"},
			},
			"types": []interface{}{"base-type"},
		},
	}
	baseB, _ := json.Marshal(base)
	basePath := filepath.Join(tmp, "base.json")
	if err := os.WriteFile(basePath, baseB, 0644); err != nil {
		t.Fatalf("write base: %v", err)
	}

	child := map[string]interface{}{
		"extends": "./base.json",
		"compilerOptions": map[string]interface{}{
			"paths": map[string]interface{}{
				"b/*": []interface{}{"child/b/*"},
				"a/*": []interface{}{"child/override-a/*"},
			},
			"types": []interface{}{"child-type"},
		},
	}
	childB, _ := json.Marshal(child)
	childPath := filepath.Join(tmp, "tsconfig.json")
	if err := os.WriteFile(childPath, childB, 0644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	mergedB, err := ParseTsConfig(childPath)
	if err != nil {
		t.Fatalf("ParseTsConfig error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(mergedB, &out); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	co := out["compilerOptions"].(map[string]interface{})

	// baseUrl should come from base (child didn't override)
	if co["baseUrl"] != "./base" {
		t.Fatalf("expected baseUrl ./base got %v", co["baseUrl"])
	}

	paths := co["paths"].(map[string]interface{})
	if _, ok := paths["base-only/*"]; ok {
		t.Fatalf("child paths must replace the entire inherited map: %v", paths)
	}
	if _, ok := paths["b/*"]; !ok {
		t.Fatalf("expected b/* from child in paths")
	}
	if v, ok := paths["a/*"]; !ok {
		t.Fatalf("expected a/* present")
	} else {
		// ensure child override
		arr := v.([]interface{})
		if arr[0] != "child/override-a/*" {
			t.Fatalf("a/* not overridden by child: %v", arr)
		}
	}

	typesArr := co["types"].([]interface{})
	if len(typesArr) != 1 || typesArr[0] != "child-type" {
		t.Fatalf("child types must replace the inherited array: %v", typesArr)
	}
}

// Extending a tsconfig shipped as a node_modules package (e.g.
// "@docusaurus/tsconfig") must resolve through node_modules and capture the
// extended config's options - a bare specifier containing "/" is a package
// name, not a relative file path.
func TestParseTsConfig_Extends_NodeModulesPackage(t *testing.T) {
	tmp := t.TempDir()

	// node_modules/@scope/cfg with package.json main -> tsconfig.json
	pkgDir := filepath.Join(tmp, "node_modules", "@scope", "cfg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	pkgJSON := `{"name":"@scope/cfg","main":"tsconfig.json"}`
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		t.Fatalf("write pkg json: %v", err)
	}
	baseCfg := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"strict": true,
			"paths": map[string]interface{}{
				"@site/*": []interface{}{"./*"},
			},
		},
	}
	baseB, _ := json.Marshal(baseCfg)
	if err := os.WriteFile(filepath.Join(pkgDir, "tsconfig.json"), baseB, 0o644); err != nil {
		t.Fatalf("write pkg tsconfig: %v", err)
	}

	// child tsconfig that extends the package by its bare name
	child := map[string]interface{}{
		"extends": "@scope/cfg",
		"compilerOptions": map[string]interface{}{
			"paths": map[string]interface{}{
				"@app/*": []interface{}{"src/*"},
			},
		},
	}
	childB, _ := json.Marshal(child)
	childPath := filepath.Join(tmp, "tsconfig.json")
	if err := os.WriteFile(childPath, childB, 0o644); err != nil {
		t.Fatalf("write child: %v", err)
	}

	mergedB, err := ParseTsConfig(childPath)
	if err != nil {
		t.Fatalf("ParseTsConfig error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(mergedB, &out); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	co := out["compilerOptions"].(map[string]interface{})
	if co["strict"] != true {
		t.Fatalf("expected strict from extended node_modules config, got %v", co)
	}
	paths, ok := co["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected paths in merged compilerOptions, got %v", co)
	}
	// The child's paths replace the package's alias map.
	if _, ok := paths["@site/*"]; ok {
		t.Fatalf("expected inherited '@site/*' alias to be replaced, got paths: %v", paths)
	}
	// The child's own alias must still be present.
	if _, ok := paths["@app/*"]; !ok {
		t.Fatalf("expected child '@app/*' alias to remain, got paths: %v", paths)
	}
}

func TestParseTsConfig_Extends_RebasePaths(t *testing.T) {
	tests := []struct {
		name  string
		child string
		want  string
	}{
		{
			name:  "inherited paths are rebased",
			child: `{"extends":"./config/tsconfig.base.json","compilerOptions":{"strict":true}}`,
			want:  `{"compilerOptions":{"strict":true,"paths":{"utils/*":["src/utils/*"]}}}`,
		},
		{
			name:  "child paths replace inherited paths and retain their original location",
			child: `{"extends":"./config/tsconfig.base.json","compilerOptions":{"paths":{"shared/*":["./src/shared/*"]}}}`,
			want:  `{"compilerOptions":{"paths":{"shared/*":["./src/shared/*"]}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTsConfigFiles(t, map[string]string{
				"tsconfig.json":             tt.child,
				"config/tsconfig.base.json": `{"compilerOptions":{"paths":{"utils/*":["../src/utils/*"]}}}`,
			}, tt.want)
		})
	}
}
