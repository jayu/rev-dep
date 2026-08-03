package dupcode

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"rev-dep-go/internal/parser"
)

// benchProject builds a synthetic tree with a realistic mix: mostly unique code, a
// scattering of copy-pasted blocks and JSX, and enough files that the parallel phases are
// actually exercised.
func benchProject(tb testing.TB, files int) string {
	tb.Helper()
	dir := tb.TempDir()

	const dupBlock = `{
  const request = buildRequest(endpoint, payload);
  const response = await client.send(request);
  if (!response.ok) {
    throw new TransportError(response.status, response.body);
  }
  return response.body;
}`
	const dupJSX = `(
    <Card className="panel">
      <CardHeader title={title} subtitle={subtitle} />
      <CardBody>
        <Text tone="muted">{description}</Text>
      </CardBody>
    </Card>
  )`

	for i := 0; i < files; i++ {
		var src string
		for j := 0; j < 12; j++ {
			src += fmt.Sprintf(`
export function unique%d_%d(input) {
  const scaled%d = input.value * %d;
  const label%d = formatLabel(input.kind, scaled%d);
  if (scaled%d > %d) {
    return { label: label%d, level: "high", index: %d };
  }
  return { label: label%d, level: "low", index: %d };
}
`, i, j, j, i+j+1, j, j, j, i*7+j, j, i, j, i)
		}
		// Every fifth file carries the copy-pasted block, every seventh the JSX.
		if i%5 == 0 {
			src += "\nasync function send" + fmt.Sprint(i) + "(endpoint, payload) " + dupBlock + "\n"
		}
		ext := ".ts"
		if i%7 == 0 {
			ext = ".tsx"
			src += "\nexport const Panel" + fmt.Sprint(i) + " = ({ title, subtitle, description }) => " + dupJSX + ";\n"
		}
		name := filepath.Join(dir, fmt.Sprintf("mod%03d", i/50), fmt.Sprintf("file%04d%s", i, ext))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return dir
}

func BenchmarkDetectLiteral(b *testing.B) {
	dir := benchProject(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := Detect(Options{Cwd: dir, Blinding: Blinding{}}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDetectStructural(b *testing.B) {
	dir := benchProject(b, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := Detect(Options{Cwd: dir, Blinding: Blinding{Identifiers: true}}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkScanCodeBlocks isolates the per-file cost of the parser extension from file IO
// and matching, which is where a regression would most likely show up first.
func BenchmarkScanCodeBlocks(b *testing.B) {
	src, err := os.ReadFile("detect.go") // any sizeable file; contents just need braces
	if err != nil {
		b.Fatal(err)
	}
	dst := &parser.BlockScan{}
	b.SetBytes(int64(len(src)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ScanCodeBlocks(src, parser.BlockScanOptions{
			Blinding:  parser.Blinding{},
			AllowJSX:  true,
			MinTokens: 38,
		}, dst)
	}
}

// TestDetectFindsPlantedDuplicatesInBenchProject guards the benchmark fixture itself: if
// it stopped containing duplicates the benchmarks would still run and measure nothing.
func TestDetectFindsPlantedDuplicatesInBenchProject(t *testing.T) {
	dir := benchProject(t, 40)
	dups := detect(t, dir, Options{Blinding: Blinding{}, MinTokens: 25})

	var block, jsx *Duplication
	for i := range dups {
		switch dups[i].Kind {
		case parser.BlockJSX:
			if jsx == nil {
				jsx = &dups[i]
			}
		case parser.BlockBrace:
			if block == nil {
				block = &dups[i]
			}
		}
	}
	if block == nil || len(block.Occurrences) != 8 {
		t.Errorf("planted brace block: %v occurrences, want 8", occCount(block))
	}
	if jsx == nil || len(jsx.Occurrences) != 6 {
		t.Errorf("planted JSX block: %v occurrences, want 6", occCount(jsx))
	}
}

func occCount(d *Duplication) int {
	if d == nil {
		return 0
	}
	return len(d.Occurrences)
}
