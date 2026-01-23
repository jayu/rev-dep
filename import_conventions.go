package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"
)

// CompiledDomain represents a processed domain with absolute path for fast matching
type CompiledDomain struct {
	Path         string // Original path from config (e.g., "src/auth")
	Alias        string // e.g., "@auth" (inferred or explicit)
	AbsolutePath string // Full absolute path for prefix matching
}

// ParsePackageJsonImports parses package.json imports from a file
func ParsePackageJsonImports(packageJsonPath string) (*PackageJsonImports, error) {
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return nil, err
	}

	var rawPackageJson map[string]interface{}
	if err := json.Unmarshal(jsonc.ToJSON(content), &rawPackageJson); err != nil {
		return nil, err
	}

	packageJsonImports := &PackageJsonImports{
		imports:                  map[string]interface{}{},
		importsRegexps:           []RegExpArrItem{},
		wildcardPatterns:         []WildcardPattern{},
		conditionNames:           []string{}, // No conditions for import conventions
		parsedImportTargets:      map[string]*ImportTargetTreeNode{},
		simpleImportTargetsByKey: map[string]string{},
	}

	if imports, ok := rawPackageJson["imports"]; ok {
		if importsMap, ok := imports.(map[string]interface{}); ok {
			packageJsonImports.imports = importsMap
			for key, target := range importsMap {
				if strings.Count(key, "*") > 1 {
					continue
				}

				// For simple string targets, store them directly
				if targetStr, ok := target.(string); ok && !strings.Contains(targetStr, "#") {
					cleanTarget := strings.TrimPrefix(targetStr, "./")
					packageJsonImports.simpleImportTargetsByKey[key] = cleanTarget
				}

				// Parse the target into tree structure
				parsedTarget := parseImportTarget(target, []string{})
				if parsedTarget == nil {
					continue
				}

				packageJsonImports.parsedImportTargets[key] = parsedTarget

				// Create wildcard pattern if key contains wildcard
				if strings.Contains(key, "*") {
					wildcardIndex := strings.Index(key, "*")
					prefix := key[:wildcardIndex]
					suffix := key[wildcardIndex+1:]
					packageJsonImports.wildcardPatterns = append(packageJsonImports.wildcardPatterns, WildcardPattern{
						key:    key,
						prefix: prefix,
						suffix: suffix,
					})
				}
			}
		}
	}

	return packageJsonImports, nil
}

// ExpandDomainGlobs expands glob patterns to concrete directory paths
// Called once at config time, NOT at runtime
// "src/*" → ["src/auth", "src/users", "src/shared"]
func ExpandDomainGlobs(patterns []string, cwd string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		// If pattern contains wildcards, expand it
		if strings.Contains(pattern, "*") {
			// Simple glob expansion for common patterns like "src/*"
			if strings.HasSuffix(pattern, "/*") {
				baseDir := strings.TrimSuffix(pattern, "/*")
				fullBaseDir := filepath.Join(cwd, baseDir)

				entries, err := os.ReadDir(fullBaseDir)
				if err != nil {
					// Directory doesn't exist, skip
					continue
				}

				for _, entry := range entries {
					if entry.IsDir() {
						fullPath := filepath.Join(fullBaseDir, entry.Name())
						if !seen[fullPath] {
							seen[fullPath] = true
							result = append(result, fullPath)
						}
					}
				}
			} else {
				// For more complex patterns, we'll need a more sophisticated approach
				// For now, treat as literal path
				absPath := filepath.Join(cwd, pattern)
				if !seen[absPath] {
					seen[absPath] = true
					result = append(result, absPath)
				}
			}
		} else {
			// No wildcards, use the path as-is
			absPath := filepath.Join(cwd, pattern)
			if !seen[absPath] {
				seen[absPath] = true
				result = append(result, absPath)
			}
		}
	}

	return result, nil
}

// CompileDomains converts domain definitions to compiled domains with absolute paths
func CompileDomains(domains []ImportConventionDomain, cwd string) ([]CompiledDomain, error) {
	var compiled []CompiledDomain

	for _, domain := range domains {
		absPath := filepath.Join(cwd, domain.Path)

		// Normalize the path
		absPath = filepath.Clean(absPath)

		compiled = append(compiled, CompiledDomain{
			Path:         domain.Path,
			Alias:        domain.Alias,
			AbsolutePath: absPath,
		})
	}

	return compiled, nil
}

// ResolveDomainForFile finds which domain a file belongs to using path prefix matching
// Simple prefix check - O(n) where n = number of domains
// Since domains cannot overlap (validated at config time), first match wins
func ResolveDomainForFile(filePath string, compiledDomains []CompiledDomain) *CompiledDomain {
	// Normalize the file path for consistent comparison
	normalizedPath := filepath.Clean(filePath)

	for i := range compiledDomains {
		if strings.HasPrefix(normalizedPath, compiledDomains[i].AbsolutePath) {
			// Additional check to ensure we're not matching partial directory names
			// e.g., "/src/auth" should not match "/src/authentication"
			if len(normalizedPath) == len(compiledDomains[i].AbsolutePath) {
				return &compiledDomains[i]
			}
			// Check if the next character is a path separator
			if strings.HasPrefix(normalizedPath[len(compiledDomains[i].AbsolutePath):], string(filepath.Separator)) {
				return &compiledDomains[i]
			}
		}
	}
	return nil
}

// InferAliasForDomain infers alias from tsconfig.json paths or package.json imports
func InferAliasForDomain(
	domainPath string,
	tsconfigParsed *TsConfigParsed,
	packageJsonImports *PackageJsonImports,
) string {
	// First try to infer from tsconfig paths using aliases map
	if tsconfigParsed != nil {
		for alias, path := range tsconfigParsed.aliases {
			// Remove wildcards from path for comparison
			cleanPath := strings.TrimSuffix(path, "/*")
			cleanPath = strings.TrimSuffix(cleanPath, "/**")

			// Check if the domain path matches this path
			if strings.HasPrefix(domainPath, cleanPath) {
				// Return the alias without wildcards
				cleanAlias := strings.TrimSuffix(alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")
				return cleanAlias
			}
		}

		// Also check wildcard patterns (for more complex cases)
		// For tsconfig, we need to match domain path with the pattern's target
		// The pattern.prefix contains the alias prefix, but we need to match with the actual path
		// This is more complex and would require reverse mapping, which we'll skip for now
		_ = tsconfigParsed.wildcardPatterns // avoid unused warning
	}

	// Then try to infer from package.json imports using simple targets
	if packageJsonImports != nil {
		for alias, path := range packageJsonImports.simpleImportTargetsByKey {
			// Remove wildcards from path for comparison
			cleanPath := strings.TrimSuffix(path, "/*")
			cleanPath = strings.TrimSuffix(cleanPath, "/**")

			// Check if the domain path matches this path
			if strings.HasPrefix(domainPath, cleanPath) {
				// Return the alias without wildcards
				cleanAlias := strings.TrimSuffix(alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")
				return cleanAlias
			}
		}
	}

	// No matching alias found
	return ""
}
