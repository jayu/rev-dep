package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tidwall/jsonc"
)

// ImportConventionViolation represents a violation of import conventions
type ImportConventionViolation struct {
	FilePath        string // Path to the file with the violation
	ImportRequest   string // The original import string (e.g., "@auth/utils")
	ImportResolved  string // The resolved import path
	ImportIndex     int    // Index of this import in the source file (0-based)
	ViolationType   string // "should-be-relative" | "should-be-aliased" | "wrong-alias"
	SourceDomain    string // Domain of the source file
	TargetDomain    string // Domain of the target import
	ExpectedPattern string // Expected import pattern
	ActualPattern   string // Actual import pattern
}

// CompiledDomain represents a processed domain with absolute path for fast matching
type CompiledDomain struct {
	Path          string // Original path from config (e.g., "src/auth")
	Alias         string // e.g., "@auth" (inferred or explicit)
	AbsolutePath  string // Full absolute path for prefix matching
	Enabled       bool   // Whether checks should be performed for this domain
	AliasExplicit bool   // Whether the alias was explicitly provided in config
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
func CompileDomains(
	domains []ImportConventionDomain,
	cwd string,
	tsconfigParsed *TsConfigParsed,
	packageJsonImports *PackageJsonImports,
) ([]CompiledDomain, error) {
	var compiled []CompiledDomain

	// Separate simple paths from glob patterns
	var simplePaths []string
	var globPatterns []string
	var domainMap = make(map[string]ImportConventionDomain)

	for _, domain := range domains {
		domainMap[domain.Path] = domain
		if strings.Contains(domain.Path, "*") {
			globPatterns = append(globPatterns, domain.Path)
		} else {
			simplePaths = append(simplePaths, domain.Path)
		}
	}

	// Process simple paths directly
	for _, path := range simplePaths {
		absPath := filepath.Join(cwd, path)
		absPath = filepath.Clean(absPath)
		domain := domainMap[path]

		alias := domain.Alias
		if alias == "" {
			alias = InferAliasForDomain(path, tsconfigParsed, packageJsonImports)
		}

		compiled = append(compiled, CompiledDomain{
			Path:          path,
			Alias:         alias,
			AbsolutePath:  absPath,
			Enabled:       domain.Enabled,
			AliasExplicit: domain.Alias != "",
		})
	}

	// Process glob patterns
	if len(globPatterns) > 0 {
		expandedPaths, err := ExpandDomainGlobs(globPatterns, cwd)
		if err != nil {
			return nil, fmt.Errorf("failed to expand domain globs: %w", err)
		}

		// Create compiled domains for each expanded path
		for _, expandedPath := range expandedPaths {
			// Check if expandedPath is already absolute
			var absPath string
			if filepath.IsAbs(expandedPath) {
				// Path is already absolute, use as-is
				absPath = filepath.Clean(expandedPath)
			} else {
				// Path is relative, join with cwd
				absPath = filepath.Join(cwd, expandedPath)
				absPath = filepath.Clean(absPath)
			}

			relPath, err := filepath.Rel(cwd, absPath)
			if err != nil {
				relPath = expandedPath // Fallback
			}

			// For expanded paths, try to infer alias from the original glob pattern
			// or use empty string if no alias can be inferred
			var alias string
			originalPattern := ""
			for _, pattern := range globPatterns {
				// Find which glob pattern this expanded path came from
				absPatternBase := filepath.Join(cwd, strings.TrimSuffix(pattern, "*"))
				if strings.HasPrefix(absPath, absPatternBase) {
					originalPattern = pattern
					break
				}
			}
			if originalPattern != "" {
				originalDomain := domainMap[originalPattern]
				if originalDomain.Alias != "" {
					// Use the alias from the original pattern
					alias = originalDomain.Alias
				} else {
					// Try to infer alias from the expanded path
					alias = InferAliasForDomain(relPath, tsconfigParsed, packageJsonImports)

					if alias == "" {
						// Fallback: generate a reasonable alias from the path
						pathParts := strings.Split(relPath, string(filepath.Separator))
						if len(pathParts) >= 2 {
							// Use the last two parts as alias (e.g., "app/auth" -> "@app/auth")
							alias = "@" + strings.Join(pathParts[len(pathParts)-2:], "/")
						} else if len(pathParts) == 1 {
							// Use the single part as alias (e.g., "auth" -> "@auth")
							alias = "@" + pathParts[0]
						}
					}
				}
			}
			// Get the original domain to access its Enabled field
			var enabled bool
			var aliasExplicit bool
			if originalPattern != "" {
				originalDomain := domainMap[originalPattern]
				enabled = originalDomain.Enabled
				aliasExplicit = originalDomain.Alias != ""
			} else {
				// Default to false if we can't find the original pattern (opt-in behavior)
				enabled = false
			}

			compiled = append(compiled, CompiledDomain{
				Path:          relPath,
				Alias:         alias,
				AbsolutePath:  absPath,
				Enabled:       enabled,
				AliasExplicit: aliasExplicit,
			})
		}
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
		compiledDomains, err := CompileDomains(rule.Domains, cwd, tsconfigParsed, packageJsonImports)
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

			// Skip if the source domain is disabled
			if !sourceDomain.Enabled {
				continue
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

	// Sort violations by file path, then by import order within each file
	sort.Slice(violations, func(i, j int) bool {
		// First sort by file path
		if violations[i].FilePath != violations[j].FilePath {
			return violations[i].FilePath < violations[j].FilePath
		}
		// If same file, sort by import index (maintains source file order)
		return violations[i].ImportIndex < violations[j].ImportIndex
	})

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

	// Skip if the source domain is disabled
	if fileDomain != nil && !fileDomain.Enabled {
		return violations
	}

	for importIndex, dep := range imports {
		// Pre-filter: Only check UserModule and MonorepoModule imports
		if dep.ResolvedType != UserModule && dep.ResolvedType != MonorepoModule {
			continue // Skip NodeModule, BuiltInModule, etc.
		}

		// Check if import targets a domain using the resolved path (ID field)
		var targetDomain *CompiledDomain
		var resolvedPath string
		if dep.ID != nil {
			// Check if the path is already absolute or relative
			if filepath.IsAbs(*dep.ID) {
				// Path is already absolute, use as-is
				resolvedPath = filepath.Clean(*dep.ID)
			} else {
				// Path is relative, convert to absolute for domain resolution
				resolvedPath = filepath.Clean(filepath.Join(cwd, *dep.ID))
			}
			targetDomain = ResolveImportTargetDomain(resolvedPath, compiledDomains)
		}

		// Check for violations based on the rule
		violation := checkImportForViolation(filePath, dep, fileDomain, targetDomain, resolvedPath, importIndex)
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
	resolvedPath string,
	importIndex int,
) *ImportConventionViolation {
	isRelative := IsRelativeImport(dep.Request)

	// Check if import is within the source domain
	isIntraDomain := false
	if targetDomain != nil && sourceDomain.Path == targetDomain.Path {
		isIntraDomain = true
	} else if resolvedPath != "" {
		// Check if resolved path is within the source domain by path prefix
		if strings.HasPrefix(resolvedPath, sourceDomain.AbsolutePath) {
			isIntraDomain = true
		}
	}

	// Intra-domain import (same domain)
	if isIntraDomain {
		if !isRelative {
			// Intra-domain import should be relative
			return &ImportConventionViolation{
				FilePath:        filePath,
				ImportRequest:   dep.Request,
				ImportIndex:     importIndex,
				ViolationType:   "should-be-relative",
				SourceDomain:    sourceDomain.Path,
				TargetDomain:    getTargetPath(targetDomain, resolvedPath),
				ExpectedPattern: "relative path (e.g., ./utils)",
				ActualPattern:   dep.Request,
			}
		}
		return nil // Valid intra-domain relative import
	}

	// Inter-domain import (outside the source domain)
	if isRelative {
		// Inter-domain import should be aliased
		expectedPattern := "alias path (e.g., @domain/utils)"
		if targetDomain != nil && targetDomain.Alias != "" {
			expectedPattern = targetDomain.Alias + "/*"
		} else {
			expectedPattern = "alias path (target domain not configured)"
		}

		return &ImportConventionViolation{
			FilePath:        filePath,
			ImportRequest:   dep.Request,
			ImportIndex:     importIndex,
			ViolationType:   "should-be-aliased",
			SourceDomain:    sourceDomain.Path,
			TargetDomain:    getTargetPath(targetDomain, resolvedPath),
			ExpectedPattern: expectedPattern,
			ActualPattern:   dep.Request,
		}
	}

	// Check if using correct alias (only if we have an explicit alias definition in config)
	if targetDomain != nil && targetDomain.Alias != "" && targetDomain.AliasExplicit {
		if !ValidateImportUsesCorrectAlias(dep.Request, targetDomain) {
			return &ImportConventionViolation{
				FilePath:        filePath,
				ImportRequest:   dep.Request,
				ImportIndex:     importIndex,
				ViolationType:   "wrong-alias",
				SourceDomain:    sourceDomain.Path,
				TargetDomain:    targetDomain.Path,
				ExpectedPattern: targetDomain.Alias + "/*",
				ActualPattern:   dep.Request,
			}
		}
	}

	return nil // Valid import
}

// getTargetPath returns the target domain path or resolved path as fallback
func getTargetPath(targetDomain *CompiledDomain, resolvedPath string) string {
	if targetDomain != nil {
		return targetDomain.Path
	}
	if resolvedPath != "" {
		// Convert absolute path to relative for display
		if strings.HasPrefix(resolvedPath, "/") {
			parts := strings.Split(strings.TrimPrefix(resolvedPath, "/"), "/")
			if len(parts) >= 2 {
				return strings.Join(parts[:2], "/")
			}
			return parts[0]
		}
		return resolvedPath
	}
	return "unknown"
}

// ... (rest of the code remains the same)
