package dupcode

import "testing"

// The two blindings the tests use most. Here rather than in the package: nothing outside the
// tests called them, and this is an internal package, so nothing outside could.
func compareExact(a, b []byte) bool { return Compare(a, b, Blinding{}) }

func compareIgnoringNames(a, b []byte) bool { return Compare(a, b, Blinding{Identifiers: true}) }

func TestCompareLiteralIgnoresFormatting(t *testing.T) {
	a := []byte(`{
  const total = items.length;
  return total * 2;
}`)
	b := []byte(`{const total=items.length;return total*2;}`)

	if !compareExact(a, b) {
		t.Fatalf("re-formatted copy did not match")
	}
}

func TestCompareLiteralIgnoresComments(t *testing.T) {
	a := []byte(`{
  // compute the total
  const total = items.length; /* inline */
  return total;
}`)
	b := []byte(`{
  const total = items.length;
  return total;
}`)

	if !compareExact(a, b) {
		t.Fatalf("comments changed the comparison")
	}
}

func TestCompareLiteralRejectsRenamedIdentifiers(t *testing.T) {
	a := []byte(`{ const total = items.length; return total; }`)
	b := []byte(`{ const count = values.length; return count; }`)

	if compareExact(a, b) {
		t.Fatalf("literal blinding matched a renamed copy")
	}
}

func TestCompareDoesNotGlueTokensAcrossWhitespace(t *testing.T) {
	// If whitespace were simply dropped, `const x` would normalise to `constx` and
	// these two would compare equal.
	a := []byte(`const x = 1`)
	b := []byte(`co nstx = 1`)

	if compareExact(a, b) {
		t.Fatalf("whitespace was dropped instead of collapsed to a separator")
	}
}

func TestCompareRejectsDifferentLengths(t *testing.T) {
	a := []byte(`{ return 1; }`)
	b := []byte(`{ return 1; extra(); }`)

	if compareExact(a, b) {
		t.Fatalf("a prefix matched the longer chunk")
	}
	if compareExact(b, a) {
		t.Fatalf("a prefix matched the longer chunk (reversed)")
	}
}

func TestCompareLiteralKeepsStringContents(t *testing.T) {
	a := []byte(`{ log("hello world") }`)
	b := []byte(`{ log("hello  world") }`)

	if compareExact(a, b) {
		t.Fatalf("whitespace inside a string literal was collapsed")
	}
}

func TestCompareStructuralMatchesRenamedCopy(t *testing.T) {
	a := []byte(`{
  const total = items.length;
  if (total > 0) {
    return total * 2;
  }
  return 0;
}`)
	b := []byte(`{
  const size = values.count;
  if (size > 0) {
    return size * 2;
  }
  return 0;
}`)

	if !compareIgnoringNames(a, b) {
		t.Fatalf("structural blinding missed a renamed copy")
	}
	if compareExact(a, b) {
		t.Fatalf("literal blinding should not match a renamed copy")
	}
}

func TestCompareStructuralStillRequiresMatchingKeywords(t *testing.T) {
	a := []byte(`{ if (x) { go() } }`)
	b := []byte(`{ while (x) { go() } }`)

	if compareIgnoringNames(a, b) {
		t.Fatalf("structural blinding treated `if` and `while` as interchangeable")
	}
}

func TestCompareStructuralStillRequiresMatchingLiterals(t *testing.T) {
	a := []byte(`{ retry(3, "fast") }`)
	b := []byte(`{ retry(5, "fast") }`)
	c := []byte(`{ retry(3, "slow") }`)

	if compareIgnoringNames(a, b) {
		t.Fatalf("numeric literals were treated as wildcards")
	}
	if compareIgnoringNames(a, c) {
		t.Fatalf("string literals were treated as wildcards")
	}
}

func TestCompareStructuralRequiresMatchingShape(t *testing.T) {
	a := []byte(`{ const a = one(two); }`)
	b := []byte(`{ const a = one(two, three); }`)

	if compareIgnoringNames(a, b) {
		t.Fatalf("an extra argument did not change the shape")
	}
}

func TestCompareStructuralMatchesRenamedJSX(t *testing.T) {
	a := []byte(`<Row gap={2}><Cell value={x} /></Row>`)
	b := []byte(`<Stack gap={2}><Item value={y} /></Stack>`)

	if !compareIgnoringNames(a, b) {
		t.Fatalf("structural blinding missed renamed JSX components")
	}
}

func TestCompareIgnoresTemplateIndentation(t *testing.T) {
	a := []byte("styled.div`\n  color: red;\n  margin: 0;\n`")
	b := []byte("styled.div`\n      color: red;\n      margin: 0;\n`")

	if !compareExact(a, b) {
		t.Fatalf("re-indented template literal did not match")
	}
}

func TestCompareHandlesRegexLiterals(t *testing.T) {
	a := []byte(`{ const re = /a{2}/g; return re }`)
	b := []byte(`{
  const re = /a{2}/g;
  return re
}`)

	if !compareExact(a, b) {
		t.Fatalf("regex literal broke the comparison")
	}
}

func TestNormalizeMatchesCompare(t *testing.T) {
	// Normalize is documented as producing exactly the bytes Compare matches on.
	a := []byte("function  f( a ) {\n // c\n return a\n}")
	b := []byte("function f(a){return a}")

	if string(Normalize(a, Blinding{})) != string(Normalize(b, Blinding{})) {
		t.Fatalf("Normalize disagrees with itself:\n %q\n %q",
			Normalize(a, Blinding{}), Normalize(b, Blinding{}))
	}
	if !compareExact(a, b) {
		t.Fatalf("Compare disagrees with Normalize")
	}
}

func TestBlindingNames(t *testing.T) {
	cases := []struct {
		b    Blinding
		want string
	}{
		{Blinding{}, "exact"},
		{Blinding{Identifiers: true}, "identifiers"},
		{Blinding{Strings: true}, "strings"},
		{Blinding{Numbers: true}, "numbers"},
		{Blinding{Identifiers: true, Strings: true}, "identifiers+strings"},
		{Blinding{Identifiers: true, Strings: true, Numbers: true}, "identifiers+strings+numbers"},
	}
	for _, tc := range cases {
		if got := tc.b.String(); got != tc.want {
			t.Errorf("%+v.String() = %q, want %q", tc.b, got, tc.want)
		}
	}
	if !(Blinding{}).IsExact() {
		t.Error("the zero value should be an exact comparison")
	}
	if (Blinding{Numbers: true}).IsExact() {
		t.Error("blinding numbers is not exact")
	}
}
