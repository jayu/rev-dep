# Package.json Exports Test Implementation Plan

## Overview
Create comprehensive test suite for package.json exports resolution in a mocked monorepo project using `GetMinimalDepsTreeForCwd` approach.

## Mock Monorepo Structure
```
__fixtures__/mockMonorepo/
├── packages/
│   ├── consumer-package/
│   │   ├── index.ts (imports from exported-package)
│   │   └── package.json
│   └── exported-package/
│       ├── src/
│       │   ├── features/
│       │   │   ├── feature-a.ts
│       │   │   ├── feature-b.ts
│       │   │   └── private-internal-utils.ts
│       │   ├── utils/
│       │   │   └── helper.ts
│       │   ├── main.ts
│       │   └── config/
│       │       └── setup.config.js
│       ├── dist/
│       │   └── (mirrors src structure)
│       └── package.json (with complex exports map)
└── package.json (workspace root)
```

## Test Cases Implementation

### 1. Different Exports Specificity
- Test specific path vs wildcard priority
- Test exact match vs pattern match
- Test nested path specificity

### 2. Conditional Exports
- Test exports with conditions (development, production, node, browser, default)
- Test condition parameter passing
- Test fallback to default

### 3. Directory Swap with File Name
- Test `"./dir/*"` -> `"./\*/file"` pattern
- Verify directory structure transformation

### 4. Multiple Wildcards (Exclusion Validation)
- Test invalid patterns with multiple wildcards
- Verify they are excluded during parsing

### 5. Basic Wildcard Scenario
- Test `"wildcard/*.js"` -> `"./src/*.ts"`
- Verify proper wildcard expansion

### 6. Root Wildcard Scenario
- Test `"root/*"` -> `"./*"`
- Verify root-level wildcard handling and expansion of the rest of the path

### 7. Exports Blocking Paths
- Test `null` values in exports map
- Verify blocked paths are not resolvable
- Test partial blocking with wildcards

## Key Implementation Notes
- Use `GetMinimalDepsTreeForCwd` for end-to-end testing
- Create imports from consumer-package to exported-package
- Test both successful resolutions and blocking scenarios
- Verify ResolvedImportType and ID values
- Use similar test patterns as in `resolveImports_pkg_json_test.go:474:519`
- Create tests in `resolveImports_pkg_json_exports_test.go`
- Note that some tests might be failing due to not working implementation. Create all test first, then investigate if it is a bug in project or test bug