package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/tidwall/jsonc"
)

// jsonedit is a self-contained, position-aware JSON/JSONC editor. It parses a JSONC
// document into a navigable node tree that carries byte offsets into the ORIGINAL
// file, then lets callers surgically remove array elements / object members or replace
// individual values — all while leaving every other byte (comments, key order,
// whitespace) untouched.
//
// It underpins two consumers in this repo:
//   - `config lint --fix`, which removes dead glob/path patterns (RemoveArrayElements,
//     RemoveObjectMembers, RemoveMember);
//   - `CompactConfigText`, which collapses detector declarations to their shorthand
//     form (ReplaceNode, RemoveMember).
//
// The whole approach relies on tidwall/jsonc.ToJSON blanking comments and trailing
// commas to spaces IN PLACE, keeping byte offsets identical between the stripped and
// original forms. ParseJSONC asserts that invariant and refuses to proceed otherwise,
// so a future change to jsonc can never silently corrupt a file.
//
// The file depends only on the standard library and tidwall/jsonc, so it can be copied
// verbatim into another codebase (adjust the `package` line).

// JSONKind classifies a parsed JSON value.
type JSONKind uint8

const (
	JSONObject JSONKind = iota
	JSONArray
	JSONString
	JSONPrimitive // number, bool, or null
)

// JSONNode is a parsed JSON value with byte offsets into the document.
// [Start, End) is the value's span; comments/whitespace are never part of it.
type JSONNode struct {
	Kind    JSONKind
	Start   int
	End     int
	Members []JSONMember // populated for JSONObject
	Elems   []*JSONNode  // populated for JSONArray
}

// JSONMember is a single `"key": value` object member.
type JSONMember struct {
	Name     string
	KeyStart int // offset of the key's opening quote
	ValueEnd int // == Value.End
	Value    *JSONNode
}

// GetMember returns the member with the given key, or nil.
func (n *JSONNode) GetMember(key string) *JSONMember {
	if n == nil || n.Kind != JSONObject {
		return nil
	}
	for i := range n.Members {
		if n.Members[i].Name == key {
			return &n.Members[i]
		}
	}
	return nil
}

// Get returns the value node for the given object key, or nil.
func (n *JSONNode) Get(key string) *JSONNode {
	m := n.GetMember(key)
	if m == nil {
		return nil
	}
	return m.Value
}

// Index returns the array element at i, or nil.
func (n *JSONNode) Index(i int) *JSONNode {
	if n == nil || n.Kind != JSONArray || i < 0 || i >= len(n.Elems) {
		return nil
	}
	return n.Elems[i]
}

// AsObjectOrElem resolves a value that may be written either as a single object or as
// an array of objects: an object is returned directly; an array yields its element at
// index. Useful for schemas with a "one or many" form.
func (n *JSONNode) AsObjectOrElem(index int) *JSONNode {
	if n == nil {
		return nil
	}
	if n.Kind == JSONObject {
		return n
	}
	if n.Kind == JSONArray {
		return n.Index(index)
	}
	return nil
}

// JSONDocument is a parsed JSONC document plus the original bytes for slicing.
type JSONDocument struct {
	Original []byte
	Root     *JSONNode
}

// ParseJSONC parses a JSON or JSONC document into a navigable, offset-carrying tree.
// It errors if comment-stripping would change byte offsets (see the package note).
func ParseJSONC(content []byte) (*JSONDocument, error) {
	stripped := jsonc.ToJSON(content)
	if len(stripped) != len(content) {
		return nil, fmt.Errorf("cannot edit JSONC: comment stripping changed byte offsets")
	}
	p := &jsonParser{buf: stripped}
	p.skipWS()
	root, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos != len(p.buf) {
		return nil, fmt.Errorf("unexpected trailing content at offset %d", p.pos)
	}
	return &JSONDocument{Original: content, Root: root}, nil
}

// RawText returns the original bytes of a node's value span, trimmed of surrounding
// whitespace (there is none inside a span, so this is effectively the literal text).
func (d *JSONDocument) RawText(n *JSONNode) string {
	if n == nil {
		return ""
	}
	return string(d.Original[n.Start:n.End])
}

// StringValue decodes a JSONString node using the original bytes.
func (d *JSONDocument) StringValue(n *JSONNode) (string, bool) {
	if n == nil || n.Kind != JSONString {
		return "", false
	}
	s, err := decodeJSONString(d.Original[n.Start:n.End])
	if err != nil {
		return "", false
	}
	return s, true
}

// ---- scanner ----

type jsonParser struct {
	buf []byte
	pos int
}

func (p *jsonParser) skipWS() {
	for p.pos < len(p.buf) {
		switch p.buf[p.pos] {
		case ' ', '\t', '\n', '\r':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonParser) parseValue() (*JSONNode, error) {
	p.skipWS()
	if p.pos >= len(p.buf) {
		return nil, fmt.Errorf("unexpected end of input")
	}
	switch c := p.buf[p.pos]; c {
	case '{':
		return p.parseObject()
	case '[':
		return p.parseArray()
	case '"':
		return p.parseString()
	default:
		return p.parsePrimitive()
	}
}

func (p *jsonParser) parseObject() (*JSONNode, error) {
	start := p.pos
	p.pos++ // consume '{'
	n := &JSONNode{Kind: JSONObject, Start: start}
	for {
		p.skipWS()
		if p.pos >= len(p.buf) {
			return nil, fmt.Errorf("unterminated object")
		}
		if p.buf[p.pos] == '}' {
			p.pos++
			n.End = p.pos
			return n, nil
		}
		if p.buf[p.pos] != '"' {
			return nil, fmt.Errorf("expected object key at offset %d", p.pos)
		}
		keyStart := p.pos
		keyNode, err := p.parseString()
		if err != nil {
			return nil, err
		}
		keyName, err := decodeJSONString(p.buf[keyNode.Start:keyNode.End])
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.buf) || p.buf[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' after key at offset %d", p.pos)
		}
		p.pos++ // consume ':'
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		n.Members = append(n.Members, JSONMember{
			Name:     keyName,
			KeyStart: keyStart,
			ValueEnd: val.End,
			Value:    val,
		})
		p.skipWS()
		if p.pos < len(p.buf) && p.buf[p.pos] == ',' {
			p.pos++
			continue
		}
	}
}

func (p *jsonParser) parseArray() (*JSONNode, error) {
	start := p.pos
	p.pos++ // consume '['
	n := &JSONNode{Kind: JSONArray, Start: start}
	for {
		p.skipWS()
		if p.pos >= len(p.buf) {
			return nil, fmt.Errorf("unterminated array")
		}
		if p.buf[p.pos] == ']' {
			p.pos++
			n.End = p.pos
			return n, nil
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		n.Elems = append(n.Elems, val)
		p.skipWS()
		if p.pos < len(p.buf) && p.buf[p.pos] == ',' {
			p.pos++
			continue
		}
	}
}

func (p *jsonParser) parseString() (*JSONNode, error) {
	start := p.pos
	p.pos++ // consume opening quote
	for p.pos < len(p.buf) {
		c := p.buf[p.pos]
		if c == '\\' {
			p.pos += 2
			continue
		}
		if c == '"' {
			p.pos++
			return &JSONNode{Kind: JSONString, Start: start, End: p.pos}, nil
		}
		p.pos++
	}
	return nil, fmt.Errorf("unterminated string at offset %d", start)
}

func (p *jsonParser) parsePrimitive() (*JSONNode, error) {
	start := p.pos
	for p.pos < len(p.buf) {
		switch p.buf[p.pos] {
		case ',', '}', ']', ' ', '\t', '\n', '\r':
			return &JSONNode{Kind: JSONPrimitive, Start: start, End: p.pos}, nil
		default:
			p.pos++
		}
	}
	return &JSONNode{Kind: JSONPrimitive, Start: start, End: p.pos}, nil
}

func decodeJSONString(raw []byte) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

// ---- edits ----

// Edit replaces the bytes in [Start, End) with Text. A pure deletion has Text == "".
type Edit struct {
	Start int
	End   int
	Text  string
}

// ApplyEdits applies non-overlapping edits to content and returns the new bytes.
// Edits may be given in any order; adjacent edits (End == next Start) are allowed.
// Overlapping or out-of-range edits are skipped defensively rather than corrupting the
// output, so callers must ensure their edits do not overlap.
func ApplyEdits(content []byte, edits []Edit) []byte {
	if len(edits) == 0 {
		return content
	}
	sorted := make([]Edit, len(edits))
	copy(sorted, edits)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	var buf bytes.Buffer
	last := 0
	for _, e := range sorted {
		if e.Start < last || e.Start < 0 || e.End < e.Start || e.End > len(content) {
			continue // overlap or out of range
		}
		buf.Write(content[last:e.Start])
		buf.WriteString(e.Text)
		last = e.End
	}
	buf.Write(content[last:])
	return buf.Bytes()
}

// ReplaceNode returns the edit that replaces a value node's span with text.
func ReplaceNode(n *JSONNode, text string) Edit {
	return Edit{Start: n.Start, End: n.End, Text: text}
}

// findTrailingCommaPositions returns the byte offsets of redundant trailing commas —
// a comma whose next significant token (skipping whitespace and comments) is a closing
// `}` or `]`. It is string- and comment-aware so commas inside strings or comments are
// never counted. The offsets index into content and each spans exactly one byte.
func findTrailingCommaPositions(content []byte) []int {
	var positions []int
	lastComma := -1 // offset of the most recent comma not yet followed by a value
	n := len(content)
	for i := 0; i < n; {
		c := content[i]
		switch {
		case c == '"':
			// Skip the string literal (a value) — the preceding comma was a separator.
			i++
			for i < n {
				if content[i] == '\\' {
					i += 2
					continue
				}
				if content[i] == '"' {
					i++
					break
				}
				i++
			}
			lastComma = -1
		case c == '/' && i+1 < n && content[i+1] == '/':
			i += 2
			for i < n && content[i] != '\n' {
				i++
			}
			// A comment does not reset lastComma: `, // note\n]` is still a trailing comma.
		case c == '/' && i+1 < n && content[i+1] == '*':
			i += 2
			for i+1 < n && !(content[i] == '*' && content[i+1] == '/') {
				i++
			}
			i += 2
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == ',':
			lastComma = i
			i++
		case c == '}' || c == ']':
			if lastComma >= 0 {
				positions = append(positions, lastComma)
			}
			lastComma = -1
			i++
		default:
			// Any other value token (number, true/false/null, or an opening bracket).
			lastComma = -1
			i++
		}
	}
	return positions
}

// TrailingCommaCount returns the number of redundant trailing commas in content.
func TrailingCommaCount(content []byte) int {
	return len(findTrailingCommaPositions(content))
}

// RemoveTrailingCommas returns the edits that delete every redundant trailing comma.
func RemoveTrailingCommas(content []byte) []Edit {
	positions := findTrailingCommaPositions(content)
	edits := make([]Edit, 0, len(positions))
	for _, p := range positions {
		edits = append(edits, Edit{Start: p, End: p + 1})
	}
	return edits
}

// ---- comment-safe structural removal ----

// span is the byte range [start,end) of a comma-separated item (array element or
// object member) within its container.
type span struct{ start, end int }

// lineIndentStart scans backward over spaces/tabs (not newlines) from pos and returns
// the offset where the indentation on pos's line begins.
func lineIndentStart(content []byte, pos int) int {
	for pos > 0 {
		c := content[pos-1]
		if c == ' ' || c == '\t' {
			pos--
			continue
		}
		break
	}
	return pos
}

// isLineStart reports whether pos begins a line (only indentation precedes it).
func isLineStart(content []byte, pos int) bool {
	return pos == 0 || content[pos-1] == '\n'
}

// includeInlineComment extends pos over trailing spaces/tabs and an inline `//`
// line-comment, stopping before the newline. If no comment follows, pos is returned
// unchanged. The terminating newline is never consumed.
func includeInlineComment(content []byte, pos int) int {
	k := pos
	for k < len(content) && (content[k] == ' ' || content[k] == '\t') {
		k++
	}
	if k+1 < len(content) && content[k] == '/' && content[k+1] == '/' {
		for k < len(content) && content[k] != '\n' {
			k++
		}
		if k > 0 && content[k-1] == '\r' {
			k--
		}
		return k
	}
	return pos
}

// endOfLineAfterComma consumes, starting just past a value: spaces/tabs, a comma,
// spaces/tabs, an optional inline comment, and the terminating newline. It returns the
// offset just past the newline (whole-line removal). Used for own-line items that have
// a following sibling.
func endOfLineAfterComma(content []byte, valEnd int) int {
	j := valEnd
	for j < len(content) && (content[j] == ' ' || content[j] == '\t') {
		j++
	}
	if j < len(content) && content[j] == ',' {
		j++
	}
	if k := includeInlineComment(content, j); k > j {
		j = k
	}
	for j < len(content) && (content[j] == ' ' || content[j] == '\t') {
		j++
	}
	if j < len(content) && content[j] == '\r' {
		j++
	}
	if j < len(content) && content[j] == '\n' {
		return j + 1
	}
	return j
}

// afterCommaInline consumes, starting just past a value: spaces/tabs, a comma, and a
// single following space. Used for inline items that have a following sibling.
func afterCommaInline(content []byte, valEnd int) int {
	j := valEnd
	for j < len(content) && (content[j] == ' ' || content[j] == '\t') {
		j++
	}
	if j < len(content) && content[j] == ',' {
		j++
	}
	if j < len(content) && content[j] == ' ' {
		j++
	}
	return j
}

// singleItemRemoval computes the deletion for one item removed on its own. prevEnd is
// the end of the preceding surviving sibling; it is only consulted when isLast.
func singleItemRemoval(content []byte, itemStart, itemEnd int, isLast bool, prevEnd int) Edit {
	if isLast {
		return Edit{Start: prevEnd, End: includeInlineComment(content, itemEnd)}
	}
	if isLineStart(content, lineIndentStart(content, itemStart)) {
		return Edit{Start: lineIndentStart(content, itemStart), End: endOfLineAfterComma(content, itemEnd)}
	}
	return Edit{Start: itemStart, End: afterCommaInline(content, itemEnd)}
}

// removeSequenceItems returns non-overlapping deletions that remove the given item
// indices from a comma-separated sequence (array elements or object members). It must
// be called only when at least one item survives; when every item is dead, remove the
// whole container/member instead.
func removeSequenceItems(content []byte, items []span, deadIdx []int) []Edit {
	n := len(items)
	if n == 0 {
		return nil
	}
	dead := make([]bool, n)
	deadCount := 0
	for _, i := range deadIdx {
		if i >= 0 && i < n && !dead[i] {
			dead[i] = true
			deadCount++
		}
	}
	if deadCount == 0 || deadCount == n {
		return nil
	}

	trailingStart := n
	for i := n - 1; i >= 0 && dead[i]; i-- {
		trailingStart = i
	}

	var edits []Edit
	handled := make([]bool, n)

	// The trailing dead run is removed as one range from the end of the last survivor
	// through the last dead item, dropping the survivor's now-dangling comma. A survivor
	// exists here, so trailingStart > 0. The newline before the closing bracket stays.
	if trailingStart < n {
		prevEnd := items[trailingStart-1].end
		delEnd := includeInlineComment(content, items[n-1].end)
		edits = append(edits, Edit{Start: prevEnd, End: delEnd})
		for i := trailingStart; i < n; i++ {
			handled[i] = true
		}
	}

	for i := 0; i < n; i++ {
		if !dead[i] || handled[i] {
			continue
		}
		edits = append(edits, singleItemRemoval(content, items[i].start, items[i].end, false, 0))
	}
	return edits
}

func arraySpans(arr *JSONNode) []span {
	items := make([]span, len(arr.Elems))
	for i, e := range arr.Elems {
		items[i] = span{e.Start, e.End}
	}
	return items
}

func objectSpans(obj *JSONNode) []span {
	items := make([]span, len(obj.Members))
	for i, m := range obj.Members {
		items[i] = span{m.KeyStart, m.ValueEnd}
	}
	return items
}

// RemoveArrayElements removes the given element indices from arr, assuming at least one
// element survives. Returns nil when arr is not an array or nothing survives.
func RemoveArrayElements(content []byte, arr *JSONNode, deadIdx []int) []Edit {
	if arr == nil || arr.Kind != JSONArray {
		return nil
	}
	return removeSequenceItems(content, arraySpans(arr), deadIdx)
}

// RemoveObjectMembers removes the members at the given key indices from obj, assuming
// at least one member survives.
func RemoveObjectMembers(content []byte, obj *JSONNode, keyIdx []int) []Edit {
	if obj == nil || obj.Kind != JSONObject {
		return nil
	}
	return removeSequenceItems(content, objectSpans(obj), keyIdx)
}

// RemoveMember removes the whole `"key": value` member from obj. Returns ok=false if
// the key is absent. When it is the object's only member, the object body is emptied,
// leaving the `{}` shape intact.
func RemoveMember(content []byte, obj *JSONNode, key string) (Edit, bool) {
	if obj == nil || obj.Kind != JSONObject {
		return Edit{}, false
	}
	idx := -1
	for i := range obj.Members {
		if obj.Members[i].Name == key {
			idx = i
			break
		}
	}
	if idx == -1 {
		return Edit{}, false
	}

	if len(obj.Members) == 1 {
		// Sole member: clear the whole interior between the braces, leaving `{}`.
		return Edit{Start: obj.Start + 1, End: obj.End - 1}, true
	}

	edits := removeSequenceItems(content, objectSpans(obj), []int{idx})
	if len(edits) == 1 {
		return edits[0], true
	}
	return Edit{}, false
}
