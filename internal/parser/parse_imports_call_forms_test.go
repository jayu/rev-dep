package parser

import "testing"

func TestParsePlainTemplateArgument(t *testing.T) {
	cases := []struct {
		code    string
		request string
	}{
		{"import(`./a`)", "./a"},
		{"require(`./b`)", "./b"},
		{"import(\n  `./multi`\n)", "./multi"},
		{"import(/* webpackChunkName: \"x\" */ `./chunk`)", "./chunk"},
		{"const a = () => import(`./a`);", "./a"},
		{"function f() { return import(`./nested`); }", "./nested"},
		{"const t = () => require(`./esc\\`aped`)", "./esc\\`aped"},
		{"import(`$dollar`)", "$dollar"},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			for _, mode := range []ParseMode{ParseModeBasic, ParseModeDetailed} {
				imports := ParseImportsByte([]byte(tc.code), false, mode)
				if len(imports) != 1 {
					t.Fatalf("mode %d: expected 1 import, got %v", mode, imports)
				}
				imp := imports[0]
				if imp.Request != tc.request || !imp.IsDynamicImport || imp.Kind != NotTypeOrMixedImport {
					t.Fatalf("mode %d: unexpected import %+v", mode, imp)
				}
				if got := tc.code[imp.RequestStart:imp.RequestEnd]; got != tc.request {
					t.Fatalf("mode %d: request range covers %q", mode, got)
				}
			}
		})
	}
}

func TestParseTemplateImportDoesNotHideLaterImports(t *testing.T) {
	code := "const a = import(`./a/${x}`);\nconst b = require(`./b`);\nimport c from './c';"
	imports := ParseImportsForTests(code)
	want := []string{"./b", "./c"}
	if len(imports) != len(want) {
		t.Fatalf("expected %v, got %v", want, imports)
	}
	for i, w := range want {
		if imports[i].Request != w {
			t.Fatalf("import %d: expected %q, got %q", i, w, imports[i].Request)
		}
	}
}
