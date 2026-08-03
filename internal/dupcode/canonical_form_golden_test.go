package dupcode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The canonical form is a compatibility contract.
//
// Snapshots record the hash of a duplicate's canonical bytes, so any change to how those
// bytes are produced changes the identity of code that did not change. That is a breaking
// change for every committed snapshot, and it must never happen by accident.
//
// TestBlockNormalizationIsSelfConsistent proves the normaliser agrees with itself; it
// would pass happily after a change that shifted every byte. This file is what makes drift
// visible: the expected canonical form of each construct is written out in full, so a diff
// here shows exactly what moved and forces a deliberate decision rather than a surprise.
//
// If one of these fails, the question is not "update the expectation" - it is "does this
// break every snapshot in the wild, and is it worth it".

// canonicalFormCases pins the output of the normaliser construct by construct. The
// expectations are literal strings rather than hashes so a failure is readable.
var canonicalFormCases = []struct {
	name    string
	source  string
	literal string
}{
	{"whitespace collapses", "const  a   =   1", "const a=1"},
	{"newlines collapse", "const a =\n  1", "const a=1"},
	{"tabs collapse", "const\ta\t=\t1", "const a=1"},
	{"line comment removed", "const a = 1 // note", "const a=1"},
	{"block comment removed", "const /* note */ a = 1", "const a=1"},
	{"comment between tokens keeps the separator", "const/* c */a = 1", "const a=1"},
	{"separator kept between identifiers", "return a", "return a"},
	{"no separator around punctuation", "f ( a , b )", "f(a,b)"},
	{"adjacent plus operators stay apart", "a + +b", "a+ +b"},
	{"adjacent minus operators stay apart", "a - -b", "a- -b"},
	{"increment is untouched", "a++", "a++"},

	{"string contents are verbatim", `x = "a  b"`, `x="a  b"`},
	{"single quotes verbatim", `x = 'a  b'`, `x='a  b'`},
	{"escaped quote survives", `x = "a\"b"`, `x="a\"b"`},
	{"comment inside a string is not a comment", `x = "// not a comment"`, `x="// not a comment"`},
	{"brace inside a string is not a brace", `x = "{"`, `x="{"`},

	{"template whitespace collapses", "css`\n  color: red;\n`", "css` color: red; `"},
	{"template interpolation is code", "`a ${ b  +  c } d`", "`a ${b+c} d`"},
	{"nested template", "`a ${ `x ${ y }` } b`", "`a ${`x ${y}`} b`"},

	{"regex literal is verbatim", `const re = /a  b/g`, `const re=/a  b/g`},
	{"regex containing a brace", `const re = /[{}]/g`, `const re=/[{}]/g`},
	{"regex containing a quote", `const re = /['"]/g`, `const re=/['"]/g`},
	{"division is not a regex", `const q = a / b / c`, `const q=a/b/c`},

	{"jsx closing tags survive", `<Row><Cell/></Row>`, `<Row><Cell/></Row>`},
	// A separator only survives between two identifier characters, so the space before
	// an attribute whose predecessor ended in a quote is dropped. The result is not valid
	// JSX, which does not matter - it only has to be unambiguous, and no valid source
	// normalises to the same bytes.
	{"jsx attributes collapse", `<Icon  name = "x"  size = {2} />`, `<Icon name="x"size={2}/>`},
	// JSX itself trims leading and trailing whitespace in a text node, so dropping it
	// here matches what the runtime does.
	{"jsx text collapses", `<p>  hello   world  </p>`, `<p>hello world</p>`},

	{"type annotations are kept", `const a: Record<string, number> = {}`, `const a:Record<string,number>={}`},
	{"generic call is kept", `new Map<string, number>()`, `new Map<string,number>()`},
	{"optional chaining", `a?.b?.[c]`, `a?.b?.[c]`},
	{"arrow function", `const f = ( a ) => a + 1`, `const f=(a)=>a+1`},
	{"async await", `async function f() { await g() }`, `async function f(){await g()}`},
	{"decorator", `@Injectable() class A {}`, `@Injectable()class A{}`},
	{"spread", `f( ...args )`, `f(...args)`},
}

func TestCanonicalFormLiteralIsPinned(t *testing.T) {
	for _, tc := range canonicalFormCases {
		got := string(Normalize([]byte(tc.source), Blinding{}))
		if got != tc.literal {
			t.Errorf("%s\n  source:   %q\n  got:      %q\n  expected: %q",
				tc.name, tc.source, got, tc.literal)
		}
	}
}

// structuralFormCases pin the identifier folding on top of the literal rules. The sentinel
// is written as \x01 so the expectations stay readable.
var structuralFormCases = []struct {
	name       string
	source     string
	structural string
}{
	{"identifier folds", "const total = 1", "const \x01=1"},
	{"keywords survive", "if (x) { return y }", "if(\x01){return \x01}"},
	{"numbers survive", "const a = 42", "const \x01=42"},
	{"strings survive", `const a = "keep"`, `const \x01="keep"`},
	{"property names fold", "a.b.c", "\x01.\x01.\x01"},
	// A run straight after `.` is a property name, so it folds however it is spelled.
	{"keyword-shaped property folds", "Array.from(a)", "\x01.\x01(\x01)"},
	{"keyword-shaped property matches an ordinary one", "Object.keys(a)", "\x01.\x01(\x01)"},
	{"optional-chained keyword-shaped property folds", "a?.from(b)", "\x01?.\x01(\x01)"},
	{"the keyword BEFORE the dot still survives", "import.meta.url", "import.\x01.\x01"},
	{"new.target keeps its keyword", "new.target", "new.\x01"},
	// The position check must not reach past the dot.
	{"keyword in declaration position survives", "type Foo = Bar", "type \x01=\x01"},
	{"decimal point is not property access", "const n = 1.5", "const \x01=1.5"},
	{"type names fold", "const a: Foo = 1", "const \x01:\x01=1"},
	{"built-in types survive", "const a: string = 'x'", "const \x01:string='x'"},
	{"jsx tag names fold", "<Row gap={2}><Cell/></Row>", "<\x01 \x01={2}><\x01/></\x01>"},
	{"separator survives folding", "const total = 1", "const \x01=1"},
	{"two identifiers keep their separator", "new Foo", "new \x01"},
}

func TestCanonicalFormStructuralIsPinned(t *testing.T) {
	for _, tc := range structuralFormCases {
		want := strings.ReplaceAll(tc.structural, `\x01`, "\x01")
		got := string(Normalize([]byte(tc.source), Blinding{Identifiers: true}))
		if got != want {
			t.Errorf("%s\n  source:   %q\n  got:      %q\n  expected: %q",
				tc.name, tc.source, got, want)
		}
	}
}

// TestCanonicalFormCorpusIsPinned covers breadth rather than specific constructs: a hash
// per fixture file, so a change that no hand-written case happens to exercise still fails.
//
// Regenerate deliberately with: go test ./internal/dupcode/ -run TestCanonicalFormCorpus -update-canonical
func TestCanonicalFormCorpusIsPinned(t *testing.T) {
	goldenPath := filepath.Join("..", "..", "testdata", "canonical-form.golden")

	var lines []string
	root := fixtureProject
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ts", ".tsx", ".js", ".jsx":
		default:
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for _, blinding := range []Blinding{{}, {Identifiers: true}} {
			sum := sha256.Sum256(Normalize(content, blinding))
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s", rel, blinding, hex.EncodeToString(sum[:])[:32]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"

	if os.Getenv("UPDATE_CANONICAL_GOLDEN") != "" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (regenerate with UPDATE_CANONICAL_GOLDEN=1): %v", err)
	}
	if got != string(want) {
		t.Errorf("the canonical form of the fixture corpus changed.\n"+
			"This breaks the identity of every committed snapshot - confirm it is intended,\n"+
			"then regenerate with UPDATE_CANONICAL_GOLDEN=1.\n\ngot:\n%s\nwant:\n%s", got, want)
	}
}
