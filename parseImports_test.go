package main

import (
	"os"
	"strings"
	"testing"
)

func importKindToString(kind ImportKind) string {
	if kind == OnlyTypeImport {
		return "only-type"
	}
	if kind == NotTypeOrMixedImport {
		return "not-type-or-mixed"
	}

	return "unknown"
}

func importsArrToString(imports []Import) string {
	str := ""

	for _, imp := range imports {
		str = str + importKindToString(imp.Kind) + "(" + imp.Request + ")" + "\n"
	}

	return str

}

func codeToString(code string) string {
	str := "\n\n"

	lines := strings.Split(code, "\n")

	for _, line := range lines {
		str += strings.TrimSpace(line) + "\n"
	}

	return str + "\n\n"
}

// No Type Imports

func TestParseImportWithoutIdentifier(t *testing.T) {
	code := `import './module'`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "./module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultImportSingleQuote(t *testing.T) {
	code := `import I from 'module'`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultImportDoubleQuote(t *testing.T) {
	code := `import I from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultImportRelativeModule(t *testing.T) {
	code := `import I from "../module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "../module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultImportMultiline(t *testing.T) {
	code := `import 
	I from 
	"module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleDefaultImports(t *testing.T) {
	code := `import A from "moduleA"
	import B from "moduleB"
	import C from "moduleC"`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == NotTypeOrMixedImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedImportSingleQuote(t *testing.T) {
	code := `import { I } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedImportDoubleQuote(t *testing.T) {
	code := `import { I } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedImportMultiline(t *testing.T) {
	code := `import { 
	I }
	       from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedImportWithMultipleIds(t *testing.T) {
	code := `import { A,B,
	someId } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleNamedImports(t *testing.T) {
	code := `import {A
	} from "moduleA"
	import {  B } from "moduleB"
	import {C} from 
	"moduleC"
	
	someDummyCode`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == NotTypeOrMixedImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultNamedImport(t *testing.T) {
	code := `import Default, { Named } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultNamedImportMultiline(t *testing.T) {
	code := `import Default
		, { 
	Named    }
	 from 
	 
	 "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

// Type imports

func TestParseDefaultTypeImportSingleQuote(t *testing.T) {
	code := `import type I from 'module'`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultTypeImportDoubleQuote(t *testing.T) {
	code := `import type I from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultTypeImportMultiline(t *testing.T) {
	code := `import 
	type 
	I from 
	"module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleDefaultTypeImports(t *testing.T) {
	code := `import type A from "moduleA"
	import type B from "moduleB"
	import type C from "moduleC"`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == OnlyTypeImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedTypeImportSingleQuote(t *testing.T) {
	code := `import type { I } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedTypeImportDoubleQuote(t *testing.T) {
	code := `import type { I } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseNamedTypeImportMultiline(t *testing.T) {
	code := `import type 
	{ 
	I }
	       from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleNamedTypeImports(t *testing.T) {
	code := `import type {A
	} from "moduleA"
	import type {  B } from "moduleB"
	import type{C} from 
	"moduleC"
	
	someDummyCode`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == OnlyTypeImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultNamedTypeImport(t *testing.T) {
	code := `import type Default, { Named } from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDefaultNamedTypeImportMultiline(t *testing.T) {
	code := `import type Default
		, { 
	Named    }
	 from 
	 
	 "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

// Dynamic and Require

func TestParseDynamicImport(t *testing.T) {
	code := `import("module")`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseRegularImportAndNestedDynamicImport(t *testing.T) {
	code := `
	  import dynamic from 'next/dynamic'

		export const Component = dynamic(
			() => import('./Component'),
		);
`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Parse invalid %s -> length %d, should be 2`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "next/dynamic" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}

	if imports[1].Request != "./Component" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDynamicImportAfterExportTypeStatement(t *testing.T) {
	code := `
		export type SomeType = A & {
			prop: boolean;
		};

		const Component = dynamic(
			() => import('@/aliased/import'),
		);
	`
	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "@/aliased/import" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseDynamicImportMultiline(t *testing.T) {
	code := `import
	(
	"module"
	)`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleDynamicImport(t *testing.T) {
	code := `import("moduleA");
	import("moduleB"); import('moduleC')`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == NotTypeOrMixedImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseRequireImport(t *testing.T) {
	code := `require("module")`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseImportAndThenRequireImport(t *testing.T) {
	code := `import {M} from 'module'; require("module2")`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Parse invalid %s -> length %d, should be 2`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}

	if imports[1].Request != "module2" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseRequireImportMultiline(t *testing.T) {
	code := `require
	(
	"module"
	)`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleRequireImport(t *testing.T) {
	code := `require("moduleA");
	require("moduleB"); require('moduleC')`

	imports := ParseImportsForTests(code)

	if len(imports) != 3 {
		t.Errorf(`Parse invalid %s -> length %d, should be 3`, codeToString(code), len(imports))
		return
	}

	isOk := true

	for _, imp := range imports {
		isOk = isOk && imp.Kind == NotTypeOrMixedImport
		isOk = isOk && (imp.Request == "moduleA" || imp.Request == "moduleB" || imp.Request == "moduleC")
	}

	if !isOk {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseMultipleDifferentImports(t *testing.T) {
	code := `
	import("moduleA");
	import B from "@/module/file/"
	import type C from 'file'
	import {D, E} from '../../relative'
	import type {
	F}  from '/absolute'
	require("asdasd")
	import DEf, {A}  from 'asd'
	import * as G from 'asdsd'
	`

	imports := ParseImportsForTests(code)

	if len(imports) != 8 {
		t.Errorf(`Parse invalid %s -> length %d, should be 8`, codeToString(code), len(imports))
		return
	}
}

func TestParseNamedExportFromMultiline(t *testing.T) {
	code := `export { 
	I }
	       from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseExportAllFromAs(t *testing.T) {
	code := `export * 
	
	as
	S 
	       from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestParseExportAllFrom(t *testing.T) {
	code := `export 
	
	* 

	       from "module"`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestShouldIgnoreLineCommentAndParseCorrectly(t *testing.T) {
	code := `// eslint-disable-next-line simple-import-sort/imports
	import '@/path/to/file';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "@/path/to/file" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestShouldIgnoreMultilineCommentAndParseCorrectly(t *testing.T) {
	code := `/* eslint-disable-next-line simple-import-sort/imports

	asd */
	import '@/path/to/file';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "@/path/to/file" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestShouldNotFindMultilineCommentStartInString(t *testing.T) {
	code := `'**/*someFile'

	import '@/path/to/file';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "@/path/to/file" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestImportSyntaxParsingOneByOne(t *testing.T) {
	examples := []struct {
		code string
		kind ImportKind
	}{
		{code: `export * from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export * as name1 from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export { name1,  nameN } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export { import1 as name1, import2 as name2,  nameN } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export { default } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export { default as name1 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `export { type MyType } from "./types"`, kind: OnlyTypeImport},
		{code: `export type { MyType } from "./types"`, kind: OnlyTypeImport},
		{code: `export { default, type MyType2 } from "./types"`, kind: NotTypeOrMixedImport},
		{code: `import defaultExport from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import * as name from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { export1 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { export1 as alias1 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { default as alias } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { export1, export2 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { export1, export2 as alias2 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { "string name" as alias } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import defaultExport, { export1 } from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import defaultExport, * as name from "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import "module-name"`, kind: NotTypeOrMixedImport},
		{code: `import { type MyType2 } from "./types"`, kind: OnlyTypeImport},
		{code: `import { type MyType1, type MyType2, type MyType3 } from "./types"`, kind: OnlyTypeImport},
		{code: `import type {  MyType2 } from "./types"`, kind: OnlyTypeImport},
		{code: `import fnA, { type MyType3 } from "./types"`, kind: NotTypeOrMixedImport},
		{code: `import fnA, { type MyType3, MyVal } from "./types"`, kind: NotTypeOrMixedImport},
		{code: `import { type MyType3, MyVal, type MyType2 } from "./types"`, kind: NotTypeOrMixedImport},
		{code: `import { MyType3, MyVal, type MyType2 } from "./types"`, kind: NotTypeOrMixedImport},
		{code: `import { MyType3, type MyVal, MyType2 } from "./types"`, kind: NotTypeOrMixedImport},
	}

	notParsed := []int{}
	wrongType := []int{}

	for idx, example := range examples {

		imports := ParseImportsForTests(example.code)

		if len(imports) != 1 {
			notParsed = append(notParsed, idx)
		} else if imports[0].Kind != example.kind {
			wrongType = append(wrongType, idx)
		}
	}

	for _, idx := range notParsed {
		t.Errorf(`Import syntax not supported "%s"`, examples[idx].code)
	}
	for _, idx := range wrongType {
		t.Errorf(`Wrong import type for "%s"`, examples[idx].code)
	}

}

func TestNotReexportingExportSyntax(t *testing.T) {
	examples := []struct {
		code string
	}{
		{code: `export default Variable`},
		{code: `export default function Some(){}`},
		{code: `export const Variable = 'value'`},
		{code: `export type SomeType = {}`},
		{code: `export interface SomeType {}`},
		{code: `export async function SomeFn() {}`},
		{code: `export function SomeFn() {}`},
		{code: `export { Variable }`},
		{code: `export { Variable, Variable2 }`},
	}

	parsed := []int{}

	for idx, example := range examples {

		imports := ParseImportsForTests(example.code)

		if len(imports) > 0 {
			parsed = append(parsed, idx)
		}
	}

	for _, idx := range parsed {
		t.Errorf(`Import syntax parsed but it should not be "%s"`, examples[idx].code)
	}

}

func TestParseCorrectlyAfterReexport(t *testing.T) {
	examples := []struct {
		code string
		kind ImportKind
	}{
		{code: `export default Variable;            export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export default function Some(){};   export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export const Variable = 'value';    export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export type SomeType = {};          export { Id } from "module"`, kind: NotTypeOrMixedImport},
		{code: `export interface SomeType {};       export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export async function SomeFn() {};  export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export function SomeFn() {};        export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export { Variable };                export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export { Variable, Variable2 };     export type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export { Variable };                import type { Id } from "module"`, kind: OnlyTypeImport},
		{code: `export type SomeType = {};          import { Id } from "module"`, kind: NotTypeOrMixedImport},
		{code: `export type Variable = {};          require("module")`, kind: NotTypeOrMixedImport},
		{code: `export { Variable };                import("module")`, kind: NotTypeOrMixedImport},
	}

	incorrect := []int{}

	for idx, example := range examples {

		imports := ParseImportsForTests(example.code)

		if len(imports) == 0 || len(imports) > 1 || imports[0].Kind != example.kind {
			incorrect = append(incorrect, idx)
		}
	}

	for _, idx := range incorrect {
		t.Errorf("Import syntax parsed incorrectly \n'%s' \ntype %s", examples[idx].code, importKindToString(examples[idx].kind))
	}
}

func TestParseVariableWithKeywordInName(t *testing.T) {
	code := `
		const exportArray = [
		'server/index.ts', // we should be careful with globals import
		"someFile.ts"
	  ]
	`
	imports := ParseImportsForTests(code)

	if len(imports) > 0 {
		t.Errorf("Incorrectly parsed import: %v", imports)
	}
}

func TestParseExportFunction(t *testing.T) {
	code := `
		export function Something() {
			const value = 'from'
			const someVar = 'asd'
		}
	`
	imports := ParseImportsForTests(code)

	if len(imports) > 0 {
		t.Errorf("Incorrectly parsed import: %v", imports)
	}
}

func TestParseRequireFollowedByVarWithImportInName(t *testing.T) {
	code := `import_recoil = __toModule(require('recoil'));`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 || imports[0].Request != "recoil" {
		t.Errorf("Incorrectly parsed require stmt: from '%v'", code)
	}
}
func TestShouldNotParseNonStaticImportSource(t *testing.T) {
	t.Run("Should not parse require with dynamic path starting with string", func(t *testing.T) {
		code := `require('path' + variable + 'some-string.js')`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse require with dynamic path starting with variable", func(t *testing.T) {
		code := `require(variable + 'some-string.js')`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse import with dynamic path starting with string", func(t *testing.T) {
		code := `import('path' + variable + 'some-string.js')`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse import with dynamic path starting with variable", func(t *testing.T) {
		code := `import(variable + 'some-string.js')`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse require with dynamic path wrapped with brackets", func(t *testing.T) {
		code := `require((('path' + variable + 'some-string.js')))`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse import with dynamic path wrapped with brackets", func(t *testing.T) {
		code := `import((('path' + variable + 'some-string.js')))`

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse require with template literal path", func(t *testing.T) {
		code := "require(`somePath`)"

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})

	t.Run("Should not parse import with template literal path", func(t *testing.T) {
		code := "import(`somePath`)"

		imports := ParseImportsForTests(code)

		if len(imports) != 0 {
			t.Errorf("Should not parse import: from '%v'", code)
		}
	})
}

func TestShouldParseRequireSourceWrappedWithBrackets(t *testing.T) {
	code := `require((('path')))`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 || imports[0].Request != "path" {
		t.Errorf("Should parse import: from '%v'", code)
	}
}

func TestShouldParseDynamicImportSourceWrappedWithBrackets(t *testing.T) {
	code := `import((('path')))`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 || imports[0].Request != "path" {
		t.Errorf("Should parse import: from '%v'", code)
	}
}

func TestShouldNotProcessCommentedCodeAfterExportKeyword(t *testing.T) {
	code := `export class MyClass {
		// asd from 'error'
	}
  `
	imports := ParseImportsForTests(code)

	if len(imports) > 0 {
		t.Errorf("Should not parse import: from '%v', got '%s' import", code, imports[0].Request)
	}

	code2 := `export class MyClass {
		/*
		 asd from 'error'
		*/
	}
  `
	imports2 := ParseImportsForTests(code2)

	if len(imports2) > 0 {
		t.Errorf("Should not parse import: from '%v', got '%s' import", code2, imports2[0].Request)
	}

	// Should parse `export * from "path"`
	code3 := `export 
		/*
		 asd from 'error'
		*/
		* from "path"
	
  `
	imports3 := ParseImportsForTests(code3)

	if len(imports3) != 1 || imports3[0].Request != "path" {
		t.Errorf("Should parse import: from '%v'", code3)
	}
}

func TestShouldNotProcessCommentedCodeAfterImportKeyword(t *testing.T) {
	// Test case for import vulnerability
	code := `import {
		something
		// from 'error'
	} from 'actual'`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 || imports[0].Request != "actual" {
		t.Errorf("Should only parse 'actual' import, got: %v", imports)
	}

	code2 := `import {
		something
		/*
		 from 'error'
		*/
	} from 'actual2'`

	imports2 := ParseImportsForTests(code2)

	if len(imports2) != 1 || imports2[0].Request != "actual2" {
		t.Errorf("Should only parse 'actual2' import, got: %v", imports2)
	}
}

func BenchmarkParseImportsWithTypes(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports)
	}
}

func BenchmarkParseImportsWithoutTypes(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports)
	}
}

func BenchmarkParseImportsWithTypes600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports)
	}
}

func BenchmarkParseImportsWithoutTypes600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports)
	}
}
