# Restricted Dev Dependencies Usage Detection

Implement a new config processor check to detect when dev dependencies are used in production entry points, preventing runtime failures in production builds.

## Overview

This feature will trace dependency graphs from specified production entry points and identify any dev dependencies that are being used, reporting them as violations since dev dependencies are not available in production builds.

## Implementation Steps

### Step 1: Add Configuration Structure
- Create `RestrictedDevDependenciesUsageOptions` struct in `config.go`
- Add `ValidEntryPoints []string` field for production entry point patterns
- Add `Enabled bool` field to enable/disable the check
- Add the new options to the `Rule` struct
- Update `RuleResult` struct to include violation results

### Step 2: Update Configuration Schema
- **Create new config schema 1.4** (since 1.3 already released)
- Add new option definitions to 1.4 schema file only
- Include validation rules for new configuration options
- Update `supportedConfigVersions` to include "1.4"
- Ensure backward compatibility with existing configs

### Step 3: Create Core Detection Functions
- **Leverage existing `MonorepoContext.PackageConfigCache`** instead of re-parsing package.json
- Create helper functions to extract dev/production dependencies from cached `PackageJsonConfig`:
  - `GetDevDependenciesFromConfig(config *PackageJsonConfig) map[string]bool`
  - `GetProductionDependenciesFromConfig(config *PackageJsonConfig) map[string]bool`
- Create `FindDevDependenciesInProduction()` function that:
  - **Uses existing `ruleTree` from `filterFilesForRule()` (same as other checks)**
  - **Filters to entry point reachable files using `buildDepsGraphForMultiple()`**
  - Uses cached package config to identify which used modules are dev dependencies
  - Returns violation information with file paths and usage chains
  - **Important: Always ignore type-only imports since they don't compile to production code**

### Step 4: Integrate with Config Processor
- Add new goroutine in `processRuleChecks()` for dev dependencies detection
- **Implement validation function for new options** in `validateRule()`
- Add result formatting and output handling
- Ensure proper error handling and reporting

### Step 5: Create Test Suite
- Add comprehensive test cases covering:
  - Basic dev dependency detection in production code
  - Complex dependency chains through multiple files
  - **Type imports should be ignored and not reported as violations**
  - Edge cases (circular dependencies, missing dependencies)
  - Configuration validation scenarios
  - Integration with existing config processor workflow

### Step 6: Documentation and Examples
- Update README.md with new configuration options
- Add usage examples for common scenarios (Next.js apps, libraries)
- Document the violation format and troubleshooting steps

## Key Implementation Details

### Dependency Graph Tracing
- **Follow existing pattern**: Use `ruleTree` from `filterFilesForRule()` (same as other checks)
- **Filter to entry points**: Use `buildDepsGraphForMultiple(ruleTree, validEntryPoints, nil, false)` to trace reachable files
- **Extract reachable dependencies**: From graph vertices, identify what's reachable from entry points
- **Critical: Always set `ignoreTypeImports=true` since type imports don't affect production runtime**
- This approach is consistent with how all other config processor checks work

### Module Classification
- **Use existing `MonorepoContext.PackageConfigCache`** to access parsed `Dependencies` and `DevDependencies`
- Create helper functions that work with cached `PackageJsonConfig` struct:
  - `GetDevDependenciesFromConfig(config *PackageJsonConfig)` → returns `config.DevDependencies`
  - `GetProductionDependenciesFromConfig(config *PackageJsonConfig)` → returns `config.Dependencies`
- Use existing `GetNodeModuleName()` for consistent module name handling
- Handle edge cases where modules exist in both sections

### Violation Reporting
- Include file paths where dev dependencies are used
- Provide usage chains showing how dev dependencies are reached from entry points
- Format output similar to existing violation types for consistency

### Performance Considerations
- Build dependency graph once per rule and reuse for analysis
- Use efficient lookups for module classification
- Minimize memory usage during graph traversal

## Configuration Example

```json
{
  "rules": [
    {
      "path": "src/**/*",
      "restrictedDevDependenciesUsageDetection": {
        "enabled": true,
        "validEntryPoints": ["src/pages/**/*.tsx", "src/main.tsx"]
      }
    }
  ]
}
```

## Expected Output

The feature will report violations like:
- `lodash` (dev dependency) used in `src/components/Button.tsx` reachable from entry point `src/pages/index.tsx`
- `eslint` (dev dependency) used in production code through import chain
- **Note: Type-only imports like `import type { ReactNode } from 'react'` will be ignored**

This ensures production builds won't fail due to missing dev dependencies, while allowing type definitions from dev dependencies to be used safely.
