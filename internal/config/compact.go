package config

import "strings"

// detectorFieldNames is the set of rule-level fields whose values are detector declarations that can
// be written in the compact (boolean / optional-`enabled`) form.
var detectorFieldNames = map[string]bool{
	"circularImportsDetection":           true,
	"orphanFilesDetection":               true,
	"unusedNodeModulesDetection":         true,
	"missingNodeModulesDetection":        true,
	"unusedExportsDetection":             true,
	"unresolvedImportsDetection":         true,
	"devDepsUsageOnProdDetection":        true,
	"restrictedImportsDetection":         true,
	"restrictedImportersDetection":       true,
	"restrictedDirectImportersDetection": true,
}

// CompactConfigText rewrites a rev-dep config document so detector declarations use their most
// compact equivalent form. It edits only detector values and leaves every other byte — including
// comments, key order, and whitespace — unchanged, so it is safe to run over a hand-written
// `.jsonc` config. Parsing and byte-precise editing are delegated to the shared jsonedit engine
// (see jsonedit.go).
//
// Each detector value is rewritten as follows:
//   - an object whose only content is `enabled` (or an empty object) collapses to the boolean
//     shorthand: {"enabled": true} → true, {"enabled": false} → false, {} → true;
//   - an enabled object that carries other options drops the now-redundant "enabled": true key;
//   - a disabled object with other options ({"enabled": false, ...}), a value already written as a
//     boolean, and an object that already omits `enabled` are all left unchanged.
//
// For an array-valued detector each element is compacted the same way, except an element is never
// collapsed to a bare boolean — booleans are only meaningful for the single-detector form, so a
// pure {"enabled": ...} element inside an array is left as-is.
func CompactConfigText(raw []byte) ([]byte, error) {
	doc, err := ParseJSONC(raw)
	if err != nil {
		return nil, err
	}

	var edits []Edit
	if rules := doc.Root.Get("rules"); rules != nil && rules.Kind == JSONArray {
		for _, ruleNode := range rules.Elems {
			if ruleNode.Kind != JSONObject {
				continue
			}
			for i := range ruleNode.Members {
				m := ruleNode.Members[i]
				if detectorFieldNames[m.Name] {
					edits = append(edits, compactDetectorValue(doc, m.Value)...)
				}
			}
		}
	}

	return ApplyEdits(raw, edits), nil
}

// compactDetectorValue returns the edits that compact a single detector value, which may be a
// single object or an array of objects.
func compactDetectorValue(doc *JSONDocument, value *JSONNode) []Edit {
	switch value.Kind {
	case JSONObject:
		return compactDetectorObject(doc, value, true)
	case JSONArray:
		var edits []Edit
		for _, el := range value.Elems {
			if el.Kind == JSONObject {
				edits = append(edits, compactDetectorObject(doc, el, false)...)
			}
		}
		return edits
	default:
		// Already a boolean (or other scalar) — nothing to compact.
		return nil
	}
}

// compactDetectorObject computes the edits for a single detector object. When allowBoolFold is true
// a pure enabled/empty object collapses to a bare boolean; when false (array element) it is left
// untouched instead.
func compactDetectorObject(doc *JSONDocument, obj *JSONNode, allowBoolFold bool) []Edit {
	enabledPresent := false
	enabledValue := true // absent `enabled` means enabled
	for i := range obj.Members {
		if obj.Members[i].Name == "enabled" {
			enabledPresent = true
			enabledValue = strings.TrimSpace(doc.RawText(obj.Members[i].Value)) == "true"
		}
	}

	otherCount := len(obj.Members)
	if enabledPresent {
		otherCount--
	}

	// Only `enabled` (or an empty object): collapse to the boolean shorthand.
	if otherCount == 0 {
		if !allowBoolFold {
			return nil
		}
		replacement := "true"
		if !enabledValue {
			replacement = "false"
		}
		return []Edit{ReplaceNode(obj, replacement)}
	}

	// Has other options. A redundant "enabled": true is dropped; a disabled object keeps its
	// explicit "enabled": false, and an object that already omits enabled is left as-is.
	if !enabledPresent || !enabledValue {
		return nil
	}
	if ch, ok := RemoveMember(doc.Original, obj, "enabled"); ok {
		return []Edit{ch}
	}
	return nil
}
