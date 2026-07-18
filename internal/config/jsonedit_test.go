package config

import "testing"

// ---- parser ----

func TestParseJSONC_TreeAndComments(t *testing.T) {
	src := `{
  // leading comment
  "configVersion": "1.11", // inline
  "ignoreFiles": [
    "a.ts",
    "b.ts"
  ],
  "count": 42,
  "flag": true,
  "nested": { "deep": ["x"] }
}`
	doc, err := ParseJSONC([]byte(src))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if doc.Root.Kind != JSONObject {
		t.Fatalf("expected object root")
	}
	arr := doc.Root.Get("ignoreFiles")
	if arr == nil || arr.Kind != JSONArray || len(arr.Elems) != 2 {
		t.Fatalf("expected ignoreFiles array of 2, got %+v", arr)
	}
	if v, ok := doc.StringValue(arr.Elems[0]); !ok || v != "a.ts" {
		t.Fatalf("expected a.ts, got %q ok=%v", v, ok)
	}
	if got := doc.RawText(doc.Root.Get("count")); got != "42" {
		t.Fatalf("count RawText=%q, want 42", got)
	}
	if got := doc.RawText(doc.Root.Get("flag")); got != "true" {
		t.Fatalf("flag RawText=%q, want true", got)
	}
	// Nested navigation.
	deep := doc.Root.Get("nested").Get("deep")
	if deep == nil || deep.Kind != JSONArray || len(deep.Elems) != 1 {
		t.Fatalf("nested.deep not navigable: %+v", deep)
	}
}

func TestParseJSONC_StringEscapes(t *testing.T) {
	// A key/value with escaped quotes and backslashes must not confuse span scanning.
	src := `{ "a\"b": "c\\d\"e", "next": 1 }`
	doc, err := ParseJSONC([]byte(src))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	m := doc.Root.GetMember(`a"b`)
	if m == nil {
		t.Fatalf("escaped key not found; members=%v", memberNames(doc.Root))
	}
	if v, _ := doc.StringValue(m.Value); v != `c\d"e` {
		t.Fatalf("escaped value=%q", v)
	}
	if doc.Root.Get("next") == nil {
		t.Fatalf("member after escaped string lost")
	}
}

func TestParseJSONC_RejectsOffsetShift(t *testing.T) {
	// Sanity: valid JSONC parses; malformed trailing content errors.
	if _, err := ParseJSONC([]byte(`{"a":1} trailing`)); err == nil {
		t.Fatalf("expected error on trailing content")
	}
}

func memberNames(n *JSONNode) []string {
	var out []string
	for _, m := range n.Members {
		out = append(out, m.Name)
	}
	return out
}

// ---- ApplyEdits ----

func TestApplyEdits_ReplaceAndDelete(t *testing.T) {
	src := "hello world"
	got := string(ApplyEdits([]byte(src), []Edit{
		{Start: 0, End: 5, Text: "HI"}, // replace "hello"
		{Start: 6, End: 11, Text: ""},  // delete "world"
	}))
	if got != "HI " {
		t.Fatalf("got %q, want %q", got, "HI ")
	}
}

func TestApplyEdits_UnorderedAndAdjacent(t *testing.T) {
	src := "abcdef"
	// Adjacent edits given out of order.
	got := string(ApplyEdits([]byte(src), []Edit{
		{Start: 3, End: 6, Text: "Z"},
		{Start: 0, End: 3, Text: "Y"},
	}))
	if got != "YZ" {
		t.Fatalf("got %q, want %q", got, "YZ")
	}
}

func TestApplyEdits_SkipsOverlap(t *testing.T) {
	src := "abcdef"
	got := string(ApplyEdits([]byte(src), []Edit{
		{Start: 0, End: 4, Text: "X"},
		{Start: 2, End: 6, Text: "Y"}, // overlaps the first -> skipped
	}))
	if got != "Xef" {
		t.Fatalf("got %q, want %q", got, "Xef")
	}
}

// ---- array element removal ----

func removeElems(t *testing.T, src, key string, idx ...int) string {
	t.Helper()
	doc, err := ParseJSONC([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	arr := doc.Root.Get(key)
	return string(ApplyEdits(doc.Original, RemoveArrayElements(doc.Original, arr, idx)))
}

func TestRemoveArrayElement_Middle(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\",\n    \"dead.ts\",\n    \"b.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\",\n    \"b.ts\"\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 1); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_Last(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\",\n    \"dead.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\"\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 1); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_First(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"dead.ts\",\n    \"b.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"b.ts\"\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 0); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_PreservesTrailingComment(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\", // keep\n    \"dead.ts\", // gone\n    \"b.ts\" // keep2\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\", // keep\n    \"b.ts\" // keep2\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 1); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_PreservesLeadingCommentOfNext(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"dead.ts\",\n    // comment for b\n    \"b.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    // comment for b\n    \"b.ts\"\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 0); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_MultipleIncludingTrailingRun(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\",\n    \"dead1.ts\",\n    \"b.ts\",\n    \"dead2.ts\",\n    \"dead3.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\",\n    \"b.ts\"\n  ]\n}"
	if got := removeElems(t, src, "ignoreFiles", 1, 3, 4); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveArrayElement_Inline(t *testing.T) {
	src := `{"ignoreFiles": ["a.ts", "dead.ts", "b.ts"]}`
	want := `{"ignoreFiles": ["a.ts", "b.ts"]}`
	if got := removeElems(t, src, "ignoreFiles", 1); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// ---- object member removal ----

func TestRemoveMember_Middle(t *testing.T) {
	src := "{\n  \"configVersion\": \"1.11\",\n  \"ignoreFiles\": [\n    \"a.ts\"\n  ],\n  \"rules\": []\n}"
	want := "{\n  \"configVersion\": \"1.11\",\n  \"rules\": []\n}"
	doc, _ := ParseJSONC([]byte(src))
	edits, ok := RemoveMember(doc.Original, doc.Root, "ignoreFiles")
	if !ok {
		t.Fatal("member not found")
	}
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveMember_Last(t *testing.T) {
	src := "{\n  \"configVersion\": \"1.11\",\n  \"ignoreFiles\": [\n    \"a.ts\"\n  ]\n}"
	want := "{\n  \"configVersion\": \"1.11\"\n}"
	doc, _ := ParseJSONC([]byte(src))
	edits, ok := RemoveMember(doc.Original, doc.Root, "ignoreFiles")
	if !ok {
		t.Fatal("member not found")
	}
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveMember_Only(t *testing.T) {
	src := `{ "enabled": true }`
	want := `{}`
	doc, _ := ParseJSONC([]byte(src))
	edits, ok := RemoveMember(doc.Original, doc.Root, "enabled")
	if !ok {
		t.Fatal("member not found")
	}
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// ---- value replacement ----

func TestReplaceNode(t *testing.T) {
	src := `{ "orphanFilesDetection": { "enabled": true } }`
	doc, _ := ParseJSONC([]byte(src))
	node := doc.Root.Get("orphanFilesDetection")
	got := string(ApplyEdits(doc.Original, []Edit{ReplaceNode(node, "true")}))
	want := `{ "orphanFilesDetection": true }`
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestRemoveArrayElement_TrailingPreservesSurvivorComment is the regression test for the
// trailing-run bug: removing the LAST element must not swallow an inline comment that
// belongs to the surviving element on the line above it.
func TestRemoveArrayElement_TrailingPreservesSurvivorComment(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\", // KEEP ME\n    \"dead.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\" // KEEP ME\n  ]\n}"
	doc, _ := ParseJSONC([]byte(src))
	arr := doc.Root.Members[0].Value
	edits := RemoveArrayElements(doc.Original, arr, []int{1})
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestRemoveArrayElement_TrailingMultipleWithComment removes several trailing dead elements
// while the survivor carries an inline comment.
func TestRemoveArrayElement_TrailingMultipleWithComment(t *testing.T) {
	src := "{\n  \"ignoreFiles\": [\n    \"a.ts\", // note\n    \"d1.ts\",\n    \"d2.ts\"\n  ]\n}"
	want := "{\n  \"ignoreFiles\": [\n    \"a.ts\" // note\n  ]\n}"
	doc, _ := ParseJSONC([]byte(src))
	arr := doc.Root.Members[0].Value
	edits := RemoveArrayElements(doc.Original, arr, []int{1, 2})
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

// TestRemoveMember_LastPreservesSurvivorComment verifies the same comment-preservation for
// removing the last object member (RemoveObjectMembers path).
func TestRemoveMember_LastPreservesSurvivorComment(t *testing.T) {
	src := "{\n  \"a\": 1, // KEEP\n  \"dead\": 2\n}"
	want := "{\n  \"a\": 1 // KEEP\n}"
	doc, _ := ParseJSONC([]byte(src))
	edits := RemoveObjectMembers(doc.Original, doc.Root, []int{1})
	if got := string(ApplyEdits(doc.Original, edits)); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}
