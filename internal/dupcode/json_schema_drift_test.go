package dupcode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// A published JSON format is a promise, and the only way to keep one is to make breaking it
// noisy. These tests are that noise.
//
// TestDuplicatedCodeJSONSchemaNoDrift compares the schema file against the structs that
// produce the output, key for key, in both directions: a struct key the schema does not
// declare would make real output fail validation, and a schema property no struct emits is a
// promise nothing keeps. Every object sets additionalProperties:false so the first of those
// is a hard failure rather than a shrug.
//
// The effect is that adding a field to the output does not compile-and-ship - it fails here
// until the schema is updated, at which point whoever is updating it has to decide whether
// JSONOutputVersion moves too. The decision is unavoidable; only the answer is open.

const schemaFile = "duplicated-code-1.0.schema.json"

func TestDuplicatedCodeJSONSchemaNoDrift(t *testing.T) {
	schema := readOutputSchema(t)

	// Every value below is FULLY populated, omitempty fields included: a zero field marshals
	// to nothing, its key never reaches the comparison, and a whole section of the format
	// could then be missing from the schema while this test passes.
	cases := []struct {
		name    string
		pointer []string
		value   interface{}
	}{
		{"report (root)", nil, JSONReport{
			Version:  JSONOutputVersion,
			Snapshot: &JSONSnapshot{},
			Findings: []JSONFinding{},
		}},
		{"settings", []string{"definitions", "settings"}, JSONSettings{
			IgnoreFiles:         []string{"a"},
			ProcessIgnoredFiles: []string{"b"},
			ConfigLevel:         &JSONConfigLevel{},
		}},
		{"configLevel", []string{"definitions", "configLevel"}, JSONConfigLevel{
			ConfigPath:          "c",
			IgnoreFiles:         []string{"a"},
			ProcessIgnoredFiles: []string{"b"},
		}},
		{"snapshot", []string{"definitions", "snapshot"}, JSONSnapshot{
			Resolved:       []JSONSnapshotEntry{{}},
			BelowThreshold: []JSONFinding{{}},
		}},
		{"deltaCounts", []string{"definitions", "deltaCounts"}, DeltaCounts{}},
		{"snapshotEntry", []string{"definitions", "snapshotEntry"}, JSONSnapshotEntry{
			Files: map[string]int{"a.ts": 1},
			Label: "x",
		}},
		{"summary", []string{"definitions", "summary"}, JSONSummary{}},
		{"finding", []string{"definitions", "finding"}, JSONFinding{
			Occurrences: []JSONOccurrence{},
			Snippet:     "x",
			Status:      "new",
			Was:         &JSONWas{},
			FileChanges: []JSONFileChange{{}},
		}},
		{"was", []string{"definitions", "was"}, JSONWas{Files: map[string]int{"a.ts": 1}}},
		{"fileChange", []string{"definitions", "fileChange"}, JSONFileChange{}},
		{"occurrence", []string{"definitions", "occurrence"}, JSONOccurrence{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := navigateOutputSchema(t, schema, tc.pointer)

			if ap, ok := node["additionalProperties"]; !ok || ap != false {
				t.Errorf("%s: object must set additionalProperties:false, or an undeclared key "+
					"would validate and the schema would stop being a promise", tc.name)
			}

			schemaKeys := outputSchemaProperties(t, node)
			structKeys := outputStructKeys(t, tc.value)

			if only := missingFrom(structKeys, schemaKeys); len(only) > 0 {
				t.Errorf("%s: the tool emits keys the schema does not declare, so real output "+
					"would FAIL validation: %v\n  add them to output-schema/%s and decide "+
					"whether JSONOutputVersion moves", tc.name, only, schemaFile)
			}
			if only := missingFrom(schemaKeys, structKeys); len(only) > 0 {
				t.Errorf("%s: the schema declares properties nothing emits, so it promises "+
					"something no consumer will ever see: %v", tc.name, only)
			}
		})
	}
}

// TestJSONOutputVersionMatchesItsSchema keeps the number in the code and the number in the
// file name from parting company - which is the one failure that would make every other
// guard here meaningless, since a consumer checks the version and nothing else.
func TestJSONOutputVersionMatchesItsSchema(t *testing.T) {
	want := "duplicated-code-" + JSONOutputVersion + ".schema.json"
	if schemaFile != want {
		t.Errorf("JSONOutputVersion is %q but the schema under test is %q; the version was "+
			"bumped without adding output-schema/%s", JSONOutputVersion, schemaFile, want)
	}
	if _, err := os.Stat(filepath.Join("..", "..", "output-schema", want)); err != nil {
		t.Errorf("no schema published for version %s: %v", JSONOutputVersion, err)
	}

	schema := readOutputSchema(t)
	props, _ := schema["properties"].(map[string]interface{})
	version, _ := props["version"].(map[string]interface{})
	if got := version["const"]; got != JSONOutputVersion {
		t.Errorf("the schema pins version %v, the tool emits %q", got, JSONOutputVersion)
	}
}

// TestJSONOutputCarriesItsVersion checks the value actually reaches the output. A version a
// consumer cannot read is not a version.
func TestJSONOutputCarriesItsVersion(t *testing.T) {
	got := snapshotJSON(t, nil, nil, nil, Options{Cwd: "/w"})
	if got.Version != JSONOutputVersion {
		t.Errorf("version = %q, want %q", got.Version, JSONOutputVersion)
	}
}

// A version that never moves is worse than none, because it says "unchanged" about something
// that changed. This pins the shape of the number so a bump is a deliberate edit rather than
// a typo that still parses.
func TestJSONOutputVersionIsWellFormed(t *testing.T) {
	parts := strings.Split(JSONOutputVersion, ".")
	if len(parts) != 2 {
		t.Fatalf("JSONOutputVersion = %q, want major.minor", JSONOutputVersion)
	}
	for _, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			t.Errorf("JSONOutputVersion = %q, want digits either side of the dot", JSONOutputVersion)
		}
	}
}

func readOutputSchema(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "output-schema", schemaFile))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}

func navigateOutputSchema(t *testing.T, schema map[string]interface{}, pointer []string) map[string]interface{} {
	t.Helper()
	node := schema
	for _, key := range pointer {
		next, ok := node[key].(map[string]interface{})
		if !ok {
			t.Fatalf("schema path %s: %q is missing or not an object",
				strings.Join(pointer, "/"), key)
		}
		node = next
	}
	return node
}

func outputSchemaProperties(t *testing.T, node map[string]interface{}) []string {
	t.Helper()
	props, ok := node["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema node declares no properties")
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// outputStructKeys marshals a value and returns its top-level JSON keys, which is the only
// honest way to ask what the tool actually emits - struct tags can carry omitempty, embedded
// structs flatten, and reading the field names would miss both.
func outputStructKeys(t *testing.T, v interface{}) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal %T: %v", v, err)
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// A field that marshalled away is a field this test cannot see, so say so rather than
	// silently passing.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct && len(keys) < rv.NumField() {
		var absent []string
		for i := 0; i < rv.NumField(); i++ {
			tag := strings.Split(rv.Type().Field(i).Tag.Get("json"), ",")[0]
			if tag == "" || tag == "-" {
				continue
			}
			if _, ok := obj[tag]; !ok {
				absent = append(absent, tag)
			}
		}
		if len(absent) > 0 {
			t.Errorf("%T: fields %v are zero in this fixture, so their schema coverage is "+
				"untested - populate them", v, absent)
		}
	}
	return keys
}

func missingFrom(want, have []string) []string {
	present := make(map[string]struct{}, len(have))
	for _, k := range have {
		present[k] = struct{}{}
	}
	var out []string
	for _, k := range want {
		if _, ok := present[k]; !ok {
			out = append(out, k)
		}
	}
	return out
}
