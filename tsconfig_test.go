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
