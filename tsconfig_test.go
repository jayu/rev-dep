package main

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
				"a/*": []interface{}{"base/a/*"},
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
	// child first then base (dedup)
	if len(typesArr) != 2 || typesArr[0] != "child-type" || typesArr[1] != "base-type" {
		t.Fatalf("types merged wrong: %v", typesArr)
	}
}

func TestParseTsConfig_Extends_RebasePaths(t *testing.T) {
	// Scenario:
	// ./tsconfig.json (child) extends ./config/tsconfig.base.json (base)
	// project layout contains ./src, ./src/shared, ./src/utils
	// child paths: "shared/*": "./src/shared/*"
	// base paths:  "utils/*": "../src/utils/*"
	// After merging, child's shared/* should remain as written (relative to child)
	// and base's utils/* should be rebased so it points correctly from child's dir.

	tmp := t.TempDir()

	// create base config under ./config
	baseDir := filepath.Join(tmp, "config")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("mkdir baseDir: %v", err)
	}

	base := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"paths": map[string]interface{}{
				"utils/*": []interface{}{"../src/utils/*"},
			},
		},
	}
	baseB, _ := json.Marshal(base)
	basePath := filepath.Join(baseDir, "tsconfig.base.json")
	if err := os.WriteFile(basePath, baseB, 0644); err != nil {
		t.Fatalf("write base: %v", err)
	}

	// child tsconfig at project root
	child := map[string]interface{}{
		"extends": "./config/tsconfig.base.json",
		"compilerOptions": map[string]interface{}{
			"paths": map[string]interface{}{
				"shared/*": []interface{}{"./src/shared/*"},
			},
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

	out := make(map[string]interface{})
	if err := json.Unmarshal(mergedB, &out); err != nil {
		t.Fatalf("unmarshal merged: %v", err)
	}

	co := out["compilerOptions"].(map[string]interface{})
	paths := co["paths"].(map[string]interface{})

	// child mapping should be preserved exactly
	v2, ok2 := paths["shared/*"]
	if !ok2 {
		t.Fatalf("expected shared/* present in paths")
	}
	arr2 := v2.([]interface{})
	if arr2[0] != "./src/shared/*" {
		t.Fatalf("expected child shared/* to be './src/shared/*' got %v", arr2)
	}

	// base mapping should be rebased to be relative to child (tmp)
	v3, ok3 := paths["utils/*"]
	if !ok3 {
		t.Fatalf("expected utils/* present in paths")
	}
	arr3 := v3.([]interface{})
	// base defined ../src/utils/* from config dir; rebased result from child should be src/utils/*
	expected := filepath.ToSlash(filepath.Join("src", "utils", "*"))
	if arr3[0] != expected {
		t.Fatalf("expected rebased utils path %q got %v", expected, arr3)
	}
}
