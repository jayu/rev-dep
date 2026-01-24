package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"
)

// ImportConventionViolation represents a violation of import conventions
type ImportConventionViolation struct {
	FilePath        string // Path to the file with the violation
	ImportRequest   string // The original import string (e.g., "@auth/utils")
	ImportResolved  string // The resolved import path
	ViolationType   string // "should-be-relative" | "should-be-aliased" | "wrong-alias"
	SourceDomain    string // Domain of the source file
	TargetDomain    string // Domain of the target import
	ExpectedPattern string // Expected import pattern
	ActualPattern   string // Actual import pattern
}

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

// IsRelativeImport checks if import uses relative path using string prefix matching
// Uses strings.HasPrefix - O(1) operation
func IsRelativeImport(request string) bool {
	return strings.HasPrefix(request, "./") ||
		strings.HasPrefix(request, "../") ||
		request == "." ||
		request == ".."
}

// ResolveImportTargetDomain finds target domain of resolved import using prefix matching
func ResolveImportTargetDomain(resolvedPath string, compiledDomains []CompiledDomain) *CompiledDomain {
	// Normalize the path for consistent comparison
	normalizedPath := filepath.Clean(resolvedPath)

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

// ValidateImportUsesCorrectAlias checks if import uses the correct alias for the target domain
func ValidateImportUsesCorrectAlias(request string, targetDomain *CompiledDomain) bool {
	if targetDomain == nil || targetDomain.Alias == "" {
		return false
	}

	// Check if the import starts with the domain's alias
	return strings.HasPrefix(request, targetDomain.Alias)
}

// CheckImportConventionsFromTree checks import conventions from dependency tree with early filtering
// Pre-filter: Only check files that belong to a domain
// Pre-filter: Only check UserModule and MonorepoModule imports
func CheckImportConventionsFromTree(
	minimalTree MinimalDependencyTree,
	files []string,
	parsedRules []ParsedImportConventionRule,
	tsconfigParsed *TsConfigParsed,
	packageJsonImports *PackageJsonImports,
	cwd string,
) []ImportConventionViolation {
	var violations []ImportConventionViolation

	// Compile domains for each rule
	for _, rule := range parsedRules {
		// Compile domains for this rule
		compiledDomains, err := CompileDomains(rule.Domains, cwd)
		if err != nil {
			continue
		}

		// Optimization: Build file-to-domain lookup map once before iterating
		fileToDomain := make(map[string]*CompiledDomain)
		for _, file := range files {
			// Check if file path is already absolute or relative
			var absoluteFilePath string
			if filepath.IsAbs(file) {
				// File is already absolute, use as-is
				absoluteFilePath = filepath.Clean(file)
			} else {
				// File is relative, convert to absolute for domain resolution
				absoluteFilePath = filepath.Clean(filepath.Join(cwd, file))
			}
			domain := ResolveDomainForFile(absoluteFilePath, compiledDomains)
			fileToDomain[file] = domain
		}

		// Check each file
		for _, file := range files {
			sourceDomain := fileToDomain[file]
			if sourceDomain == nil {
				continue // File not in any domain, skip
			}

			// Get imports for this file
			imports, exists := minimalTree[file]
			if !exists {
				continue
			}

			// Check file import conventions
			fileViolations := checkFileImportConventions(
				file,
				imports,
				compiledDomains,
				sourceDomain,
				cwd,
			)
			violations = append(violations, fileViolations...)
		}
	}

	return violations
}

// checkFileImportConventions checks import conventions for a single file
func checkFileImportConventions(
	filePath string,
	imports []MinimalDependency,
	compiledDomains []CompiledDomain,
	fileDomain *CompiledDomain,
	cwd string,
) []ImportConventionViolation {
	violations := []ImportConventionViolation{}

	for _, dep := range imports {
		// Pre-filter: Only check UserModule and MonorepoModule imports
		if dep.ResolvedType != UserModule && dep.ResolvedType != MonorepoModule {
			continue // Skip NodeModule, BuiltInModule, etc.
		}

		// Check if import targets a domain using the resolved path (ID field)
		var targetDomain *CompiledDomain
		if dep.ID != nil {
			// Check if the path is already absolute or relative
			var resolvedPath string
			if filepath.IsAbs(*dep.ID) {
				// Path is already absolute, use as-is
				resolvedPath = filepath.Clean(*dep.ID)
			} else {
				// Path is relative, convert to absolute for domain resolution
				resolvedPath = filepath.Clean(filepath.Join(cwd, *dep.ID))
			}
			targetDomain = ResolveImportTargetDomain(resolvedPath, compiledDomains)
		}

		if targetDomain == nil {
			continue // Import doesn't target a domain, skip
		}

		// Check for violations based on the rule
		violation := checkImportForViolation(filePath, dep, fileDomain, targetDomain)
		if violation != nil {
			violations = append(violations, *violation)
		}
	}

	return violations
}

// checkImportForViolation checks a single import for violations
func checkImportForViolation(
	filePath string,
	dep MinimalDependency,
	sourceDomain *CompiledDomain,
	targetDomain *CompiledDomain,
) *ImportConventionViolation {
	isRelative := IsRelativeImport(dep.Request)

	// Intra-domain import (same domain)
	if sourceDomain.Path == targetDomain.Path {
		if !isRelative {
			// Intra-domain import should be relative
			return &ImportConventionViolation{
				FilePath:        filePath,
				ImportRequest:   dep.Request,
				ViolationType:   "should-be-relative",
				SourceDomain:    sourceDomain.Path,
				TargetDomain:    targetDomain.Path,
				ExpectedPattern: "relative path (e.g., ./utils)",
				ActualPattern:   dep.Request,
			}
		}
		return nil // Valid intra-domain relative import
	}

	// Inter-domain import (different domains)
	if isRelative {
		// Inter-domain import should be aliased
		return &ImportConventionViolation{
			FilePath:        filePath,
			ImportRequest:   dep.Request,
			ViolationType:   "should-be-aliased",
			SourceDomain:    sourceDomain.Path,
			TargetDomain:    targetDomain.Path,
			ExpectedPattern: "alias path (e.g., @domain/utils)",
			ActualPattern:   dep.Request,
		}
	}

	// Check if using correct alias
	if !ValidateImportUsesCorrectAlias(dep.Request, targetDomain) {
		return &ImportConventionViolation{
			FilePath:        filePath,
			ImportRequest:   dep.Request,
			ViolationType:   "wrong-alias",
			SourceDomain:    sourceDomain.Path,
			TargetDomain:    targetDomain.Path,
			ExpectedPattern: targetDomain.Alias + "/*",
			ActualPattern:   dep.Request,
		}
	}

	return nil // Valid inter-domain aliased import
}
