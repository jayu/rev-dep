package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dual-blinding scan produces the structural canonical form as a by-product of a literal
// pass, so that running both comparisons costs one traversal rather than two.
//
// That is only sound if the by-product is IDENTICAL to what a direct structural scan
// produces - byte for byte, and block for block. A snapshot records the hash of that form,
// so a discrepancy would silently change the identity of code that did not change.

func dualCorpus(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, root := range []string{"../../__fixtures__", "../../docs/src", "../../testdata"} {
		_ = filepath.Walk(root, func(p string, i os.FileInfo, e error) error {
			if e != nil || i.IsDir() || i.Size() > 400_000 {
				return nil
			}
			switch strings.ToLower(filepath.Ext(p)) {
			case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
				if c, err := os.ReadFile(p); err == nil {
					out[p] = c
				}
			}
			return nil
		})
	}
	if len(out) < 50 {
		t.Fatalf("corpus too small: %d files", len(out))
	}
	return out
}

func TestDualBlindingAltFormMatchesDirectScan(t *testing.T) {
	for path, content := range dualCorpus(t) {
		allowJSX := true
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ts", ".mts", ".cts":
			allowJSX = false
		}

		direct := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{Identifiers: true}, AllowJSX: allowJSX,
		}, nil)
		dual := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: allowJSX, AltIdentifiers: true,
		}, nil)

		if string(dual.NormAlt) != string(direct.Norm) {
			t.Fatalf("%s: derived structural form differs from a direct scan\n derived: %q\n direct:  %q",
				path, truncateForDiff(string(dual.NormAlt)), truncateForDiff(string(direct.Norm)))
		}
		if len(dual.Blocks) != len(direct.Blocks) {
			t.Fatalf("%s: %d blocks dual vs %d direct", path, len(dual.Blocks), len(direct.Blocks))
		}
		for i := range direct.Blocks {
			d, x := dual.Blocks[i], direct.Blocks[i]
			if d.Start != x.Start || d.End != x.End {
				t.Errorf("%s block %d: original span %d-%d vs %d-%d",
					path, i, d.Start, d.End, x.Start, x.End)
			}
			// The alternate span must locate the block in the derived buffer exactly
			// where the direct scan located it in its own.
			if d.AltNormStart != x.NormStart || d.AltNormEnd != x.NormEnd {
				t.Errorf("%s block %d: structural span %d-%d vs %d-%d",
					path, i, d.AltNormStart, d.AltNormEnd, x.NormStart, x.NormEnd)
			}
			if d.Role != x.Role || d.Depth != x.Depth || d.Statements != x.Statements {
				t.Errorf("%s block %d: metrics differ: %+v vs %+v", path, i, d, x)
			}
		}
	}
}

// TestDualBlinding{}FormIsUnaffected checks the other half: asking for the structural form
// as well must not perturb the literal one.
func TestDualBlindingPrimaryFormIsUnaffected(t *testing.T) {
	for path, content := range dualCorpus(t) {
		allowJSX := !strings.HasSuffix(path, ".ts")

		alone := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: allowJSX,
		}, nil)
		literalNorm := string(alone.Norm)
		aloneBlocks := append([]CodeBlock(nil), alone.Blocks...)

		both := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: allowJSX, AltIdentifiers: true,
		}, nil)

		if string(both.Norm) != literalNorm {
			t.Fatalf("%s: literal form changed when the structural one was requested", path)
		}
		if len(both.Blocks) != len(aloneBlocks) {
			t.Fatalf("%s: %d blocks with alt vs %d without", path, len(both.Blocks), len(aloneBlocks))
		}
		for i := range aloneBlocks {
			if both.Blocks[i].NormStart != aloneBlocks[i].NormStart ||
				both.Blocks[i].NormEnd != aloneBlocks[i].NormEnd {
				t.Errorf("%s block %d: literal span moved", path, i)
			}
		}
	}
}

// TestDualBlindingSharedTokenFloor pins why one floor can serve both forms: a token count
// does not move when names are blinded, because folding swaps one token for another rather
// than removing it. A character floor could not have done this - the same code measures
// roughly half as long once names are folded.
func TestDualBlindingSharedTokenFloor(t *testing.T) {
	for path, content := range dualCorpus(t) {
		allowJSX := true
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ts", ".mts", ".cts":
			allowJSX = false
		}
		exact := ScanCodeBlocks(content, BlockScanOptions{AllowJSX: allowJSX}, nil)
		blind := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{Identifiers: true}, AllowJSX: allowJSX,
		}, nil)

		if len(exact.Blocks) != len(blind.Blocks) {
			t.Fatalf("%s: %d blocks exact vs %d blinded", path, len(exact.Blocks), len(blind.Blocks))
		}
		for i := range exact.Blocks {
			if exact.Blocks[i].Tokens != blind.Blocks[i].Tokens {
				t.Errorf("%s block %d: %d tokens exact vs %d blinded - the count must not move",
					path, i, exact.Blocks[i].Tokens, blind.Blocks[i].Tokens)
			}
		}
	}
}

// TestTokenCountIsStableAcrossEveryBlinding extends that to all eight combinations.
func TestTokenCountIsStableAcrossEveryBlinding(t *testing.T) {
	code := []byte(`function handle(request, options) {
  const label = "a message";
  const limit = 42;
  if (request.ok) { return render(label, limit); }
  return null;
}`)
	var want uint32
	for i := 0; i < 8; i++ {
		b := Blinding{Identifiers: i&1 != 0, Strings: i&2 != 0, Numbers: i&4 != 0}
		res := ScanCodeBlocks(code, BlockScanOptions{Blinding: b}, nil)
		if len(res.Blocks) == 0 {
			t.Fatalf("%v: no blocks", b)
		}
		got := res.Blocks[len(res.Blocks)-1].Tokens
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Errorf("%v: %d tokens, want %d - blinding must not change the count", b, got, want)
		}
	}
}

func truncateForDiff(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
