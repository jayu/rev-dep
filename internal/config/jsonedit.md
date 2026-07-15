# `jsonedit` — shared JSONC in-place editor

`internal/config/jsonedit.go` is a self-contained, position-aware JSON/JSONC editor.
It parses a document into a navigable node tree carrying byte offsets, then removes
array elements / object members or replaces individual values **without touching any
other byte** — comments, key order, and whitespace are preserved.

It replaced two separate implementations that had drifted apart:

- the `config lint --fix` scanner/remover (comment-safe structural **removal**);
- `CompactConfigText`'s decoder-based editor (value **replacement** + property drop).

If you brought the old `compact.go` (with its `textEdit` / `applyTextEdits` /
`findDetectorSpans` / `objectPropertySpans` / `arrayElementSpans` helpers), **delete
those helpers** and call `jsonedit` instead. This doc tells you how.

## Why this one won

| Capability | Kept from | Notes |
|---|---|---|
| Navigable node tree (any depth) | lint scanner | one parse, then `Get`/`Index`/`GetMember` |
| Comment-safe element/member removal | lint scanner | preserves trailing + next-sibling leading comments |
| Arbitrary value replacement | old `compact.go` | `ReplaceNode(node, "true")` |
| Offset-preservation guard | old `compact.go` | `ParseJSONC` errors if `jsonc.ToJSON` ever changes length |
| Zero non-stdlib deps (only `tidwall/jsonc`) | old `compact.go` | copy the file, change the `package` line |

## Portability

The file imports only `bytes`, `encoding/json`, `fmt`, `sort`, and
`github.com/tidwall/jsonc`. To use it in another module: copy `jsonedit.go`, change the
`package` declaration, and ensure `github.com/tidwall/jsonc` is in `go.mod`. Nothing
else is required — it does **not** depend on `internal/sourceedit` or anything else in
this repo.

## The invariant it relies on

`jsonc.ToJSON` blanks comments and trailing commas to spaces **in place**, so the
stripped bytes are the same length as the original and every offset lines up. All edits
are computed against the stripped bytes but sliced/written against the original.
`ParseJSONC` asserts `len(stripped) == len(original)` and returns an error otherwise, so
a future `jsonc` change can never silently corrupt a file.

## API

```go
doc, err := ParseJSONC(raw)          // *JSONDocument{ Original []byte; Root *JSONNode }
```

Navigation (all nil-safe):

```go
n.Get("key")            // *JSONNode value of an object member, or nil
n.GetMember("key")      // *JSONMember (Name, KeyStart, ValueEnd, Value)
n.Index(i)              // *JSONNode i-th array element, or nil
n.AsObjectOrElem(i)     // object -> itself; array -> element i (for "one or many" schemas)
doc.StringValue(n)      // decoded Go string of a JSONString node
doc.RawText(n)          // literal source text of a node's span (e.g. "true", "42")
```

`JSONNode` fields: `Kind` (`JSONObject|JSONArray|JSONString|JSONPrimitive`),
`Start`, `End` (value span, no surrounding whitespace), `Members`, `Elems`.

Edits — build a `[]Edit`, then apply once:

```go
type Edit struct { Start, End int; Text string } // replace [Start,End) with Text; Text=="" deletes

ReplaceNode(node, "true")                       // replace a value span
RemoveMember(doc.Original, obj, "enabled")      // drop a whole "key": value member -> (Edit, ok)
RemoveArrayElements(doc.Original, arr, []int{1, 3})  // drop element indices (>=1 must survive)
RemoveObjectMembers(doc.Original, obj, []int{0})     // drop member indices (>=1 must survive)

out := ApplyEdits(raw, edits)                   // []byte; edits may be unordered; overlaps skipped
```

Rules:

- Callers must ensure their edits are **non-overlapping**. `ApplyEdits` skips overlaps
  defensively rather than corrupting output, so an overlap silently drops an edit —
  don't rely on that, keep them disjoint. Adjacent edits (`End == next Start`) are fine.
- For removal, if **every** element/member of a container is dead, don't call
  `RemoveArrayElements`/`RemoveObjectMembers` (they no-op when nothing survives) — remove
  the owning member with `RemoveMember`, or replace the whole value with `ReplaceNode`.

## Migrating the old `compact.go` helpers

| Old helper | Replace with |
|---|---|
| `findDetectorSpans` + `walk*` decoder walk | `ParseJSONC` then walk `doc.Root.Get("rules").Elems` and each rule's `Members` |
| `objectPropertySpans` | `objNode.Members` (each `JSONMember` has `KeyStart`, `Value.Start/End`) |
| `arrayElementSpans` | `arrNode.Elems` (each `*JSONNode` has `Start/End`) |
| `textEdit{start,end,replacement}` | `Edit{Start,End,Text}` |
| `applyTextEdits` | `ApplyEdits` |
| `removeProperty` | `RemoveMember(content, obj, key)` — and it is comment-safe, unlike the old one |
| collapse object to bool | `ReplaceNode(node, "true"|"false")` |
| length-preservation check | built into `ParseJSONC` |

The current `compact.go` in this repo is already ported and is the reference example —
`CompactConfigText` is ~30 lines of decision logic over the tree, with all parsing and
byte manipulation delegated to `jsonedit`.

## Tests

`jsonedit_test.go` covers the engine (parsing incl. escaped strings and nesting,
`ApplyEdits` ordering/overlap/replace, every element-removal shape incl. comment
preservation, member removal, `ReplaceNode`). `compact_test.go` covers the compaction
consumer end-to-end incl. comment/format preservation. Mirror these when you adapt.
