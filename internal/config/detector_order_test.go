package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfiguredDetectorOrderReachesResults(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{
		"configVersion": "2.2",
		"workspaces": [
			{"path":"packages/z", "unusedExportsDetection":true,
			 "orphanFilesDetection":false, "missingNodeModulesDetection":[false,true], "unresolvedImportsDetection":true},
			{"missingNodeModulesDetection":true, "path":"packages/a", "unusedExportsDetection":true}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, workspace := range []string{"packages/z", "packages/a"} {
		dir := filepath.Join(root, workspace)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range map[string]string{"package.json": `{"name":"` + filepath.Base(dir) + `"}`, "main.ts": "export const unused = 1;\n"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	result, err := ProcessConfig(&cfg, root, false, true)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range [][]string{
		{"unused-exports", "missing-node-modules", "unresolved-imports"},
		{"missing-node-modules", "unused-exports"},
	} {
		if !reflect.DeepEqual(result.RuleResults[i].EnabledChecks, want) || result.RuleResults[i].RulePath != cfg.Rules[i].Path {
			t.Fatalf("config order lost: %+v", result.RuleResults[i])
		}
	}
	wantAll := []string{"unused-exports", "orphan-files", "missing-node-modules", "unresolved-imports"}
	if !reflect.DeepEqual(result.RuleResults[0].DetectorOrder, wantAll) {
		t.Fatalf("disabled detector property order lost: %v", result.RuleResults[0].DetectorOrder)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil || strings.Contains(string(encoded), "DetectorOrder") || strings.Contains(string(encoded), "detectorOrder") {
		t.Fatalf("display metadata leaked into config: %s %v", encoded, err)
	}
}

func TestDetectorOrderSkipsNestedPropertiesAndKeepsFallback(t *testing.T) {
	order, err := readDetectorOrder([]byte(`{"unusedExportsDetection":{"ignore":{"main.ts":["missingNodeModulesDetection"]}},"moduleBoundaries":[],"orphanFilesDetection":false,"importConventions":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"unused-exports", "module-boundaries", "orphan-files", "import-conventions"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("incorrect property order: %v", order)
	}
	got := OrderDetectors([]string{"missing-node-modules", "unused-exports", "orphan-files"}, []string{"unused-exports", "unused-exports"})
	if !reflect.DeepEqual(got, []string{"unused-exports", "orphan-files", "missing-node-modules"}) {
		t.Fatalf("wrong removed-detector fallback: %v", got)
	}
}
