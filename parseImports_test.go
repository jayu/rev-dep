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

func TestImportSourceLocationTracking(t *testing.T) {
	t.Run("Static import with double quotes", func(t *testing.T) {
		code := `import a from "source"`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `import a from "source"`, the string "source" content starts at index 15 and ends at index 21
		// (excluding the closing quote at index 21)
		if imports[0].RequestStart != 15 || imports[0].RequestEnd != 21 {
			t.Errorf("Expected start=15, end=21, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Static import with single quotes", func(t *testing.T) {
		code := `import a from 'source'`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `import a from 'source'`, the string 'source' content starts at index 15 and ends at index 21
		if imports[0].RequestStart != 15 || imports[0].RequestEnd != 21 {
			t.Errorf("Expected start=15, end=21, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Dynamic import", func(t *testing.T) {
		code := `import("source")`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `import("source")`, the string "source" content starts at index 8 and ends at index 14
		if imports[0].RequestStart != 8 || imports[0].RequestEnd != 14 {
			t.Errorf("Expected start=8, end=14, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Dynamic import with expression", func(t *testing.T) {
		code := `import((('module')))`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `import((('module')))`, the string "module" content starts at index 10 and ends at index 16
		if imports[0].RequestStart != 10 || imports[0].RequestEnd != 16 {
			t.Errorf("Expected start=10, end=16, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Require import", func(t *testing.T) {
		code := `require("source")`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `require("source")`, the string "source" content starts at index 9 and ends at index 15
		if imports[0].RequestStart != 9 || imports[0].RequestEnd != 15 {
			t.Errorf("Expected start=9, end=15, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Export from", func(t *testing.T) {
		code := `export { a } from "source"`
		imports := ParseImportsForTests(code)

		if len(imports) != 1 {
			t.Fatalf("Expected 1 import, got %d", len(imports))
		}

		// In `export { a } from "source"`, the string "source" content starts at index 19 and ends at index 25
		if imports[0].RequestStart != 19 || imports[0].RequestEnd != 25 {
			t.Errorf("Expected start=19, end=25, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}
	})

	t.Run("Multiple imports with different locations", func(t *testing.T) {
		code := `import "first"; import "second"`
		imports := ParseImportsForTests(code)

		if len(imports) != 2 {
			t.Fatalf("Expected 2 imports, got %d", len(imports))
		}

		// First import "first" content starts at index 8, ends at index 13
		if imports[0].RequestStart != 8 || imports[0].RequestEnd != 13 {
			t.Errorf("First import: Expected start=8, end=13, got start=%d, end=%d", imports[0].RequestStart, imports[0].RequestEnd)
		}

		// Second import "second" content starts at index 24, ends at index 30
		if imports[1].RequestStart != 24 || imports[1].RequestEnd != 30 {
			t.Errorf("Second import: Expected start=24, end=30, got start=%d, end=%d", imports[1].RequestStart, imports[1].RequestEnd)
		}
	})
}

func BenchmarkParseImportsWithTypes(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports, ParseModeBasic)
	}
}

func BenchmarkParseImportsWithoutTypes(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports, ParseModeBasic)
	}
}

func BenchmarkParseImportsWithTypes600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports, ParseModeBasic)
	}
}

func BenchmarkParseImportsWithoutTypes600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")
	ignoreTypeImports := true

	for b.Loop() {
		ParseImportsByte(fixtureContent, ignoreTypeImports, ParseModeBasic)
	}
}

// ==================== Detailed Mode Tests ====================

// --- Import Keyword Extraction ---

func TestDetailedImportDefault(t *testing.T) {
	code := `import Default from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw == nil || kw.Len() != 1 {
		t.Fatalf("Expected 1 keyword, got %v", kw)
	}
	k := kw.Keywords[0]
	if k.Name != "default" || k.Alias != "Default" || k.IsType || k.Position != 0 {
		t.Errorf("Unexpected keyword: %+v", k)
	}
}

func TestDetailedImportNamespace(t *testing.T) {
	code := `import * as Ns from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw == nil || kw.Len() != 1 {
		t.Fatalf("Expected 1 keyword, got %v", kw)
	}
	k := kw.Keywords[0]
	if k.Name != "*" || k.Alias != "Ns" || k.IsType {
		t.Errorf("Unexpected keyword: %+v", k)
	}
}

func TestDetailedImportNamedSingle(t *testing.T) {
	code := `import { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw == nil || kw.Len() != 1 {
		t.Fatalf("Expected 1 keyword, got %v", kw)
	}
	k := kw.Keywords[0]
	if k.Name != "A" || k.Alias != "" || k.IsType {
		t.Errorf("Unexpected keyword: %+v", k)
	}
}

func TestDetailedImportNamedAlias(t *testing.T) {
	code := `import { A as B } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || k.Alias != "B" {
		t.Errorf("Unexpected keyword: %+v", k)
	}
}

func TestDetailedImportNamedMultiple(t *testing.T) {
	code := `import { A, B, C } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 3 {
		t.Fatalf("Expected 3 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "A" || kw.Keywords[1].Name != "B" || kw.Keywords[2].Name != "C" {
		t.Errorf("Unexpected keywords: %+v", kw.Keywords)
	}
}

func TestDetailedImportInlineType(t *testing.T) {
	code := `import { type A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || !k.IsType {
		t.Errorf("Expected type keyword A, got: %+v", k)
	}
}

func TestDetailedImportInlineTypeMixed(t *testing.T) {
	code := `import { type A, B } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "A" || !kw.Keywords[0].IsType {
		t.Errorf("Expected type A, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "B" || kw.Keywords[1].IsType {
		t.Errorf("Expected non-type B, got: %+v", kw.Keywords[1])
	}
}

func TestDetailedImportTypeStatement(t *testing.T) {
	code := `import type { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || !k.IsType {
		t.Errorf("Expected type keyword A, got: %+v", k)
	}
}

func TestDetailedImportDefaultAndNamed(t *testing.T) {
	code := `import Default, { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "default" || kw.Keywords[0].Alias != "Default" {
		t.Errorf("Expected default import, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "A" {
		t.Errorf("Expected named import A, got: %+v", kw.Keywords[1])
	}
}

func TestDetailedImportDefaultAndNamespace(t *testing.T) {
	code := `import Default, * as Ns from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "default" || kw.Keywords[0].Alias != "Default" {
		t.Errorf("Expected default import, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "*" || kw.Keywords[1].Alias != "Ns" {
		t.Errorf("Expected namespace import, got: %+v", kw.Keywords[1])
	}
}

func TestDetailedImportDefaultAsAlias(t *testing.T) {
	code := `import { default as A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "default" || k.Alias != "A" {
		t.Errorf("Expected default as A, got: %+v", k)
	}
}

func TestDetailedImportStringName(t *testing.T) {
	code := `import { "string name" as A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "string name" || k.Alias != "A" {
		t.Errorf("Expected string name with alias, got: %+v", k)
	}
}

func TestDetailedImportSideEffect(t *testing.T) {
	code := `import "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords != nil {
		t.Errorf("Expected nil keywords for side-effect import, got: %+v", imports[0].Keywords)
	}
}

func TestDetailedImportDynamic(t *testing.T) {
	code := `import("mod")`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords != nil {
		t.Errorf("Expected nil keywords for dynamic import, got: %+v", imports[0].Keywords)
	}
}

func TestDetailedRequire(t *testing.T) {
	code := `require("mod")`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords != nil {
		t.Errorf("Expected nil keywords for require, got: %+v", imports[0].Keywords)
	}
}

// --- Export Keyword Extraction (Re-exports) ---

func TestDetailedExportNamedReexport(t *testing.T) {
	code := `export { A, B } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw == nil || kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %v", kw)
	}
	if kw.Keywords[0].Name != "A" || kw.Keywords[1].Name != "B" {
		t.Errorf("Unexpected keywords: %+v", kw.Keywords)
	}
}

func TestDetailedExportNamedAliasReexport(t *testing.T) {
	code := `export { A as B } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || k.Alias != "B" {
		t.Errorf("Expected A as B, got: %+v", k)
	}
}

func TestDetailedExportStarReexport(t *testing.T) {
	code := `export * from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "*" || k.Alias != "" {
		t.Errorf("Expected star export, got: %+v", k)
	}
}

func TestDetailedExportStarAsReexport(t *testing.T) {
	code := `export * as Ns from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "*" || k.Alias != "Ns" {
		t.Errorf("Expected * as Ns, got: %+v", k)
	}
}

func TestDetailedExportTypeReexport(t *testing.T) {
	code := `export type { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || !k.IsType {
		t.Errorf("Expected type export A, got: %+v", k)
	}
}

func TestDetailedExportInlineTypeReexport(t *testing.T) {
	code := `export { type A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "A" || !k.IsType {
		t.Errorf("Expected type export A, got: %+v", k)
	}
}

func TestDetailedExportDefaultReexport(t *testing.T) {
	code := `export { default } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "default" {
		t.Errorf("Expected default reexport, got: %+v", k)
	}
}

func TestDetailedExportDefaultAndTypeReexport(t *testing.T) {
	code := `export { default, type A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "default" {
		t.Errorf("Expected default, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "A" || !kw.Keywords[1].IsType {
		t.Errorf("Expected type A, got: %+v", kw.Keywords[1])
	}
}

// --- Local Export Keyword Extraction ---

func TestDetailedExportDefaultLocal(t *testing.T) {
	code := `export default Variable`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	imp := imports[0]
	if !imp.IsLocalExport {
		t.Fatal("Expected IsLocalExport=true")
	}
	if imp.Keywords.Keywords[0].Name != "default" {
		t.Errorf("Expected default keyword, got: %+v", imp.Keywords.Keywords[0])
	}
}

func TestDetailedExportDefaultFunction(t *testing.T) {
	code := `export default function Fn(){}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "default" {
		t.Errorf("Expected default, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportConst(t *testing.T) {
	code := `export const Var = 'val'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "Var" {
		t.Errorf("Expected Var, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportLet(t *testing.T) {
	code := `export let Var = 'val'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "Var" {
		t.Errorf("Expected Var, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportFunction(t *testing.T) {
	code := `export function Fn(){}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "Fn" {
		t.Errorf("Expected Fn, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportAsyncFunction(t *testing.T) {
	code := `export async function Fn(){}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "Fn" {
		t.Errorf("Expected Fn, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportClass(t *testing.T) {
	code := `export class Cls {}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords.Keywords[0].Name != "Cls" {
		t.Errorf("Expected Cls, got: %+v", imports[0].Keywords.Keywords[0])
	}
}

func TestDetailedExportTypeLocal(t *testing.T) {
	code := `export type T = {}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "T" || !k.IsType {
		t.Errorf("Expected type T, got: %+v", k)
	}
}

func TestDetailedExportInterface(t *testing.T) {
	code := `export interface I {}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "I" || !k.IsType {
		t.Errorf("Expected type I, got: %+v", k)
	}
}

func TestDetailedExportEnum(t *testing.T) {
	code := `export enum E {}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	k := imports[0].Keywords.Keywords[0]
	if k.Name != "E" || !k.IsType {
		t.Errorf("Expected type E, got: %+v", k)
	}
}

func TestDetailedExportLocalNamedList(t *testing.T) {
	code := `export { A, B }`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if !imports[0].IsLocalExport {
		t.Fatal("Expected IsLocalExport=true")
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "A" || kw.Keywords[1].Name != "B" {
		t.Errorf("Unexpected keywords: %+v", kw.Keywords)
	}
}

func TestDetailedExportLocalNamedWithType(t *testing.T) {
	code := `export { A, type B }`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Keywords[0].Name != "A" || kw.Keywords[0].IsType {
		t.Errorf("Expected A non-type, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "B" || !kw.Keywords[1].IsType {
		t.Errorf("Expected type B, got: %+v", kw.Keywords[1])
	}
}

// --- ExportKeyStart/ExportKeyEnd accuracy ---

func TestDetailedExportKeyStartEnd(t *testing.T) {
	code := `export { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].ExportKeyStart != 0 {
		t.Errorf("Expected ExportKeyStart=0, got %d", imports[0].ExportKeyStart)
	}
	if imports[0].ExportKeyEnd != 7 {
		t.Errorf("Expected ExportKeyEnd=7, got %d", imports[0].ExportKeyEnd)
	}
}

func TestDetailedLocalExportKeyStartEnd(t *testing.T) {
	code := `export const Var = 1`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].ExportKeyStart != 0 {
		t.Errorf("Expected ExportKeyStart=0, got %d", imports[0].ExportKeyStart)
	}
	if imports[0].ExportKeyEnd != 7 {
		t.Errorf("Expected ExportKeyEnd=7, got %d", imports[0].ExportKeyEnd)
	}
}

func TestDetailedExportKeyStartEndWithLeadingCode(t *testing.T) {
	code := `const x = 1; export const Var = 1`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].ExportKeyStart != 13 {
		t.Errorf("Expected ExportKeyStart=13, got %d", imports[0].ExportKeyStart)
	}
}

// --- Multiline ---

func TestDetailedImportMultiline(t *testing.T) {
	code := `import {
	A,
	B as C,
	type D
} from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 3 {
		t.Fatalf("Expected 3 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "A" {
		t.Errorf("Expected A, got: %+v", kw.Keywords[0])
	}
	if kw.Keywords[1].Name != "B" || kw.Keywords[1].Alias != "C" {
		t.Errorf("Expected B as C, got: %+v", kw.Keywords[1])
	}
	if kw.Keywords[2].Name != "D" || !kw.Keywords[2].IsType {
		t.Errorf("Expected type D, got: %+v", kw.Keywords[2])
	}
}

func TestDetailedExportMultilineReexport(t *testing.T) {
	code := `export {
	A,
	B as C
} from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
}

// --- Comments within statements ---

func TestDetailedImportWithLineComment(t *testing.T) {
	code := `import {
	A, // comment
	B
} from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].Name != "A" || kw.Keywords[1].Name != "B" {
		t.Errorf("Unexpected keywords: %+v", kw.Keywords)
	}
}

func TestDetailedImportWithBlockComment(t *testing.T) {
	code := `import { A, /* some comment */ B } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
}

// --- Mixed statements ---

func TestDetailedMixedStatements(t *testing.T) {
	code := `import A from "modA"
export { B } from "modB"
export const C = 1
import { D, type E } from "modD"
require("modR")`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 5 {
		t.Fatalf("Expected 5 imports, got %d", len(imports))
	}
	// import A
	if imports[0].Keywords == nil || imports[0].Keywords.Keywords[0].Name != "default" {
		t.Errorf("Import 0: expected default import A")
	}
	// export { B } from "modB"
	if imports[1].Keywords == nil || imports[1].Keywords.Keywords[0].Name != "B" {
		t.Errorf("Import 1: expected export B")
	}
	// export const C
	if !imports[2].IsLocalExport || imports[2].Keywords.Keywords[0].Name != "C" {
		t.Errorf("Import 2: expected local export C")
	}
	// import { D, type E }
	if imports[3].Keywords == nil || imports[3].Keywords.Len() != 2 {
		t.Errorf("Import 3: expected 2 keywords")
	}
	// require("modR")
	if imports[4].Keywords != nil {
		t.Errorf("Import 4: expected nil keywords for require")
	}
}

// --- Edge Cases ---

func TestDetailedTrailingComma(t *testing.T) {
	code := `import { A, B, } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
}

func TestDetailedEmptyBraces(t *testing.T) {
	code := `import {} from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Empty braces should produce Keywords with 0 entries
	if imports[0].Keywords != nil && imports[0].Keywords.Len() != 0 {
		t.Errorf("Expected 0 keywords for empty braces, got %d", imports[0].Keywords.Len())
	}
}

// --- Basic mode unchanged ---

func TestBasicModeNilKeywords(t *testing.T) {
	code := `import { A } from "mod"`
	imports := ParseImportsForTests(code) // Uses ParseModeBasic
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Keywords != nil {
		t.Errorf("Expected nil keywords in basic mode, got: %+v", imports[0].Keywords)
	}
}

func TestBasicModeNoLocalExports(t *testing.T) {
	code := `export const Var = 1`
	imports := ParseImportsForTests(code) // Uses ParseModeBasic
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports in basic mode for local export, got %d", len(imports))
	}
}

// --- KeywordMap.Get() ---

func TestKeywordMapGet(t *testing.T) {
	code := `import { A, B as C } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	kw := imports[0].Keywords

	a, ok := kw.Get("A")
	if !ok || a.Name != "A" {
		t.Errorf("Get(A) failed: %+v, ok=%v", a, ok)
	}

	b, ok := kw.Get("B")
	if !ok || b.Name != "B" || b.Alias != "C" {
		t.Errorf("Get(B) failed: %+v, ok=%v", b, ok)
	}

	_, ok = kw.Get("Z")
	if ok {
		t.Error("Get(Z) should return false")
	}
}

// --- Table-driven comprehensive test ---

func TestDetailedKeywordsComprehensive(t *testing.T) {
	type expectedKw struct {
		Name   string
		Alias  string
		IsType bool
	}
	tests := []struct {
		name          string
		code          string
		wantCount     int
		wantRequest   string
		isLocalExport bool
		keywords      []expectedKw
	}{
		// Imports
		{
			name: "default import", code: `import Default from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "default", Alias: "Default"}},
		},
		{
			name: "namespace import", code: `import * as name from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "*", Alias: "name"}},
		},
		{
			name: "single named import", code: `import { export1 } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "export1"}},
		},
		{
			name: "named import with alias", code: `import { export1 as alias1 } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "export1", Alias: "alias1"}},
		},
		{
			name: "multiple named imports", code: `import { export1, export2 } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "export1"}, {Name: "export2"}},
		},
		{
			name: "named import mixed alias", code: `import { export1, export2 as alias2 } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "export1"}, {Name: "export2", Alias: "alias2"}},
		},
		{
			name: "default and named import", code: `import defaultExport, { export1 } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "default", Alias: "defaultExport"}, {Name: "export1"}},
		},
		{
			name: "default and namespace import", code: `import defaultExport, * as name from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "default", Alias: "defaultExport"}, {Name: "*", Alias: "name"}},
		},
		{
			name: "inline type imports", code: `import { type MyType1, type MyType2, type MyType3 } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{
				{Name: "MyType1", IsType: true},
				{Name: "MyType2", IsType: true},
				{Name: "MyType3", IsType: true},
			},
		},
		{
			name: "mixed type and value imports", code: `import { type MyType3, MyVal, type MyType2 } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{
				{Name: "MyType3", IsType: true},
				{Name: "MyVal"},
				{Name: "MyType2", IsType: true},
			},
		},
		{
			name: "import type statement", code: `import type { MyType2 } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{{Name: "MyType2", IsType: true}},
		},
		{
			name: "default import with inline type", code: `import fnA, { type MyType3 } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{{Name: "default", Alias: "fnA"}, {Name: "MyType3", IsType: true}},
		},
		// Re-exports
		{
			name: "export star", code: `export * from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "*"}},
		},
		{
			name: "export star as", code: `export * as name1 from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "*", Alias: "name1"}},
		},
		{
			name: "export named", code: `export { name1, nameN } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "name1"}, {Name: "nameN"}},
		},
		{
			name: "export named with alias", code: `export { import1 as name1, import2 as name2, nameN } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "import1", Alias: "name1"}, {Name: "import2", Alias: "name2"}, {Name: "nameN"}},
		},
		{
			name: "export default reexport", code: `export { default } from "module-name"`,
			wantCount: 1, wantRequest: "module-name",
			keywords: []expectedKw{{Name: "default"}},
		},
		{
			name: "export type named reexport", code: `export type { MyType } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{{Name: "MyType", IsType: true}},
		},
		{
			name: "export inline type reexport", code: `export { type MyType } from "./types"`,
			wantCount: 1, wantRequest: "./types",
			keywords: []expectedKw{{Name: "MyType", IsType: true}},
		},
		// Local exports
		{
			name: "local export default", code: `export default Variable`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "default"}},
		},
		{
			name: "local export const", code: `export const Var = 'val'`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "Var"}},
		},
		{
			name: "local export function", code: `export function Fn(){}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "Fn"}},
		},
		{
			name: "local export async function", code: `export async function Fn(){}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "Fn"}},
		},
		{
			name: "local export class", code: `export class Cls {}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "Cls"}},
		},
		{
			name: "local export type", code: `export type T = {}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "T", IsType: true}},
		},
		{
			name: "local export interface", code: `export interface I {}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "I", IsType: true}},
		},
		{
			name: "local export enum", code: `export enum E {}`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "E", IsType: true}},
		},
		{
			name: "local export list", code: `export { A, B }`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "A"}, {Name: "B"}},
		},
		{
			name: "local export list with type", code: `export { A, type B }`,
			wantCount: 1, isLocalExport: true,
			keywords: []expectedKw{{Name: "A"}, {Name: "B", IsType: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imports := ParseImportsForTestsDetailed(tt.code)
			if len(imports) != tt.wantCount {
				t.Fatalf("Expected %d imports, got %d", tt.wantCount, len(imports))
			}
			imp := imports[0]

			if tt.wantRequest != "" && imp.Request != tt.wantRequest {
				t.Errorf("Expected request=%q, got %q", tt.wantRequest, imp.Request)
			}

			if tt.isLocalExport && !imp.IsLocalExport {
				t.Error("Expected IsLocalExport=true")
			}

			if tt.keywords != nil {
				if imp.Keywords == nil {
					t.Fatal("Expected keywords, got nil")
				}
				if imp.Keywords.Len() != len(tt.keywords) {
					t.Fatalf("Expected %d keywords, got %d", len(tt.keywords), imp.Keywords.Len())
				}
				for j, want := range tt.keywords {
					got := imp.Keywords.Keywords[j]
					if got.Name != want.Name {
						t.Errorf("Keyword %d: expected Name=%q, got %q", j, want.Name, got.Name)
					}
					if got.Alias != want.Alias {
						t.Errorf("Keyword %d: expected Alias=%q, got %q", j, want.Alias, got.Alias)
					}
					if got.IsType != want.IsType {
						t.Errorf("Keyword %d: expected IsType=%v, got %v", j, want.IsType, got.IsType)
					}
				}
			}
		})
	}
}

// --- Keyword Start/End position tests ---

func TestDetailedKeywordPositions(t *testing.T) {
	code := `import { A } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	k := imports[0].Keywords.Keywords[0]
	// "A" starts at index 9 and ends at 10
	if code[int(k.Start):int(k.End)] != "A" {
		t.Errorf("Expected source slice 'A', got %q (start=%d, end=%d)", code[int(k.Start):int(k.End)], k.Start, k.End)
	}
}

func TestDetailedKeywordPositionsMultiple(t *testing.T) {
	code := `import { Foo, Bar as Baz } from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	kw := imports[0].Keywords

	// Foo
	if code[int(kw.Keywords[0].Start):int(kw.Keywords[0].End)] != "Foo" {
		t.Errorf("Expected 'Foo', got %q", code[int(kw.Keywords[0].Start):int(kw.Keywords[0].End)])
	}
	// Bar as Baz - Start at 'Bar', End at end of 'Baz'
	slice := code[int(kw.Keywords[1].Start):int(kw.Keywords[1].End)]
	if slice != "Bar as Baz" {
		t.Errorf("Expected 'Bar as Baz', got %q", slice)
	}
}

func TestDetailedKeywordPositionsDefault(t *testing.T) {
	code := `import MyDefault from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	k := imports[0].Keywords.Keywords[0]
	if code[int(k.Start):int(k.End)] != "MyDefault" {
		t.Errorf("Expected 'MyDefault', got %q", code[int(k.Start):int(k.End)])
	}
}

func TestDetailedKeywordPositionsNamespace(t *testing.T) {
	code := `import * as Ns from "mod"`
	imports := ParseImportsForTestsDetailed(code)
	k := imports[0].Keywords.Keywords[0]
	slice := code[int(k.Start):int(k.End)]
	if slice != "* as Ns" {
		t.Errorf("Expected '* as Ns', got %q", slice)
	}
}

// --- Benchmarks for Detailed mode ---

func BenchmarkParseImportsBasicMode(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")

	for b.Loop() {
		ParseImportsByte(fixtureContent, true, ParseModeBasic)
	}
}

func BenchmarkParseImportsDetailedMode(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports.ts")

	for b.Loop() {
		ParseImportsByte(fixtureContent, true, ParseModeDetailed)
	}
}

func BenchmarkParseImportsBasicMode600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")

	for b.Loop() {
		ParseImportsByte(fixtureContent, true, ParseModeBasic)
	}
}

func BenchmarkParseImportsDetailedMode600Loc(b *testing.B) {
	fixtureContent, _ := os.ReadFile("./__fixtures__/parseImports600Loc.ts")

	for b.Loop() {
		ParseImportsByte(fixtureContent, true, ParseModeDetailed)
	}
}

// --- ExportDeclStart tests ---

func TestDetailedExportDeclStart_Const(t *testing.T) {
	code := `export const X = 1`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// ExportDeclStart should point at `const`
	if imports[0].ExportDeclStart != 7 {
		t.Errorf("Expected ExportDeclStart=7, got %d", imports[0].ExportDeclStart)
	}
	if string(code[int(imports[0].ExportDeclStart):int(imports[0].ExportDeclStart)+5]) != "const" {
		t.Errorf("ExportDeclStart does not point at 'const': %q", string(code[int(imports[0].ExportDeclStart):int(imports[0].ExportDeclStart)+5]))
	}
}

func TestDetailedExportDeclStart_Function(t *testing.T) {
	code := `export function Fn(){}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].ExportDeclStart != 7 {
		t.Errorf("Expected ExportDeclStart=7, got %d", imports[0].ExportDeclStart)
	}
}

func TestDetailedExportDeclStart_DefaultFunction(t *testing.T) {
	code := `export default function Fn(){}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// ExportDeclStart should point at `function` (after `export default `)
	if imports[0].ExportDeclStart != 15 {
		t.Errorf("Expected ExportDeclStart=15, got %d", imports[0].ExportDeclStart)
	}
	if string(code[int(imports[0].ExportDeclStart):int(imports[0].ExportDeclStart)+8]) != "function" {
		t.Errorf("ExportDeclStart does not point at 'function': %q", string(code[int(imports[0].ExportDeclStart):int(imports[0].ExportDeclStart)+8]))
	}
}

func TestDetailedExportDeclStart_DefaultVar(t *testing.T) {
	code := `export default someVar`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// ExportDeclStart should point at `someVar`
	if imports[0].ExportDeclStart != 15 {
		t.Errorf("Expected ExportDeclStart=15, got %d", imports[0].ExportDeclStart)
	}
}

func TestDetailedExportDeclStart_Type(t *testing.T) {
	code := `export type T = {}`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// ExportDeclStart should point at `type`
	if imports[0].ExportDeclStart != 7 {
		t.Errorf("Expected ExportDeclStart=7, got %d", imports[0].ExportDeclStart)
	}
}

// --- ExportBraceStart/ExportBraceEnd tests ---

func TestDetailedExportBracePositions_LocalBraceExport(t *testing.T) {
	code := `export { A, B }`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// `{` is at position 7, `}` is at position 14, braceEnd = 15
	if imports[0].ExportBraceStart != 7 {
		t.Errorf("Expected ExportBraceStart=7, got %d", imports[0].ExportBraceStart)
	}
	if imports[0].ExportBraceEnd != 15 {
		t.Errorf("Expected ExportBraceEnd=15, got %d", imports[0].ExportBraceEnd)
	}
}

func TestDetailedExportBracePositions_ReexportBrace(t *testing.T) {
	code := `export { A } from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].ExportBraceStart != 7 {
		t.Errorf("Expected ExportBraceStart=7, got %d", imports[0].ExportBraceStart)
	}
	// `}` is at position 11, braceEnd = 12
	if imports[0].ExportBraceEnd != 12 {
		t.Errorf("Expected ExportBraceEnd=12, got %d", imports[0].ExportBraceEnd)
	}
}

func TestDetailedExportBracePositions_StarExport(t *testing.T) {
	code := `export * from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Star exports have no braces
	if imports[0].ExportBraceStart != 0 {
		t.Errorf("Expected ExportBraceStart=0, got %d", imports[0].ExportBraceStart)
	}
	if imports[0].ExportBraceEnd != 0 {
		t.Errorf("Expected ExportBraceEnd=0, got %d", imports[0].ExportBraceEnd)
	}
}

func TestDetailedExportBracePositions_SingleDecl(t *testing.T) {
	code := `export const X = 1`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Single declarations have no braces
	if imports[0].ExportBraceStart != 0 {
		t.Errorf("Expected ExportBraceStart=0, got %d", imports[0].ExportBraceStart)
	}
	if imports[0].ExportBraceEnd != 0 {
		t.Errorf("Expected ExportBraceEnd=0, got %d", imports[0].ExportBraceEnd)
	}
}

// --- ExportStatementEnd tests ---

func TestDetailedExportStatementEnd_ReexportNoSemicolon(t *testing.T) {
	code := `export { A } from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Statement ends after closing quote
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_ReexportWithSemicolon(t *testing.T) {
	code := `export { A } from './file';`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Statement ends after `;`
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_StarReexport(t *testing.T) {
	code := `export * from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_StarReexportWithSemicolon(t *testing.T) {
	code := `export * from './file';`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_LocalBraceExport(t *testing.T) {
	code := `export { A, B }`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_LocalBraceExportWithSemicolon(t *testing.T) {
	code := `export { A, B };`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if int(imports[0].ExportStatementEnd) != len(code) {
		t.Errorf("Expected ExportStatementEnd=%d, got %d", len(code), imports[0].ExportStatementEnd)
	}
}

func TestDetailedExportStatementEnd_SingleDeclIsZero(t *testing.T) {
	code := `export const X = 1`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	// Single declarations don't use ExportStatementEnd
	if imports[0].ExportStatementEnd != 0 {
		t.Errorf("Expected ExportStatementEnd=0 for single decl, got %d", imports[0].ExportStatementEnd)
	}
}

// --- CommaAfter tests ---

func TestDetailedCommaAfter_BraceListWithCommas(t *testing.T) {
	code := `export { A, B, C } from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 3 {
		t.Fatalf("Expected 3 keywords, got %d", kw.Len())
	}
	// A has comma after
	if kw.Keywords[0].CommaAfter == 0 {
		t.Error("Expected CommaAfter != 0 for A")
	}
	if code[int(kw.Keywords[0].CommaAfter)] != ',' {
		t.Errorf("CommaAfter for A does not point at ',': got %q", string(code[int(kw.Keywords[0].CommaAfter)]))
	}
	// B has comma after
	if kw.Keywords[1].CommaAfter == 0 {
		t.Error("Expected CommaAfter != 0 for B")
	}
	// C has no comma after (last keyword, no trailing comma)
	if kw.Keywords[2].CommaAfter != 0 {
		t.Errorf("Expected CommaAfter=0 for C (last, no trailing comma), got %d", kw.Keywords[2].CommaAfter)
	}
}

func TestDetailedCommaAfter_TrailingComma(t *testing.T) {
	code := `export { A, B, } from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	// Both A and B have commas after (trailing comma style)
	if kw.Keywords[0].CommaAfter == 0 {
		t.Error("Expected CommaAfter != 0 for A")
	}
	if kw.Keywords[1].CommaAfter == 0 {
		t.Error("Expected CommaAfter != 0 for B (trailing comma)")
	}
}

func TestDetailedCommaAfter_SingleItem(t *testing.T) {
	code := `export { A } from './file'`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 1 {
		t.Fatalf("Expected 1 keyword, got %d", kw.Len())
	}
	if kw.Keywords[0].CommaAfter != 0 {
		t.Errorf("Expected CommaAfter=0 for single item, got %d", kw.Keywords[0].CommaAfter)
	}
}

func TestDetailedCommaAfter_LocalBraceExport(t *testing.T) {
	code := `export { A, type B }`
	imports := ParseImportsForTestsDetailed(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	kw := imports[0].Keywords
	if kw.Len() != 2 {
		t.Fatalf("Expected 2 keywords, got %d", kw.Len())
	}
	if kw.Keywords[0].CommaAfter == 0 {
		t.Error("Expected CommaAfter != 0 for A")
	}
	if kw.Keywords[1].CommaAfter != 0 {
		t.Errorf("Expected CommaAfter=0 for B (last), got %d", kw.Keywords[1].CommaAfter)
	}
}

func TestDynamicImportFlag(t *testing.T) {
	code := `
import { helper } from './utils'
import './side-effect'
const Component = dynamic(() => import('./Component'))
const mod = require('./module')
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 4 {
		t.Fatalf("Expected 4 imports, got %d", len(imports))
	}

	// Static named import — not dynamic
	if imports[0].Request != "./utils" || imports[0].IsDynamicImport {
		t.Errorf("Expected static import './utils', got request='%s' dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}

	// Side-effect import — not dynamic
	if imports[1].Request != "./side-effect" || imports[1].IsDynamicImport {
		t.Errorf("Expected side-effect import './side-effect', got request='%s' dynamic=%v", imports[1].Request, imports[1].IsDynamicImport)
	}

	// Dynamic import() — IS dynamic
	if imports[2].Request != "./Component" || !imports[2].IsDynamicImport {
		t.Errorf("Expected dynamic import './Component', got request='%s' dynamic=%v", imports[2].Request, imports[2].IsDynamicImport)
	}

	// require() — IS dynamic
	if imports[3].Request != "./module" || !imports[3].IsDynamicImport {
		t.Errorf("Expected dynamic require './module', got request='%s' dynamic=%v", imports[3].Request, imports[3].IsDynamicImport)
	}
}

func TestMemberImportCallIsNotDynamicImport(t *testing.T) {
	long := strings.Repeat("x", 5000)
	code := `
const Q = {}
Q.import("` + long + `")
const m = import('./real-dynamic')
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./real-dynamic" || !imports[0].IsDynamicImport {
		t.Fatalf("Expected only dynamic import './real-dynamic', got request=%q dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}
}

func TestMemberImportCallWithSpacesAfterDotIsNotDynamicImport(t *testing.T) {
	code := `
const Module = {}
Module.   import("./ignored-member-call")
const m = import('./real-dynamic-spaced')
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./real-dynamic-spaced" || !imports[0].IsDynamicImport {
		t.Fatalf("Expected only dynamic import './real-dynamic-spaced', got request=%q dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}
}

func TestMemberImportCallInsideBlockIsNotDynamicImport(t *testing.T) {
	long := strings.Repeat("y", 5000)
	code := `
function load() {
	const Q = {}
	Q.import("` + long + `")
	return import('./inside')
}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./inside" || !imports[0].IsDynamicImport {
		t.Fatalf("Expected only dynamic import './inside', got request=%q dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}
}

func TestMemberRequireCallIsNotDynamicImport(t *testing.T) {
	long := strings.Repeat("r", 5000)
	code := `
const Q = {}
Q.require("` + long + `")
const mod = require('./real-require')
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./real-require" || !imports[0].IsDynamicImport {
		t.Fatalf("Expected only require './real-require', got request=%q dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}
}

func TestMemberRequireCallInsideBlockIsNotDynamicImport(t *testing.T) {
	long := strings.Repeat("z", 5000)
	code := `
function load() {
	const Q = {}
	Q.require("` + long + `")
	return require('./inside-require')
}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./inside-require" || !imports[0].IsDynamicImport {
		t.Fatalf("Expected only require './inside-require', got request=%q dynamic=%v", imports[0].Request, imports[0].IsDynamicImport)
	}
}

func TestMemberExportCallDoesNotHideRealImport(t *testing.T) {
	code := `
const obj = {}
obj.export("ignored")
import item from "./real-import-after-member-export"
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "./real-import-after-member-export" {
		t.Fatalf("Expected import './real-import-after-member-export', got %q", imports[0].Request)
	}
}

func TestMemberFromAndTypeDoNotAffectExportAndImportParsing(t *testing.T) {
	code := `
const api = {}
api.from("./fake-member-from")
api.type("ignored")
import type { T } from "./types"
export { B } from "./real-reexport"
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	if len(imports) != 2 {
		t.Fatalf("Expected 2 imports, got %d", len(imports))
	}
	if imports[0].Request != "./types" || imports[0].Kind != OnlyTypeImport {
		t.Fatalf("Expected first import to be type import './types', got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
	if imports[1].Request != "./real-reexport" {
		t.Fatalf("Expected second import to be re-export './real-reexport', got %q", imports[1].Request)
	}
}

func TestDeclareModuleExportsIgnored(t *testing.T) {
	code := `
import { something } from './real-import'

declare module 'blitz' {
  export interface Ctx {
    session: SessionContext;
  }
  export interface AuthenticatedMiddlewareCtx extends Omit<Ctx, 'session'> {
    session: AuthenticatedSessionContext;
  }
}

export function realExport() {}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// Should have: 1 real import + 1 real local export = 2
	// The exports inside declare module should be ignored
	if len(imports) != 2 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 2 imports (1 real import + 1 local export), got %d", len(imports))
	}

	if imports[0].Request != "./real-import" {
		t.Errorf("Expected first import to be './real-import', got '%s'", imports[0].Request)
	}

	if !imports[1].IsLocalExport || imports[1].Keywords == nil || imports[1].Keywords.Keywords[0].Name != "realExport" {
		t.Errorf("Expected second import to be local export 'realExport', got %+v", imports[1])
	}
}

func TestDeclareGlobalExportsIgnored(t *testing.T) {
	code := `
declare global {
  export interface Window {
    customProp: string;
  }
}

export const myConst = 42
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	if len(imports) != 1 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 1 import (local export), got %d", len(imports))
	}

	if !imports[0].IsLocalExport || imports[0].Keywords.Keywords[0].Name != "myConst" {
		t.Errorf("Expected local export 'myConst', got %+v", imports[0])
	}
}

func TestDeclareNamespaceExportsIgnored(t *testing.T) {
	code := `
declare namespace NodeJS {
  export interface ProcessEnv {
    NODE_ENV: string;
  }
}

export type MyType = string
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	if len(imports) != 1 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 1 import (local type export), got %d", len(imports))
	}
}

func TestDeclareModuleWithNestedBraces(t *testing.T) {
	code := `
declare module 'complex' {
  export interface Config {
    nested: {
      deep: {
        value: string;
      };
    };
  }
}

export function afterDeclare() {}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	if len(imports) != 1 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v", i, imp.Request, imp.IsLocalExport)
		}
		t.Fatalf("Expected 1 import (local export after declare), got %d", len(imports))
	}

	if !imports[0].IsLocalExport || imports[0].Keywords.Keywords[0].Name != "afterDeclare" {
		t.Errorf("Expected local export 'afterDeclare'")
	}
}

func TestExportNamespaceBodySkipped(t *testing.T) {
	code := `
export namespace Intercom {
  export function trackEvent(name: string, payload?: any) {
    if (canUseIntercom(window.Intercom)) {
      window.Intercom('trackEvent', name);
    }
  }

  export function setUserProperties(userProperties: Record<string, any>) {
    if (canUseIntercom(window.Intercom)) {
      window.Intercom('update', userProperties);
    }
  }
}

export function realExport() {}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// Should have 2 local exports:
	// 1. The namespace "Intercom" itself (export namespace Intercom)
	// 2. The function "realExport"
	// Inner exports (trackEvent, setUserProperties) should NOT appear
	if len(imports) != 2 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 2 imports (namespace + realExport), got %d", len(imports))
	}

	if !imports[0].IsLocalExport || imports[0].Keywords.Keywords[0].Name != "Intercom" {
		t.Errorf("Expected first export to be namespace 'Intercom', got %+v", imports[0])
	}

	if !imports[1].IsLocalExport || imports[1].Keywords.Keywords[0].Name != "realExport" {
		t.Errorf("Expected second export to be 'realExport', got %+v", imports[1])
	}
}

func TestExportNamespaceWithTemplatesAndCommentsBodySkipped(t *testing.T) {
	code := "import { tick } from 'clock-lib';\n" +
		"import { isHost, sanitize } from '@/runtime/helpers';\n" +
		"import { Flags } from '@/runtime/flags';\n" +
		"import type { BridgeFn } from './bridge.types';\n" +
		"\n" +
		"export namespace Bridge {\n" +
		"  export function fire(name: string, payload?: any) {\n" +
		"    if (Flags.enabled) {\n" +
		"      const data = payload ? sanitize(payload) : {};\n" +
		"      // We'll execute a delayed refresh to flush queued events.\n" +
		"      setTimeout(() => {\n" +
		"        if (!isHost()) return;\n" +
		"        window.Client?.('refresh', {\n" +
		"          at: tick(new Date()),\n" +
		"        });\n" +
		"        // Don't change this timeout without validating telemetry behavior.\n" +
		"      }, 250);\n" +
		"    }\n" +
		"  }\n" +
		"\n" +
		"  export function launch(id: number) {\n" +
		"    if (!isHost()) {\n" +
		"      window.open(\n" +
		"        `${window.location.href}?launch_id=${id}`,\n" +
		"        '_blank',\n" +
		"      );\n" +
		"      return;\n" +
		"    }\n" +
		"    window.Client?.('launch', id);\n" +
		"  }\n" +
		"}\n"

	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// 4 top-level imports + 1 local export (namespace Intercom).
	if len(imports) != 5 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 5 entries (4 imports + namespace export), got %d", len(imports))
	}

	if !imports[4].IsLocalExport || imports[4].Keywords == nil || imports[4].Keywords.Keywords[0].Name != "Bridge" {
		t.Fatalf("Expected last entry to be local namespace export 'Bridge', got %+v", imports[4])
	}

	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil && len(imp.Keywords.Keywords) > 0 {
			if imp.Keywords.Keywords[0].Name == "trackEvent" || imp.Keywords.Keywords[0].Name == "openSurvey" {
				t.Fatalf("Inner namespace exports should not be parsed as top-level exports: %+v", imp)
			}
		}
	}
}

func TestLargeNamespaceBodyDoesNotLeakInnerExports(t *testing.T) {
	code := "import { fn } from 'pkg-a';\n" +
		"import { util } from '@/pkg-b';\n" +
		"import type { T } from './types';\n" +
		"\n" +
		"export namespace Toolkit {\n" +
		"  export function alpha(id: number) {\n" +
		"    if (!util()) {\n" +
		"      window.open(`${window.location.href}?alpha_id=${id}`, '_blank');\n" +
		"      return;\n" +
		"    }\n" +
		"    setTimeout(() => {\n" +
		"      fn('alpha', { id });\n" +
		"      // We'll keep this timer for now.\n" +
		"    }, 100);\n" +
		"  }\n" +
		"\n" +
		"  export function beta(id: number) {\n" +
		"    if (!util()) {\n" +
		"      window.open(`${window.location.href}?beta_id=${id}`, '_blank');\n" +
		"      return;\n" +
		"    }\n" +
		"    window.Client?.('beta', id);\n" +
		"  }\n" +
		"\n" +
		"  export function gamma(id: number) {\n" +
		"    if (!util()) {\n" +
		"      window.open(`${window.location.href}?gamma_id=${id}`, '_blank');\n" +
		"      return;\n" +
		"    }\n" +
		"    window.Client?.('gamma', id);\n" +
		"  }\n" +
		"\n" +
		"  export function delta(id: number) {\n" +
		"    if (!util()) {\n" +
		"      window.open(`${window.location.href}?delta_id=${id}`, '_blank');\n" +
		"      return;\n" +
		"    }\n" +
		"    window.Client?.('delta', id);\n" +
		"  }\n" +
		"}\n"

	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	if len(imports) != 4 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 4 entries (3 imports + namespace export), got %d", len(imports))
	}

	if !imports[3].IsLocalExport || imports[3].Keywords == nil || imports[3].Keywords.Keywords[0].Name != "Toolkit" {
		t.Fatalf("Expected last entry local namespace export 'Toolkit', got %+v", imports[3])
	}

	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil && len(imp.Keywords.Keywords) > 0 {
			switch imp.Keywords.Keywords[0].Name {
			case "alpha", "beta", "gamma", "delta":
				t.Fatalf("Inner namespace exports should not be parsed as top-level exports: %+v", imp)
			}
		}
	}
}

func TestExportNamespaceBasicMode(t *testing.T) {
	// In basic mode, namespace body should also be skipped (no inner exports parsed)
	code := `
export namespace Foo {
  export const bar = 1;
  export function baz() {}
}

import { something } from './other'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)

	// Basic mode doesn't parse local exports, so only the import should appear
	if len(imports) != 1 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v", i, imp.Request, imp.IsLocalExport)
		}
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}

	if imports[0].Request != "./other" {
		t.Errorf("Expected import './other', got '%s'", imports[0].Request)
	}
}

func TestExportDeclareNamespaceBodySkipped(t *testing.T) {
	code := `
export declare namespace SDK {
  export function inside(): void;
}

import { real } from './real'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// `export declare namespace` is declaration-only here; parser should still not leak
	// inner exports and should continue parsing subsequent imports.
	if len(imports) != 1 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 1 entry, got %d", len(imports))
	}
	if imports[0].Request != "./real" {
		t.Fatalf("Expected import './real', got %+v", imports[0])
	}

	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil && len(imp.Keywords.Keywords) > 0 && imp.Keywords.Keywords[0].Name == "inside" {
			t.Fatalf("Inner declare-namespace export should not be parsed as top-level export: %+v", imp)
		}
	}
}

func TestExportModuleBodySkipped(t *testing.T) {
	code := `
export module Legacy {
  export function run(): void {}
}

import { modern } from './modern'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// Expected:
	// 1) local export module Legacy
	// 2) import './modern'
	if len(imports) != 2 {
		for i, imp := range imports {
			t.Logf("import[%d]: request=%q isLocal=%v keywords=%v", i, imp.Request, imp.IsLocalExport, imp.Keywords)
		}
		t.Fatalf("Expected 2 entries, got %d", len(imports))
	}

	if !imports[0].IsLocalExport || imports[0].Keywords == nil || imports[0].Keywords.Keywords[0].Name != "Legacy" {
		t.Fatalf("Expected first entry local export 'Legacy', got %+v", imports[0])
	}
	if imports[1].Request != "./modern" {
		t.Fatalf("Expected second entry import './modern', got %+v", imports[1])
	}

	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil && len(imp.Keywords.Keywords) > 0 && imp.Keywords.Keywords[0].Name == "run" {
			t.Fatalf("Inner module export should not be parsed as top-level export: %+v", imp)
		}
	}
}

// =============================================================================
// Brace-depth edge cases — these tests verify that the parser correctly handles
// import/export/require keywords at various brace depths, inside strings,
// comments, and template literals. Essential for brace-depth optimizations.
// =============================================================================

func TestDynamicImportInsideNestedFunctionBody(t *testing.T) {
	code := `
import { useState } from 'react'

function App() {
	const handler = () => {
		if (condition) {
			import('./lazy-component').then(m => m.default)
		}
	}
}

export default App
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	// Should find: static import 'react', dynamic import './lazy-component', local export 'App'
	var staticImport, dynamicImport *Import
	for i := range imports {
		if imports[i].Request == "react" {
			staticImport = &imports[i]
		}
		if imports[i].Request == "./lazy-component" {
			dynamicImport = &imports[i]
		}
	}

	if staticImport == nil {
		t.Error("Expected static import 'react'")
	}
	if dynamicImport == nil {
		t.Error("Expected dynamic import './lazy-component'")
	}
	if dynamicImport != nil && !dynamicImport.IsDynamicImport {
		t.Error("Expected dynamic import flag to be true for './lazy-component'")
	}
}

func TestRequireInsideClassMethod(t *testing.T) {
	code := `
import { Base } from './base'

class Service extends Base {
	loadModule() {
		const mod = require('./plugin')
		return mod
	}

	async init() {
		const config = require('./config')
		return config
	}
}

export { Service }
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./base"] {
		t.Error("Expected import './base'")
	}
	if !requests["./plugin"] {
		t.Error("Expected require './plugin'")
	}
	if !requests["./config"] {
		t.Error("Expected require './config'")
	}
}

func TestStaticExportsAfterFunctionBodies(t *testing.T) {
	code := `
function helper() {
	const x = 1
	return x
}

export const A = 1

class MyClass {
	method() {
		return { key: 'value' }
	}
}

export const B = 2

const obj = {
	nested: {
		deep: {
			value: 42
		}
	}
}

export function C() {}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	exportNames := make(map[string]bool)
	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil {
			for _, kw := range imp.Keywords.Keywords {
				exportNames[kw.Name] = true
			}
		}
	}

	for _, name := range []string{"A", "B", "C"} {
		if !exportNames[name] {
			t.Errorf("Expected export '%s' to be found", name)
		}
	}
}

func TestStringWithBracesDoesNotAffectParsing(t *testing.T) {
	code := `
const config = "{ import: true, export: false }"
const json = '{"require": "./module", "braces": "{{}}"}'

export const realExport = 1

import { something } from './real-module'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	var hasRealModule bool
	var hasRealExport bool
	for _, imp := range imports {
		if imp.Request == "./real-module" {
			hasRealModule = true
		}
		if imp.IsLocalExport && imp.Keywords != nil && imp.Keywords.Keywords[0].Name == "realExport" {
			hasRealExport = true
		}
	}

	if !hasRealModule {
		t.Error("Expected import './real-module'")
	}
	if !hasRealExport {
		t.Error("Expected local export 'realExport'")
	}

	// Should NOT have any import from the strings
	for _, imp := range imports {
		if imp.Request == "./module" {
			t.Error("Should not parse import from string content")
		}
	}
}

func TestTemplateLiteralWithBracesAndKeywords(t *testing.T) {
	code := "import { A } from './a'\n" +
		"\n" +
		"const template = `some text ${ variable } more text`\n" +
		"const complex = `import { B } from 'fake'`\n" +
		"const braces = `{ export const C = 1 }`\n" +
		"const nested = `outer ${ `inner` } end`\n" +
		"\n" +
		"export const realExport = 1\n"

	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	var hasA bool
	var hasRealExport bool
	for _, imp := range imports {
		if imp.Request == "./a" {
			hasA = true
		}
		if imp.IsLocalExport && imp.Keywords != nil && imp.Keywords.Keywords[0].Name == "realExport" {
			hasRealExport = true
		}
	}

	if !hasA {
		t.Error("Expected import './a'")
	}
	if !hasRealExport {
		t.Error("Expected local export 'realExport'")
	}

	// Should NOT parse fake imports from template literals
	for _, imp := range imports {
		if imp.Request == "fake" {
			t.Error("Should not parse import from template literal content")
		}
	}
}

func TestCommentsWithBracesDoNotAffectParsing(t *testing.T) {
	code := `
// function foo() {
//   import { X } from 'commented-out'
// }

/*
class Bar {
	require('./also-commented')
}
*/

import { real } from './real'

export const afterComments = 1
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	var hasReal bool
	var hasAfterComments bool
	for _, imp := range imports {
		if imp.Request == "./real" {
			hasReal = true
		}
		if imp.IsLocalExport && imp.Keywords != nil && imp.Keywords.Keywords[0].Name == "afterComments" {
			hasAfterComments = true
		}
	}

	if !hasReal {
		t.Error("Expected import './real'")
	}
	if !hasAfterComments {
		t.Error("Expected local export 'afterComments'")
	}

	// Should NOT have imports from comments
	for _, imp := range imports {
		if imp.Request == "commented-out" || imp.Request == "./also-commented" {
			t.Errorf("Should not parse import from comment: %s", imp.Request)
		}
	}
}

func TestObjectLiteralWithKeywordPropertyNames(t *testing.T) {
	code := `
import { config } from './config'

const settings = {
	import: true,
	export: false,
	require: './not-a-module',
	from: 'nowhere',
	module: 'test'
}

export const result = settings
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./config"] {
		t.Error("Expected import './config'")
	}

	// Should NOT parse keyword property names as imports
	if requests["./not-a-module"] {
		t.Error("Should not parse object property 'require' as an import")
	}
	if requests["nowhere"] {
		t.Error("Should not parse object property 'from' as an import")
	}
}

func TestDeepNestingWithDynamicImports(t *testing.T) {
	code := `
import { init } from './init'

async function bootstrap() {
	try {
		const config = await loadConfig()
		if (config.plugins) {
			for (const plugin of config.plugins) {
				const mod = await import('./plugins/' + plugin)
				if (mod.setup) {
					await mod.setup({
						callback: () => {
							require('./fallback')
						}
					})
				}
			}
		}
	} catch (e) {
		console.error(e)
	}
}

export { bootstrap }
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./init"] {
		t.Error("Expected import './init'")
	}
	// Dynamic import with concatenation — parser doesn't handle non-static paths,
	// so './plugins/' + plugin won't be detected. This is expected behavior.
	if !requests["./fallback"] {
		t.Error("Expected require './fallback' inside deeply nested callback")
	}
}

func TestExportsBetweenMultipleBraceScopes(t *testing.T) {
	code := `
function a() {
	return 1
}

export const X = a()

class B {
	method() {
		return { nested: { value: 2 } }
	}
}

export const Y = new B()

if (typeof window !== 'undefined') {
	window.Z = 'test'
}

export function Z() {}

const obj = {
	key: {
		deep: {
			value: require('./deep-value')
		}
	}
}

export default obj
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	exportNames := make(map[string]bool)
	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.IsLocalExport && imp.Keywords != nil {
			for _, kw := range imp.Keywords.Keywords {
				exportNames[kw.Name] = true
			}
		}
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	for _, name := range []string{"X", "Y", "Z", "default"} {
		if !exportNames[name] {
			t.Errorf("Expected export '%s' to be found", name)
		}
	}

	if !requests["./deep-value"] {
		t.Error("Expected require './deep-value' inside nested object literal")
	}
}

func TestArrowFunctionWithDynamicImport(t *testing.T) {
	code := `
export const loader = () => import('./lazy')

export const fetcher = async () => {
	const data = await import('./data')
	return data.default
}
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./lazy"] {
		t.Error("Expected dynamic import './lazy'")
	}
	if !requests["./data"] {
		t.Error("Expected dynamic import './data'")
	}
}

func TestReexportAfterComplexCode(t *testing.T) {
	code := `
const complexObject = {
	a: { b: { c: { d: 1 } } },
	e: [1, 2, 3].map(x => ({ value: x })),
}

function process() {
	return { result: true }
}

export { something } from './re-exported'
export * from './barrel'
export { default as Named } from './named'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./re-exported"] {
		t.Error("Expected re-export from './re-exported'")
	}
	if !requests["./barrel"] {
		t.Error("Expected re-export from './barrel'")
	}
	if !requests["./named"] {
		t.Error("Expected re-export from './named'")
	}
}

func TestConditionalRequire(t *testing.T) {
	code := `
import { env } from './env'

function loadDriver() {
	if (env === 'production') {
		return require('./driver-prod')
	} else {
		return require('./driver-dev')
	}
}

export { loadDriver }
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./env"] {
		t.Error("Expected import './env'")
	}
	if !requests["./driver-prod"] {
		t.Error("Expected require './driver-prod'")
	}
	if !requests["./driver-dev"] {
		t.Error("Expected require './driver-dev'")
	}
}

func TestStringsWithUnbalancedBraces(t *testing.T) {
	code := `
const openBrace = "{"
const closeBrace = "}"
const mixed = '{ "import": true }'
const escaped = "import { A } from \"fake\""
const template = "export { B } from 'also-fake'"

export const realExport = 1
import { C } from './real'
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	var hasReal bool
	var hasExport bool
	for _, imp := range imports {
		if imp.Request == "./real" {
			hasReal = true
		}
		if imp.IsLocalExport && imp.Keywords != nil && imp.Keywords.Keywords[0].Name == "realExport" {
			hasExport = true
		}
	}

	if !hasReal {
		t.Error("Expected import './real'")
	}
	if !hasExport {
		t.Error("Expected local export 'realExport'")
	}

	// Should NOT parse from string content
	for _, imp := range imports {
		if imp.Request == "fake" || imp.Request == "also-fake" {
			t.Errorf("Should not parse import from string: %s", imp.Request)
		}
	}
}

func TestSwitchStatementWithRequire(t *testing.T) {
	code := `
import { type } from './types'

function getHandler(action) {
	switch (action) {
		case 'upload':
			return require('./handlers/upload')
		case 'download': {
			const handler = require('./handlers/download')
			return handler
		}
		default:
			return require('./handlers/default')
	}
}

export { getHandler }
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./types"] {
		t.Error("Expected import './types'")
	}
	if !requests["./handlers/upload"] {
		t.Error("Expected require './handlers/upload'")
	}
	if !requests["./handlers/download"] {
		t.Error("Expected require './handlers/download'")
	}
	if !requests["./handlers/default"] {
		t.Error("Expected require './handlers/default'")
	}
}

func TestIIFEWithImports(t *testing.T) {
	code := `
import { setup } from './setup'

;(function() {
	const mod = require('./iife-module')
	mod.init()
})()

;(() => {
	import('./async-iife').then(m => m.run())
})()

export { setup }
`
	imports := ParseImportsByte([]byte(code), false, ParseModeDetailed)

	requests := make(map[string]bool)
	for _, imp := range imports {
		if imp.Request != "" {
			requests[imp.Request] = true
		}
	}

	if !requests["./setup"] {
		t.Error("Expected import './setup'")
	}
	if !requests["./iife-module"] {
		t.Error("Expected require './iife-module' inside IIFE")
	}
	if !requests["./async-iife"] {
		t.Error("Expected dynamic import './async-iife' inside arrow IIFE")
	}
}

func TestImportWordInsideJSXTextDoesNotCreateFakeImport(t *testing.T) {
	code := `
import { A } from './a'

function Comp() {
  return (
    <Text>
      Do you want to import your information?
    </Text>
  )
}
`

	imports := ParseImportsByte([]byte(code), false, ParseModeBasic)
	if len(imports) != 1 {
		t.Fatalf("Expected exactly 1 import, got %d: %+v", len(imports), imports)
	}
	if imports[0].Request != "./a" {
		t.Fatalf("Expected only './a' import, got %q", imports[0].Request)
	}
}
