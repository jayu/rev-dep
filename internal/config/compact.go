package config

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
// `.jsonc` config. All parsing and byte manipulation is delegated to the position-aware jsonedit
// engine (see jsonedit.go), which is also what makes the offset bookkeeping and comment-safe
// member removal reliable.
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
	return ApplyEdits(doc.Original, compactEdits(doc)), nil
}

// compactEdits returns the edits that rewrite a parsed config's detector declarations to
// their compact form (see CompactConfigText). An empty slice means the config is already
// compact. This is shared by CompactConfigText and the `compact` lint rule.
func compactEdits(doc *JSONDocument) []Edit {
	rules := doc.Root.Get("rules")
	if rules == nil || rules.Kind != JSONArray {
		return nil
	}

	var edits []Edit
	for _, rule := range rules.Elems {
		if rule.Kind != JSONObject {
			continue
		}
		for i := range rule.Members {
			member := &rule.Members[i]
			if !detectorFieldNames[member.Name] {
				continue
			}
			edits = append(edits, compactDetectorValue(doc, member.Value)...)
		}
	}
	return edits
}

// compactDetectorValue returns the edits that compact a single detector value. Objects fold to the
// boolean shorthand or drop a redundant `enabled`; array elements are compacted individually but are
// never collapsed to a bare boolean; anything else (a value already written as a boolean) is left
// alone.
func compactDetectorValue(doc *JSONDocument, value *JSONNode) []Edit {
	switch value.Kind {
	case JSONObject:
		return compactDetectorObject(doc, value, true)
	case JSONArray:
		var edits []Edit
		for _, elem := range value.Elems {
			if elem.Kind == JSONObject {
				edits = append(edits, compactDetectorObject(doc, elem, false)...)
			}
		}
		return edits
	default:
		return nil
	}
}

// compactDetectorObject returns the edits for a single detector object. When allowBoolFold is true a
// pure enabled/empty object collapses to a bare boolean; when false (array element) it is left
// untouched instead.
func compactDetectorObject(doc *JSONDocument, obj *JSONNode, allowBoolFold bool) []Edit {
	enabled := obj.GetMember("enabled")
	enabledValue := true // an absent `enabled` means the detector is enabled
	if enabled != nil {
		enabledValue = doc.RawText(enabled.Value) == "true"
	}

	otherCount := len(obj.Members)
	if enabled != nil {
		otherCount--
	}

	// Only `enabled` (or an empty object): collapse to the boolean shorthand.
	if otherCount == 0 {
		if !allowBoolFold {
			return nil
		}
		text := "true"
		if !enabledValue {
			text = "false"
		}
		return []Edit{ReplaceNode(obj, text)}
	}

	// Has other options. A redundant enabled==true is dropped; a disabled object keeps its explicit
	// "enabled": false, and an object that already omits enabled is left as-is.
	if enabled == nil || !enabledValue {
		return nil
	}
	if edit, ok := RemoveMember(doc.Original, obj, "enabled"); ok {
		return []Edit{edit}
	}
	return nil
}
