package baseline

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotSetComparisonAndDeterministicStorage(t *testing.T) {
	old := New()
	old.Add("packages/a", "missing-node-modules", Issue{File: "packages/a/main.ts", Module: "old"})
	old.Add("removed-workspace", "orphan-files", Issue{File: "removed-workspace/a.ts"})
	current := New()
	current.Add("packages/a", "missing-node-modules", Issue{File: "packages/a/main.ts", Module: "new"})
	current.Add("packages/a", "circular-imports", Issue{Files: []string{"b.ts", "a.ts", "a.ts"}})
	current.Add("packages/a", "circular-imports", Issue{Files: []string{"a.ts", "b.ts"}})
	for _, s := range []*Snapshot{old, current} {
		if err := s.Canonicalize(); err != nil {
			t.Fatal(err)
		}
	}
	r := Compare(current, old, nil)
	if r.Status != "different" || r.Counts.New != 2 || r.Counts.Obsolete != 2 {
		t.Fatalf("unexpected delta: %+v", r)
	}
	selected := Compare(current, old, []string{"packages/a"})
	if selected.Counts.Obsolete != 1 {
		t.Fatalf("unselected workspace treated as removed: %+v", selected)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "nested", "snapshot.json")
	if err := Save(file, current); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(file)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(first, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope) != 2 || string(envelope["schemaVersion"]) != `"1.0"` || envelope["workspaces"] == nil {
		t.Fatalf("unexpected snapshot envelope: %s", first)
	}
	loaded, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := Compare(current, loaded, nil); got.Status != "matched" || got.Counts.Acknowledged != 2 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
	if err := Save(file, loaded); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(file)
	if !bytes.Equal(first, second) {
		t.Fatal("same findings did not produce identical bytes")
	}
	if bytes.Contains(first, []byte(dir)) {
		t.Fatal("absolute checkout path leaked into snapshot")
	}
}

func TestEmptyScopesAndUnsupportedFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "snapshot.json")
	empty := New()
	empty.Workspaces["."] = map[string][]Issue{"orphan-files": {}}
	if err := Save(file, empty); err != nil {
		t.Fatal(err)
	}
	if len(empty.Workspaces) != 0 {
		t.Fatal("empty coverage was persisted")
	}
	for _, raw := range []string{
		`{}`, `{"schemaVersion":"1.0","workspaces":null}`,
		`{"schemaVersion":"9.0","workspaces":{}}`,
		`{"schemaVersion":"1.0","workspaces":{},"configuration":{}}`,
		`{"schemaVersion":"1.0","workspaces":{".":{"unknown":[]}}}`,
		`{"schemaVersion":"1.0","workspaces":{".":{"orphan-files":[{"file":"/absolute.ts"}]}}}`,
		`{"schemaVersion":"1.0","workspaces":{".":{"orphan-files":[{"file":"a.ts","module":"bad-extra-field"}]}}}`,
		`{"schemaVersion":"1.0","workspaces":{".":{"orphan-files":[{"file":"a.ts","startLine":1}]}}}`,
		`{"schemaVersion":"1.0","workspaces":{}} {}`,
	} {
		if err := os.WriteFile(file, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(file); err == nil {
			t.Errorf("accepted invalid snapshot: %s", raw)
		}
	}
}

func TestFailedSavePreservesSnapshot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "snapshot.json")
	original := New()
	original.Add(".", "orphan-files", Issue{File: "old.ts"})
	if err := Save(file, original); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(file)
	invalid := New()
	invalid.Add(".", "orphan-files", Issue{File: "/bad.ts"})
	if err := Save(file, invalid); err == nil {
		t.Fatal("invalid capture succeeded")
	}
	after, _ := os.ReadFile(file)
	if !bytes.Equal(before, after) {
		t.Fatal("failed capture replaced previous snapshot")
	}
}

func TestSaveProtectsUnrelatedFilesAndRegeneratesBaselines(t *testing.T) {
	for _, test := range []struct {
		name  string
		raw   string
		allow bool
	}{
		{"source", "export const value = 1;\n", false},
		{"manifest", `{"name":"app","dependencies":{}}`, false},
		{"other versioned JSON", `{"schemaVersion":"1.0","results":{}}`, false},
		{"damaged baseline", `{"schemaVersion":"1.0",`, false},
		{"unsupported version", `{"schemaVersion":"9.0","workspaces":{}}`, true},
		{"invalid records", `{"schemaVersion":"1.0","workspaces":{".":{"unknown":[null]}}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "destination.json")
			if err := os.WriteFile(file, []byte(test.raw), 0644); err != nil {
				t.Fatal(err)
			}
			err := Save(file, New())
			if test.allow {
				if err != nil {
					t.Fatalf("could not regenerate recognizable baseline: %v", err)
				}
				if _, err := Load(file); err != nil {
					t.Fatalf("regenerated baseline is invalid: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
					t.Fatalf("unrelated destination was not rejected: %v", err)
				}
				after, err := os.ReadFile(file)
				if err != nil || string(after) != test.raw {
					t.Fatalf("existing file changed: %v", err)
				}
			}
		})
	}
}

func TestLoadedFindingRequiresExactDetectorShape(t *testing.T) {
	file := filepath.Join(t.TempDir(), "snapshot.json")
	for _, record := range []string{
		`{"file":"main.ts","module":"missing","target":null}`,
		`{"file":"main.ts","module":"missing","target":""}`,
		`{"file":"main.ts","module":"missing","files":[]}`,
		`{"file":"main.ts","module":"missing","isType":null}`,
		`{"file":"main.ts","module":"missing","unknown":null}`,
		`{"file":"main.ts","module":null}`,
		`{"file":null,"module":"missing"}`,
		`{"file":"main.ts","module":""}`,
		`{"File":"main.ts","module":"missing"}`,
		`null`,
	} {
		raw := `{"schemaVersion":"1.0","workspaces":{".":{"missing-node-modules":[` + record + `]}}}`
		if err := os.WriteFile(file, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(file); err == nil {
			t.Errorf("accepted malformed record: %s", record)
		}
	}
	// False is a present, valid identity field, unlike null or an extra empty field.
	raw := `{"schemaVersion":"1.0","workspaces":{".":{"unused-exports":[{"file":"main.ts","export":"unused","isType":false}]}}}`
	if err := os.WriteFile(file, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err != nil {
		t.Fatalf("rejected valid false isType: %v", err)
	}
}

func TestPublishedSchemaCoversEveryDetector(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "output-schema", "findings-baseline-1.0.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	definitions := schema["definitions"].(map[string]any)
	for _, detector := range Detectors() {
		if definitions[detector] == nil {
			t.Errorf("schema missing %s", detector)
		}
	}
	if len(definitions) != len(Detectors()) {
		t.Fatal("schema and supported detector inventory drifted")
	}
	for _, value := range []string{"C:/repo/a.ts", "../a.ts", "./a.ts", "a\\b.ts"} {
		if isCanonicalPath(value) != (value == "../a.ts") {
			t.Errorf("unexpected path validation for %q", value)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "--update-snapshot") {
		t.Fatal("missing snapshot needs actionable guidance")
	}
}
