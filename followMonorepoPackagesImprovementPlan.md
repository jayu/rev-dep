# Enhance `followMonorepoPackages` to Accept Package List or Boolean

## Objective

Extend `followMonorepoPackages` from a boolean-only switch to a union type:

- `true`: follow all discovered workspace packages
- `false`: follow none
- `["pkg-a", "@scope/*"]`: follow only matching package names

This change applies to both config-driven execution and CLI-driven execution, while preserving current default behavior:

- Config default remains `true` (existing behavior)
- CLI default remains `false` unless flag is passed (existing behavior)

## Why This Matters

Large monorepos frequently mix:

- source-first packages (safe to follow)
- build-artifact packages (unsafe/noisy to follow)
- generated or private tooling packages

Following everything over-resolves dependencies and reduces signal quality. Selective follow allows users to keep resolution focused.

## Scope

### In Scope

- Config model update for rule-level `followMonorepoPackages`
- Config parser and validator updates
- JSON schema update
- CLI flag parsing update
- Resolver behavior update for selective package matching
- Config processor update for selective filtering
- Debug output update
- Tests for parsing, behavior, and compatibility

### Out of Scope

- New config keys beyond `followMonorepoPackages`
- Package metadata discovery redesign
- Performance optimization beyond what is required for correctness

## Design Decisions

1. A dedicated value type is used internally instead of `interface{}` to avoid type assertions spread across code.
2. Package list matching uses existing glob semantics already used in the project (do not introduce a new matcher style).
3. Empty array in config is invalid (`minItems: 1`) to avoid ambiguous "enabled but no targets" semantics.
4. CLI behavior must preserve existing UX:
   - no flag: disabled
   - bare flag: enabled for all
   - flag with values: selective mode

## Proposed Data Model

### File

- `config.go`

### Changes

1. Add type:

```go
type FollowMonorepoPackagesValue struct {
    FollowAll bool
    Packages  []string
}
```

2. Add helper methods:

- `IsEnabled() bool`  
  Returns `true` when `FollowAll == true` or `len(Packages) > 0`.
- `ShouldFollowAll() bool`  
  Returns `FollowAll`.
- `ShouldFollowPackage(name string) bool`  
  Returns `true` when `FollowAll` is true, otherwise evaluates package globs.

3. Update `Rule`:

- from: `FollowMonorepoPackages bool`
- to: `FollowMonorepoPackages FollowMonorepoPackagesValue`

4. Keep raw JSON decode path explicit (`json:"-"`) and parse from raw map in config parsing pipeline.

## Parsing and Validation

### File

- `config.go`

### Parser Behavior (`ParseConfig`)

For each rule:

1. If `followMonorepoPackages` is missing: set `FollowAll: true`.
2. If boolean:
   - `true` -> `FollowAll: true`
   - `false` -> zero value (disabled)
3. If array of strings:
   - normalize entries (`trim`)
   - reject empty string entries
   - set `Packages: []string{...}`
4. Reject any other type with clear field-specific error.

### Validator Behavior (`validateRawRule`)

Accept only:

- `bool`
- array where every element is `string`

Reject:

- numbers, objects, null, mixed arrays, empty array, empty strings

Error style should include:

- rule index (or name if available)
- field name `followMonorepoPackages`
- expected type: `boolean or array of strings`

## Schema Changes

### File

- `config-schema/1.3.schema.json`

Update property to:

```json
"followMonorepoPackages": {
  "oneOf": [
    { "type": "boolean" },
    {
      "type": "array",
      "items": { "type": "string", "minLength": 1 },
      "minItems": 1
    }
  ],
  "description": "Whether and which monorepo packages to follow. true=all, false=none, array=specific package-name globs.",
  "default": true
}
```

## CLI Changes

### File

- `main.go`

### Flag Contract

Use `StringSliceVar` (same flag name `--follow-monorepo-packages`) and map parsed state to `FollowMonorepoPackagesValue`.

Interpretation:

- flag absent -> disabled (legacy CLI default)
- flag present with no values -> `FollowAll: true`
- flag present with values -> `Packages: values`

> Note that this might be trickly to implement with current cli flag parsing library

### Implementation Note

To distinguish "flag absent" vs "present but empty", track flag set state explicitly (e.g., via `Flags().Changed(...)` or dedicated boolean set by custom flag.Value wrapper).

### Signature Updates

Replace `followMonorepoPackages bool` with `FollowMonorepoPackagesValue` across:

- `resolveCmdFn`
- `entryPointsCmdFn`
- `circularCmdFn`
- `filesCmdFn`
- `importedByCmdFn`
- `unresolvedCmdRun`
- `getUnresolvedOutput`
- `GetMinimalDepsTreeForCwd`
- call sites in `devCommands.go`
- `ResolveImports`
- `NewResolverManager`

## Resolver Logic

### File

- `resolveImports.go`

### Internal Manager

Change manager field:

- from: `followMonorepoPackages bool`
- to: `followMonorepoPackages FollowMonorepoPackagesValue`

### Behavior

1. Disabled mode (`IsEnabled()==false`):
   - do not create subpackage resolvers
2. Follow-all mode (`FollowAll==true`):
   - preserve existing behavior
3. Selective mode (`len(Packages)>0`):
   - detect workspace packages as usual
   - create subpackage resolvers only for matched package names
   - unmatched workspace package imports should resolve as external/non-followed

### Matching Rules

- Match against declared package name from package metadata (not filesystem path)
- Glob matching should be deterministic and case-sensitive (match existing project conventions)

## Config Processor Logic

### File

- `configProcessor.go`

### `buildDependencyTreeForConfig`

Keep comprehensive analysis baseline if needed by existing logic, but ensure per-rule filtering applies before final output.

### `filterFilesForRule`

Update function signature to accept `FollowMonorepoPackagesValue` and `resolverManager` so selective package filtering can be done using workspace package path metadata.

Selected implementation (Phase 1):

1. Build rule graph exactly as today from rule-path files:
   - `graph := buildDepsGraphForMultiple(fullTree, filesWithinRule, nil, false)`
2. Derive allowed workspace package paths from `resolverManager.monorepoContext.PackageToPath` using package-name matching from `FollowMonorepoPackagesValue`.
3. Keep graph vertices that satisfy at least one condition:
   - file is under the rule path
   - file is under an allowed workspace package path
4. Build `ruleFiles` and `ruleTree` from that filtered vertex set.

Important note:

- No edge rewrite is planned in this phase. We keep current semantics and scope this change to vertex/package subset selection only.
- If later we find check-level leakage from deps pointing to filtered-out files, that will be a follow-up hardening task.

## Debug and Diagnostics

### File

- `debugUtils.go`

Update resolver manager stringify output so mode is explicit:

- `followMonorepoPackages=disabled`
- `followMonorepoPackages=all`
- `followMonorepoPackages=selective:[pkg-a,@scope/*]`

This avoids ambiguity during regression debugging.

## Backward Compatibility

### Config

- Existing `true`/`false` values remain valid.
- Omitted value remains default `true`.

### CLI

- Existing usage `--follow-monorepo-packages` keeps same effect (follow all).
- Not passing flag keeps disabled default.

### Potential Breaking Edge

- If any user currently passes empty array in config (unlikely and previously invalid by type), it will now be explicitly rejected with clear error.

## Detailed Implementation Plan

### Phase 1: Type + Parsing + Validation

1. Add `FollowMonorepoPackagesValue` and helpers in `config.go`.
2. Update `Rule` type.
3. Update `ParseConfig` mapping logic.
4. Update raw validation for union type.
5. Update schema file.

### Phase 2: CLI Plumbing

1. Replace bool flag variable with slice + changed-state detection.
2. Convert parsed flag state into `FollowMonorepoPackagesValue`.
3. Propagate new type through command entrypoints and helper functions.

### Phase 3: Resolver + Processor Behavior

1. Update `ResolverManager` field and constructor.
2. Add selective package filtering in resolver setup.
3. Implement selective package subset in `filterFilesForRule` using post-graph vertex filtering by allowed package paths.
4. Update debug output formatting.

### Phase 4: Tests + Regression Pass

1. Update compile-break call sites.
2. Add/adjust tests listed below.
3. Run focused tests then full suite.
4. Validate smoke scenarios manually with fixture configs.

## Test Plan

### Unit Tests: Config Parsing

File: `config_test.go`

- parse `true`
- parse `false`
- parse omitted value (default true)
- parse selective array (`["pkg-a","@scope/*"]`)
- reject mixed array (`["pkg-a",1]`)
- reject empty array (`[]`)
- reject invalid type (`{}` / `"yes"`)
- reject array containing empty string

### Unit Tests: Resolver Behavior

Files: `monorepo_resolution_test.go`, `resolveImports_test.go`, `nodeModules_test.go`

- follow-all still resolves workspace package imports
- disabled mode does not follow workspace packages
- selective mode follows only matching package names
- wildcard selective mode (`@scope/*`) behaves correctly
- unmatched package remains unresolved/external as expected

### Unit Tests: Config Processor Filtering

File: `configProcessor_test.go` (or new focused test file)

- selective rule keeps files under rule path + allowed package paths only
- disallowed workspace package files are excluded from `ruleFiles`
- `follow=false` behavior unchanged
- `follow=true` behavior unchanged
- wildcard pattern selects matching package paths correctly

### CLI/Smoke Tests

File: `main_smoke_test.go`

- no flag -> disabled
- bare flag -> all
- flag values -> selective
- ensure unchanged behavior for existing boolean-like workflows

### Dev Command Compilation

File: `devCommands.go`

- ensure all call sites compile and preserve previous defaults where intended

## Verification Commands

```bash
cd /Users/jakubmazurek/Programming/rev-dep && go test -run TestParseConfig_FollowMonorepoPackages -v
cd /Users/jakubmazurek/Programming/rev-dep && go test ./...
cd /Users/jakubmazurek/Programming/rev-dep && go build ./...
```

## Risk Register

1. CLI changed-state detection may be implemented incorrectly, causing absent flag to behave like bare flag.
2. Resolver selective filtering may still instantiate full resolver set, causing hidden perf/correctness regressions.
3. Path-prefix package filtering may leave deps in kept files that reference filtered-out vertices.
4. Existing tests may encode old bool signatures and need broad updates.

## Mitigations

1. Add explicit test for flag absent vs present-empty.
2. Add debug snapshot assertions for resolver manager mode.
3. Keep filtering logic centralized in `filterFilesForRule` and add targeted tests that assert returned `ruleFiles` subset.
4. Run full suite and inspect failures for semantic drift, not only compile fixes.

## Definition of Done

1. All bool-based signatures migrated to `FollowMonorepoPackagesValue`.
2. Config accepts boolean or string array with strict validation.
3. CLI behavior matches compatibility contract.
4. Selective monorepo follow works in resolver and downstream filtering.
5. Debug output clearly indicates follow mode.
6. All tests pass (`go test ./...`) and project builds (`go build ./...`).
