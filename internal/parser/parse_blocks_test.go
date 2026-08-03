package parser

import (
	"strings"
	"testing"
)

// scanForTest returns the original-source text of every block the scanner reported, in the
// order they closed (innermost first), so a test can assert on boundaries directly.
func scanForTest(t *testing.T, code string, opts BlockScanOptions) (blocks []string, kinds []BlockKind, norm string) {
	t.Helper()
	src := []byte(code)
	res := ScanCodeBlocks(src, opts, nil)
	for _, b := range res.Blocks {
		if int(b.End) > len(src) || b.Start > b.End {
			t.Fatalf("block offsets out of range: %+v (len %d)", b, len(src))
		}
		blocks = append(blocks, string(src[b.Start:b.End]))
		kinds = append(kinds, b.Kind)
	}
	return blocks, kinds, string(res.Norm)
}

func literalOpts() BlockScanOptions {
	return BlockScanOptions{Blinding: Blinding{}, AllowJSX: true, MinTokens: 1}
}

func structuralOpts() BlockScanOptions {
	return BlockScanOptions{Blinding: Blinding{Identifiers: true}, AllowJSX: true, MinTokens: 1}
}

func normalizeOnly(t *testing.T, code string, blinding Blinding) string {
	t.Helper()
	res := ScanCodeBlocks([]byte(code), BlockScanOptions{Blinding: blinding, AllowJSX: true}, nil)
	return string(res.Norm)
}

// ---------------------------------------------------------------------------
// brace blocks
// ---------------------------------------------------------------------------

func TestScanBlocksRecordsBraceBoundaries(t *testing.T) {
	code := `function a() { return 1 }`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %q", len(blocks), blocks)
	}
	if blocks[0] != "{ return 1 }" {
		t.Errorf("block = %q, want %q", blocks[0], "{ return 1 }")
	}
	if kinds[0] != BlockBrace {
		t.Errorf("kind = %v, want BlockBrace", kinds[0])
	}
}

func TestScanBlocksRecordsNestedBlocksAtEveryLevel(t *testing.T) {
	code := `function outer() {
  if (x) {
    while (y) { z() }
  }
}`
	blocks, _, _ := scanForTest(t, code, literalOpts())

	// One block per level: the while body, the if body, the function body.
	if len(blocks) != 3 {
		t.Fatalf("expected 3 nested blocks, got %d: %#v", len(blocks), blocks)
	}
	// Blocks close innermost-first.
	if blocks[0] != "{ z() }" {
		t.Errorf("innermost block = %q", blocks[0])
	}
	if !strings.HasPrefix(blocks[1], "{\n    while") {
		t.Errorf("if body = %q", blocks[1])
	}
	if !strings.HasPrefix(blocks[2], "{\n  if") || !strings.HasSuffix(blocks[2], "}") {
		t.Errorf("function body = %q", blocks[2])
	}
}

// Depth is how many levels a block SPANS, counting itself - which is what --min-depth acts
// on. Blocks are emitted on close, so the inner one comes first.
func TestScanBlocksReportsDepth(t *testing.T) {
	code := `function a() { if (b) { c() } }`
	res := ScanCodeBlocks([]byte(code), literalOpts(), nil)
	if len(res.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(res.Blocks))
	}
	if res.Blocks[0].Depth != 1 {
		t.Errorf("the innermost block spans %d levels, want 1", res.Blocks[0].Depth)
	}
	if res.Blocks[1].Depth != 2 {
		t.Errorf("the function body contains one nested level, so it spans %d, want 2",
			res.Blocks[1].Depth)
	}
}

func TestScanBlocksIncludesObjectLiterals(t *testing.T) {
	code := `const config = { retries: 3, timeout: 1000 }`
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 || blocks[0] != "{ retries: 3, timeout: 1000 }" {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestScanBlocksAppliesMinTokens(t *testing.T) {
	// Canonical forms are `{c()}` (5 bytes) and `{if(b){c()}}` (12 bytes).
	code := `function a() { if (b) { c() } }`
	opts := literalOpts()
	opts.MinTokens = 10
	res := ScanCodeBlocks([]byte(code), opts, nil)

	for _, b := range res.Blocks {
		if b.NormEnd-b.NormStart < 10 {
			t.Fatalf("block below MinTokens survived: %+v", b)
		}
	}
	if len(res.Blocks) != 1 {
		t.Fatalf("expected only the outer block to survive, got %d", len(res.Blocks))
	}
}

// ---------------------------------------------------------------------------
// things the scanner must not mistake for blocks
// ---------------------------------------------------------------------------

func TestScanBlocksIgnoresBracesInsideStrings(t *testing.T) {
	code := `function a() { const s = "} not a close {"; return s }`
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %#v", len(blocks), blocks)
	}
	if !strings.HasSuffix(blocks[0], "return s }") {
		t.Errorf("block ended early: %q", blocks[0])
	}
}

func TestScanBlocksIgnoresBracesInsideComments(t *testing.T) {
	code := `function a() { /* } */ return 1 // }
}`
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %#v", len(blocks), blocks)
	}
}

func TestScanBlocksIgnoresBracesInTemplateText(t *testing.T) {
	code := "const css = styled.div`\n  .a { color: red; }\n`\nfunction f() { return 1 }"
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 || blocks[0] != "{ return 1 }" {
		t.Fatalf("blocks = %#v", blocks)
	}
}

func TestScanBlocksHandlesTemplateInterpolation(t *testing.T) {
	code := "function f() { return `a ${ cond ? `x ${ y }` : 'z' } b` }"
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %#v", len(blocks), blocks)
	}
	if !strings.HasSuffix(blocks[0], "b` }") {
		t.Errorf("template nesting desynced the scanner: %q", blocks[0])
	}
}

func TestScanBlocksIgnoresBracesInsideRegexLiterals(t *testing.T) {
	code := `function a() { const re = /[{}]/g; return re }`
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %#v", len(blocks), blocks)
	}
	if !strings.HasSuffix(blocks[0], "return re }") {
		t.Errorf("block ended early: %q", blocks[0])
	}
}

// ---------------------------------------------------------------------------
// normalisation
// ---------------------------------------------------------------------------

func TestNormalizeStripsCommentsAndCollapsesWhitespace(t *testing.T) {
	a := normalizeOnly(t, "function  a ( ) {\n\t// note\n\treturn   1;\n}", Blinding{})
	b := normalizeOnly(t, "function a(){return 1;}", Blinding{})
	if a != b {
		t.Fatalf("formatting changed the canonical form:\n a = %q\n b = %q", a, b)
	}
}

func TestNormalizeKeepsSeparatorsThatMatter(t *testing.T) {
	got := normalizeOnly(t, "const  x = 1", Blinding{})
	if got != "const x=1" {
		t.Fatalf("norm = %q, want %q", got, "const x=1")
	}
}

func TestNormalizeDoesNotMergeAdjacentPlusOperators(t *testing.T) {
	got := normalizeOnly(t, "a + +b", Blinding{})
	if got != "a+ +b" {
		t.Fatalf("norm = %q, want %q - `a++b` would change meaning", got, "a+ +b")
	}
}

func TestNormalizeKeepsStringContentsVerbatim(t *testing.T) {
	got := normalizeOnly(t, `x = "a  b"`, Blinding{})
	if got != `x="a  b"` {
		t.Fatalf("norm = %q, want %q", got, `x="a  b"`)
	}
}

func TestNormalizeCollapsesTemplateIndentation(t *testing.T) {
	a := normalizeOnly(t, "css`\n  color: red;\n`", Blinding{})
	b := normalizeOnly(t, "css`\n      color: red;\n`", Blinding{})
	if a != b {
		t.Fatalf("re-indented template did not normalise the same:\n a = %q\n b = %q", a, b)
	}
}

func TestBlindIdentifiersFoldsIdentifiersButKeepsKeywords(t *testing.T) {
	a := normalizeOnly(t, "function getUser(id) { return db.find(id) }", Blinding{Identifiers: true})
	b := normalizeOnly(t, "function loadItem(key) { return store.lookup(key) }", Blinding{Identifiers: true})
	if a != b {
		t.Fatalf("renamed copy did not fold to the same shape:\n a = %q\n b = %q", a, b)
	}
	if !strings.Contains(a, "function") || !strings.Contains(a, "return") {
		t.Fatalf("keywords were folded away: %q", a)
	}
}

func TestBlindIdentifiersKeepsKeywordsDistinct(t *testing.T) {
	a := normalizeOnly(t, "if (x) { go() }", Blinding{Identifiers: true})
	b := normalizeOnly(t, "while (x) { go() }", Blinding{Identifiers: true})
	if a == b {
		t.Fatalf("if and while folded to the same shape: %q", a)
	}
}

func TestBlindIdentifiersKeepsNumbersAndStrings(t *testing.T) {
	a := normalizeOnly(t, "const limit = 10", Blinding{Identifiers: true})
	b := normalizeOnly(t, "const limit = 20", Blinding{Identifiers: true})
	if a == b {
		t.Fatalf("numeric literals were folded: %q", a)
	}
}

// ---------------------------------------------------------------------------
// JSX
// ---------------------------------------------------------------------------

func TestScanBlocksFindsJSXElement(t *testing.T) {
	code := `const el = <div className="a">text</div>;`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 JSX block, got %d: %#v", len(blocks), blocks)
	}
	if blocks[0] != `<div className="a">text</div>` {
		t.Errorf("jsx block = %q", blocks[0])
	}
	if kinds[0] != BlockJSX {
		t.Errorf("kind = %v, want BlockJSX", kinds[0])
	}
}

func TestScanBlocksFindsSelfClosingJSX(t *testing.T) {
	code := `const el = <Icon name="x" />;`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 || blocks[0] != `<Icon name="x" />` {
		t.Fatalf("blocks = %#v", blocks)
	}
	if kinds[0] != BlockJSX {
		t.Errorf("kind = %v, want BlockJSX", kinds[0])
	}
}

func TestScanBlocksFindsFragments(t *testing.T) {
	code := `const el = <><a/><b/></>;`
	_, kinds, _ := scanForTest(t, code, literalOpts())
	jsx := 0
	for _, k := range kinds {
		if k == BlockJSX {
			jsx++
		}
	}
	if jsx != 3 {
		t.Fatalf("expected fragment plus two children, got %d JSX blocks", jsx)
	}
}

func TestScanBlocksFindsNestedJSXAtEveryLevel(t *testing.T) {
	code := `function List({ items }) {
  return (
    <ul className="list">
      {items.map((item) => (
        <li key={item.id}>{item.label}</li>
      ))}
    </ul>
  );
}`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())

	var jsx []string
	for i, k := range kinds {
		if k == BlockJSX {
			jsx = append(jsx, blocks[i])
		}
	}
	if len(jsx) != 2 {
		t.Fatalf("expected the <li> and the wrapping <ul>, got %d: %#v", len(jsx), jsx)
	}
	if !strings.HasPrefix(jsx[0], "<li") || !strings.HasSuffix(jsx[0], "</li>") {
		t.Errorf("inner jsx block = %q", jsx[0])
	}
	if !strings.HasPrefix(jsx[1], "<ul") || !strings.HasSuffix(jsx[1], "</ul>") {
		t.Errorf("outer jsx block = %q", jsx[1])
	}
}

func TestScanBlocksDoesNotTreatComparisonAsJSX(t *testing.T) {
	code := `function f(a, b) { return a < b && c > d }`
	_, kinds, _ := scanForTest(t, code, literalOpts())
	for _, k := range kinds {
		if k == BlockJSX {
			t.Fatalf("comparison was reported as JSX")
		}
	}
}

func TestScanBlocksDoesNotTreatGenericsAsJSXWhenDisabled(t *testing.T) {
	code := `const x = new Map<string, number>(); function f() { return x }`
	opts := literalOpts()
	opts.AllowJSX = false
	_, kinds, _ := scanForTest(t, code, opts)
	for _, k := range kinds {
		if k == BlockJSX {
			t.Fatalf("generic type argument was reported as JSX")
		}
	}
}

func TestScanBlocksRecoversFromMisdetectedJSX(t *testing.T) {
	// `<T,>` in a .tsx file is a generic arrow, not an element. The scanner may open a
	// JSX frame for it, but it must not swallow the rest of the file.
	code := `const id = <T,>(v: T) => v;
function after() { return 42 }`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())

	found := false
	for i, k := range kinds {
		if k == BlockBrace && strings.Contains(blocks[i], "42") {
			found = true
		}
	}
	if !found {
		t.Fatalf("blocks after the misdetected element were lost: %#v", blocks)
	}
}

func TestScanBlocksJSXStructuralFoldsTagNames(t *testing.T) {
	a := normalizeOnly(t, `<Row gap={2}><Cell/></Row>`, Blinding{Identifiers: true})
	b := normalizeOnly(t, `<Stack gap={2}><Item/></Stack>`, Blinding{Identifiers: true})
	if a != b {
		t.Fatalf("renamed components did not fold to the same shape:\n a = %q\n b = %q", a, b)
	}
}

// ---------------------------------------------------------------------------
// offsets
// ---------------------------------------------------------------------------

func TestScanBlocksOriginalOffsetsPointAtRealSource(t *testing.T) {
	code := `
// leading comment
function a() {
  return { k: 1 }
}
`
	src := []byte(code)
	res := ScanCodeBlocks(src, literalOpts(), nil)
	if len(res.Blocks) == 0 {
		t.Fatal("no blocks")
	}
	for _, b := range res.Blocks {
		text := src[b.Start:b.End]
		if len(text) == 0 {
			t.Fatalf("empty block %+v", b)
		}
		first, last := text[0], text[len(text)-1]
		if first != '{' && first != '<' {
			t.Errorf("block starts with %q, want '{' or '<': %q", first, text)
		}
		if last != '}' && last != '>' {
			t.Errorf("block ends with %q, want '}' or '>': %q", last, text)
		}
	}
}

func TestScanBlocksNormOffsetsMatchNormBuffer(t *testing.T) {
	code := `function a() {  return   1  }`
	res := ScanCodeBlocks([]byte(code), literalOpts(), nil)
	if len(res.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(res.Blocks))
	}
	b := res.Blocks[0]
	got := string(res.Norm[b.NormStart:b.NormEnd])
	if got != "{return 1}" {
		t.Fatalf("normalised block = %q, want %q", got, "{return 1}")
	}
}

func TestScanBlocksReusesDestination(t *testing.T) {
	dst := &BlockScan{}
	ScanCodeBlocks([]byte(`function a() { return 1 }`), literalOpts(), dst)
	firstLen := len(dst.Blocks)
	ScanCodeBlocks([]byte(`const x = 1`), literalOpts(), dst)

	if firstLen == 0 {
		t.Fatal("first scan found nothing")
	}
	if len(dst.Blocks) != 0 {
		t.Fatalf("second scan kept stale blocks: %d", len(dst.Blocks))
	}
	if string(dst.Norm) != "const x=1" {
		t.Fatalf("second scan kept stale norm: %q", string(dst.Norm))
	}
}

// ---------------------------------------------------------------------------
// complexity metrics
// ---------------------------------------------------------------------------

// lastBlock returns the outermost block, which closes last.
func lastBlock(t *testing.T, code string, opts BlockScanOptions) CodeBlock {
	t.Helper()
	res := ScanCodeBlocks([]byte(code), opts, nil)
	if len(res.Blocks) == 0 {
		t.Fatalf("no blocks for %q", code)
	}
	return res.Blocks[len(res.Blocks)-1]
}

func TestScanBlocksInnerDepth(t *testing.T) {
	// Depth counts the block itself, so a flat block is 1.
	cases := []struct {
		code string
		want uint8
	}{
		{`const o = { a: 1, b: 2, c: 3 }`, 1},
		{`const o = { a: 1, b: { c: 2 } }`, 2},
		{`const o = { a: { b: { c: 1 } } }`, 3},
		{`function f() { return 1 }`, 1},
		{`function f() { if (x) { y() } }`, 2},
		{`function f() { if (x) { while (y) { z() } } }`, 3},
	}
	for _, tc := range cases {
		got := lastBlock(t, tc.code, literalOpts()).Depth
		if got != tc.want {
			t.Errorf("Depth(%q) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

func TestScanBlocksDepthIgnoresJSXExpressionContainers(t *testing.T) {
	// Attribute and child expressions are punctuation, not structure: an element with
	// props must not look as nested as one with a nested element.
	flat := lastBlock(t, `const a = <Icon name={x} size={y} />`, literalOpts())
	if flat.Depth != 1 {
		t.Errorf("Depth of a flat element with props = %d, want 1", flat.Depth)
	}
	nested := lastBlock(t, `const a = <Row><Icon name={x} /></Row>`, literalOpts())
	if nested.Depth != 2 {
		t.Errorf("Depth of an element with a child element = %d, want 2", nested.Depth)
	}
	deep := lastBlock(t, `const a = <Row>{items.map((i) => <Cell><Dot /></Cell>)}</Row>`, literalOpts())
	if deep.Depth < 3 {
		t.Errorf("InnerDepth through a map expression = %d, want >= 3", deep.Depth)
	}
}

func TestScanBlocksStatementCount(t *testing.T) {
	cases := []struct {
		code string
		want uint16
	}{
		{`function f() { a(); b(); }`, 2}, // trailing separator is not a third unit
		{`function f() { a(); b() }`, 2},  // ... and neither is its absence
		{`function f() { return 1 }`, 1},
		{`function f() {}`, 0},
		{`const o = { a: 1, b: 2, c: 3 }`, 0},
		{`const o = { a: 1, b: 2, c: 3, }`, 0}, // an object literal has no statements
		{`function f() { if (x) { a(); b(); } }`, 1},
		{`function f() { const x = 1; if (x) { a() } return x; }`, 3},
		{`function f() { if (a) { x() } if (b) { y() } }`, 2},
		{`function f() { let a = 1, b = 2; }`, 1},
		{`class A { m() { return 1 } }`, 0},
	}
	for _, tc := range cases {
		got := lastBlock(t, tc.code, literalOpts()).Statements
		if got != tc.want {
			t.Errorf("Statements(%q) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

// TestScanBlocksJSXHasNoStatements pins that JSX is an expression: its children are
// structure, measured by Depth, not statements.
func TestScanBlocksJSXHasNoStatements(t *testing.T) {
	for _, code := range []string{
		`const a = <Icon name={x} size={y} />`,
		`const a = <Row><A /></Row>`,
		`const a = <Row><A /><B /><C /></Row>`,
		`const a = <Row><A />{label}</Row>`,
	} {
		b := lastBlock(t, code, literalOpts())
		if b.Statements != 0 {
			t.Errorf("Statements(%q) = %d, want 0", code, b.Statements)
		}
		if b.IsStatementBlock() {
			t.Errorf("%q was classified as a statement block", code)
		}
	}
}

// TestScanBlocksClassifiesStatementPosition covers the rule that decides whether counting
// statements means anything for a given brace.
func TestScanBlocksClassifiesStatementPosition(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{`function f() { a() }`, true},
		{`if (x) { a() }`, true},
		{`while (x) { a() }`, true},
		{`for (;;) { a() }`, true},
		{`try { a() } finally { b() }`, true},
		{`const f = () => { a() }`, true},
		{`const o = { a: 1 }`, false},
		{`foo({ a: 1 })`, false},
		{`const o = [{ a: 1 }]`, false},
		{`return { a: 1 }`, false},
		{`class A { m() { return 1 } }`, false}, // a member list, not statements
	}
	for _, tc := range cases {
		res := ScanCodeBlocks([]byte(tc.code), literalOpts(), nil)
		if len(res.Blocks) == 0 {
			t.Fatalf("no blocks for %q", tc.code)
		}
		// The outermost brace is the one being classified.
		got := res.Blocks[len(res.Blocks)-1].IsStatementBlock()
		if got != tc.want {
			t.Errorf("IsStatementBlock(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestScanBlocksMetricsSurviveReformatting(t *testing.T) {
	// The metrics are computed on the canonical form, so every copy of a duplicate is
	// guaranteed to score identically no matter how it was formatted.
	a := lastBlock(t, "function f() {\n  const x = 1;\n  // note\n  return { a: 1, b: { c: 2 } };\n}", literalOpts())
	b := lastBlock(t, "function f(){const x=1;return {a:1,b:{c:2}};}", literalOpts())

	if a.Depth != b.Depth || a.Statements != b.Statements {
		t.Fatalf("formatting changed the metrics: %+v vs %+v", a, b)
	}
}

func TestScanBlocksMinDepthFilter(t *testing.T) {
	code := `const flat = { a: 1, b: 2, c: 3 };
const nested = { a: 1, b: { c: 2 } };`

	opts := literalOpts()
	opts.MinDepth = 2
	res := ScanCodeBlocks([]byte(code), opts, nil)

	for _, b := range res.Blocks {
		if b.Depth < 2 {
			t.Fatalf("block below MinDepth survived: %+v", b)
		}
	}
	if len(res.Blocks) != 1 {
		t.Fatalf("expected only the nested object's outer block, got %d", len(res.Blocks))
	}
}

func TestScanBlocksMinStatementsFilter(t *testing.T) {
	code := `function one() { return 1 }
function three() { a(); b(); c(); }`

	opts := literalOpts()
	opts.MinStatements = 3
	res := ScanCodeBlocks([]byte(code), opts, nil)

	if len(res.Blocks) != 1 {
		t.Fatalf("expected only the three-statement body, got %d: %+v", len(res.Blocks), res.Blocks)
	}
	if res.Blocks[0].Statements != 3 {
		t.Errorf("Statements = %d, want 3", res.Blocks[0].Statements)
	}
}

func TestScanBlocksSingleLineNestedJSXClosingTags(t *testing.T) {
	// Regression: `</Cell>` reaches the scanner as operator-then-slash, which is regex
	// position. Without a guard the closing tag is eaten as a regex running to the '/'
	// of the next closing tag, and every enclosing element is lost. Multi-line JSX hid
	// this because a regex literal cannot span lines.
	code := `const a = <Row><Cell><Dot /></Cell></Row>`
	blocks, kinds, _ := scanForTest(t, code, literalOpts())

	if len(blocks) != 3 {
		t.Fatalf("expected Dot, Cell and Row, got %d: %#v", len(blocks), blocks)
	}
	for _, k := range kinds {
		if k != BlockJSX {
			t.Errorf("kind = %v, want BlockJSX", k)
		}
	}
	if blocks[2] != code[len("const a = "):] {
		t.Errorf("outermost element = %q", blocks[2])
	}
}

func TestScanBlocksStillSkipsRealRegexLiterals(t *testing.T) {
	// The `</` guard must not stop ordinary regex literals from being skipped.
	code := `function f() { const re = /a{2}\/b/g; return re }`
	blocks, _, _ := scanForTest(t, code, literalOpts())
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %#v", len(blocks), blocks)
	}
	if !strings.HasSuffix(blocks[0], "return re }") {
		t.Errorf("regex literal desynced the scanner: %q", blocks[0])
	}
}

func TestScanBlocksStatementCountIgnoresSeparatorsInsideExpressions(t *testing.T) {
	cases := []struct {
		code string
		want uint16
	}{
		// Call arguments are not statements of the enclosing block.
		{`function f() { return build(a, b, c, d); }`, 1},
		// Neither are the clauses of a for header.
		{`function f() { for (let i = 0; i < n; i++) { go(i) } }`, 1},
		// Array elements are not statements either.
		{`function f() { return [one, two, three]; }`, 1},
		// Real statements still count alongside them.
		{`function f() { setup(a, b); run(c, d); }`, 2},
		// An object literal passed as an argument does not end the statement holding it.
		{`function f() { call({ a: 1 }, { b: 2 }); other(); }`, 2},
	}
	for _, tc := range cases {
		got := lastBlock(t, tc.code, literalOpts()).Statements
		if got != tc.want {
			t.Errorf("Statements(%q) = %d, want %d", tc.code, got, tc.want)
		}
	}
}

// TestScanBlocksClassifiesTypeScriptSignatures covers the case where the byte before the
// body is the end of a return type rather than the `)` of the parameter list.
func TestScanBlocksClassifiesTypeScriptSignatures(t *testing.T) {
	cases := []struct {
		code string
		// innermost checks the first block to close rather than the outermost one.
		innermost bool
		want      bool
	}{
		{code: `function f(): Promise<User> { a(); }`, want: true},
		{code: `async function f(x: string): Promise<Map<string, number>> { a(); }`, want: true},
		{code: `function f(a: number): void { a(); }`, want: true},
		{code: `const f = (a: number): string => { return "x" }`, want: true},
		// The method body is a statement block; the class body around it is a member
		// list and is checked separately above.
		{code: `class A { m(): number { return 1 } }`, innermost: true, want: true},
		// Still refused where a type-ish run precedes an object literal.
		{code: `const x: Record<string, number> = { a: 1 }`, want: false},
		{code: `const o = { key: { nested: 1 } }`, want: false},
		{code: `type T = { a: number }`, want: false},
	}
	for _, tc := range cases {
		res := ScanCodeBlocks([]byte(tc.code), literalOpts(), nil)
		if len(res.Blocks) == 0 {
			t.Fatalf("no blocks for %q", tc.code)
		}
		b := res.Blocks[len(res.Blocks)-1]
		if tc.innermost {
			b = res.Blocks[0]
		}
		if b.IsStatementBlock() != tc.want {
			t.Errorf("IsStatementBlock(%q) = %v, want %v", tc.code, b.IsStatementBlock(), tc.want)
		}
	}
}
