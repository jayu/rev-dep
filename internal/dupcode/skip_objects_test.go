package dupcode

import (
	"strings"
	"testing"

	"rev-dep-go/internal/parser"
)

// A nested object literal is structurally repetitive by nature: "three keys, one of them a
// nested pair" is a shape that recurs across unrelated code. Under structural comparison
// those match each other, which reads as a false positive because the two places have
// nothing to do with each other. --skip-objects is how they are excluded.

const objectA = `{
  retryAttemptLimit: 3,
  requestTimeoutMilliseconds: 30000,
  transportOptions: { keepAliveEnabled: true, poolSizeMaximum: 12 },
}`

// objectB shares objectA's SHAPE - same key count, same nesting, same value kinds - while
// having nothing to do with it.
const objectB = `{
  maximumUploadedFileCount: 3,
  thumbnailGenerationTimeoutMs: 30000,
  storageBackendSettings: { compressionEnabled: true, shardCountMaximum: 12 },
}`

const realLogic = `{
  const parsed = JSON.parse(rawInput);
  const cleaned = normalise(parsed, defaults);
  return validate(cleaned, schemaFor(kind));
}`

func TestSkipObjectsDropsObjectLiteralsOnly(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const cfgA = " + objectA + ";\nexport function runA(rawInput, kind) " + realLogic + "\n",
		"b.ts": "export const cfgB = " + objectA + ";\nexport function runB(rawInput, kind) " + realLogic + "\n",
	})
	base := Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1}

	all := detect(t, dir, base)
	if len(all) != 2 {
		t.Fatalf("expected the object and the logic, got %d:\n%s", len(all), describe(all))
	}

	skipped := detect(t, dir, withSkipObjects(base))
	if len(skipped) != 1 {
		t.Fatalf("expected only the logic to survive, got %d:\n%s", len(skipped), describe(skipped))
	}
	if !strings.Contains(skipped[0].Snippet, "JSON.parse") {
		t.Errorf("the wrong block survived: %q", truncate(skipped[0].Snippet))
	}
	if skipped[0].IsObjectLiteral {
		t.Error("an object literal survived --skip-objects")
	}
}

// TestSkipObjectsAddressesStructuralFalsePositives is the case that motivates the flag:
// two unrelated configs with the same shape match in structural blinding.
func TestSkipObjectsAddressesStructuralFalsePositives(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const cfgA = " + objectA + ";\n",
		"b.ts": "export const cfgB = " + objectB + ";\n",
	})
	base := Options{Blinding: Blinding{Identifiers: true}, MinTokens: 5, MinLines: 1}

	matched := detect(t, dir, base)
	if len(matched) == 0 {
		t.Fatal("the fixture no longer demonstrates the false positive it exists for")
	}
	for _, d := range matched {
		if !d.IsObjectLiteral {
			t.Errorf("expected only object literals to match, got %q", truncate(d.Snippet))
		}
	}

	if got := detect(t, dir, withSkipObjects(base)); len(got) != 0 {
		t.Errorf("--skip-objects should leave nothing here, got %d:\n%s", len(got), describe(got))
	}
}

func TestSkipObjectsKeepsJSXAndDeclarations(t *testing.T) {
	jsx := `(<Card className="panel"><Body><Text>{label}</Text></Body></Card>)`
	class := `class Repository {
  find(id) { return this.store.get(id); }
  save(item) { return this.store.put(item.id, item); }
}`
	dir := writeProject(t, map[string]string{
		"a.tsx": "export const A = ({ label }) => " + jsx + ";\nexport " + class + "\n",
		"b.tsx": "export const B = ({ label }) => " + jsx + ";\nexport " + class + "\n",
	})

	got := detect(t, dir, withSkipObjects(Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1}))

	var sawJSX, sawClass bool
	for _, d := range got {
		if d.Kind == parser.BlockJSX {
			sawJSX = true
		}
		if strings.Contains(d.Snippet, "find(id)") {
			sawClass = true
		}
		if d.IsObjectLiteral {
			t.Errorf("an object literal survived: %q", truncate(d.Snippet))
		}
	}
	if !sawJSX {
		t.Error("--skip-objects dropped JSX, which is not an object literal")
	}
	if !sawClass {
		t.Errorf("--skip-objects dropped a class body, which holds members not entries:\n%s", describe(got))
	}
}

// TestObjectLiteralClassification pins what counts as an object literal, since that is what
// the flag acts on.
func TestObjectLiteralClassification(t *testing.T) {
	cases := []struct {
		code string
		want parser.BraceRole
	}{
		{`const o = { a: 1, b: 2 }`, parser.BraceObjectLiteral},
		{`foo({ a: 1, b: 2 })`, parser.BraceObjectLiteral},
		{`return { a: 1, b: 2 }`, parser.BraceObjectLiteral},
		{`type T = { a: number, b: string }`, parser.BraceObjectLiteral},
		{`const a: { x: number } = q`, parser.BraceObjectLiteral},
		{`class A { m() { return 1 } }`, parser.BraceDeclarationBody},
		{`interface I { a: number }`, parser.BraceDeclarationBody},
		{`enum E { A, B }`, parser.BraceDeclarationBody},
		{`namespace N { const a = 1; }`, parser.BraceDeclarationBody},
		{`class A extends B { m() { return 1 } }`, parser.BraceDeclarationBody},
		{`function f() { return 1 }`, parser.BraceStatementList},
		{`if (x) { go() }`, parser.BraceStatementList},
		{`const f = () => { go() }`, parser.BraceStatementList},
		{`function f(): Promise<T> { go() }`, parser.BraceStatementList},
	}
	for _, tc := range cases {
		res := parser.ScanCodeBlocks([]byte(tc.code), parser.BlockScanOptions{AllowJSX: true}, nil)
		if len(res.Blocks) == 0 {
			t.Fatalf("no blocks for %q", tc.code)
		}
		got := res.Blocks[len(res.Blocks)-1].Role
		if got != tc.want {
			t.Errorf("Role(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func withSkipObjects(o Options) Options { o.SkipObjects = true; return o }

// TestSkipObjectsRevealsWhatWasNestedInside documents the behaviour that makes the flag
// look like it did nothing.
//
// Dropping object literals removes them from the block set entirely, so a real duplication
// that was nested inside one - a handler body, a JSX tree - is no longer suppressed by it
// and becomes visible. The reported total therefore falls by LESS than the number of object
// findings removed, and on a codebase whose objects are full of callbacks it can barely
// move at all.
//
// That is the right trade: the alternative hides real duplication because its parent
// happened to be an object. But it has to be stated, or the flag reads as broken.
func TestSkipObjectsRevealsWhatWasNestedInside(t *testing.T) {
	// An object literal whose values are function bodies - the shape that makes this
	// visible. The object is duplicated, and so is the handler inside it.
	objectWithHandler := `{
  identifierForTheThing: 1,
  onActivate: (event) => {
    const parsed = JSON.parse(event.payload);
    const cleaned = normalise(parsed, defaults);
    return validate(cleaned, schemaFor(event.kind));
  },
}`
	dir := writeProject(t, map[string]string{
		"a.ts": "export const a = " + objectWithHandler + ";\n",
		"b.ts": "export const b = " + objectWithHandler + ";\n",
	})
	base := Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1}

	all := detect(t, dir, base)
	// The object is reported; the handler inside it is suppressed as nested.
	if len(all) != 1 || !all[0].IsObjectLiteral {
		t.Fatalf("expected the object literal to be the single finding:\n%s", describe(all))
	}

	skipped := detect(t, dir, withSkipObjects(base))
	// The object is gone, and the handler it was hiding is now visible - so the total
	// did not change even though an object finding was removed.
	if len(skipped) != 1 {
		t.Fatalf("expected the nested handler to surface:\n%s", describe(skipped))
	}
	if skipped[0].IsObjectLiteral {
		t.Error("an object literal survived --skip-objects")
	}
	if !skipped[0].IsStatementBlock {
		t.Errorf("expected the revealed finding to be the handler body, got %+v", skipped[0])
	}
	if !strings.Contains(skipped[0].Snippet, "JSON.parse") {
		t.Errorf("wrong block surfaced: %q", truncate(skipped[0].Snippet))
	}
}

// TestCompositionIsReported checks the counts that let a reader tell whether SkipObjects is
// the right lever for their project at all.
func TestCompositionIsReported(t *testing.T) {
	jsx := `(<Card className="panel"><Body><Text>{label}</Text></Body></Card>)`
	dir := writeProject(t, map[string]string{
		"a.tsx": "export const cfgA = " + objectA + ";\nexport function runA(rawInput, kind) " + realLogic +
			"\nexport const A = ({ label }) => " + jsx + ";\n",
		"b.tsx": "export const cfgB = " + objectA + ";\nexport function runB(rawInput, kind) " + realLogic +
			"\nexport const B = ({ label }) => " + jsx + ";\n",
	})

	_, stats, err := Detect(Options{Cwd: dir, Blinding: Blinding{}, MinTokens: 5, MinLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObjectFindings != 1 {
		t.Errorf("ObjectFindings = %d, want 1", stats.ObjectFindings)
	}
	if stats.StatementFindings != 1 {
		t.Errorf("StatementFindings = %d, want 1", stats.StatementFindings)
	}
	if stats.JSXFindings != 1 {
		t.Errorf("JSXFindings = %d, want 1", stats.JSXFindings)
	}
}
