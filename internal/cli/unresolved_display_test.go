package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rev-dep-go/internal/checks"
)

func writeSource(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// A literal request names itself. A non-literal one is an expression, which only reads as an import
// once the call around it is shown, so the label wraps it in the keyword that precedes it.
func TestUnresolvedLabeller(t *testing.T) {
	dir := t.TempDir()
	labeller := newUnresolvedLabeller(dir)

	nonLiteral := func(path string, request string, start int) string {
		return labeller.Label(checks.UnresolvedImport{
			FilePath: path, Request: request, IsNonLiteral: true,
			RequestStart: uint32(start), RequestEnd: uint32(start + len(request)),
		})
	}

	t.Run("literal request is unchanged", func(t *testing.T) {
		path := writeSource(t, dir, "literal.ts", "import a from 'missing-pkg';\n")
		got := labeller.Label(checks.UnresolvedImport{FilePath: path, Request: "missing-pkg"})
		if got != "missing-pkg" {
			t.Fatalf("expected the request alone, got %q", got)
		}
	})

	t.Run("import call is rebuilt around the expression", func(t *testing.T) {
		code := "export const load = (n) => import('@/api/' + n);\n"
		path := writeSource(t, dir, "import.ts", code)
		got := nonLiteral(path, "'@/api/' + n", strings.Index(code, "'@/api/'"))
		if want := "import('@/api/' + n) - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("require call is rebuilt too", func(t *testing.T) {
		code := "const m = require(name);\n"
		path := writeSource(t, dir, "require.ts", code)
		got := nonLiteral(path, "name", strings.Index(code, "name)"))
		if want := "require(name) - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("a call split across lines is rebuilt on one line", func(t *testing.T) {
		code := "const m = import(\n  '@/api/' +\n  name\n);\n"
		path := writeSource(t, dir, "multi.ts", code)
		// The parser stores the expression with its whitespace collapsed.
		got := nonLiteral(path, "'@/api/' + name", strings.Index(code, "'@/api/'"))
		if want := "import('@/api/' + name) - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("unreadable file falls back to the expression", func(t *testing.T) {
		got := nonLiteral(filepath.Join(dir, "gone.ts"), "name", 0)
		if want := "name - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("offsets past the end of the file fall back to the expression", func(t *testing.T) {
		code := "const m = import(name);\n"
		path := writeSource(t, dir, "short.ts", code)
		got := labeller.Label(checks.UnresolvedImport{
			FilePath: path, Request: "name", IsNonLiteral: true,
			RequestStart: 0, RequestEnd: uint32(len(code) + 100),
		})
		if want := "name - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("something other than import or require is left alone", func(t *testing.T) {
		code := "const m = load(name);\n"
		path := writeSource(t, dir, "other.ts", code)
		got := nonLiteral(path, "name", strings.Index(code, "name)"))
		if want := "name - " + nonLiteralTag; got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("a long call is shortened", func(t *testing.T) {
		expr := "'" + strings.Repeat("a", 400) + "' + n"
		code := "const x = import(" + expr + ");\n"
		path := writeSource(t, dir, "long.ts", code)
		got := nonLiteral(path, expr, strings.Index(code, "'"))
		if !strings.HasSuffix(got, "... - "+nonLiteralTag) {
			t.Fatalf("expected a shortened call, got %q", got)
		}
		if len(got) > maxCallTextInReport+40 {
			t.Fatalf("label is too long (%d chars): %q", len(got), got)
		}
	})
}
