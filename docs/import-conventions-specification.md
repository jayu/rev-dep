# Import Conventions - Functional Specification

## Overview

The Import Conventions feature enforces consistent import patterns across a codebase by validating imports against configurable domain rules and automatically fixing violations when possible. This feature supports both explicit domain configuration and shortcut syntax for rapid setup.

## Core Concepts

### Domains
A **domain** represents a logical grouping of related code within the codebase. Each domain can have:
- A path pattern (e.g., `src/auth`, `src/shared/*`)
- An optional alias (e.g., `@/auth`, `@/shared`)
- An enabled/disabled state

### Import Types
The feature recognizes three main import patterns:
1. **Relative imports**: `./utils`, `../shared/component`
2. **Absolute imports with aliases**: `@/auth/utils`, `@shared/component`
3. **Direct path imports**: `src/auth/utils`, `app/messagesTemplates/constants`

## Configuration

### Domain Configuration

#### Explicit Configuration
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
            {
              "path": "src/auth",
              "alias": "@/auth",
              "enabled": true
            },
            {
              "path": "src/shared/ui",
              "alias": "@ui",
              "enabled": true
            }
          ]
        }
      ]
    }
  ]
}
```

#### Shortcut Configuration
```json
{
  "configVersion": "1.0",
  "rules": [
    {
      "path": ".",
      "importConventions": [
        {
          "rule": "relative-internal-absolute-external",
          "domains": ["src/*", "packages/*"]
        }
      ]
    }
  ]
}
```

**Note**: In shortcut configuration, `domains` is an array of strings that can be:
- Glob patterns (e.g., `"src/*"`, `"packages/*/lib"`)
- Specific directory paths (e.g., `"src/shared"`, `"packages/utils"`)

### TypeScript Path Mapping Integration
The feature integrates with TypeScript's `tsconfig.json` path mapping and `package.json` imports map to resolve aliases and determine appropriate import patterns.

## Rules and Violations

### Rule 1: Intra-Domain Imports
**Description**: Imports within the same domain should use relative paths.

**Violation**: `should-be-relative`
- **Trigger**: Using absolute/aliased imports for files within the same domain
- **Fix**: Convert to relative path (e.g., `@/auth/utils` → `./utils`)

**Examples**:
- ❌ `import utils from '@/auth/utils'` (in `src/auth/service.ts`)
- ✅ `import utils from './utils'` (in `src/auth/service.ts`)

### Rule 2: Inter-Domain Imports
**Description**: Imports between different domains should use aliased paths.

**Violation**: `should-be-aliased`
- **Trigger**: Using relative imports for files in different domains
- **Fix**: Convert to appropriate alias (e.g., `../shared/utils` → `@/shared/utils`)

**Examples**:
- ❌ `import utils from '../shared/utils'` (from `src/auth/service.ts`)
- ✅ `import utils from '@/shared/utils'` (from `src/auth/service.ts`)

### Rule 3: Correct Alias Usage
**Description**: Imports should use the correct alias for the target domain.

**Violation**: `wrong-alias`
- **Trigger**: Using an incorrect or non-existent alias for the target domain
- **Fix**: Replace with correct alias (e.g., `@legacy/shared` → `@/shared`)

**Examples**:
- ❌ `import utils from '@legacy/shared/utils'` (when correct is `@/shared/utils`)
- ✅ `import utils from '@/shared/utils'`

## Alias Resolution and Precedence

### Alias Types
1. **Explicit Aliases**: Defined in domain configuration
2. **Inferred Aliases**: Derived from TypeScript path mapping or package.json imports map
3. **Catch-all Aliases**: Wildcard aliases (`"*"`) that match any path

### Alias Precedence Rules
When multiple aliases could match an import path:

1. **Explicit aliases** take highest precedence
2. **Specific path aliases** (`@/shared/*`) take precedence over **catch-all aliases** (`*`)
3. **More specific paths** take precedence over less specific ones

### Wildcard Alias Behavior

#### When Wildcard is Explicitly Defined
If `tsconfig.json` contains `"*": ["./*"]`:
- **Fix Generation**: Enabled for unconfigured domains
- **Validation**: Accepts both direct paths and more specific aliases
- **Example**: `../../../../../messagesTemplates/constants` → `app/messagesTemplates/constants`

#### When Wildcard is Auto-Generated
If `tsconfig.json` has `baseUrl` but no explicit `"*"`:
- **Fix Generation**: Disabled (no automatic fixes)
- **Validation**: Violations detected but no fixes suggested
- **Rationale**: Prevents invalid alias generation

## Shortcut Syntax Behavior

### Domain Discovery
When using shortcut syntax (e.g., `"path": "src/*"`):
1. **Expand Pattern**: Discover all directories matching the pattern
2. **Infer Aliases**: Use TypeScript path mapping or package.json imports map to determine appropriate aliases
3. **Create Domains**: Generate domain configurations for each discovered directory

### Alias Inference for Shortcut Domains
- **Specific aliases available**: Use the most specific matching alias from TypeScript path mapping or package.json imports map
- **Only catch-all available**: Use catch-all behavior if explicitly defined
- **No suitable aliases**: Domain remains without alias (relative imports only)

## Fix Generation Logic

### Fix Generation Conditions
Fixes are generated only when ALL of the following are met:
1. **Violation detected**: Import violates one of the rules
2. **Autofix enabled**: User has requested automatic fixes
3. **Valid target resolution**: Target file can be resolved
4. **Appropriate alias available**: Suitable alias exists for the target

### Fix Generation Scenarios

#### Scenario 1: Relative to Aliased (Inter-Domain)
**Input**: `import { utils } from '../../../shared/utils'`
**Conditions**: Target domain has explicit alias
**Output**: `import { utils } from '@/shared/utils'`

#### Scenario 2: Aliased to Relative (Intra-Domain)
**Input**: `import { utils } from '@/auth/utils'`
**Conditions**: Source and target in same domain
**Output**: `import { utils } from './utils'`

#### Scenario 3: Relative to Catch-all (Unconfigured Domain)
**Input**: `import { constants } from '../../../../../messagesTemplates/constants'`
**Conditions**: 
- Target domain not configured
- Explicit catch-all alias exists in tsconfig
**Output**: `import { constants } from 'app/messagesTemplates/constants'`

#### Scenario 4: Wrong Alias to Correct Alias
**Input**: `import { utils } from '@legacy/shared/utils'`
**Conditions**: Target domain has different explicit alias
**Output**: `import { utils } from '@/shared/utils'`

**Note**: This scenario only applies to explicit configuration. In shortcut syntax, wrong alias detection is not available since aliases are inferred from TypeScript path mapping or package.json imports map rather than explicitly defined.

### No-Fix Scenarios
Fixes are NOT generated when:

1. **Catch-all not explicit**: Wildcard alias only auto-generated from baseUrl
2. **Target not found**: Cannot resolve target file path
3. **Ambiguous resolution**: Multiple possible targets for the import
4. **Domain disabled**: Target domain is explicitly disabled
5. **No suitable alias**: No appropriate alias available for the target

## Style Preservation

### Import Path Style
When generating fixes, the feature preserves the original import style:
- **File extensions**: Maintained if present in original import
- **Index files**: `/index` suffix preserved when used
- **Trailing slashes**: Path separators normalized appropriately

### Examples
- `import utils from './utils.js'` → `import utils from '@/shared/utils.js'`
- `import Component from './Button/index'` → `import Component from '@/ui/Button/index'`

## Error Handling and Edge Cases

### Unresolvable Imports
- **Node modules**: Ignored (not validated)
- **Built-in modules**: Ignored (not validated)
- **Missing files**: Reported as violations but no fixes generated
- **Circular imports**: Validated but not automatically fixed

### Complex Domain Structures
- **Nested domains**: Supported (domains can contain subdirectories)
- **Overlapping patterns**: Rejected during configuration validation
- **Disabled domains**: Skipped during validation but still considered as targets

### TypeScript Integration
- **Multiple tsconfig files**: Uses nearest tsconfig.json
- **Path mapping conflicts**: Uses most specific match
- **Base URL resolution**: Respects TypeScript baseUrl configuration

## Configuration Validation

### Domain Validation Rules
1. **Non-overlapping paths**: Domain paths cannot overlap
2. **Valid patterns**: Path patterns must be resolvable
3. **Required fields**: Path is required, alias is optional
4. **Enabled state**: Domains default to enabled if not specified

### Alias Validation
1. **TypeScript consistency**: Aliases should match tsconfig paths
2. **Format requirements**: Aliases must follow import identifier rules
3. **Uniqueness**: No duplicate aliases across domains

## Output and Reporting

### Violation Format
Each violation includes:
- **File path**: Location of the violating import
- **Import request**: The exact import string
- **Violation type**: One of `should-be-relative`, `should-be-aliased`, `wrong-alias`
- **Source domain**: Domain containing the importing file
- **Target domain**: Domain containing the imported file
- **Expected pattern**: What the import should look like
- **Actual pattern**: What the import currently looks like
- **Fix suggestion**: Optional automatic fix (when available)

### Summary Statistics
The feature provides summary information:
- Total violations found
- Violations by type
- Files with violations
- Fix generation success rate

## Performance Considerations

### Optimization Strategies
- **Early filtering**: Only process files within configured domains
- **Import type filtering**: Skip Node modules and built-in modules
- **Caching**: Cache domain resolution and TypeScript parsing
- **Parallel processing**: Process multiple files concurrently when possible

### Scalability
- **Large codebases**: Designed to handle thousands of files
- **Complex domain structures**: Supports hundreds of domains
- **Deep directory hierarchies**: No practical limit on nesting depth

## Migration and Compatibility

### Backward Compatibility
- **Existing configurations**: Supported without changes
- **Gradual adoption**: Can be enabled domain by domain
- **Non-breaking changes**: New features add options without changing existing behavior

### Migration Paths
1. **Start with violations**: Run without fixes to see current state
2. **Add domains gradually**: Configure high-traffic domains first
3. **Enable autofixes**: Turn on automatic fixes after validation
4. **Refactor systematically**: Address violations by domain or priority

## Examples and Use Cases

### Monorepo with Shared Libraries
```
src/
  auth/           # Domain: @/auth
  shared/         # Domain: @/shared  
  features/       # Domain: @/features
  utils/          # Domain: @/utils
```

### Micro-Frontend Application
```
apps/
  web/            # Domain: @web
  mobile/         # Domain: @mobile
packages/
  shared/         # Domain: @shared
  components/     # Domain: @ui
```

### Component Library
```
packages/
  components/     # Domain: @components
  icons/          # Domain: @icons
  themes/         # Domain: @themes
  utils/          # Domain: @utils
```

This specification provides a complete functional description of the Import Conventions feature, enabling reimplementation without reference to internal implementation details.
