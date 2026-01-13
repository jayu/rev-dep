# Smoke Test Enhancement Plan

## Overview
This document outlines a comprehensive plan to enhance the smoke test suite by adding tests that combine multiple CLI parameters for each command. The goal is to achieve non-empty assertions files while testing realistic parameter combinations.

## Current Test Coverage Analysis

### Existing Tests
- **circular**: Basic functionality
- **list-cwd-files**: Basic and --count
- **entry-points**: Basic, --print-deps-count
- **files**: Basic with entry-point, --count
- **resolve**: Basic with file, with entry-points
- **node-modules**: used, unused, missing
- **node-modules installed**: Basic, installed-duplicates
- **node-modules analyze**: analyze-size, dirs-size
- **lines-of-code**: Basic

## Test Structure Pattern

**IMPORTANT**: Follow the existing pattern from `main_smoke_test.go`:
- One test function per CLI command (e.g., `TestResolveCmd`, `TestEntryPoints`)
- Use `t.Run()` for each parameter combination
- Use the actual CLI command with parameters as the test name
- Do NOT create separate test functions for each parameter combination
- Use appropriate fixture directories that support the tested functionality

Example:
```go
func TestResolveCmd(t *testing.T) {
    t.Run("resolve --file src/types.ts --entry-points index.ts --graph-exclude 'src/nested/**'", func(t *testing.T) {
        mockProjectPath := filepath.Join("__fixtures__", "mockProject")
        // Test implementation
    })
}
```

## Fixture Strategy

**Primary Fixtures**:
- **mockProject**: Standard project with various file types and dependencies
- **mockMonorepo**: Multi-package monorepo for testing `--follow-monorepo-packages`
- **nodeModulesCmdSmoke**: For node-modules related tests

**File Path Realities**:
- No `.test.ts` or `.spec.ts` files exist in fixtures - use realistic patterns like `src/nested/**`, `**/*.d.ts`
- Use actual file paths that exist in the fixtures
- For monorepo tests, use paths like `packages/consumer-package/index.ts`

## Implementation Status Tracking

**Status Legend:**
- 🟡 **PENDING** - Test not yet implemented
- ✅ **DONE** - Test successfully implemented and working
- ❌ **FAILED** - Test implementation failed or has issues

**How to use:**
1. Each test has a status indicator before the code block
2. Update status as tests are implemented
3. Add notes about any issues or special considerations

---

## Planned Test Enhancements

### 1. Resolve Command Tests
**Current Parameters**: file, entry-points, graph-exclude, ignore-type-imports, all, compact-summary, package-json, tsconfig-json, condition-names, follow-monorepo-packages

🟡 **PENDING** - Multiple entry points with graph exclusion
```go
t.Run("resolve --file src/types.ts --entry-points index.ts,src/importFileA.ts --graph-exclude 'src/nested/**'", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Type imports with all paths
```go
t.Run("resolve --file src/types.ts --entry-points index.ts --ignore-type-imports --all", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Compact summary with conditions
```go
t.Run("resolve --file src/types.ts --entry-points index.ts --compact-summary --condition-names node,imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Custom config files
```go
t.Run("resolve --file src/types.ts --package-json custom.package.json --tsconfig-json custom.tsconfig.json", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

### 2. Entry Points Command Tests
**Current Parameters**: cwd, ignore-type-imports, count, print-deps-count, graph-exclude, result-exclude, result-include, package-json, tsconfig-json, condition-names, follow-monorepo-packages

🟡 **PENDING** - Exclude patterns with count
```go
t.Run("entry-points --ignore-type-imports --graph-exclude 'src/nested/**' --result-exclude '**/*.d.ts' --count", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Include patterns with deps count
```go
t.Run("entry-points --result-include 'src/**/*.ts' --print-deps-count --ignore-type-imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Conditions with monorepo
```go
t.Run("entry-points --condition-names node,imports --follow-monorepo-packages --print-deps-count", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Complex filtering with monorepo
```go
t.Run("entry-points --graph-exclude 'packages/exported-package/src/deep/**' --result-exclude '**/*.d.ts' --result-include 'packages/**/*.ts' --follow-monorepo-packages", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

### 3. Files Command Tests
**Current Parameters**: cwd, entry-point, ignore-type-imports, count, package-json, tsconfig-json, condition-names, follow-monorepo-packages

🟡 **PENDING** - Type imports with custom config
```go
t.Run("files --entry-point index.ts --ignore-type-imports --package-json custom.package.json", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Conditions with count in monorepo
```go
t.Run("files --entry-point packages/exported-package/src/main.ts --condition-names node,imports --follow-monorepo-packages --count", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Monorepo with type imports
```go
t.Run("files --entry-point packages/consumer-package/index.ts --follow-monorepo-packages --ignore-type-imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

### 4. Node Modules Command Tests
**Current Parameters**: cwd, ignore-type-imports, entry-points, count, group-by-module, group-by-file, pkg-fields-with-binaries, files-with-binaries, files-with-node-modules, include-modules, exclude-modules, package-json, tsconfig-json, condition-names, follow-monorepo-packages

🟡 **PENDING** - Grouping with entry points
```go
t.Run("node-modules used --entry-points index.ts,src/importFileA.ts --group-by-module --ignore-type-imports", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")
    // Test implementation
})
```

🟡 **PENDING** - File filters with conditions in monorepo
```go
t.Run("node-modules used --files-with-binaries fileWithBinary.txt --files-with-node-modules fileWithModule.txt --condition-names node,imports", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Exclude patterns with count
```go
t.Run("node-modules unused --exclude-modules @types/*,lodash-* --count --zero-exit-code", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")
    // Test implementation
})
```

🟡 **PENDING** - Missing modules with monorepo conditions
```go
t.Run("node-modules missing --entry-points packages/consumer-package/index.ts --condition-names node,imports --follow-monorepo-packages --zero-exit-code", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Include/exclude module patterns
```go
t.Run("node-modules installed --include-modules @myorg/* --exclude-modules @types/*", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")
    // Test implementation
})
```

🟡 **PENDING** - Optimization with stats
```go
t.Run("node-modules installed-duplicates --optimize --size-stats --verbose --isolate", func(t *testing.T) {
    nodeModulesPath := filepath.Join("__fixtures__", "nodeModulesCmdSmoke")
    // Test implementation
})
```

### 5. List CWD Files Command Tests
**Current Parameters**: cwd, include, exclude, count

🟡 **PENDING** - Include/exclude patterns
```go
t.Run("list-cwd-files --include 'src/**/*.ts' --exclude 'src/nested/**'", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Complex patterns with count
```go
t.Run("list-cwd-files --include 'src/**/*.ts' --exclude '**/*.d.ts' --count", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Multiple include patterns
```go
t.Run("list-cwd-files --include '**/*.ts' --include '**/*.js' --exclude 'src/nested/**'", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

### 6. Circular Command Tests
**Current Parameters**: cwd, ignore-type-imports, package-json, tsconfig-json, condition-names, follow-monorepo-packages

🟡 **PENDING** - Conditions with type imports in monorepo
```go
t.Run("circular --ignore-type-imports --condition-names node,imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

🟡 **PENDING** - Custom config files
```go
t.Run("circular --package-json custom.package.json --tsconfig-json custom.tsconfig.json", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "multipleCyclesFromSameNode")
    // Test implementation
})
```

🟡 **PENDING** - Monorepo with type imports
```go
t.Run("circular --follow-monorepo-packages --ignore-type-imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

### 7. Lines of Code Command Tests
**Current Parameters**: cwd

**New Test Combinations**:
```go
// Note: lines-of-code command only has cwd parameter, so no combinations possible
// Current test coverage is sufficient
```

### 8. Development Commands Tests (dev build only)
**Current Parameters**: browser (cwd, entry-point, ignore-type-imports), debug-parse-file (file, cwd), debug-get-tree-for-cwd (cwd, ignore-type-imports)

🟡 **PENDING** - Browser with type imports and custom config
```go
t.Run("browser --entry-point index.ts --ignore-type-imports --package-json custom.package.json", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Debug parse file with type imports
```go
t.Run("debug-parse-file --file src/types.ts --ignore-type-imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockProject")
    // Test implementation
})
```

🟡 **PENDING** - Debug tree with conditions in monorepo
```go
t.Run("debug-get-tree-for-cwd --ignore-type-imports --condition-names node,imports", func(t *testing.T) {
    mockProjectPath := filepath.Join("__fixtures__", "mockMonorepo")
    // Test implementation
})
```

## Test Structure Summary

**Key Points**:
- One test function per CLI command (e.g., `TestResolveCmd`, `TestEntryPoints`, `TestNodeModules`)
- Use `t.Run()` for each parameter combination within the command test function
- Use the actual CLI command with parameters as the test name string
- All tests should use the `captureOutput` function for CLI execution
- Each test should generate a corresponding golden file

## Golden Files Strategy

Each new test will have corresponding golden files:
- `resolve-with-multiple-entry-points.golden`
- `entry-points-exclude-patterns-count.golden`
- `node-modules-used-grouping-entry-points.golden`
- etc.

## Implementation Priority

### High Priority (Core Functionality)
1. **resolve** command with multiple entry-points and exclusion patterns
2. **entry-points** with complex filtering options
3. **node-modules used** with grouping and entry-points
4. **files** with custom configuration files

### Medium Priority (Advanced Features)
1. **node-modules unused/missing** with exclude/include patterns
2. **list-cwd-files** with complex glob patterns
3. **circular** with type imports and conditions

### Low Priority (Development Tools)
1. **browser** command variations
2. **debug-parse-file** with type imports
3. **debug-get-tree-for-cwd** with conditions

## Test Data Requirements

New fixture directories may be needed for:
- Projects with multiple entry points
- Projects with custom package.json/tsconfig.json
- Projects with complex file structures for glob pattern testing
- Monorepo-style projects for follow-monorepo-packages testing

## Expected Outcomes

1. **Increased Coverage**: From ~15 test cases to ~40+ test cases
2. **Better Parameter Validation**: Ensure parameter combinations work correctly
3. **Regression Prevention**: Catch issues with parameter interactions
4. **Documentation**: Tests serve as usage examples
5. **Confidence**: Higher confidence in CLI parameter handling

## Notes

- Each test should use the `captureOutput` function to handle CLI execution
- Tests should use appropriate fixture directories that support the tested functionality
- Golden files should be generated once and then version controlled
- Tests should focus on realistic parameter combinations that users would actually use
- Avoid testing every possible combination (combinatorial explosion) - focus on meaningful scenarios
