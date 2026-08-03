package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ParseImportsByte now runs on the unified scan rather than its own loop. The existing
// suite covers what imports SHOULD be found; this file covers the thing that suite cannot
// see - what the scan does to every file in the repository, not just to the cases someone
// thought to write a test for.
//
// For a while that was done by diffing against the loop the unified scan replaced. That
// proved the migration was faithful, which was the whole point, and then it became the
// obstacle: parity with the old scan also pinned the old scan's bugs, so `</` being read as
// a regex literal could not be fixed without a test failing for doing the right thing. A
// test whose failure means "you improved something" is a test that will be deleted or
// ignored, and neither is a good outcome.
//
// So the sweep is now a golden. It still catches an accidental change - that was always the
// real value - but regenerating it is a deliberate act, and the diff shows exactly which
// files the change affected. Fixing a bug now produces evidence instead of an obstacle.

func importsEqual(a, b []Import) (string, bool) {
	if len(a) != len(b) {
		return fmt.Sprintf("count %d vs %d", len(a), len(b)), false
	}
	for i := range a {
		// Keywords is a pointer, so the structs must be compared field-wise: two runs
		// legitimately allocate different KeywordMaps holding identical entries.
		x, y := a[i], b[i]
		xk, yk := x.Keywords, y.Keywords
		x.Keywords, y.Keywords = nil, nil
		if x != y {
			return fmt.Sprintf("import %d:\n  a: %+v\n  b: %+v", i, a[i], b[i]), false
		}
		if (xk == nil) != (yk == nil) {
			return fmt.Sprintf("import %d: keywords %v vs %v", i, xk, yk), false
		}
		if xk == nil {
			continue
		}
		if len(xk.Keywords) != len(yk.Keywords) {
			return fmt.Sprintf("import %d: %d keywords vs %d\n  a: %+v\n  b: %+v",
				i, len(xk.Keywords), len(yk.Keywords), xk.Keywords, yk.Keywords), false
		}
		for j := range xk.Keywords {
			if xk.Keywords[j] != yk.Keywords[j] {
				return fmt.Sprintf("import %d keyword %d:\n  a: %+v\n  b: %+v",
					i, j, xk.Keywords[j], yk.Keywords[j]), false
			}
		}
	}
	return "", true
}

// corpusFiles collects every JS/TS source in the repository's fixtures and test data.
// Between them these cover the import forms the project supports plus the surrounding
// code that makes scanning hard: JSX, decorators, generics, regex literals and templates.
func corpusFiles(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, root := range []string{"../../__fixtures__", "../../testdata", "../../docs/src"} {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
			default:
				return nil
			}
			if info.Size() > 500_000 {
				return nil
			}
			if content, err := os.ReadFile(path); err == nil {
				out[path] = content
			}
			return nil
		})
	}
	if len(out) < 50 {
		t.Fatalf("corpus too small to be meaningful: %d files", len(out))
	}
	return out
}

// renderImports writes one scan result as text a human can read in a diff. A digest would
// make the golden shorter and every failure unreadable, which is the wrong trade for a file
// whose entire job is to be reviewed when it changes.
func renderImports(imports []Import) string {
	if len(imports) == 0 {
		return "    (none)"
	}
	var b strings.Builder
	for i, imp := range imports {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "    %s kind=%d resolved=%d dynamic=%t range=%d..%d",
			imp.Request, imp.Kind, imp.ResolvedType, imp.IsDynamicImport,
			imp.RequestStart, imp.RequestEnd)
		if imp.Keywords != nil {
			fmt.Fprintf(&b, " keywords=%d", len(imp.Keywords.Keywords))
		}
	}
	return b.String()
}

const importScanGolden = "import-scan.golden"

// checkImportScanGolden compares a rendered scan against the committed one.
//
// Regenerate deliberately with:
//
//	UPDATE_IMPORT_SCAN_GOLDEN=1 go test ./internal/parser/ -run 'ImportScan.*Golden'
func checkImportScanGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)

	if os.Getenv("UPDATE_IMPORT_SCAN_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (regenerate with UPDATE_IMPORT_SCAN_GOLDEN=1): %v", err)
	}
	if got == string(want) {
		return
	}
	// The first differing line is what a reader needs; printing two multi-thousand-line
	// blobs and asking them to spot it is not a test failure anyone acts on.
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		g, w := "", ""
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("import scanning changed at %s line %d.\n  got:  %s\n  want: %s\n\n"+
				"If this change is intended, confirm it is an improvement and regenerate with "+
				"UPDATE_IMPORT_SCAN_GOLDEN=1.", name, i+1, g, w)
		}
	}
}

// TestImportScanCorpusGolden pins what the scan finds in every source file the repository
// has, across both parse modes and both type-import settings.
//
// This is the broad net: no case here was written by hand, so it covers the constructs
// nobody thought to test as well as the ones they did.
func TestImportScanCorpusGolden(t *testing.T) {
	files := corpusFiles(t)

	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("# imports found by ParseImportsByte, per file and per setting\n")
	b.WriteString("# regenerate: UPDATE_IMPORT_SCAN_GOLDEN=1 go test ./internal/parser/ -run 'ImportScan.*Golden'\n")
	for _, path := range paths {
		for _, mode := range []ParseMode{ParseModeBasic, ParseModeDetailed} {
			for _, ignoreTypes := range []bool{false, true} {
				fmt.Fprintf(&b, "\n%s [mode=%d ignoreTypes=%t]\n", path, mode, ignoreTypes)
				b.WriteString(renderImports(ParseImportsByte(files[path], ignoreTypes, mode)))
				b.WriteString("\n")
			}
		}
	}
	checkImportScanGolden(t, importScanGolden, b.String())
}

// trickySources are the constructs most likely to break the scan, since strings, templates
// and regex literals are handled by a state machine that has to stay in step with itself
// across all of them.
var trickySources = []struct {
	name string
	code string
}{
	{"import inside a string", `const s = "import foo from 'x'"; import real from 'y';`},
	{"import inside a line comment", "// import fake from 'x'\nimport real from 'y';"},
	{"import inside a block comment", "/* import fake from 'x' */ import real from 'y';"},
	{"require inside a template", "const t = `require('fake')`; const r = require('real');"},
	{"dynamic import in a nested block", `function f() { if (x) { return import('./a'); } }`},
	{"require in a nested block", `function f() { const m = require('./b'); return m; }`},
	{"member access is not an import", `obj.import('x'); obj.require('y'); import real from 'z';`},
	{"regex containing quotes", `const re = /['"]/g; import real from 'x';`},
	{"regex containing a slash", `const re = /a\/b/g; import real from 'x';`},
	{"template with interpolation", "const t = `a ${ b } c`; import real from 'x';"},
	{"nested template literals", "const t = `a ${ `x ${ y }` } b`; import real from 'x';"},
	// An interpolation holds real code, so it can hold a regex literal, and a regex literal
	// can hold a quote. Skipping the template as one opaque run has to step over the regex,
	// or the quote inside it reads as a string opener and the scan runs on to the next quote
	// in the FILE - taking every later import with it.
	{"regex with a double quote inside an interpolation",
		"const t = `${ s.replace(/\"/g, '-') }`; import real from 'x';"},
	{"regex with a single quote inside an interpolation",
		"const t = `${ s.replace(/'/g, '-') }`; const r = require('y');"},
	{"regex with a brace inside an interpolation",
		"const t = `${ s.replace(/[{}]/g, '-') }`; import real from 'x';"},
	{"quoted regex inside a nested template",
		"const t = `${ `${ s.replace(/\"/g, '-') }` }`; import real from 'x';"},
	{"comment containing a brace inside an interpolation",
		"const t = `${ /* } */ b }`; import real from 'x';"},
	{"line comment inside an interpolation",
		"const t = `${ b // }\n }`; import real from 'x';"},
	{"division inside an interpolation", "const t = `${ a / b }`; import real from 'x';"},
	{"export from", `export { a, b } from './m'; export * from './n';`},
	{"export default function with body", `export default function f() { const m = require('./inner'); return m; }`},
	{"type import", `import type { A } from './types'; import { B } from './values';`},
	{"mixed type specifiers", `import { type A, B } from './m';`},
	{"declare module block", `declare module 'x' { import fake from './fake'; } import real from './real';`},
	{"import assertion", `import data from './d.json' assert { type: 'json' };`},
	{"side effect import", `import './styles.css';`},
	{"namespace import", `import * as ns from './m';`},
	{"require with template arg", "const m = require(`./x`);"},
	{"jsx with closing tags", `import React from 'react'; const a = <Row><Cell><Dot /></Cell></Row>;`},
	{"jsx expression with import", `const a = <div>{load(() => import('./lazy'))}</div>;`},
	{"class with methods", `export class A { m() { return require('./x'); } n() { return 1; } }`},
	{"object literal with braces", `const o = { a: { b: { c: 1 } } }; import real from 'x';`},
	{"arrow returning object", `const f = () => ({ a: 1 }); import real from 'x';`},
	{"labelled statement", `outer: for (;;) { break outer; } import real from 'x';`},
	{"semicolonless", "import a from './a'\nimport b from './b'\nconst c = require('./c')"},
	{"windows line endings", "import a from './a';\r\nimport b from './b';\r\n"},
	{"unicode identifiers", `import { café } from './c'; const naïve = 1;`},
	{"deeply nested dynamic import", `function a(){ function b(){ function c(){ return import('./deep'); } } }`},
	{"export declare", `export declare const x: number; import real from './r';`},
	{"import in default param", `function f(g = () => import('./p')) {}`},
	{"comment between keyword and specifier", "import /* c */ { a } /* c */ from /* c */ './m';"},
	{"empty file", ``},
	{"only whitespace", "   \n\t\n  "},
	{"unterminated string", `import a from './a'; const s = "unterminated`},
	{"unterminated template", "import a from './a'; const t = `unterminated"},
	{"unterminated block comment", "import a from './a'; /* unterminated"},
	{"lone angle bracket", `const a = 1 < 2; import real from 'x';`},
	{"generic call", `const m = new Map<string, number>(); import real from 'x';`},

	// A '/' after '<' is a closing tag, not a regex. Read as a regex it runs to the next '/'
	// on the line - the '/' of the next closing tag - and everything between is swallowed,
	// which on one line of JSX is where the imports live. Multi-line JSX hid this for a long
	// time because a regex literal cannot span a line break.
	{"adjacent elements on one line",
		`const a = <div><b></b><i>{require('./x')}</i></div>;`},
	{"empty element then a sibling",
		`const a = <><Icon /><Label>{require('./y')}</Label></>;`},
	{"dynamic import between two closing tags",
		`const a = <ul><li></li><li>{import('./z')}</li></ul>;`},
	{"closing tag then a division",
		`const a = <p></p>; const q = total / count; import real from 'x';`},
	// What the rule gives up, pinned so the trade is visible rather than assumed.
	{"less-than followed by a regex", `const a = x < /re/.source.length; import real from 'y';`},
}

// TestImportScanTrickySourcesGolden is the narrow net: hand-picked constructs, each with
// its source printed beside the result, so a golden diff here reads as an explanation
// rather than as a line number.
func TestImportScanTrickySourcesGolden(t *testing.T) {
	var b strings.Builder
	b.WriteString("# imports found in hand-written awkward sources\n")
	b.WriteString("# regenerate: UPDATE_IMPORT_SCAN_GOLDEN=1 go test ./internal/parser/ -run 'ImportScan.*Golden'\n")
	for _, tc := range trickySources {
		for _, mode := range []ParseMode{ParseModeBasic, ParseModeDetailed} {
			for _, ignoreTypes := range []bool{false, true} {
				fmt.Fprintf(&b, "\n%s [mode=%d ignoreTypes=%t]\n  source: %q\n",
					tc.name, mode, ignoreTypes, tc.code)
				b.WriteString(renderImports(ParseImportsByte([]byte(tc.code), ignoreTypes, mode)))
				b.WriteString("\n")
			}
		}
	}
	checkImportScanGolden(t, "import-scan-tricky.golden", b.String())
}

// TestUnifiedScanCollectsImportsAndBlocksTogether is the reason the merge exists: one
// traversal has to produce both results, and they must be the same results either entry
// point would have produced alone.
func TestUnifiedScanCollectsImportsAndBlocksTogether(t *testing.T) {
	files := corpusFiles(t)

	for path, content := range files {
		allowJSX := true
		switch strings.ToLower(filepath.Ext(path)) {
		case ".ts", ".mts", ".cts":
			allowJSX = false
		}

		both := ScanCodeBlocks(content, BlockScanOptions{
			Blinding:       Blinding{},
			AllowJSX:       allowJSX,
			MinTokens:      10,
			CollectImports: true,
			ImportMode:     ParseModeDetailed,
		}, nil)

		importsOnly := ParseImportsByte(content, false, ParseModeDetailed)
		if diff, ok := importsEqual(both.Imports, importsOnly); !ok {
			t.Errorf("%s: combined scan disagrees with the import-only scan: %s", path, diff)
		}

		blocksOnly := ScanCodeBlocks(content, BlockScanOptions{
			Blinding:  Blinding{},
			AllowJSX:  allowJSX,
			MinTokens: 10,
		}, nil)
		if len(both.Blocks) != len(blocksOnly.Blocks) {
			t.Fatalf("%s: combined scan found %d blocks, block-only scan found %d",
				path, len(both.Blocks), len(blocksOnly.Blocks))
		}
		for i := range both.Blocks {
			if both.Blocks[i] != blocksOnly.Blocks[i] {
				t.Errorf("%s: block %d differs:\n  combined: %+v\n  blocks:   %+v",
					path, i, both.Blocks[i], blocksOnly.Blocks[i])
			}
		}
		if string(both.Norm) != string(blocksOnly.Norm) {
			t.Errorf("%s: canonical form differs when imports are collected too", path)
		}
	}
}

// TestSkipBlocksDoesNotChangeImports checks the switch that import-only callers use.
func TestSkipBlocksDoesNotChangeImports(t *testing.T) {
	files := corpusFiles(t)
	for path, content := range files {
		withBlocks := ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: true, MinTokens: 10,
			CollectImports: true, ImportMode: ParseModeDetailed,
		}, nil)
		skipped := ScanCodeBlocks(content, BlockScanOptions{
			SkipBlocks: true, CollectImports: true, ImportMode: ParseModeDetailed,
		}, nil)
		if diff, ok := importsEqual(skipped.Imports, withBlocks.Imports); !ok {
			t.Errorf("%s: SkipBlocks changed the imports: %s", path, diff)
		}
		if len(skipped.Blocks) != 0 {
			t.Errorf("%s: SkipBlocks still recorded %d blocks", path, len(skipped.Blocks))
		}
	}
}

// The benchmarks below measure the cost of the merge rather than assuming it.

func BenchmarkImportsUnified600Loc(b *testing.B) {
	content, _ := os.ReadFile(fixturePath(b, "parseImports600Loc.ts"))
	for b.Loop() {
		ParseImportsByte(content, true, ParseModeDetailed)
	}
}

// BenchmarkImportsThenBlocksSeparately is what a config run costs WITHOUT the merge: two
// full traversals of every file.
func BenchmarkImportsThenBlocksSeparately600Loc(b *testing.B) {
	content, _ := os.ReadFile(fixturePath(b, "parseImports600Loc.ts"))
	dst := &BlockScan{}
	for b.Loop() {
		ParseImportsByte(content, true, ParseModeDetailed)
		ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: true, MinTokens: 38,
		}, dst)
	}
}

// BenchmarkImportsAndBlocksUnified600Loc is what it costs WITH the merge: one traversal
// producing both.
func BenchmarkImportsAndBlocksUnified600Loc(b *testing.B) {
	content, _ := os.ReadFile(fixturePath(b, "parseImports600Loc.ts"))
	dst := &BlockScan{}
	for b.Loop() {
		ScanCodeBlocks(content, BlockScanOptions{
			Blinding: Blinding{}, AllowJSX: true, MinTokens: 38,
			CollectImports: true, ImportMode: ParseModeDetailed, IgnoreTypeImports: true,
		}, dst)
	}
}
