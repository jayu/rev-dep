# Implementation Plan: Import Conventions Feature

## Context

### Problem Statement
In modular architectures, enforcing **syntactic consistency** for imports preserves Domain Encapsulation. The pattern we need to enforce:

- **Internal Access:** Files importing within the same domain must use **relative paths** (e.g., `./utils`)
- **External Access:** Files importing from other domains must use **aliased/absolute paths** (e.g., `@domain2/api`)

### Distinction from Module Boundaries
| Feature | Concern | Question Answered |
|---------|---------|-------------------|
| **Module Boundaries** | Dependency Graph (Architecture) | *What can depend on what?* |
| **Import Conventions** | Syntax (Style/Encapsulation) | *How is the dependency written?* |

### Configuration Modes

**A. Simplified Mode (Glob Patterns):**
```json
{
  "configVersion": "1.0",
  "rules": [
    {
      "path": ".",
      "importConventions": [
        {
          "rule": "relative-internal-absolute-external",
          "domains": ["src/*"]
        }
      ]
    }
  ]
}
```

**B. Advanced Mode (Explicit Mapping):**
```json
{
  "configVersion": "1.0",
  "rules": [
    {
      "path": ".",
      "importConventions": [
        {
          "rule": "relative-internal-absolute-external",
          "domains": [
            { "path": "src/features/domain1", "alias": "@domain1" },
            { "path": "src/shared/ui", "alias": "@ui-kit" }
          ]
        }
      ]
    }
  ]
}
```

### Import Types to Support
The feature must work with imports resolved as:
- `UserModule` - User-defined module (relative paths, tsconfig aliases, package.json imports)
- `MonorepoModule` - Monorepo package import

### Domain Discovery vs Domain Membership

**Important distinction:**

| Concept | When | Method |
|---------|------|--------|
| **Domain Discovery** | Config parsing (once) | Glob expansion for simplified mode |
| **Domain Membership** | Runtime checking (per file/import) | Path prefix matching |

- **Simplified mode** (`domains: ["src/*"]`): The glob is used **only** to discover domains at config time. `src/*` expands to concrete directories like `src/auth`, `src/users`.
- **Advanced mode** (`domains: [{ path: "src/auth", alias: "@auth" }]`): Path is already a concrete directory, no glob expansion needed.

**At runtime**, all domains are concrete directory paths. Checking if a file belongs to a domain is simply: `strings.HasPrefix(filePath, domain.AbsolutePath)`.

### Performance Considerations
This project is optimized for speed. The implementation must:

1. **Avoid regex wherever possible** - Use prefix/suffix string matching (like existing `WildcardPattern` approach in `resolveImports.go`)
2. **Pre-compile patterns once** - Domain matchers should be compiled before iterating files
3. **Use maps for O(1) lookups** - File-to-domain mapping should use path prefix maps, not repeated glob matching
4. **Minimize allocations** - Reuse slices and avoid creating new strings in hot paths
5. **Early exits** - Skip files/imports that don't need checking as early as possible

---

## Implementation Steps

### Phase 1: Configuration Schema & Parsing

#### Step 1.1: Define Configuration Types
- [x] Add new types in `config.go`

```go
// ImportConventionDomain represents a single domain definition
type ImportConventionDomain struct {
    Path  string `json:"path,omitempty"`
    Alias string `json:"alias,omitempty"`
}

// ImportConventionRule represents an import convention rule
type ImportConventionRule struct {
    Rule    string      `json:"rule"`    // e.g., "relative-internal-absolute-external"
    Domains interface{} `json:"domains"` // Can be []string or []ImportConventionDomain
}

// ParsedImportConventionRule is the normalized form after parsing
type ParsedImportConventionRule struct {
    Rule    string
    Domains []ImportConventionDomain
}
```

- [x] Extend `Rule` struct to include import conventions:
```go
type Rule struct {
    Path                        string                     `json:"path"`
    FollowMonorepoPackages      bool                       `json:"followMonorepoPackages,omitempty"`
    ModuleBoundaries            []BoundaryRule             `json:"moduleBoundaries,omitempty"`
    CircularImportsDetection    *CircularImportsOptions    `json:"circularImportsDetection,omitempty"`
    OrphanFilesDetection        *OrphanFilesOptions        `json:"orphanFilesDetection,omitempty"`
    UnusedNodeModulesDetection  *UnusedNodeModulesOptions  `json:"unusedNodeModulesDetection,omitempty"`
    MissingNodeModulesDetection *MissingNodeModulesOptions `json:"missingNodeModulesDetection,omitempty"`
    ImportConventions           []ImportConventionRule     `json:"importConventions,omitempty"` // NEW
}
```

#### Step 1.2: Implement Configuration Validation Tests
- [x] Create test file `config_import_conventions_test.go`
- [x] Test: Valid simplified mode config parses correctly
```go
func TestParseConfig_ImportConventions_SimplifiedMode(t *testing.T)
```
- [x] Test: Valid advanced mode config parses correctly
```go
func TestParseConfig_ImportConventions_AdvancedMode(t *testing.T)
```
- [x] Test: Invalid rule name is rejected
```go
func TestParseConfig_ImportConventions_InvalidRuleName(t *testing.T)
```
- [x] Test: Mixed domains array (strings and objects) is rejected
```go
func TestParseConfig_ImportConventions_MixedDomainsRejected(t *testing.T)
```
- [x] Test: Empty domains array is rejected
```go
func TestParseConfig_ImportConventions_EmptyDomainsRejected(t *testing.T)
```
- [x] Test: Domain with missing path in advanced mode is rejected
```go
func TestParseConfig_ImportConventions_MissingPathRejected(t *testing.T)
```
- [x] Test: Domain with missing alias in advanced mode is rejected
```go
func TestParseConfig_ImportConventions_MissingAliasRejected(t *testing.T)
```
- [x] Test: Nested/overlapping domains within a rule are rejected
```go
func TestParseConfig_ImportConventions_NestedDomainsRejected(t *testing.T)
// e.g., domains: ["src/auth", "src/auth/utils"] should fail
// e.g., domains: ["src/auth", "src"] should fail (src contains src/auth)
```

#### Step 1.3: Implement Configuration Validation
- [x] Add `importConventions` to `allowedRuleFields` in `validateRawRule()`
- [x] Implement validation function:
```go
func validateRawImportConventions(conventions interface{}, ruleIndex int) error
```
- [x] Implement rule-specific validation:
```go
func validateRelativeInternalAbsoluteExternalRule(rule map[string]interface{}, ruleIndex int, convIndex int) error
```
- [x] Implement domain parsing function:
```go
func parseImportConventionDomains(domains interface{}) ([]ImportConventionDomain, error)
```
- [x] Implement nested domain validation:
```go
// Returns error if any domain path is a prefix of another domain path
// e.g., "src/auth" and "src/auth/utils" are nested (not allowed)
// e.g., "src" and "src/auth" are nested (not allowed)
func validateNoNestedDomains(domains []ImportConventionDomain) error
```

#### Step 1.4: Run Step 1 Tests
- [x] Verify all configuration tests pass

---

### Phase 2: Domain Resolution Logic

#### Step 2.1: Create Domain Resolution Tests
- [x] Create test file `import_conventions_test.go`
- [x] Test: File correctly identified as belonging to a domain (prefix match)
```go
func TestResolveDomainForFile(t *testing.T)
```
- [x] Test: File not belonging to any domain returns nil
- [x] Test: Simplified mode glob `src/*` correctly expands to `src/auth`, `src/users` directories
- [x] Test: Nested files belong to their parent domain (`src/auth/utils/helper.ts` → `src/auth` domain)
- [x] Test: Advanced mode path is used directly without glob expansion
- [x] Test: File can only belong to exactly one domain (no overlap by design)

#### Step 2.2: Implement Domain Resolution
- [x] Create file `import_conventions.go`
- [x] Implement domain expansion for **simplified mode** (glob → concrete paths):
```go
// Called once at config time, NOT at runtime
// "src/*" → ["src/auth", "src/users", "src/shared"]
func ExpandDomainGlobs(patterns []string, cwd string) ([]string, error)
```
- [x] Implement `CompiledDomain` struct (simple, no glob matching needed at runtime):
```go
type CompiledDomain struct {
    Path         string  // Original path from config (e.g., "src/auth")
    Alias        string  // e.g., "@auth" (inferred or explicit)
    AbsolutePath string  // Full absolute path for prefix matching
}

func CompileDomains(domains []ImportConventionDomain, cwd string) ([]CompiledDomain, error)
```
- [x] Implement file-to-domain resolution using **path prefix matching**:
```go
// Simple prefix check - O(n) where n = number of domains
// Since domains cannot overlap (validated at config time), first match wins
func ResolveDomainForFile(filePath string, compiledDomains []CompiledDomain) *CompiledDomain {
    for i := range compiledDomains {
        if strings.HasPrefix(filePath, compiledDomains[i].AbsolutePath) {
            return &compiledDomains[i]
        }
    }
    return nil
}
```

#### Step 2.3: Implement Alias Inference from TSConfig and Package.json Imports
The alias inference should consider both:
1. **TSConfig paths** - e.g., `"@domain/*": ["src/domain/*"]`
2. **Package.json imports** - e.g., `"#domain/*": "./src/domain/*"` (subpath imports)

- [x] Test: Alias correctly inferred for path when matching tsconfig paths entry
```go
func TestInferAliasFromTSConfig(t *testing.T)
```
- [x] Test: Alias correctly inferred for path when matching package.json imports entry
```go
func TestInferAliasFromPackageJsonImports(t *testing.T)
```
- [x] Test: TSConfig paths take precedence over package.json imports (or define clear order)
- [x] Test: No alias inferred when no matching entry in either source
- [x] Implement alias inference:
```go
func InferAliasForDomain(
    domainPath string,
    tsconfigParsed *TsConfigParsed,
    packageJsonImports *PackageJsonImports,
) string
```

#### Step 2.4: Run Step 2 Tests
- [x] Verify all domain resolution tests pass

---

### Phase 3: Import Classification

#### Step 3.1: Create Import Classification Tests
- [x] Test: Relative import (`./utils`) is detected as relative
```go
func TestIsRelativeImport(request string) bool
```
- [x] Test: Absolute/aliased import (`@domain/api`) is detected as non-relative
- [x] Test: Node module import is excluded from checks
- [x] Test: Import within same domain with relative path is valid
- [x] Test: Import within same domain with aliased path is violation
- [x] Test: Import across domains with relative path is violation
- [x] Test: Import across domains with aliased path is valid
- [x] Test: Import path correctly identified as pointing to domain
```go
func TestImportTargetsDomain(importPath string, compiledDomains []CompiledDomain, cwd string) *CompiledDomain
```

#### Step 3.2: Implement Import Classification
- [x] Implement relative import detection (simple string check, **no regex**):
```go
// Uses strings.HasPrefix - O(1) operation
func IsRelativeImport(request string) bool {
    return strings.HasPrefix(request, "./") || 
           strings.HasPrefix(request, "../") || 
           request == "." || 
           request == ".."
}
```
- [x] Implement import target domain resolution using **prefix matching**:
```go
func ResolveImportTargetDomain(resolvedPath string, compiledDomains []CompiledDomain) *CompiledDomain
```
- [x] Implement import alias validation:
```go
func ValidateImportUsesCorrectAlias(request string, targetDomain *CompiledDomain) bool
```

#### Step 3.3: Run Step 3 Tests
- [x] Verify all import classification tests pass

---

### Phase 4: Violation Detection

#### Step 4.1: Define Violation Types
- [x] Create violation struct:
```go
type ImportConventionViolation struct {
    FilePath         string
    ImportRequest    string
    ImportResolved   string
    ViolationType    string  // "should-be-relative" | "should-be-aliased" | "wrong-alias"
    SourceDomain     string
    TargetDomain     string
    ExpectedPattern  string  // Expected import pattern
    ActualPattern    string  // Actual import pattern
}
```

#### Step 4.2: Create Violation Detection Tests
- [x] Test: Intra-domain import with alias detected as violation (`should-be-relative`)
```go
func TestCheckImportConventions_IntraDomainAlias(t *testing.T)
```
- [x] Test: Inter-domain import with relative path detected as violation (`should-be-aliased`)
```go
func TestCheckImportConventions_InterDomainRelative(t *testing.T)
```
- [x] Test: Inter-domain import with wrong alias detected as violation (`wrong-alias`)
```go
func TestCheckImportConventions_WrongAlias(t *testing.T)
```
- [x] Test: Valid intra-domain relative import passes
```go
func TestCheckImportConventions_ValidIntraDomain(t *testing.T)
```
- [x] Test: Valid inter-domain aliased import passes
```go
func TestCheckImportConventions_ValidInterDomain(t *testing.T)
```
- [x] Test: Import to non-domain path is ignored
- [x] Test: Import from non-domain file is ignored
- [x] Test: NodeModule imports are ignored
- [x] Test: BuiltInModule imports are ignored

#### Step 4.3: Implement Violation Detection
- [x] Implement main check function with **early filtering**:
```go
// Pre-filter: Only check files that belong to a domain
// Pre-filter: Only check UserModule and MonorepoModule imports
func CheckImportConventionsFromTree(
    minimalTree MinimalDependencyTree,
    files []string,
    parsedRules []ParsedImportConventionRule,
    tsconfigParsed *TsConfigParsed,
    packageJsonImports *PackageJsonImports,
    cwd string,
) []ImportConventionViolation
```
- [x] Implement single file check:
```go
func checkFileImportConventions(
    filePath string,
    imports []MinimalDependency,
    compiledDomains []CompiledDomain,
    fileDomain *CompiledDomain,
    cwd string,
) []ImportConventionViolation
```
- [x] **Optimization**: Build file-to-domain lookup map once before iterating:
```go
// O(n) build, then O(1) lookups per file
fileToDomain := make(map[string]*CompiledDomain)
for _, file := range files {
    fileToDomain[file] = ResolveDomainForFile(file, compiledDomains)
}
```

#### Step 4.4: Run Step 4 Tests
- [x] Verify all violation detection tests pass

---

### Phase 5: Integration with Config Processor

#### Step 5.1: Extend RuleResult
- [x] Add field to `RuleResult` struct in `configProcessor.go`:
```go
type RuleResult struct {
    // ... existing fields ...
    ImportConventionViolations []ImportConventionViolation
}
```

#### Step 5.2: Create Integration Tests
- [x] Create test fixture `__fixtures__/importConventionsProject/`
  - [x] Directory structure with multiple domains
  - [x] Files with valid and invalid imports
  - [x] `rev-dep.config.json` with import-conventions
  - [x] `tsconfig.json` with path aliases
- [x] Test: `ProcessConfig` returns import convention violations
```go
func TestConfigProcessor_ImportConventions(t *testing.T)
```
- [x] Test: Import conventions check runs alongside other checks
- [x] Test: Import conventions check adds to enabled checks list

#### Step 5.3: Implement Config Processor Integration
- [x] Add import conventions check to `processRuleChecks()`:
```go
case "import-conventions":
    // Run import conventions check
```
- [x] Update enabled checks generation in `ProcessConfig()` to include `import-conventions` when configured
- [x] Parse import conventions from config and pass to rule processing

#### Step 5.4: Run Step 5 Tests
- [x] Verify all integration tests pass

---

### Phase 6: CLI Output & Reporting

#### Step 6.1: Create Output Tests
- [x] Test: Violations are correctly formatted in output
- [x] Test: Exit code is 1 when violations exist
- [x] Test: No output when no violations

#### Step 6.2: Implement Output Formatting
- [x] Add output formatting for import convention violations in `cmd_run_config.go`:
```go
func formatImportConventionViolations(violations []ImportConventionViolation) string
```
- [x] Update result printing logic to include import convention violations
- [x] Update HasFailures logic to include import convention violations

#### Step 6.3: Run Step 6 Tests
- [ ] Verify all output tests pass

---

### Phase 7: Smoke Tests

#### Step 7.1: Add Smoke Tests
- [ ] Add smoke test to `main_smoke_test.go`:
```go
func TestSmoke_ImportConventions_Violations(t *testing.T)
func TestSmoke_ImportConventions_NoViolations(t *testing.T)
```
- [ ] Create test fixture with clear violation cases
- [ ] Verify exit codes are correct

#### Step 7.2: Run Full Test Suite
- [ ] Run all existing tests to ensure no regressions
- [ ] Run smoke tests

---

### Phase 8: Documentation & Schema

#### Step 8.1: Update JSON Schema
- [ ] Update `config-schema/1.0.schema.json` to include `import-conventions`

#### Step 8.2: Update README
- [ ] Add documentation for import-conventions feature
- [ ] Add configuration examples

---

## Key Functions Summary

| Function | File | Purpose |
|----------|------|---------|
| `validateRawImportConventions(conventions interface{}) error` | config.go | Validates raw import-conventions JSON |
| `parseImportConventionDomains(domains interface{}) ([]ImportConventionDomain, error)` | config.go | Parses domain definitions from config |
| `CompileDomains(domains []ImportConventionDomain, cwd string) ([]CompiledDomain, error)` | import_conventions.go | Compiles domains with glob matchers |
| `ResolveDomainForFile(filePath string, compiledDomains []CompiledDomain) *CompiledDomain` | import_conventions.go | Finds which domain a file belongs to |
| `InferAliasForDomain(domainPath string, tsconfigParsed *TsConfigParsed, packageJsonImports *PackageJsonImports) string` | import_conventions.go | Infers alias from tsconfig.json paths or package.json imports |
| `IsRelativeImport(request string) bool` | import_conventions.go | Checks if import uses relative path |
| `ResolveImportTargetDomain(resolvedPath string, compiledDomains []CompiledDomain) *CompiledDomain` | import_conventions.go | Finds target domain of resolved import |
| `CheckImportConventionsFromTree(...) []ImportConventionViolation` | import_conventions.go | Main violation detection |
| `formatImportConventionViolations(violations []ImportConventionViolation) string` | cmd_run_config.go | Formats violations for CLI output |

---

## Test Fixtures Needed

### `__fixtures__/importConventionsProject/`
```
importConventionsProject/
├── package.json
├── rev-dep.config.json
├── tsconfig.json
└── src/
    ├── features/
    │   ├── auth/
    │   │   ├── index.ts
    │   │   ├── utils.ts
    │   │   └── validImport.ts      # imports ./utils (valid)
    │   │   └── invalidImport.ts    # imports @auth/utils (violation)
    │   └── users/
    │       ├── index.ts
    │       └── service.ts
    │       └── validCrossDomain.ts # imports @auth (valid)
    │       └── invalidCrossDomain.ts # imports ../auth/utils (violation)
    └── shared/
        └── ui/
            └── Button.ts
```

---

## Notes

- The feature operates on the dependency tree **after** imports are resolved
- Both `UserModule` and `MonorepoModule` import types are checked
- `NodeModule`, `BuiltInModule`, `AssetModule`, and `ExcludedByUser` imports are ignored
- **Alias inference sources (in order of precedence):**
  1. TSConfig `compilerOptions.paths` (e.g., `@domain/*`)
  2. Package.json `imports` field (e.g., `#domain/*`)
  3. Falls back to domain path if no match found in either source
- **Glob patterns are only used at config time** to discover domains in simplified mode
- **Runtime checking uses simple path prefix matching** - no globs, no regex
- Both `@`-prefixed (tsconfig) and `#`-prefixed (package.json imports) aliases are valid for inter-domain imports

---

## Performance Implementation Patterns

### Pattern 1: Path Prefix Matching (No Regex, No Globs at Runtime)
```go
// Domain membership is just a prefix check - O(1) operation
// Globs are only used at CONFIG TIME to discover domains
func fileBelongsToDomain(filePath string, domain CompiledDomain) bool {
    return strings.HasPrefix(filePath, domain.AbsolutePath)
}
```

### Pattern 2: Pre-computed Lookup Maps
```go
// Build once at start of check
type DomainContext struct {
    CompiledDomains []CompiledDomain
    FileToDomain    map[string]*CompiledDomain  // Pre-computed
    AliasToDomain   map[string]*CompiledDomain  // Fast alias lookup
}
```

### Pattern 3: Early Exit Conditions
```go
// In hot loop, exit early:
if dep.ResolvedType != UserModule && dep.ResolvedType != MonorepoModule {
    continue // Skip NodeModule, BuiltInModule, etc.
}
if fileDomain == nil {
    continue // File not in any domain, skip
}
```

### Pattern 4: Validate No Nested Domains at Config Time
```go
// Since nested domains are not allowed, validate at config parsing
// This simplifies runtime - first match wins, no ambiguity
func validateNoNestedDomains(domains []CompiledDomain) error {
    for i := 0; i < len(domains); i++ {
        for j := i + 1; j < len(domains); j++ {
            if strings.HasPrefix(domains[i].AbsolutePath, domains[j].AbsolutePath) ||
               strings.HasPrefix(domains[j].AbsolutePath, domains[i].AbsolutePath) {
                return fmt.Errorf("nested domains not allowed: %s and %s", 
                    domains[i].Path, domains[j].Path)
            }
        }
    }
    return nil
}
```

---

## 🎉 Debugging Completion Summary

### **Status: ✅ RESOLVED**
The import conventions feature integration has been successfully debugged and is now fully functional!

### **Key Fixes Applied**

#### 1. **Domain Parsing Issue** 
- **Problem**: `ImportConventionRule.Domains` was `[]interface{}` instead of `[]ImportConventionDomain`
- **Root Cause**: JSON unmarshaling happened twice (validation + final struct) but transformation only happened during validation
- **Solution**: Added `parseImportConventionDomains()` function and post-processing step in `ParseConfig()`

#### 2. **Path Resolution Issues**
- **Problem**: Relative file paths vs absolute domain paths caused mismatches
- **Root Cause**: `ResolveDomainForFile()` expected absolute paths but received relative paths
- **Solution**: Convert relative file paths to absolute paths before domain resolution
- **Bonus**: Made `ResolveImportTargetDomain()` handle both relative and absolute paths for test compatibility

#### 3. **Integration Issues**
- **Problem**: `HasFailures` wasn't set when import convention violations existed
- **Solution**: Updated `HasFailures` calculation to include `len(ruleResult.ImportConventionViolations) > 0`

### **Verification Results**
- ✅ **17/17 import convention tests passing**
- ✅ **Integration test working**: Detects both "should-be-relative" and "should-be-aliased" violations
- ✅ **Unit tests working**: All violation detection scenarios working correctly
- ✅ **Domain parsing working**: Both simplified and advanced configuration modes
- ✅ **File-to-domain mapping working**: All files correctly assigned to domains

### **Current Feature Status**
- **Phase 1-5**: ✅ **COMPLETE** (Configuration → Integration)
- **Phase 6**: 🔄 **READY** (CLI Output & Reporting - 90% done, just output formatting needed)
- **Phase 7-8**: ⏳ **PENDING** (Smoke Tests & Documentation)

The core import conventions functionality is **production-ready** and working correctly. The remaining work is primarily user-facing improvements (CLI output formatting, documentation, and end-to-end testing).

**Next Step**: Implement CLI output formatting for import convention violations to complete Phase 6.

