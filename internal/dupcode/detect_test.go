package dupcode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func detect(t *testing.T, dir string, opts Options) []Duplication {
	t.Helper()
	opts.Cwd = dir
	dups, _, err := Detect(opts)
	if err != nil {
		t.Fatal(err)
	}
	return dups
}

// occurrenceList renders a duplication's locations as "path:line" for easy assertions.
func occurrenceList(d Duplication) []string {
	var out []string
	for _, o := range d.Occurrences {
		out = append(out, o.File+":"+itoa(o.Line))
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

const dupBody = `{
  const request = buildRequest(endpoint, payload);
  const response = await client.send(request);
  if (!response.ok) {
    throw new TransportError(response.status, response.body);
  }
  return response.body;
}`

func TestDetectFindsDuplicateAcrossFiles(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export async function sendA(endpoint, payload) " + dupBody + "\n",
		"b.ts": "export async function sendB(endpoint, payload) " + dupBody + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}})
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplication, got %d: %#v", len(dups), dups)
	}
	d := dups[0]
	if len(d.Occurrences) != 2 {
		t.Fatalf("expected 2 occurrences, got %d", len(d.Occurrences))
	}
	if d.FileCount != 2 {
		t.Errorf("FileCount = %d, want 2", d.FileCount)
	}
	got := occurrenceList(d)
	want := []string{"a.ts:1", "b.ts:1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("occurrences = %v, want %v", got, want)
	}
	if !strings.Contains(d.Snippet, "TransportError") {
		t.Errorf("snippet does not look like the duplicated body: %q", d.Snippet)
	}
}

func TestDetectFindsDuplicateWithinOneFile(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"only.ts": "async function one(endpoint, payload) " + dupBody +
			"\nasync function two(endpoint, payload) " + dupBody + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10})
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplication, got %d", len(dups))
	}
	if len(dups[0].Occurrences) != 2 {
		t.Fatalf("expected 2 occurrences in the same file, got %d", len(dups[0].Occurrences))
	}
	if dups[0].FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", dups[0].FileCount)
	}
	if dups[0].Occurrences[0].Line == dups[0].Occurrences[1].Line {
		t.Errorf("both occurrences reported at the same line")
	}
}

func TestDetectIgnoresFormattingDifferences(t *testing.T) {
	compact := strings.ReplaceAll(strings.ReplaceAll(dupBody, "\n", " "), "  ", "")
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + compact + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}})
	if len(dups) != 1 || len(dups[0].Occurrences) != 2 {
		t.Fatalf("re-formatted copy was not matched: %#v", dups)
	}
}

func TestDetectIgnoresComments(t *testing.T) {
	commented := strings.Replace(dupBody, "{\n", "{\n  // send it\n", 1)
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + commented + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}})
	if len(dups) != 1 || len(dups[0].Occurrences) != 2 {
		t.Fatalf("commented copy was not matched: %#v", dups)
	}
}

func TestDetectLiteralModeIgnoresRenamedCopy(t *testing.T) {
	renamed := strings.NewReplacer(
		"request", "req", "response", "res", "endpoint", "url", "payload", "data",
	).Replace(dupBody)

	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(url, data) " + renamed + "\n",
	})

	if dups := detect(t, dir, Options{Blinding: Blinding{}}); len(dups) != 0 {
		t.Fatalf("literal blinding reported a renamed copy: %#v", dups)
	}

	dups := detect(t, dir, Options{Blinding: Blinding{Identifiers: true}, MinTokens: 8})
	if len(dups) != 1 || len(dups[0].Occurrences) != 2 {
		t.Fatalf("structural blinding missed the renamed copy: %#v", dups)
	}
}

func TestDetectFindsDuplicatedJSX(t *testing.T) {
	jsx := `(
    <Card className="panel">
      <CardHeader title={title} subtitle={subtitle} />
      <CardBody>
        <Text tone="muted">{description}</Text>
      </CardBody>
    </Card>
  )`

	dir := writeProject(t, map[string]string{
		"A.tsx": "export const A = ({ title, subtitle, description }) => " + jsx + ";\n",
		"B.tsx": "export const B = ({ title, subtitle, description }) => " + jsx + ";\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 25})
	if len(dups) == 0 {
		t.Fatal("duplicated JSX was not found")
	}
	found := false
	for _, d := range dups {
		if strings.HasPrefix(d.Snippet, "<Card") && len(d.Occurrences) == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the <Card> element reported twice, got %#v", dups)
	}
}

func TestDetectCollapsesNestedDuplicates(t *testing.T) {
	// The whole function body is duplicated, so the `if` inside it is duplicated too.
	// Only the outer one is worth reporting.
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + dupBody + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1})
	if len(dups) != 1 {
		t.Fatalf("nested duplicates were reported separately: %d entries\n%#v", len(dups), dups)
	}
}

func TestDetectReportsInnerBlockWhenItHasMoreCopies(t *testing.T) {
	inner := `{
    throw new TransportError(response.status, response.body);
  }`
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + dupBody + "\n",
		"c.ts": "function c(response) { if (!response.ok) " + inner + " }\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1})

	var outer, innerDup *Duplication
	for i := range dups {
		if strings.Contains(dups[i].Snippet, "buildRequest") {
			outer = &dups[i]
		} else if strings.Contains(dups[i].Snippet, "TransportError") {
			innerDup = &dups[i]
		}
	}
	if outer == nil {
		t.Fatalf("outer duplication missing: %#v", dups)
	}
	if innerDup == nil {
		t.Fatalf("inner block is duplicated 3 times and must still be reported: %#v", dups)
	}
	if len(innerDup.Occurrences) != 3 {
		t.Errorf("inner occurrences = %d, want 3", len(innerDup.Occurrences))
	}
}

// The three tests below pin the arithmetic behind the nesting collapse.
//
// buildReport drops a group only when ONE already-accepted group accounts for every one of
// its copies. That is a stronger condition than "each copy sits inside something", and the
// two are easy to confuse: the collapse is counted per enclosing group precisely so that
// copies explained by different parents, or by one parent twice over, are not silently
// treated as explained by a single one. Getting it wrong hides real duplication rather than
// reporting too much, which is the failure nobody would notice.

// innerThrow is small enough to nest inside each of the bodies below and distinctive enough
// to find in the results.
const innerThrow = `{
      throw new TransportFailure(response.status, response.body, requestId);
    }`

func findBySnippet(dups []Duplication, needle string) *Duplication {
	for i := range dups {
		if strings.Contains(dups[i].Snippet, needle) {
			return &dups[i]
		}
	}
	return nil
}

// findInnerThrow locates the nested block itself rather than anything CONTAINING it. A
// substring search cannot do that here - every outer body has the inner block written out
// inside it - so the match is on the whole block, through the package's own comparison so
// that indentation does not enter into it.
func findInnerThrow(dups []Duplication) *Duplication {
	for i := range dups {
		if compareExact([]byte(dups[i].Snippet), []byte(innerThrow)) {
			return &dups[i]
		}
	}
	return nil
}

// TestDetectKeepsInnerBlockExplainedByDifferentParents covers a block duplicated inside two
// UNRELATED duplicated bodies. Neither parent accounts for all four copies, so extracting
// both parents would still leave the inner block written out twice - it has to be reported.
func TestDetectKeepsInnerBlockExplainedByDifferentParents(t *testing.T) {
	send := `{
  const request = buildRequest(endpoint, payload);
  const response = await transport.send(request);
  if (!response.ok) ` + innerThrow + `
  return response.body;
}`
	fetch := `{
  const parsed = JSON.parse(rawInput);
  const response = await gateway.fetch(parsed, options);
  if (!response.ok) ` + innerThrow + `
  return parsed.value;
}`
	dir := writeProject(t, map[string]string{
		"send1.ts":  "async function s1(endpoint, payload, requestId) " + send + "\n",
		"send2.ts":  "async function s2(endpoint, payload, requestId) " + send + "\n",
		"fetch1.ts": "async function f1(rawInput, options, requestId) " + fetch + "\n",
		"fetch2.ts": "async function f2(rawInput, options, requestId) " + fetch + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 8, MinLines: 1})

	if findBySnippet(dups, "buildRequest") == nil {
		t.Fatalf("the send body is duplicated and must be reported: %#v", dups)
	}
	if findBySnippet(dups, "JSON.parse") == nil {
		t.Fatalf("the fetch body is duplicated and must be reported: %#v", dups)
	}
	inner := findInnerThrow(dups)
	if inner == nil {
		t.Fatalf("the inner block has copies inside two different duplicated bodies, so no "+
			"single accepted group explains all of them and it must still be reported: %#v", dups)
	}
	if len(inner.Occurrences) != 4 {
		t.Errorf("inner occurrences = %d, want 4", len(inner.Occurrences))
	}
}

// TestDetectKeepsInnerBlockRepeatedWithinItsParent covers a block written twice inside a
// body that is itself duplicated. Every copy is inside the parent, but the parent has half
// as many copies as the block does - extracting it leaves the block still duplicated within
// the extraction, so it is not the same copy-paste seen from closer up.
func TestDetectKeepsInnerBlockRepeatedWithinItsParent(t *testing.T) {
	body := `{
  const first = await transport.send(primaryRequest);
  if (!first.ok) ` + innerThrow + `
  const second = await transport.send(fallbackRequest);
  if (!second.ok) ` + innerThrow + `
  return second.body;
}`
	dir := writeProject(t, map[string]string{
		"a.ts": "async function a(primaryRequest, fallbackRequest, requestId) " + body + "\n",
		"b.ts": "async function b(primaryRequest, fallbackRequest, requestId) " + body + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 8, MinLines: 1})

	outer := findBySnippet(dups, "fallbackRequest")
	if outer == nil {
		t.Fatalf("the outer body is duplicated and must be reported: %#v", dups)
	}
	if len(outer.Occurrences) != 2 {
		t.Errorf("outer occurrences = %d, want 2", len(outer.Occurrences))
	}
	inner := findInnerThrow(dups)
	if inner == nil {
		t.Fatalf("the inner block appears twice per copy of its parent, so the parent does "+
			"not account for it and it must still be reported: %#v", dups)
	}
	if len(inner.Occurrences) != 4 {
		t.Errorf("inner occurrences = %d, want 4", len(inner.Occurrences))
	}
}

// TestDetectCollapsesInnerBlockFullyExplainedByOneParent is the case the two above are
// contrasted with: same number of copies, all inside one accepted group. Here the collapse
// is correct and the inner block must NOT appear, or every duplicated function would be
// reported once per level of nesting inside it.
func TestDetectCollapsesInnerBlockFullyExplainedByOneParent(t *testing.T) {
	body := `{
  const request = buildRequest(endpoint, payload);
  const response = await transport.send(request);
  if (!response.ok) ` + innerThrow + `
  return response.body;
}`
	dir := writeProject(t, map[string]string{
		"a.ts": "async function a(endpoint, payload, requestId) " + body + "\n",
		"b.ts": "async function b(endpoint, payload, requestId) " + body + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 8, MinLines: 1})

	if findBySnippet(dups, "buildRequest") == nil {
		t.Fatalf("the outer body must be reported: %#v", dups)
	}
	if inner := findInnerThrow(dups); inner != nil {
		t.Errorf("the inner block has exactly the copies its parent has and sits inside it "+
			"every time, so it is the same finding seen from closer up: %d occurrences reported",
			len(inner.Occurrences))
	}
	if len(dups) != 1 {
		t.Errorf("expected only the outer body, got %d findings", len(dups))
	}
}

func TestDetectSortsByOccurrenceCount(t *testing.T) {
	other := `{
  const parsed = JSON.parse(rawInput);
  const cleaned = normalise(parsed, defaults);
  return validate(cleaned, schemaFor(kind));
}`
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + dupBody + "\n",
		"c.ts": "function c(rawInput, kind) " + other + "\n",
		"d.ts": "function d(rawInput, kind) " + other + "\n",
		"e.ts": "function e(rawInput, kind) " + other + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 25})
	if len(dups) != 2 {
		t.Fatalf("expected 2 duplications, got %d", len(dups))
	}
	if len(dups[0].Occurrences) < len(dups[1].Occurrences) {
		t.Fatalf("results are not sorted by occurrence count: %d then %d",
			len(dups[0].Occurrences), len(dups[1].Occurrences))
	}
	if len(dups[0].Occurrences) != 3 {
		t.Errorf("most-duplicated pattern has %d occurrences, want 3", len(dups[0].Occurrences))
	}
}

func TestDetectRespectsMinTokens(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "function a() { return 1 }\nfunction b() { return 1 }\n",
	})

	if dups := detect(t, dir, Options{Blinding: Blinding{}}); len(dups) != 0 {
		t.Fatalf("tiny block passed the default threshold: %#v", dups)
	}
	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 2, MinLines: 1})
	if len(dups) != 1 {
		t.Fatalf("lowering min-tokens did not surface the block: %#v", dups)
	}
}

func TestDetectRespectsMinLines(t *testing.T) {
	oneLine := "{ " + strings.Repeat("callSomethingWithAName(argumentValue); ", 8) + "}"
	dir := writeProject(t, map[string]string{
		"a.ts": "function a() " + oneLine + "\nfunction b() " + oneLine + "\n",
	})

	if dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 3}); len(dups) != 0 {
		t.Fatalf("single-line duplication passed min-lines=3: %#v", dups)
	}
	if dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10, MinLines: 1}); len(dups) != 1 {
		t.Fatalf("min-lines=1 did not surface the duplication: %#v", dups)
	}
}

func TestDetectReportsAccurateLineAndColumn(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "// header\n\nfunction a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + dupBody + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}})
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplication")
	}
	for _, o := range dups[0].Occurrences {
		content, err := os.ReadFile(filepath.Join(dir, o.File))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(content), "\n")
		if o.Line < 1 || o.Line > len(lines) {
			t.Fatalf("line %d out of range for %s", o.Line, o.File)
		}
		col := o.Col - 1
		if col < 0 || col >= len(lines[o.Line-1]) || lines[o.Line-1][col] != '{' {
			t.Errorf("%s:%d:%d does not point at the block start: %q",
				o.File, o.Line, o.Col, lines[o.Line-1])
		}
	}
}

func TestDetectStatsAreReported(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(endpoint, payload) " + dupBody + "\n",
		"b.ts": "function b(endpoint, payload) " + dupBody + "\n",
	})

	_, stats, err := Detect(Options{Cwd: dir, Blinding: Blinding{}})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Files != 2 {
		t.Errorf("Files = %d, want 2", stats.Files)
	}
	if stats.Total == 0 {
		t.Error("no timing recorded")
	}
}

func TestDetectEmptyProject(t *testing.T) {
	dir := t.TempDir()
	dups, stats, err := Detect(Options{Cwd: dir, Blinding: Blinding{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(dups) != 0 {
		t.Errorf("empty project produced %d duplications", len(dups))
	}
	if stats.Files != 0 {
		t.Errorf("Files = %d, want 0", stats.Files)
	}
}

// ---------------------------------------------------------------------------
// complexity and repetition filters
// ---------------------------------------------------------------------------

// flatObject is the case character counts cannot filter: only three entries, but long
// keys and long string values push it well past any sane size floor.
const flatObject = `{
  applicationDisplayNameForHeader: "The Very Long Application Name Here",
  applicationSupportContactAddress: "support@example-company-domain.com",
  applicationMarketingTaglineText: "Everything you need, all in one place",
}`

const nestedLogic = `{
  const parsed = JSON.parse(rawInput);
  if (parsed.kind === "batch") {
    return parsed.items.map((item) => normalise(item, defaults));
  }
  return [normalise(parsed, defaults)];
}`

func TestDetectMinDepthFiltersFlatObjects(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const configA = " + flatObject + ";\nfunction runA(rawInput) " + nestedLogic + "\n",
		"b.ts": "export const configB = " + flatObject + ";\nfunction runB(rawInput) " + nestedLogic + "\n",
	})

	// Without the filter both the object and the logic are reported.
	all := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10})
	if len(all) != 2 {
		t.Fatalf("expected object and logic, got %d: %#v", len(all), all)
	}

	// The object is flat (depth 1); the logic nests (depth 2). min-depth separates them,
	// which no size floor could: the object is the LONGER of the two in characters.
	objLen, logicLen := len(Normalize([]byte(flatObject), Blinding{})), len(Normalize([]byte(nestedLogic), Blinding{}))
	if objLen <= logicLen {
		t.Fatalf("fixture is not exercising the point: object %d chars, logic %d", objLen, logicLen)
	}

	filtered := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10, MinDepth: 2})
	if len(filtered) != 1 {
		t.Fatalf("expected only the nested logic, got %d: %#v", len(filtered), filtered)
	}
	if !strings.Contains(filtered[0].Snippet, "JSON.parse") {
		t.Errorf("wrong survivor: %q", filtered[0].Snippet)
	}
}

func TestDetectMinStatementsFiltersShallowBlocks(t *testing.T) {
	oneStatement := `{
  return buildTheThing(firstArgument, secondArgument, thirdArgument, fourthArg);
}`
	dir := writeProject(t, map[string]string{
		"a.ts": "function a() " + oneStatement + "\nfunction c(rawInput) " + nestedLogic + "\n",
		"b.ts": "function b() " + oneStatement + "\nfunction d(rawInput) " + nestedLogic + "\n",
	})

	all := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 8})
	if len(all) != 2 {
		t.Fatalf("expected both blocks, got %d: %#v", len(all), all)
	}

	filtered := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 8, MinStatements: 2})
	if len(filtered) != 1 {
		t.Fatalf("expected only the multi-statement block, got %d: %#v", len(filtered), filtered)
	}
	if !strings.Contains(filtered[0].Snippet, "JSON.parse") {
		t.Errorf("wrong survivor: %q", filtered[0].Snippet)
	}
}

func TestDetectReportsDepthAndStatements(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "export const configA = " + flatObject + ";\n",
		"b.ts": "export const configB = " + flatObject + ";\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 10})
	if len(dups) != 1 {
		t.Fatalf("expected 1 duplication, got %d", len(dups))
	}
	if dups[0].Depth != 1 {
		t.Errorf("Depth = %d, want 1 for a flat object", dups[0].Depth)
	}
	if dups[0].Statements != 0 || dups[0].IsStatementBlock {
		t.Errorf("an object literal should report no statements, got %d (isStatementBlock=%v)",
			dups[0].Statements, dups[0].IsStatementBlock)
	}
}

func TestDetectMinDuplicates(t *testing.T) {
	twice := strings.Replace(nestedLogic, "batch", "pair", 1)
	dir := writeProject(t, map[string]string{
		// nestedLogic appears 3 times, `twice` only 2.
		"a.ts": "function a(rawInput) " + nestedLogic + "\nfunction p(rawInput) " + twice + "\n",
		"b.ts": "function b(rawInput) " + nestedLogic + "\nfunction q(rawInput) " + twice + "\n",
		"c.ts": "function c(rawInput) " + nestedLogic + "\n",
	})

	all := detect(t, dir, Options{Blinding: Blinding{}})
	if len(all) != 2 {
		t.Fatalf("expected both patterns at the default, got %d: %#v", len(all), all)
	}

	three := detect(t, dir, Options{Blinding: Blinding{}, MinDuplicates: 3})
	if len(three) != 1 {
		t.Fatalf("min-duplicates=3 should leave only the tripled pattern, got %d: %#v", len(three), three)
	}
	if len(three[0].Occurrences) != 3 {
		t.Errorf("survivor has %d occurrences, want 3", len(three[0].Occurrences))
	}

	if four := detect(t, dir, Options{Blinding: Blinding{}, MinDuplicates: 4}); len(four) != 0 {
		t.Fatalf("min-duplicates=4 should report nothing, got %#v", four)
	}
}

func TestDetectMinDuplicatesBelowTwoIsClamped(t *testing.T) {
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(rawInput) " + nestedLogic + "\n",
		"b.ts": "function b(rawInput) " + nestedLogic + "\n",
	})
	// 1 or 0 would mean "report every block in the project"; it clamps to 2.
	for _, n := range []int{0, 1, 2} {
		dups := detect(t, dir, Options{Blinding: Blinding{}, MinDuplicates: n})
		if len(dups) != 1 {
			t.Fatalf("MinDuplicates=%d gave %d duplications, want 1", n, len(dups))
		}
	}
}

func TestDetectFiltersComposeWithoutSplittingGroups(t *testing.T) {
	// Metrics come off the canonical form, so a re-indented copy must score the same and
	// stay in the same group rather than being filtered away from its twin.
	compact := strings.ReplaceAll(strings.ReplaceAll(nestedLogic, "\n", " "), "  ", "")
	dir := writeProject(t, map[string]string{
		"a.ts": "function a(rawInput) " + nestedLogic + "\n",
		"b.ts": "function b(rawInput) " + compact + "\n",
	})

	dups := detect(t, dir, Options{Blinding: Blinding{}, MinDepth: 2, MinStatements: 2})
	if len(dups) != 1 || len(dups[0].Occurrences) != 2 {
		t.Fatalf("formatting split the group under the complexity filters: %#v", dups)
	}
}

// TestMinStatementsOnlyFiltersStatementBlocks pins that the statement floor is a question
// asked OF blocks, not a way of excluding everything that is not one. An object literal or
// a JSX element has no statements to be short of, so it passes at any value; only
// function and control-flow bodies are held to the number.
func TestMinStatementsOnlyFiltersStatementBlocks(t *testing.T) {
	jsx := `(
    <Card className="panel">
      <CardHeader title={title} />
      <CardBody><Text>{description}</Text></CardBody>
    </Card>
  )`
	dir := writeProject(t, map[string]string{
		"a.tsx": "export const A = ({ title, description }) => " + jsx + ";\n" +
			"export const cfgA = " + flatObject + ";\n" +
			"export function runA(rawInput) " + nestedLogic + "\n",
		"b.tsx": "export const B = ({ title, description }) => " + jsx + ";\n" +
			"export const cfgB = " + flatObject + ";\n" +
			"export function runB(rawInput) " + nestedLogic + "\n",
	})

	base := Options{Blinding: Blinding{}, MinTokens: 5, MinLines: 1}
	all := detect(t, dir, base)
	if len(all) != 3 {
		t.Fatalf("fixture should produce JSX, object and logic duplications, got %d: %s",
			len(all), describe(all))
	}

	// The logic block has 3 statements. Raising the floor past that drops it and leaves
	// the two expressions untouched.
	for _, tc := range []struct {
		floor int
		want  int
	}{
		{1, 3}, {3, 3}, {4, 2}, {99, 2},
	} {
		opts := base
		opts.MinStatements = tc.floor
		got := detect(t, dir, opts)
		if len(got) != tc.want {
			t.Errorf("min-statements %d: got %d duplications, want %d:\n%s",
				tc.floor, len(got), tc.want, describe(got))
		}
		for _, d := range got {
			if !d.IsStatementBlock {
				continue
			}
			if d.Statements < tc.floor {
				t.Errorf("min-statements %d: a statement block with %d statements survived",
					tc.floor, d.Statements)
			}
		}
	}

	// At a floor above every statement block, the survivors are exactly the expressions.
	survivors := detect(t, dir, withMinStatements(base, 99))
	for _, d := range survivors {
		if d.IsStatementBlock {
			t.Errorf("a statement block survived an impossible floor: %+v", d)
		}
	}
}
