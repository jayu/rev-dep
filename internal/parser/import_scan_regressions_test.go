package parser

import (
	"strings"
	"testing"
)

// The cases here were found by diffing the unified scan against the loop it replaced, over a
// real dependency tree, back when both existed. That comparison is gone - see
// unified_scan_test.go for why - so these are the record of what it found. They assert on
// what the answer IS rather than on two implementations agreeing, which is why they outlived
// the thing that produced them.

// requestsOf lists the module requests an import scan produced, in order.
func requestsOf(imports []Import) []string {
	out := make([]string, 0, len(imports))
	for _, im := range imports {
		if im.Request != "" {
			out = append(out, im.Request)
		}
	}
	return out
}

// realWorldDivergences are constructs that appear in shipped packages and that the scan got
// wrong. Each names the package it was found in, because a reduced case says what broke but
// not that it was ever going to happen.
var realWorldDivergences = []struct {
	name string
	code string
	want []string
}{
	{
		// webpack/lib/FileSystemInfo.js:
		//   str = `"${str.slice(1, -1).replace(/"/g, '\\"')}"`;
		// ...and ~5 KB later the file's require("es-module-lexer") went missing.
		name: "regex holding a double quote inside a template interpolation",
		code: "const f = (s) => `\"${ s.replace(/\"/g, '\\\\\"') }\"`;\n" +
			"const lexer = require(\"es-module-lexer\");\n",
		want: []string{"es-module-lexer"},
	},
	{
		name: "regex holding a single quote inside a template interpolation",
		code: "const f = (s) => `${ s.replace(/'/g, '-') }`;\n" +
			"import real from './real';\n",
		want: []string{"./real"},
	},
	{
		// The same shape, but the interpolation is nested one level down.
		name: "quoted regex inside a nested template interpolation",
		code: "const f = (s) => `${ `${ s.replace(/\"/g, '-') }` }`;\n" +
			"import real from './real';\n",
		want: []string{"./real"},
	},
	{
		// A quote in a regex is only harmful because the skip has to guess where the
		// interpolation ends. A regex without one has always worked, and must keep working.
		name: "regex without a quote inside a template interpolation",
		code: "const f = (s) => `${ s.replace(/[a-z]/g, '-') }`;\n" +
			"import real from './real';\n",
		want: []string{"./real"},
	},
	{
		name: "several interpolations, the first holding a quoted regex",
		code: "const t = `${ a.replace(/\"/g, '-') } and ${ b }`;\n" +
			"import first from './first';\n" +
			"import second from './second';\n",
		want: []string{"./first", "./second"},
	},
	{
		// A brace inside the regex is the other way the skip can lose count.
		name: "regex holding braces inside a template interpolation",
		code: "const t = `${ s.replace(/[{}]/g, '') }`;\n" +
			"const dep = require('./dep');\n",
		want: []string{"./dep"},
	},
}

// TestImportsSurviveAwkwardTemplateInterpolations is the direct form of the divergence: it
// does not care what any other implementation says, only that the imports are found.
func TestImportsSurviveAwkwardTemplateInterpolations(t *testing.T) {
	for _, tc := range realWorldDivergences {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []ParseMode{ParseModeBasic, ParseModeDetailed} {
				got := requestsOf(ParseImportsByte([]byte(tc.code), false, mode))
				if strings.Join(got, ",") != strings.Join(tc.want, ",") {
					t.Errorf("mode=%v: requests = %v, want %v\n  source:\n%s",
						mode, got, tc.want, tc.code)
				}
			}
		})
	}
}

// TestTemplateSkipDoesNotDesyncTheRestOfTheFile states the failure mode rather than the
// symptom: whatever the skip does with the template, the scanner has to come out of it at
// the character after the closing backtick.
//
// Asserting on a later import is what makes this visible at all - a desynced scan produces
// no error, it just silently stops finding things, and how much it loses depends on where
// the next quote in the file happens to be.
func TestTemplateSkipDoesNotDesyncTheRestOfTheFile(t *testing.T) {
	const trailer = "import marker from './marker';\n"
	interpolations := []string{
		"${ s.replace(/\"/g, '-') }",
		"${ s.replace(/'/g, '-') }",
		"${ s.replace(/`/g, '-') }",
		"${ s.split(/[\"']/) }",
		"${ /* } */ s }",
		"${ s // }\n }",
		"${ a / b }",
		"${ { key: 'value' } }",
		"${ `${ inner }` }",
	}
	for _, interp := range interpolations {
		code := "const t = `head " + interp + " tail`;\n" + trailer
		got := requestsOf(ParseImportsByte([]byte(code), false, ParseModeDetailed))
		if len(got) != 1 || got[0] != "./marker" {
			t.Errorf("interpolation %q swallowed the import that follows it: got %v\n  source:\n%s",
				interp, got, code)
		}
	}
}

// TestExportsInsideDeclareNamespace pins a behaviour the unified scan CHANGED.
//
// The loop it replaced tracked brace depth itself, and `export = X;` at the top of a
// declaration file left that count behind: everything after it read as top level, so the
// `export`s inside `declare namespace X { ... }` were reported as module exports. The
// unified scan tracks depth from the canonical stream and does not drift, so it refuses
// them - `detectImportAt` only offers `export` at depth zero.
//
// This is almost certainly the better answer, but it is a real change: unused-exports sees
// 29 fewer exports in @types/react and 44 fewer in preact/compat than it used to. The test
// exists so the change is a decision rather than a side effect - if ambient exports should
// count, this is the test to invert.
func TestExportsInsideDeclareNamespace(t *testing.T) {
	const code = "export = React;\n" +
		"export as namespace React;\n" +
		"declare namespace React {\n" +
		"    export interface FragmentProps { children?: unknown }\n" +
		"    export function useId(): string;\n" +
		"}\n"

	localExports := 0
	for _, im := range ParseImportsByte([]byte(code), false, ParseModeDetailed) {
		if im.IsLocalExport {
			localExports++
		}
	}
	if localExports != 0 {
		t.Errorf("exports nested in `declare namespace` are being reported (%d of them); "+
			"if that is now intended, invert this test", localExports)
	}

	// The same declarations at the top level are reported, so what is being pinned above is
	// the nesting and not some blanket failure to see `export interface`.
	const flat = "export interface FragmentProps { children?: unknown }\n" +
		"export function useId(): string;\n"
	topLevel := 0
	for _, im := range ParseImportsByte([]byte(flat), false, ParseModeDetailed) {
		if im.IsLocalExport {
			topLevel++
		}
	}
	if topLevel != 2 {
		t.Errorf("top-level exports = %d, want 2 - the nesting is not what is being tested", topLevel)
	}
}

// truncatedSources end in the middle of a construct. Every one of them is something a
// reader can produce by saving a file mid-edit, so none may take the process down.
var truncatedSources = []string{
	"const x = 1; require   ",
	"const x = 1; require",
	"const x = 1; import   ",
	"const x = 1; import",
	"require   ",
	"import   ",
	"require",
	"import",
	"function f(){ require   ",
	"function f(){ import   ",
	"export   ",
	"export",
	"declare   ",
	"import a from",
	"import a from   ",
	"const m = require(",
	"const m = require('",
	"const p = import(",
	"export { a } from",
	"import type",
	"`unterminated ${ require   ",
}

// TestScanningTruncatedSourceDoesNotPanic covers a file that stops mid-construct.
//
// A parser that panics on one file takes down the whole run, and the run is over a whole
// repository - so the file that does it is by definition one the author did not choose.
// Predates the unified scan: parseExpression indexes past the end when a `require` or
// `import` keyword is the last thing in the file.
func TestScanningTruncatedSourceDoesNotPanic(t *testing.T) {
	for _, src := range truncatedSources {
		t.Run(strings.ReplaceAll(src, " ", "_"), func(t *testing.T) {
			for _, mode := range []ParseMode{ParseModeBasic, ParseModeDetailed} {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("ParseImportsByte panicked on %q [mode=%v]: %v", src, mode, r)
						}
					}()
					ParseImportsByte([]byte(src), false, mode)
				}()
			}
			// The block scanner walks the same input for duplicate detection, so it has to
			// survive it too.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("ScanCodeBlocks panicked on %q: %v", src, r)
					}
				}()
				ScanCodeBlocks([]byte(src), BlockScanOptions{
					Blinding: Blinding{}, AllowJSX: true, MinTokens: 1,
					CollectImports: true, ImportMode: ParseModeDetailed,
				}, nil)
			}()
		})
	}
}
