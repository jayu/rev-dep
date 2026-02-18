package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to create a MinimalDependency with basic fields
func mkDep(id string) MinimalDependency {
	kind := NotTypeOrMixedImport
	return MinimalDependency{
		ID:         id,
		ImportKind: kind,
	}
}

// Helper to create a local export dependency with keywords
func mkLocalExport(keywords []KeywordInfo, exportKeyStart, exportKeyEnd, exportDeclStart int) MinimalDependency {
	emptyID := ""
	kw := &KeywordMap{Keywords: keywords}
	return MinimalDependency{
		ID:              emptyID,
		IsLocalExport:   true,
		Keywords:        kw,
		ExportKeyStart:  uint32(exportKeyStart),
		ExportKeyEnd:    uint32(exportKeyEnd),
		ExportDeclStart: uint32(exportDeclStart),
	}
}

// Helper to create a local brace export dependency
func mkLocalBraceExport(keywords []KeywordInfo, exportKeyStart, braceStart, braceEnd, statementEnd int) MinimalDependency {
	emptyID := ""
	kw := &KeywordMap{Keywords: keywords}
	return MinimalDependency{
		ID:                 emptyID,
		IsLocalExport:      true,
		Keywords:           kw,
		ExportKeyStart:     uint32(exportKeyStart),
		ExportKeyEnd:       uint32(exportKeyStart + 6), // after "export"
		ExportBraceStart:   uint32(braceStart),
		ExportBraceEnd:     uint32(braceEnd),
		ExportStatementEnd: uint32(statementEnd),
	}
}

// Helper to create a re-export dependency
func mkReexport(id string, keywords []KeywordInfo, exportKeyStart, exportKeyEnd, braceStart, braceEnd, statementEnd int) MinimalDependency {
	kind := NotTypeOrMixedImport
	kw := &KeywordMap{Keywords: keywords}
	return MinimalDependency{
		ID:                 id,
		ImportKind:         kind,
		Keywords:           kw,
		ExportKeyStart:     uint32(exportKeyStart),
		ExportKeyEnd:       uint32(exportKeyEnd),
		ExportBraceStart:   uint32(braceStart),
		ExportBraceEnd:     uint32(braceEnd),
		ExportStatementEnd: uint32(statementEnd),
	}
}

// Helper to create an import dependency with specific keywords
func mkImportWithKeywords(id string, keywords []KeywordInfo) MinimalDependency {
	kind := NotTypeOrMixedImport
	kw := &KeywordMap{Keywords: keywords}
	return MinimalDependency{
		ID:         id,
		ImportKind: kind,
		Keywords:   kw,
	}
}

func TestFindUnusedExports_BasicUnused(t *testing.T) {
	// File exports A and B, only A is imported → B is unused
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "helperA", Start: 7, End: 14},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "helperB", Start: 7, End: 14},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/utils.ts", []KeywordInfo{
				{Name: "helperA"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/", nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "helperB" {
		t.Errorf("Expected 'helperB', got '%s'", results[0].ExportName)
	}
	if results[0].FilePath != "/src/utils.ts" {
		t.Errorf("Expected '/src/utils.ts', got '%s'", results[0].FilePath)
	}
}

func TestFindUnusedExports_AllUsed(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "helperA"},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/utils.ts", []KeywordInfo{
				{Name: "helperA"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/", nil,
	)

	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports, got %d", len(results))
	}
}

func TestFindUnusedExports_DefaultExport(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
		"/src/consumer.ts": {},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/", nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "default" {
		t.Errorf("Expected 'default', got '%s'", results[0].ExportName)
	}
}

func TestFindUnusedExports_TypeExportWithIgnore(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/types.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "Foo", IsType: true},
				{Name: "Bar", IsType: false},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {},
	}

	// With ignoreTypeExports=true, only Bar should be reported
	results := FindUnusedExports(
		[]string{"/src/types.ts", "/src/consumer.ts"},
		tree,
		nil, nil, true, false, "/src/", nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "Bar" {
		t.Errorf("Expected 'Bar', got '%s'", results[0].ExportName)
	}
}

func TestFindUnusedExports_TypeExportWithoutIgnore(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/types.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "Foo", IsType: true},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {},
	}

	// With ignoreTypeExports=false, Foo should be reported
	results := FindUnusedExports(
		[]string{"/src/types.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/", nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "Foo" {
		t.Errorf("Expected 'Foo', got '%s'", results[0].ExportName)
	}
	if !results[0].IsType {
		t.Error("Expected IsType to be true")
	}
}

func TestFindUnusedExports_EntryPoint(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/index.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "main"},
			}, 0, 6, 7),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/index.ts"},
		tree,
		[]string{"**/index.ts"},
		nil, false, false, "/src/",
		nil,
	)

	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports for entry point, got %d", len(results))
	}
}

func TestFindUnusedExports_StarImportMarksAllUsed(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "B"},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/utils.ts", []KeywordInfo{
				{Name: "*"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/",
		nil,
	)

	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports with star import, got %d", len(results))
	}
}

func TestFindUnusedExports_SideEffectImportDoesNotMarkUsed(t *testing.T) {
	// Side-effect import has nil Keywords
	sideEffectDep := mkDep("/src/utils.ts")
	sideEffectDep.Keywords = nil

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "helper"},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {
			sideEffectDep,
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, false, "/src/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
}

func TestFindUnusedExports_ReexportMarksSourceUsed(t *testing.T) {
	// index.ts re-exports A from utils.ts
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "B"},
			}, 0, 6, 7),
		},
		"/src/index.ts": {
			mkReexport("/src/utils.ts", []KeywordInfo{
				{Name: "A"},
			}, 0, 6, 9, 14, 30),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/index.ts"},
		tree,
		[]string{"**/index.ts"},
		nil, false, true, "/src/",
		nil,
	)

	// A is used (re-exported by index.ts which is an entry point), B is unused
	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "B" {
		t.Errorf("Expected 'B', got '%s'", results[0].ExportName)
	}
}

func TestFindUnusedExports_StarReexportMarksAllUsed(t *testing.T) {
	// export * from './utils' — no Keywords, just ID set
	starReexport := mkDep("/src/utils.ts")
	starReexport.ExportKeyEnd = 6
	starReexport.Keywords = nil

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "B"},
			}, 0, 6, 7),
		},
		"/src/index.ts": {
			starReexport,
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/index.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	// All exports of utils.ts marked used by star re-export
	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports with star re-export, got %d", len(results))
	}
}

func TestFindUnusedExports_GraphExclude(t *testing.T) {
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "helper"},
			}, 0, 6, 7),
		},
		"/src/test.spec.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "testHelper"},
			}, 0, 6, 7),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/test.spec.ts"},
		tree,
		nil,
		[]string{"**/*.spec.ts"},
		false, true, "/src/",
		nil,
	)

	// Only utils.ts should be checked (test.spec.ts excluded)
	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].FilePath != "/src/utils.ts" {
		t.Errorf("Expected '/src/utils.ts', got '%s'", results[0].FilePath)
	}
}

func TestFindUnusedExports_BarrelStarReexportDetailedMode(t *testing.T) {
	// In detailed parse mode, `export * from './utils'` has Keywords with a `*` keyword.
	// The barrel file should NOT have the star re-export reported as unused,
	// even when consumers import specific names through it.
	starReexport := mkDep("/src/utils.ts")
	starReexport.ExportKeyEnd = 6
	starReexport.ExportKeyStart = 0
	starReexport.Keywords = &KeywordMap{Keywords: []KeywordInfo{
		{Name: "*", Start: 7, End: 8},
	}}

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "ManageDweetAdmins"},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "unusedExport"},
			}, 100, 106, 107),
		},
		"/src/index.ts": {
			starReexport,
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/index.ts", []KeywordInfo{
				{Name: "ManageDweetAdmins"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/index.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	// Star re-export marks all source exports as used, so no unused exports.
	// The barrel file itself should not have `*` reported as unused.
	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports for barrel pattern, got %d", len(results))
	}
}

func TestFindUnusedExports_StarAsNameReexport(t *testing.T) {
	// export * as utils from './utils' — defines a named export "utils" in the barrel file
	starAsReexport := mkDep("/src/utils.ts")
	starAsReexport.ExportKeyEnd = 6
	starAsReexport.ExportKeyStart = 0
	starAsReexport.Keywords = &KeywordMap{Keywords: []KeywordInfo{
		{Name: "*", Alias: "utils", Start: 7, End: 20},
	}}

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
		},
		"/src/index.ts": {
			starAsReexport,
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/index.ts", []KeywordInfo{
				{Name: "utils"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/index.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	// "utils" is imported by consumer → no unused exports
	if len(results) != 0 {
		t.Fatalf("Expected 0 unused exports, got %d", len(results))
	}
}

func TestFindUnusedExports_StarAsNameReexport_Unused(t *testing.T) {
	// export * as utils from './utils' — nobody imports "utils"
	starAsReexport := mkDep("/src/utils.ts")
	starAsReexport.ExportKeyEnd = 6
	starAsReexport.ExportKeyStart = 0
	starAsReexport.ExportStatementEnd = 32
	starAsReexport.Keywords = &KeywordMap{Keywords: []KeywordInfo{
		{Name: "*", Alias: "utils", Start: 7, End: 17},
	}}

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
		},
		"/src/index.ts": {
			starAsReexport,
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/index.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	// "utils" is not imported by anyone → should be reported as unused
	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	if results[0].ExportName != "utils" {
		t.Errorf("Expected unused export 'utils', got '%s'", results[0].ExportName)
	}
	if results[0].Fix == nil {
		t.Error("Expected Fix to be non-nil (Strategy 4)")
	}
}

func TestFindUnusedExports_StarAsNameReexport_Autofix(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export * as utils from './utils';\nconst other = 1;\n"
	srcFile := filepath.Join(tmpDir, "index.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	starAsReexport := mkDep("/src/utils.ts")
	starAsReexport.ExportKeyStart = 0
	starAsReexport.ExportKeyEnd = 6
	starAsReexport.ExportStatementEnd = 33 // "export * as utils from './utils';" is 32 chars + 1 for ; maybe?
	// actually let's just count:
	// export (6) * (2) as (3) utils (6) from (5) './utils' (9) ; (1) = 32
	// "export * as utils from './utils';" length is 33.
	starAsReexport.ExportStatementEnd = uint32(len("export * as utils from './utils';"))
	starAsReexport.Keywords = &KeywordMap{Keywords: []KeywordInfo{
		{Name: "*", Alias: "utils", Start: 7, End: 17},
	}}

	tree := MinimalDependencyTree{
		srcFile: {
			starAsReexport,
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil")
	}

	// Apply the fix
	fixed := applyChange(source, results[0].Fix)
	expected := "const other = 1;\n"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_DynamicImportMarksAllUsed(t *testing.T) {
	// dynamic(() => import('./utils').then(m => m.A)) — marks all exports used
	// because we can't statically analyze what properties are accessed
	dynImport := mkDep("/src/utils.ts")
	dynImport.IsDynamicImport = true
	// Keywords is nil for dynamic imports

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "A"},
			}, 0, 6, 7),
			mkLocalExport([]KeywordInfo{
				{Name: "B"},
			}, 100, 106, 107),
		},
		"/src/consumer.ts": {
			dynImport,
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	if len(results) != 0 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.FilePath + ":" + r.ExportName
		}
		t.Fatalf("Expected 0 unused exports with dynamic import, got %d: %v", len(results), names)
	}
}

func TestFindUnusedExports_SideEffectImportDoesNotMarkUsed_NotDynamic(t *testing.T) {
	// Verify that side-effect imports (import './file') still don't mark exports as used
	sideEffectImport := mkDep("/src/utils.ts")
	// Keywords is nil, IsDynamicImport is false (default)

	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "helper"},
			}, 0, 6, 7),
		},
		"/src/consumer.ts": {
			sideEffectImport,
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export with side-effect import, got %d", len(results))
	}
	if results[0].ExportName != "helper" {
		t.Errorf("Expected 'helper', got '%s'", results[0].ExportName)
	}
}

func TestFindUnusedExports_LocalBraceExportWithAlias(t *testing.T) {
	// export { RetailConfirmationStatusDao as RetailConfirmationStatus }
	// Consumer imports { RetailConfirmationStatus } — should match the alias
	tree := MinimalDependencyTree{
		"/src/dao.ts": {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "RetailConfirmationStatusDao", Alias: "RetailConfirmationStatus", Start: 9, End: 64},
			}, 0, 7, 66, 67),
		},
		"/src/consumer.ts": {
			mkImportWithKeywords("/src/dao.ts", []KeywordInfo{
				{Name: "RetailConfirmationStatus"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/dao.ts", "/src/consumer.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	if len(results) != 0 {
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.FilePath + ":" + r.ExportName
		}
		t.Fatalf("Expected 0 unused exports (alias is imported), got %d: %v", len(results), names)
	}
}

func TestFindUnusedExports_LocalBraceExportWithAlias_Unused(t *testing.T) {
	// export { Dao as PublicName } — nobody imports PublicName
	tree := MinimalDependencyTree{
		"/src/dao.ts": {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "Dao", Alias: "PublicName", Start: 9, End: 30},
			}, 0, 7, 32, 33),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/dao.ts"},
		tree,
		nil, nil, false, true, "/src/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 unused export, got %d", len(results))
	}
	// Should report the alias (public name), not the internal name
	if results[0].ExportName != "PublicName" {
		t.Errorf("Expected unused export 'PublicName' (the alias), got '%s'", results[0].ExportName)
	}
}

// ==================== Autofix Tests ====================

func TestFindUnusedExports_Strategy1_RemoveExportPrefix(t *testing.T) {
	autofix := true
	// export const X = 1
	// ExportKeyStart=0, ExportDeclStart=7 (after "export ")
	tree := MinimalDependencyTree{
		"/src/utils.ts": {
			mkLocalExport([]KeywordInfo{
				{Name: "X"},
			}, 0, 6, 7),
		},
	}

	results := FindUnusedExports(
		[]string{"/src/utils.ts"},
		tree,
		nil, nil, false, autofix, "/src/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for Strategy 1")
	}
	if results[0].Fix.Start != 0 || results[0].Fix.End != 7 {
		t.Errorf("Expected Fix Start=0, End=7, got Start=%d, End=%d", results[0].Fix.Start, results[0].Fix.End)
	}
	if results[0].Fix.Text != "" {
		t.Errorf("Expected empty replacement text, got '%s'", results[0].Fix.Text)
	}
}

func TestFindUnusedExports_Strategy1_DefaultFunction(t *testing.T) {
	// export default function Fn(){}
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default function Fn(){}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for default function")
	}
	if results[0].Fix.Start != 0 || results[0].Fix.End != 15 {
		t.Errorf("Expected Fix Start=0, End=15, got Start=%d, End=%d", results[0].Fix.Start, results[0].Fix.End)
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "function Fn(){}"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy1_DefaultIdentifier(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default MyVariable"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for default identifier")
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "MyVariable"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy1_DefaultClass(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default class MyClass {}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for default class")
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "class MyClass {}"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy1_DefaultAsyncFunction(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default          async function Fn(){}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for default async function")
	}

	fixed := strings.TrimSpace(applyChange(source, results[0].Fix))
	expected := "async function Fn(){}"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy1_DefaultObject_Unsafe(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default {key: 'value'}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix != nil {
		t.Error("Expected Fix to be nil for default object export (unsafe to remove)")
	}
}

func TestFindUnusedExports_Strategy1_DefaultArray_Unsafe(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default [1, 2, 3]"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix != nil {
		t.Error("Expected Fix to be nil for default array export (unsafe to remove)")
	}
}

func TestFindUnusedExports_Strategy1_DefaultArrowFn_Unsafe(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export default () => {}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalExport([]KeywordInfo{
				{Name: "default"},
			}, 0, 6, 15),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix != nil {
		t.Error("Expected Fix to be nil for default arrow function export (unsafe to remove)")
	}
}

func TestFindUnusedExports_Strategy3_AllBraceUnused(t *testing.T) {
	// Create a temp file for Strategy 3 (reads source file)
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte("export { A, B };\nconst other = 1;\n"), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13},
			}, 0, 7, 15, 16),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Both should share the same fix (Strategy 3 — remove entire statement + line)
	hasFix := false
	for _, r := range results {
		if r.Fix != nil {
			hasFix = true
			// The fix should remove the entire first line "export { A, B };\n"
			if r.Fix.Start != 0 {
				t.Errorf("Expected Fix Start=0, got %d", r.Fix.Start)
			}
			if r.Fix.Text != "" {
				t.Errorf("Expected empty replacement text, got '%s'", r.Fix.Text)
			}
		}
	}
	if !hasFix {
		t.Error("Expected at least one result with a Fix")
	}
}

func TestFindUnusedExports_Strategy2_SurgicalRemoval_SingleLine(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	// export { A, B, C }
	source := "export { A, B, C }"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13, CommaAfter: 13},
				{Name: "C", Start: 15, End: 16},
			}, 0, 7, 18, 18),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "B"},
				{Name: "C"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].ExportName != "A" {
		t.Errorf("Expected 'A', got '%s'", results[0].ExportName)
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil for Strategy 2")
	}

	// Apply the fix to check the result
	fixed := applyChange(source, results[0].Fix)
	expected := "export { B, C }"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy2_RemoveMiddle(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export { A, B, C }"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13, CommaAfter: 13},
				{Name: "C", Start: 15, End: 16},
			}, 0, 7, 18, 18),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "A"},
				{Name: "C"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil")
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "export { A, C }"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy2_RemoveLast(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export { A, B, C }"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13, CommaAfter: 13},
				{Name: "C", Start: 15, End: 16},
			}, 0, 7, 18, 18),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "A"},
				{Name: "B"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil")
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "export { A, B }"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy2_MultiLine(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export {\n  A,\n  B,\n  C,\n}"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	// Positions: { at 7, A at 11, , at 12, B at 16, , at 17, C at 21, , at 22, } at 24
	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 11, End: 12, CommaAfter: 12},
				{Name: "B", Start: 16, End: 17, CommaAfter: 17},
				{Name: "C", Start: 21, End: 22, CommaAfter: 22},
			}, 0, 7, 25, 25),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "A"},
				{Name: "C"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil")
	}

	fixed := applyChange(source, results[0].Fix)
	expected := "export {\n  A,\n  C,\n}"
	if fixed != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy2_SemicolonPreserved(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export { A, B };"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13},
			}, 0, 7, 15, 16),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "B"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Fix == nil {
		t.Fatal("Expected Fix to be non-nil")
	}

	fixed := applyChange(source, results[0].Fix)
	// Semicolon should be preserved (it's outside the brace range)
	expected := "export { B };"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestFindUnusedExports_Strategy2_RemoveNonConsecutive(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "unused-exports-test")
	defer os.RemoveAll(tmpDir)
	autofix := true

	source := "export { A, B, C }"
	srcFile := filepath.Join(tmpDir, "utils.ts")
	os.WriteFile(srcFile, []byte(source), 0644)

	tree := MinimalDependencyTree{
		srcFile: {
			mkLocalBraceExport([]KeywordInfo{
				{Name: "A", Start: 9, End: 10, CommaAfter: 10},
				{Name: "B", Start: 12, End: 13, CommaAfter: 13},
				{Name: "C", Start: 15, End: 16},
			}, 0, 7, 18, 18),
		},
		filepath.Join(tmpDir, "consumer.ts"): {
			mkImportWithKeywords(srcFile, []KeywordInfo{
				{Name: "B"},
			}),
		},
	}

	results := FindUnusedExports(
		[]string{srcFile, filepath.Join(tmpDir, "consumer.ts")},
		tree,
		nil, nil, false, autofix, tmpDir+"/",
		nil,
	)

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	// Find result with Fix (Strategy 2 assigns fix)
	var fixResult *UnusedExport
	for i := range results {
		if results[i].Fix != nil {
			fixResult = &results[i]
			break
		}
	}
	if fixResult == nil {
		t.Fatal("Expected at least one result with Fix")
	}

	fixed := applyChange(source, fixResult.Fix)
	expected := "export { B }"
	if fixed != expected {
		t.Errorf("Expected '%s', got '%s'", expected, fixed)
	}
}

func TestAnyRuleChecksForUnusedExports(t *testing.T) {
	t.Run("returns false with no unused exports", func(t *testing.T) {
		config := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", CircularImportsDetection: &CircularImportsOptions{Enabled: true}},
			},
		}
		if anyRuleChecksForUnusedExports(config) {
			t.Error("Expected false")
		}
	})

	t.Run("returns true with unused exports enabled", func(t *testing.T) {
		config := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", UnusedExportsDetection: &UnusedExportsOptions{Enabled: true}},
			},
		}
		if !anyRuleChecksForUnusedExports(config) {
			t.Error("Expected true")
		}
	})

	t.Run("returns false with unused exports disabled", func(t *testing.T) {
		config := &RevDepConfig{
			Rules: []Rule{
				{Path: ".", UnusedExportsDetection: &UnusedExportsOptions{Enabled: false}},
			},
		}
		if anyRuleChecksForUnusedExports(config) {
			t.Error("Expected false")
		}
	})
}

// applyChange applies a single Change to a source string
func applyChange(source string, change *Change) string {
	b := []byte(source)
	result := make([]byte, 0, len(b))
	result = append(result, b[:change.Start]...)
	result = append(result, []byte(change.Text)...)
	result = append(result, b[change.End:]...)
	return string(result)
}
