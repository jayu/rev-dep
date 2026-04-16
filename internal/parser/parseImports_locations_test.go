package parser

import "testing"

func assertSimpleLocation(t *testing.T, got SimpleLocation, startLine, startCol, endLine, endCol int) {
	t.Helper()
	if got.StartLine != startLine || got.StartCol != startCol || got.EndLine != endLine || got.EndCol != endCol {
		t.Fatalf("expected range %d:%d-%d:%d, got %d:%d-%d:%d",
			startLine, startCol, endLine, endCol,
			got.StartLine, got.StartCol, got.EndLine, got.EndCol)
	}
}

func TestResolvePrimaryLocation_DefaultImport(t *testing.T) {
	code := `import Foo from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 18, 1, 21)
}

func TestResolvePrimaryLocation_NamespaceImport(t *testing.T) {
	code := `import * as Ns from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 22, 1, 25)
}

func TestResolvePrimaryLocation_NamedMultilineImport(t *testing.T) {
	code := "import {\n  A,\n  B as C,\n} from \"pkg\""
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 4, 9, 4, 12)
}

func TestResolvePrimaryLocation_TypeImport(t *testing.T) {
	code := `import type { T } from "types"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 25, 1, 30)
}

func TestResolvePrimaryLocation_SideEffectImport(t *testing.T) {
	code := "import \"side-effect\"\nconst x = 1"
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 9, 1, 20)
}

func TestResolvePrimaryLocation_DynamicImport(t *testing.T) {
	code := "const x = 1;\nconst y = import(\"dyn\")"
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 2, 19, 2, 22)
}

func TestResolvePrimaryLocation_ReexportNamed(t *testing.T) {
	code := "export { Foo as Bar } from \"pkg\""
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 10, 1, 20)
}

func TestResolvePrimaryLocation_ReexportStar(t *testing.T) {
	code := "export * from \"pkg\""
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 8, 1, 9)
}

func TestResolvePrimaryLocation_ExportTypeReexport(t *testing.T) {
	code := "export type { T } from \"types\""
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 15, 1, 16)
}

func TestResolvePrimaryLocation_LocalExport(t *testing.T) {
	code := "export const Foo = 1"
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	loc := ResolvePrimaryLocation([]byte(code), imports[0])
	assertSimpleLocation(t, loc, 1, 14, 1, 17)
}
