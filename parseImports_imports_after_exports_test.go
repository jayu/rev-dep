package main

import (
	"testing"
)

func onlyNonLocal(imports []Import) []Import {
	res := make([]Import, 0, len(imports))
	for _, imp := range imports {
		if !imp.IsLocalExport {
			res = append(res, imp)
		}
	}
	return res
}

func TestExportTypeWithImport(t *testing.T) {
	code := `export type { ILogger };

import value form 'wait-for-expect';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Parse invalid %s -> length %d, should be 1`, codeToString(code), len(imports))
		return
	}

	if imports[0].Request != "wait-for-expect" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Parse invalid %s -> %s`, codeToString(code), importsArrToString(imports))
	}
}

func TestExportTypeWithoutFromShouldNotInterfereWithImport(t *testing.T) {
	code := `export type { ILogger };
import 'wait-for-expect';`

	imports := ParseImportsForTests(code)

	// Should find 1 import (the second line)
	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	// The import should be 'wait-for-expect'
	if imports[0].Request != "wait-for-expect" {
		t.Errorf(`Expected import 'wait-for-expect', got '%s'`, imports[0].Request)
	}
}

func TestExportTypeWithImportDetailed(t *testing.T) {
	code := `export type { ILogger };

import value form 'wait-for-expect';`

	imports := ParseImportsForTestsDetailed(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "wait-for-expect" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import 'wait-for-expect' with NotTypeOrMixedImport, got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeWithoutFromShouldNotInterfereWithImportDetailed(t *testing.T) {
	code := `export type { ILogger };
import 'wait-for-expect';`

	imports := ParseImportsForTestsDetailed(code)

	// Should find 1 import (the second line)
	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	// The import should be 'wait-for-expect'
	if imports[0].Request != "wait-for-expect" {
		t.Errorf(`Expected import 'wait-for-expect', got '%s'`, imports[0].Request)
	}
}

// Tests based on existing test cases but with swapped order (export first, then import)

func TestExportDefaultThenImportType(t *testing.T) {
	// Based on: {code: `export default Variable;            export type { Id } from "module"`, kind: OnlyTypeImport}
	// Swapped to: export first, then import
	code := `export default Variable;
import type { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportConstThenImportType(t *testing.T) {
	// Based on: {code: `export const Variable = 'value';    export type { Id } from "module"`, kind: OnlyTypeImport}
	// Swapped to: export first, then import
	code := `export const Variable = 'value';
import type { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeDefinitionThenImport(t *testing.T) {
	// Based on: {code: `export type SomeType = {};          import { Id } from "module"`, kind: NotTypeOrMixedImport}
	// Swapped to: export first, then import
	code := `export type SomeType = {};
import { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected mixed import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportInterfaceThenImportType(t *testing.T) {
	// Based on: {code: `export interface SomeType {};       export type { Id } from "module"`, kind: OnlyTypeImport}
	// Swapped to: export first, then import
	code := `export interface SomeType {};
import type { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportFunctionThenImportType(t *testing.T) {
	// Based on: {code: `export function SomeFn() {};        export type { Id } from "module"`, kind: OnlyTypeImport}
	// Swapped to: export first, then import
	code := `export function SomeFn() {};
import type { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportNamedThenImportType(t *testing.T) {
	// Based on: {code: `export { Variable };                import type { Id } from "module"`, kind: OnlyTypeImport}
	// Swapped to: export first, then import
	code := `export { Variable };
import type { Id } from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeDefinitionThenRequire(t *testing.T) {
	// Based on: {code: `export type Variable = {};          require("module")`, kind: NotTypeOrMixedImport}
	// Swapped to: export first, then require
	code := `export type Variable = {};
require("module");`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected require import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportNamedThenDynamicImport(t *testing.T) {
	// Based on: {code: `export { Variable };                import("module")`, kind: NotTypeOrMixedImport}
	// Swapped to: export first, then dynamic import
	code := `export { Variable };
import("module");`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected dynamic import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

// More targeted tests to isolate the exact pattern

func TestExportTypeWithBracesThenImport(t *testing.T) {
	// Test the specific failing pattern: export type { ... } without from clause
	code := `export type { SomeType };
import 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeWithBracesThenImportType(t *testing.T) {
	// Test with type import after export type braces
	code := `export type { SomeType };
import type { OtherType } from 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeAssignmentThenImport(t *testing.T) {
	// Test export type assignment (not braces) - this should work
	code := `export type SomeType = {};
import 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeWithFromThenImport(t *testing.T) {
	// Test export type with from clause (re-export) - this should work
	code := `export type { SomeType } from 'other-module';
import 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	// First should be the re-export
	if imports[0].Request != "other-module" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type re-export from 'other-module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	// Second should be the import
	if imports[1].Request != "module" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportInterfaceThenImport(t *testing.T) {
	// Test export interface (similar to export type assignment)
	code := `export interface SomeType {};
import 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

// Detailed versions of the targeted tests

func TestExportTypeWithBracesThenImportDetailed(t *testing.T) {
	// Test the specific failing pattern with detailed parser
	code := `export type { SomeType };
import 'module';`

	imports := ParseImportsForTestsDetailed(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeAssignmentThenImportDetailed(t *testing.T) {
	// Test export type assignment (not braces) with detailed parser - this should work
	code := `export type SomeType = {};
import 'module';`

	imports := ParseImportsForTestsDetailed(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

// Comprehensive test combinations mixing various import and export syntaxes

func TestExportStarThenImportDefault(t *testing.T) {
	code := `export * from "module-a";
import DefaultExport from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	// First should be the re-export
	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	// Second should be the default import
	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected default import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportStarAsThenImportNamespace(t *testing.T) {
	code := `export * as name from "module-a";
import * as ns from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected star re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected namespace import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportNamedThenImportNamed(t *testing.T) {
	code := `export { name1, name2 } from "module-a";
import { export1, export2 } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected named re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected named import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportNamedWithAliasThenImportNamedWithAlias(t *testing.T) {
	code := `export { import1 as name1, import2 as name2 } from "module-a";
import { export1 as alias1, export2 as alias2 } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected aliased re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected aliased import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportDefaultThenImportDefault(t *testing.T) {
	code := `export default Variable;
import DefaultExport from "module";`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected default import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportDefaultAsThenImportDefaultAs(t *testing.T) {
	code := `export { default as name1 } from "module-a";
import { default as alias } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected default re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected default import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportTypeThenImportType(t *testing.T) {
	code := `export type { MyType } from "module-a";
import type { OtherType } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestExportMixedTypeThenImportMixedType(t *testing.T) {
	code := `export { default, type MyType2 } from "module-a";
import { type MyType3, MyVal } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected mixed re-export from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected mixed type import from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestImportDefaultThenExportStar(t *testing.T) {
	// Swap order: import first, then export
	code := `import DefaultExport from "module-a";
export * from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected default import from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected star re-export from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestImportNamespaceThenExportNamed(t *testing.T) {
	// Swap order: import first, then export
	code := `import * as ns from "module-a";
export { name1, name2 } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected namespace import from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected named re-export from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestImportTypeThenExportType(t *testing.T) {
	// Swap order: import first, then export
	code := `import type { MyType } from "module-a";
export type { OtherType } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != OnlyTypeImport {
		t.Errorf(`Expected type import from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != OnlyTypeImport {
		t.Errorf(`Expected type re-export from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestImportMixedThenExportMixed(t *testing.T) {
	// Swap order: import first, then export
	code := `import { type MyType3, MyVal } from "module-a";
export { default, type MyType2 } from "module-b";`

	imports := ParseImportsForTests(code)

	if len(imports) != 2 {
		t.Errorf(`Expected 2 imports, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected mixed type import from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}

	if imports[1].Request != "module-b" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected mixed re-export from 'module-b', got request='%s' kind=%v`, imports[1].Request, imports[1].Kind)
	}
}

func TestImportSideEffectThenExportDefault(t *testing.T) {
	// Swap order: import first, then export
	code := `import "module-a";
export default Variable;`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected side-effect import from 'module-a', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

// Test specific patterns user asked about

func TestExportTypeBracesThenImportNamespace(t *testing.T) {
	// Test: export type { } + import * as
	code := `export type { SomeType };
import * as D from 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected namespace import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeBracesThenImportNamed(t *testing.T) {
	// Test: export type { } + import { }
	code := `export type { SomeType };
import {V} from 'module';`

	imports := ParseImportsForTests(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected named import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeBracesThenImportNamespaceDetailed(t *testing.T) {
	// Test: export type { } + import * as (detailed parser)
	code := `export type { SomeType };
import * as D from 'module';`

	imports := ParseImportsForTestsDetailed(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected namespace import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeBracesThenImportNamedDetailed(t *testing.T) {
	// Test: export type { } + import { } (detailed parser)
	code := `export type { SomeType };
import {V} from 'module';`

	imports := ParseImportsForTestsDetailed(code)

	if len(imports) != 1 {
		t.Errorf(`Expected 1 import, got %d`, len(imports))
		return
	}

	if imports[0].Request != "module" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf(`Expected named import from 'module', got request='%s' kind=%v`, imports[0].Request, imports[0].Kind)
	}
}

// Additional export-first coverage for mixed syntaxes.

func TestExportDefaultFunctionThenImportNamespace(t *testing.T) {
	code := `export default function Factory() {};
import * as Ns from "module-a";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "module-a" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected namespace import from module-a, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportDefaultClassThenImportDefaultNamed(t *testing.T) {
	code := `export default class Service {};
import DefaultValue, { Value, type TypeValue } from "module-b";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "module-b" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected mixed import from module-b, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportLetVarThenImportType(t *testing.T) {
	code := `export let A = 1;
export var B = 2;
import type { SomeType } from "types-lib";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "types-lib" || imports[0].Kind != OnlyTypeImport {
		t.Errorf("Expected type import from types-lib, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportEnumThenImportDefault(t *testing.T) {
	code := `export enum Status { Ready, Done }
import StatusClient from "status-client";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "status-client" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected default import from status-client, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportNamespaceThenImportSideEffect(t *testing.T) {
	code := `export namespace Intercom {
  export const enabled = true;
}
import "intercom-runtime";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "intercom-runtime" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected side-effect import from intercom-runtime, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportModuleThenImportNamed(t *testing.T) {
	code := `export module Legacy {
  export function run() {}
}
import { modern } from "modern-lib";`

	imports := ParseImportsForTests(code)
	if len(imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(imports))
	}
	if imports[0].Request != "modern-lib" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected named import from modern-lib, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportTypeStarReexportThenImport(t *testing.T) {
	code := `export type * from "types-only";
import "runtime-lib";`

	imports := ParseImportsForTests(code)
	if len(imports) != 2 {
		t.Fatalf("Expected 2 imports, got %d", len(imports))
	}

	if imports[0].Request != "types-only" || imports[0].Kind != OnlyTypeImport {
		t.Errorf("Expected type star re-export from types-only, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
	if imports[1].Request != "runtime-lib" || imports[1].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected side-effect import from runtime-lib, got request=%q kind=%v", imports[1].Request, imports[1].Kind)
	}
}

func TestExportNamespaceThenImportDetailedNonLocal(t *testing.T) {
	code := `export namespace Intercom {
  export const enabled = true;
}
import "intercom-runtime";`

	imports := onlyNonLocal(ParseImportsForTestsDetailed(code))
	if len(imports) != 1 {
		t.Fatalf("Expected 1 non-local import, got %d", len(imports))
	}
	if imports[0].Request != "intercom-runtime" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected side-effect import from intercom-runtime, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}

func TestExportModuleThenImportDetailedNonLocal(t *testing.T) {
	code := `export module Legacy {
  export function run() {}
}
import { modern } from "modern-lib";`

	imports := onlyNonLocal(ParseImportsForTestsDetailed(code))
	if len(imports) != 1 {
		t.Fatalf("Expected 1 non-local import, got %d", len(imports))
	}
	if imports[0].Request != "modern-lib" || imports[0].Kind != NotTypeOrMixedImport {
		t.Errorf("Expected named import from modern-lib, got request=%q kind=%v", imports[0].Request, imports[0].Kind)
	}
}
