package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestJSONOutputSchemaNoDrift guards output-schema/1.1.schema.json against silent drift from the Go
// structs that produce `config run --format json`. Every object in the schema sets
// additionalProperties:false, so a struct field whose JSON key is missing from the schema would make
// real output fail validation, and a schema property with no backing struct field is dead weight.
//
// For each output type we marshal a FULLY populated instance (so even omitempty fields — notably the
// optional startLine/startCol/endLine/endCol location fields, which are emitted only when the locator
// resolves a position — appear) and assert its key set matches the schema definition's property set
// exactly. Location fields stay OPTIONAL in the schema on purpose: the config processor parses in
// basic mode by default and the locator can return nil even under detailed parsing, so locations are
// best-effort, not guaranteed. This test pins the key *vocabulary*, not presence.
func TestJSONOutputSchemaNoDrift(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "output-schema", "1.1.schema.json")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	loc := jsonLocationFields{StartLine: intPtr(1), StartCol: intPtr(2), EndLine: intPtr(3), EndCol: intPtr(4)}

	allChecks := jsonChecks{
		CircularDependencies:           &jsonCheckResult{Issues: []interface{}{}},
		OrphanFiles:                    &jsonCheckResult{Issues: []interface{}{}},
		ModuleBoundaries:               &jsonCheckResult{Issues: []interface{}{}},
		UnusedNodeModules:              &jsonCheckResult{Issues: []interface{}{}},
		MissingNodeModules:             &jsonCheckResult{Issues: []interface{}{}},
		ImportConventions:              &jsonCheckResult{Issues: []interface{}{}},
		UnresolvedImports:              &jsonCheckResult{Issues: []interface{}{}},
		UnusedExports:                  &jsonCheckResult{Issues: []interface{}{}},
		RestrictedDevDependenciesUsage: &jsonCheckResult{Issues: []interface{}{}},
		RestrictedImports:              &jsonCheckResult{Issues: []interface{}{}},
		RestrictedImporters:            &jsonCheckResult{Issues: []interface{}{}},
	}

	cases := []struct {
		name    string   // human label
		pointer []string // path of keys into the schema to the object node ({} = root)
		value   interface{}
	}{
		{"output (root)", nil, jsonOutput{Version: "1.1", Rules: []jsonRuleResult{}}},
		{"ruleResult", []string{"definitions", "ruleResult"}, jsonRuleResult{}},
		{"checks", []string{"definitions", "checks"}, allChecks},
		{"checkResult", []string{"definitions", "checkResult"}, jsonCheckResult{Issues: []interface{}{}}},
		{"fixSummary", []string{"definitions", "fixSummary"}, jsonFixSummary{}},
		{"circularDependencyIssue", []string{"definitions", "circularDependencyIssue"}, jsonCircularDependencyIssue{}},
		{"orphanFileIssue", []string{"definitions", "orphanFileIssue"}, jsonOrphanFileIssue{}},
		{"moduleBoundaryIssue", []string{"definitions", "moduleBoundaryIssue"}, jsonModuleBoundaryIssue{jsonLocationFields: loc}},
		{"unusedNodeModuleIssue", []string{"definitions", "unusedNodeModuleIssue"}, jsonUnusedNodeModuleIssue{jsonLocationFields: loc}},
		{"missingNodeModuleIssue", []string{"definitions", "missingNodeModuleIssue"}, jsonMissingNodeModuleIssue{Locations: []jsonLocation{{}}}},
		{"missingNodeModuleIssue.locations.items", []string{"definitions", "missingNodeModuleIssue", "properties", "locations", "items"}, jsonLocation{}},
		{"importConventionIssue", []string{"definitions", "importConventionIssue"}, jsonImportConventionIssue{jsonLocationFields: loc}},
		{"unresolvedImportIssue", []string{"definitions", "unresolvedImportIssue"}, jsonUnresolvedImportIssue{jsonLocationFields: loc}},
		{"unusedExportIssue", []string{"definitions", "unusedExportIssue"}, jsonUnusedExportIssue{jsonLocationFields: loc}},
		{"restrictedDevDepsIssue", []string{"definitions", "restrictedDevDepsIssue"}, jsonRestrictedDevDepsIssue{jsonLocationFields: loc}},
		{"restrictedImportIssue", []string{"definitions", "restrictedImportIssue"}, jsonRestrictedImportIssue{DeniedFile: "f", DeniedModule: "m", ImportRequest: "r", jsonLocationFields: loc}},
		{"restrictedImporterIssue", []string{"definitions", "restrictedImporterIssue"}, jsonRestrictedImporterIssue{File: "f", Module: "m"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := navigateSchema(t, schema, tc.pointer)

			if ap, ok := node["additionalProperties"]; !ok || ap != false {
				t.Errorf("%s: object must set additionalProperties:false so drift is caught", tc.name)
			}

			schemaKeys := propertyKeys(t, node)
			structKeys := marshalKeys(t, tc.value)

			if onlyStruct := difference(structKeys, schemaKeys); len(onlyStruct) > 0 {
				t.Errorf("%s: struct emits keys absent from schema (output would FAIL validation): %v", tc.name, onlyStruct)
			}
			if onlySchema := difference(schemaKeys, structKeys); len(onlySchema) > 0 {
				t.Errorf("%s: schema declares properties no struct field emits (dead schema): %v", tc.name, onlySchema)
			}
		})
	}
}

// navigateSchema walks the parsed schema following the given object keys.
func navigateSchema(t *testing.T, schema map[string]interface{}, pointer []string) map[string]interface{} {
	t.Helper()
	node := schema
	for _, key := range pointer {
		next, ok := node[key].(map[string]interface{})
		if !ok {
			t.Fatalf("schema path %s: key %q not found or not an object", strings.Join(pointer, "/"), key)
		}
		node = next
	}
	return node
}

// propertyKeys returns the sorted keys of an object node's "properties".
func propertyKeys(t *testing.T, node map[string]interface{}) []string {
	t.Helper()
	props, ok := node["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("schema node has no properties object")
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// marshalKeys marshals a value to JSON and returns its sorted top-level keys.
func marshalKeys(t *testing.T, v interface{}) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %T to map: %v", v, err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// difference returns elements present in a but not in b.
func difference(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, x := range b {
		set[x] = true
	}
	var out []string
	for _, x := range a {
		if !set[x] {
			out = append(out, x)
		}
	}
	return out
}
