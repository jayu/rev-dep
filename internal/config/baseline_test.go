package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rev-dep-go/internal/baseline"
	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/model"
	"rev-dep-go/internal/node"
)

func TestBaselineSnapshotPathParsing(t *testing.T) {
	for _, value := range []string{`123`, `null`, `false`, `""`, `"  "`} {
		_, err := ParseConfig([]byte(`{"configVersion":"2.2","baselineSnapshotPath":` + value + `,"workspaces":[{"path":"."}]}`))
		if err == nil || !strings.Contains(err.Error(), "baselineSnapshotPath") {
			t.Fatalf("accepted invalid path %s: %v", value, err)
		}
	}
	cfg, err := ParseConfig([]byte(`{"configVersion":"2.2","baselineSnapshotPath":"snapshots/findings.json","workspaces":[{"path":"."}]}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"baselineSnapshotPath":"snapshots/findings.json"`) {
		t.Fatal("path not preserved through marshal")
	}
}

func allBaselineIssues(root string) RuleResult {
	f := filepath.Join(root, "src", "main.ts")
	target := filepath.Join(root, "src", "private.ts")
	return RuleResult{
		RulePath: ".", FileCount: 2,
		CircularDependencies: [][]string{{f, target, f}}, CircularComponents: [][]string{{target, f}},
		OrphanFiles:                              []string{target, target},
		ModuleBoundaryViolations:                 []checks.ModuleBoundaryViolation{{FilePath: f, ImportPath: target, RuleName: "rule one", ViolationType: "denied"}, {FilePath: f, ImportPath: target, RuleName: "rule two", ViolationType: "not_allowed"}},
		MissingNodeModules:                       []node.MissingNodeModuleResult{{ModuleName: "missing", ImportedFrom: []string{f, f}}},
		UnusedNodeModules:                        []node.UnusedNodeModuleIssue{{ModuleName: "unused", PackageJsonPath: filepath.Join(root, "package.json")}},
		UnusedExports:                            []checks.UnusedExport{{FilePath: f, ExportName: "unused", IsType: true}},
		UnresolvedImports:                        []checks.UnresolvedImport{{FilePath: f, Request: "./missing"}, {FilePath: f, Request: " name + '.js' ", IsNonLiteral: true}},
		ImportConventionViolations:               []checks.ImportConventionViolation{{FilePath: f, ImportRequest: "@bad/path", ViolationType: "wrong-alias", ImportIndex: 3}},
		RestrictedDevDependenciesUsageViolations: []checks.RestrictedDevDependenciesUsageViolation{{FilePath: f, DevDependency: "dev", EntryPoint: "selected witness"}},
		RestrictedImportsViolations:              []checks.RestrictedImportViolation{{ViolationType: "file", DeniedFile: target, EntryPoint: "selected witness"}, {ViolationType: "module", ImportRequest: "node:fs/promises", ImporterFile: f}},
		RestrictedImportersViolations:            []checks.RestrictedImporterViolation{{EntryPoint: f, File: target}, {EntryPoint: f, Module: "restricted"}},
		RestrictedDirectImportersViolations:      []checks.RestrictedDirectImporterViolation{{ViolationType: "file", ImporterFile: f, File: target}, {ViolationType: "module", ImporterFile: f, ImportRequest: "restricted/subpath"}},
	}
}

func TestBaselineAdaptersAndPostProcessing(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	capture := &ConfigProcessingResult{HasFailures: true, RuleResults: []RuleResult{allBaselineIssues(root)}}
	if err := ApplyFindingsBaseline(&cfg, root, capture, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	recorded, err := baseline.Load(filepath.Join(root, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recorded.Workspaces["."]) != len(baseline.Detectors()) {
		t.Fatalf("adapters missing a detector: %+v", recorded.Workspaces)
	}
	if len(recorded.Findings(nil)) != 16 {
		t.Fatalf("unexpected deduplication: %d findings", len(recorded.Findings(nil)))
	}
	// Raw check processing is unaffected; filtering is an explicit subsequent layer.
	raw := allBaselineIssues(root)
	raw.ImportConventionViolations[0].ImportIndex = 99
	raw.UnresolvedImports[0].RequestStart = 999
	raw.RestrictedDevDependenciesUsageViolations[0].EntryPoint = "different witness"
	raw.ModuleBoundaryViolations[0].RuleName = "renamed"
	raw.CircularDependencies = [][]string{{raw.CircularComponents[0][0], raw.CircularComponents[0][0]}}
	run := &ConfigProcessingResult{HasFailures: true, RuleResults: []RuleResult{raw}}
	if err := ApplyFindingsBaseline(&cfg, root, run, recorded, false, nil); err != nil {
		t.Fatal(err)
	}
	if run.HasFailures || run.Baseline.Status != "matched" || len(run.RuleResults[0].UnresolvedImports) > 0 {
		t.Fatalf("acknowledged issues not filtered: %+v", run.Baseline)
	}
	// Replacing an issue is a removal and addition, regardless of equal totals.
	raw = allBaselineIssues(root)
	raw.MissingNodeModules[0].ModuleName = "different"
	run = &ConfigProcessingResult{RuleResults: []RuleResult{raw}}
	if err := ApplyFindingsBaseline(&cfg, root, run, recorded, false, nil); err != nil {
		t.Fatal(err)
	}
	if !run.HasFailures || run.Baseline.Counts.New != 1 || run.Baseline.Counts.Obsolete != 1 || len(run.RuleResults[0].MissingNodeModules) != 1 {
		t.Fatalf("wrong replacement delta: %+v", run.Baseline)
	}
	// No current workspace/check rows still removes every recorded identity.
	run = &ConfigProcessingResult{}
	if err := ApplyFindingsBaseline(&cfg, root, run, recorded, false, nil); err != nil {
		t.Fatal(err)
	}
	if run.Baseline.Counts.Obsolete != 16 || !run.HasFailures {
		t.Fatalf("removed scopes ignored: %+v", run.Baseline)
	}
}

func TestBaselineDuplicatesAreIndependent(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	failed := DuplicatedCodeRuleResult{Result: checks.DuplicatedCodeResult{Snippets: 1}}
	run := &ConfigProcessingResult{RuleResults: []RuleResult{{RulePath: ".", FileCount: 1, DuplicatedCode: []DuplicatedCodeRuleResult{failed}}}}
	if err := ApplyFindingsBaseline(&cfg, root, run, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	if !run.HasFailures || len(run.RuleResults[0].DuplicatedCode) != 1 {
		t.Fatal("duplication was acknowledged/filtered")
	}
	saved, err := baseline.Load(filepath.Join(root, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Workspaces) != 0 {
		t.Fatal("duplication leaked into general baseline")
	}
}

func TestSCCBaselineIgnoresInternalEdges(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a.ts")
	b := filepath.Join(root, "b.ts")
	c := filepath.Join(root, "c.ts")
	tree := model.MinimalDependencyTree{a: {{ID: b}, {ID: c}}, b: {{ID: a}}, c: {{ID: a}}}
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	result := func() *ConfigProcessingResult {
		found := checks.FindCircularDependenciesWithComponents(tree, []string{a, b, c}, false)
		return &ConfigProcessingResult{RuleResults: []RuleResult{{RulePath: ".", FileCount: 3, CircularDependencies: found.Cycles, CircularComponents: found.Components}}}
	}
	initial := result()
	if len(initial.RuleResults[0].CircularDependencies[0]) != 3 {
		t.Fatal("fixture should display a cycle omitting a member")
	}
	if err := ApplyFindingsBaseline(&cfg, root, initial, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	saved, _ := baseline.Load(filepath.Join(root, "snapshot.json"))
	if got := saved.Workspaces["."]["circular-imports"][0].Files; !reflect.DeepEqual(got, []string{"a.ts", "b.ts", "c.ts"}) {
		t.Fatalf("not all SCC members captured: %v", got)
	}
	tree[b] = []model.MinimalDependency{{ID: c}}
	next := result()
	if err := ApplyFindingsBaseline(&cfg, root, next, saved, false, nil); err != nil {
		t.Fatal(err)
	}
	if next.HasFailures {
		t.Fatalf("internal edge changed SCC identity: %+v", next.Baseline)
	}
	tree[c] = nil
	next = result()
	if err := ApplyFindingsBaseline(&cfg, root, next, saved, false, nil); err != nil {
		t.Fatal(err)
	}
	if !next.HasFailures || next.Baseline.Counts.Obsolete != 1 {
		t.Fatal("resolved SCC was not removed")
	}
}

func TestBaselineRejectsOverlappingDestinations(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{SourcePath: filepath.Join(root, "config.json"), BaselineSnapshotPath: "config.json"}
	if _, err := BaselineSnapshotFile(&cfg, root); err == nil {
		t.Fatal("would overwrite config")
	}
	cfg.BaselineSnapshotPath = "snapshot.json"
	cfg.Rules = []Rule{{Path: ".", DuplicatedCodeDetections: []*DuplicatedCodeOptions{{Enabled: true, SnapshotPath: "snapshot.json"}}}}
	if _, err := BaselineSnapshotFile(&cfg, root); err == nil {
		t.Fatal("would overwrite duplication snapshot")
	}
	// Failed captures preserve an existing file.
	cfg.Rules = nil
	if err := os.WriteFile(filepath.Join(root, "snapshot.json"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	run := &ConfigProcessingResult{RuleResults: []RuleResult{{RulePath: ".", FailOnEmptyWorkspace: true}}}
	if err := ApplyFindingsBaseline(&cfg, root, run, nil, true, nil); err == nil {
		t.Fatal("captured failed empty workspace")
	}
	raw, _ := os.ReadFile(filepath.Join(root, "snapshot.json"))
	if string(raw) != "existing" {
		t.Fatal("failure replaced existing file")
	}
}

func TestBaselineRejectsUnknownEnabledDetector(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	run := &ConfigProcessingResult{RuleResults: []RuleResult{{RulePath: ".", FileCount: 1, EnabledChecks: []string{"future-detector"}}}}
	if err := ApplyFindingsBaseline(&cfg, root, run, nil, true, nil); err == nil {
		t.Fatal("future detector silently omitted")
	}
}

func TestCurrentConfigSchemaDeclaresBaselineField(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "config-schema", CurrentConfigVersion+".schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Properties["baselineSnapshotPath"] == nil {
		t.Fatal("current config schema missing baselineSnapshotPath")
	}
}

func TestBaselineSnapshotLocationIsIndependentOfWorkspace(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{SourcePath: filepath.Join(root, "rev-dep.config.json"), BaselineSnapshotPath: "snapshots/findings.json", Rules: []Rule{{Path: "packages/app"}}}
	filename, err := BaselineSnapshotFile(&cfg, root)
	if err != nil || filename != filepath.Join(root, "snapshots/findings.json") {
		t.Fatalf("path was not relative to config: %s %v", filename, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	cfg.BaselineSnapshotPath = outside
	filename, err = BaselineSnapshotFile(&cfg, root)
	if err != nil || filename != outside {
		t.Fatalf("absolute snapshot path was rewritten: %s %v", filename, err)
	}
	cfg.Rules = []Rule{{Path: "packages/app", DuplicatedCodeDetections: []*DuplicatedCodeOptions{{Enabled: true, SnapshotPath: outside}}}}
	if _, err := BaselineSnapshotFile(&cfg, root); err == nil {
		t.Fatal("absolute snapshot collision was missed")
	}
}

func TestBaselineCaptureRequiresSemanticLocation(t *testing.T) {
	root := t.TempDir()
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	run := &ConfigProcessingResult{RuleResults: []RuleResult{{RulePath: ".", FileCount: 1, UnusedNodeModules: []node.UnusedNodeModuleIssue{{ModuleName: "unused"}}}}}
	if err := ApplyFindingsBaseline(&cfg, root, run, nil, true, nil); err == nil {
		t.Fatal("missing owning manifest was replaced with a fabricated root path")
	}
	if _, err := os.Stat(filepath.Join(root, "snapshot.json")); !os.IsNotExist(err) {
		t.Fatal("invalid capture wrote a snapshot")
	}
}

func TestBaselineIdentitiesSurviveCheckoutMove(t *testing.T) {
	firstRoot, secondRoot := t.TempDir(), t.TempDir()
	cfg := RevDepConfig{BaselineSnapshotPath: "snapshot.json"}
	first := &ConfigProcessingResult{RuleResults: []RuleResult{allBaselineIssues(firstRoot)}}
	if err := ApplyFindingsBaseline(&cfg, firstRoot, first, nil, true, nil); err != nil {
		t.Fatal(err)
	}
	recorded, err := baseline.Load(filepath.Join(firstRoot, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	moved := &ConfigProcessingResult{RuleResults: []RuleResult{allBaselineIssues(secondRoot)}}
	if err := ApplyFindingsBaseline(&cfg, secondRoot, moved, recorded, false, nil); err != nil {
		t.Fatal(err)
	}
	if moved.HasFailures || moved.Baseline.Counts.Acknowledged != 16 {
		t.Fatalf("checkout paths leaked into identities: %+v", moved.Baseline)
	}
}

func TestBaselineFilteringPreservesLiveDiagnosticOrder(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.ts")
	cfg := RevDepConfig{BaselineSnapshotPath: "baseline.json"}
	result := &ConfigProcessingResult{RuleResults: []RuleResult{{
		RulePath: ".", FileCount: 1,
		UnresolvedImports: []checks.UnresolvedImport{
			{FilePath: file, Request: "./z", RequestStart: 10},
			{FilePath: file, Request: "./acknowledged", RequestStart: 20},
			{FilePath: file, Request: "./a", RequestStart: 30},
			{FilePath: file, Request: "./z", RequestStart: 40},
		},
		MissingNodeModules: []node.MissingNodeModuleResult{
			{ModuleName: "z-package", ImportedFrom: []string{file}},
			{ModuleName: "a-package", ImportedFrom: []string{file}},
		},
	}}}
	previous := baseline.New()
	previous.Add(".", "unresolved-imports", baseline.Issue{File: "main.ts", Request: "./acknowledged", Kind: "literal"})
	if err := ApplyFindingsBaseline(&cfg, root, result, previous, false, nil); err != nil {
		t.Fatal(err)
	}
	imports := result.RuleResults[0].UnresolvedImports
	if len(imports) != 2 || imports[0].Request != "./z" || imports[0].RequestStart != 10 || imports[1].Request != "./a" || imports[1].RequestStart != 30 {
		t.Fatalf("filter changed order or selected a later duplicate: %+v", imports)
	}
	modules := result.RuleResults[0].MissingNodeModules
	if len(modules) != 2 || modules[0].ModuleName != "z-package" || modules[1].ModuleName != "a-package" {
		t.Fatalf("filter changed detector-array result order: %+v", modules)
	}
	// Storage/comparison order remains canonical independently of display order.
	current := baseline.New()
	current.Add(".", "unresolved-imports", baseline.Issue{File: "main.ts", Request: "./z", Kind: "literal"})
	current.Add(".", "unresolved-imports", baseline.Issue{File: "main.ts", Request: "./a", Kind: "literal"})
	if err := current.Canonicalize(); err != nil {
		t.Fatal(err)
	}
	if current.Workspaces["."]["unresolved-imports"][0].Request != "./a" {
		t.Fatal("snapshot canonical order changed")
	}
}
