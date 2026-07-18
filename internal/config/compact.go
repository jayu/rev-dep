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
// For an array-valued detector: a single-element array is equivalent to the bare value, so it is
// unwrapped ([{"entryPoints": ...}] → {"entryPoints": ...}, [true] → true) and its element compacted
// like a single detector. A multi-element array has each element compacted in place, except an
// element is never collapsed to a bare boolean — booleans are only meaningful for the single form,
// so a pure {"enabled": ...} element inside a multi-element array is left as-is.
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
// boolean shorthand or drop a redundant `enabled`. A single-element array is equivalent to the bare
// value, so it is unwrapped and its element compacted as if written directly (`[true]` → `true`,
// `[{...}]` → `{...}`). A multi-element array has each element compacted individually but is never
// collapsed, and its elements are never folded to a bare boolean. Anything else (a value already
// written as a boolean) is left alone.
func compactDetectorValue(doc *JSONDocument, value *JSONNode) []Edit {
	switch value.Kind {
	case JSONObject:
		return compactDetectorObject(doc, value, true)
	case JSONArray:
		if len(value.Elems) == 1 && canUnwrapArray(doc.Original, value, value.Elems[0]) {
			return []Edit{unwrapSingleElementArray(doc, value, value.Elems[0])}
		}
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

// unwrapSingleElementArray produces the single edit that replaces a one-element detector array with
// its element in compact form — dropping the surrounding brackets and any inner whitespace. The
// element is compacted with the single-value rules (bool-fold allowed), so an already-compactable
// element is simplified in the same pass, and its continuation lines are dedented to make up for
// the array nesting level that is being removed.
func unwrapSingleElementArray(doc *JSONDocument, array, elem *JSONNode) Edit {
	elemText := applyEditsToSpan(doc.Original, elem.Start, elem.End, compactDetectorValue(doc, elem))
	elemText = reindentUnwrappedElement(doc.Original, array, elem, elemText)
	return ReplaceNode(array, elemText)
}

// reindentUnwrappedElement removes the extra indentation an array element carried from being nested
// one level inside the array. Once the brackets are dropped the element sits at the key's
// indentation, so each of its continuation lines (everything after the first) is dedented by the
// difference between the element's indent and the key line's indent — which is exactly the file's
// indentation step at this point. Single-line elements are returned unchanged.
func reindentUnwrappedElement(content []byte, array, elem *JSONNode, elemText string) string {
	if !strings.Contains(elemText, "\n") {
		return elemText
	}
	dedent := lineIndentWidth(content, elem.Start) - lineIndentWidth(content, array.Start)
	if dedent <= 0 {
		return elemText
	}
	lines := strings.Split(elemText, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = trimLeadingWhitespaceUpTo(lines[i], dedent)
	}
	return strings.Join(lines, "\n")
}

// lineIndentWidth returns the number of leading whitespace characters on the line that contains pos.
func lineIndentWidth(content []byte, pos int) int {
	lineStart := pos
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	width := 0
	for lineStart+width < len(content) {
		if ch := content[lineStart+width]; ch != ' ' && ch != '\t' {
			break
		}
		width++
	}
	return width
}

// trimLeadingWhitespaceUpTo drops at most limit leading whitespace characters from line.
func trimLeadingWhitespaceUpTo(line string, limit int) string {
	removed, index := 0, 0
	for index < len(line) && removed < limit && (line[index] == ' ' || line[index] == '\t') {
		index++
		removed++
	}
	return line[index:]
}

// canUnwrapArray reports whether a one-element array can be safely collapsed to its element. The
// space between the brackets and the element (on both sides) must hold nothing but whitespace and
// at most the element's trailing comma — a comment there would be lost by the collapse, so such an
// array is left as an array to honor the comment-preservation guarantee.
func canUnwrapArray(content []byte, array, elem *JSONNode) bool {
	beforeElem := content[array.Start+1 : elem.Start] // after '['
	afterElem := content[elem.End : array.End-1]      // before ']'
	return isWhitespaceOrComma(beforeElem) && isWhitespaceOrComma(afterElem)
}

// isWhitespaceOrComma reports whether b contains only JSON whitespace and comma bytes.
func isWhitespaceOrComma(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' && c != ',' {
			return false
		}
	}
	return true
}

// applyEditsToSpan applies edits (given in whole-document coordinates and lying within
// [start, end)) to that sub-slice of content and returns the rewritten span as a string.
func applyEditsToSpan(content []byte, start, end int, edits []Edit) string {
	local := make([]Edit, len(edits))
	for i, edit := range edits {
		local[i] = Edit{Start: edit.Start - start, End: edit.End - start, Text: edit.Text}
	}
	return string(ApplyEdits(content[start:end], local))
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
	if edits, ok := RemoveMember(doc.Original, obj, "enabled"); ok {
		return edits
	}
	return nil
}
