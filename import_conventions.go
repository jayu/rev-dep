package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ImportConventionViolation represents a violation of import conventions
type ImportConventionViolation struct {
	FilePath        string  // Path to the file with the violation
	ImportRequest   string  // The original import string (e.g., "@auth/utils")
	ImportResolved  string  // The resolved import path
	ImportIndex     int     // Index of this import in the source file (0-based)
	ViolationType   string  // "should-be-relative" | "should-be-aliased" | "wrong-alias"
	SourceDomain    string  // Domain of the source file
	TargetDomain    string  // Domain of the target import
	ExpectedPattern string  // Expected import pattern
	ActualPattern   string  // Actual import pattern
	Fix             *Change // Optional autofix change
}

// CompiledDomain represents a processed domain with absolute path for fast matching
type CompiledDomain struct {
	Path          string // Original path from config (e.g., "src/auth")
	Alias         string // e.g., "@auth" (inferred or explicit)
	AbsolutePath  string // Full absolute path for prefix matching
	Enabled       bool   // Whether checks should be performed for this domain
	AliasExplicit bool   // Whether the alias was explicitly provided in config
}

// ExpandDomainGlobs expands glob patterns to concrete directory paths
// Called once at config time, NOT at runtime
// "src/*" → ["src/auth", "src/users", "src/shared"]
func ExpandDomainGlobs(patterns []string, cwd string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/*") {
			baseDir := strings.TrimSuffix(pattern, "/*")
			fullBaseDir := filepath.Join(cwd, baseDir)

			entries, err := os.ReadDir(fullBaseDir)
			if err != nil {
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
	domainMap := make(map[string]ImportConventionDomain)
	var patterns []string

	for _, d := range domains {
		domainMap[d.Path] = d
		patterns = append(patterns, d.Path)
	}

	expandedPaths, err := ExpandDomainGlobs(patterns, cwd)
	if err != nil {
		return nil, fmt.Errorf("failed to expand domain globs: %w", err)
	}

	for _, absPath := range expandedPaths {
		absPath = filepath.Clean(absPath)
		relPath, _ := filepath.Rel(cwd, absPath)

		// Find which original pattern this expanded path came from for Enabled/Alias settings
		var originalPattern string
		for _, p := range patterns {
			absPatternBase := filepath.Join(cwd, strings.TrimSuffix(p, "*"))
			if strings.HasPrefix(absPath, absPatternBase) {
				originalPattern = p
				break
			}
		}

		domain := domainMap[originalPattern]
		alias := domain.Alias
		if alias == "" {
			alias = InferAliasForDomain(relPath, tsconfigParsed, packageJsonImports)
		}

		compiled = append(compiled, CompiledDomain{
			Path:          relPath,
			Alias:         alias,
			AbsolutePath:  absPath,
			Enabled:       domain.Enabled,
			AliasExplicit: domain.Alias != "",
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
		type aliasPair struct {
			alias string
			path  string
		}
		var pairs []aliasPair
		for alias, path := range tsconfigParsed.aliases {
			pairs = append(pairs, aliasPair{alias, path})
		}

		// Sort by path length descending - most specific (longest path) first
		sort.Slice(pairs, func(i, j int) bool {
			return len(pairs[i].path) > len(pairs[j].path)
		})

		for _, p := range pairs {
			// Remove wildcards from path for comparison
			cleanPath := strings.TrimSuffix(p.path, "/*")
			cleanPath = strings.TrimSuffix(cleanPath, "/**")

			// Check if the domain path matches this path
			// If cleanPath is ".", it matches everything (equivalent to root)
			isMatch := strings.HasPrefix(domainPath, cleanPath)
			if !isMatch && cleanPath == "." {
				isMatch = true
			}

			if isMatch {
				// Return the alias without wildcards
				cleanAlias := strings.TrimSuffix(p.alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")
				return cleanAlias
			}
		}
	}

	// Then try to infer from package.json imports using simple targets
	if packageJsonImports != nil {
		type aliasPair struct {
			alias string
			path  string
		}
		var pairs []aliasPair
		for alias, path := range packageJsonImports.simpleImportTargetsByKey {
			pairs = append(pairs, aliasPair{alias, path})
		}

		// Sort by path length descending - most specific (longest path) first
		sort.Slice(pairs, func(i, j int) bool {
			return len(pairs[i].path) > len(pairs[j].path)
		})

		for _, p := range pairs {
			// Remove wildcards from path for comparison
			cleanPath := strings.TrimSuffix(p.path, "/*")
			cleanPath = strings.TrimSuffix(cleanPath, "/**")

			// Check if the domain path matches this path
			if strings.HasPrefix(domainPath, cleanPath) {
				// Return the alias without wildcards
				cleanAlias := strings.TrimSuffix(p.alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")
				return cleanAlias
			}
		}
	}

	// No matching alias found, try fallback from path
	pathParts := strings.Split(domainPath, string(filepath.Separator))
	if len(pathParts) >= 2 {
		// Use the last two parts as alias (e.g., "app/auth" -> "@app/auth")
		return "@" + strings.Join(pathParts[len(pathParts)-2:], "/")
	} else if len(pathParts) == 1 && pathParts[0] != "." {
		// Use the single part as alias (e.g., "auth" -> "@auth")
		return "@" + pathParts[0]
	}

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

// ValidateImportUsesCorrectAlias checks if import uses the correct alias for the target domain
func ValidateImportUsesCorrectAlias(request string, targetDomain *CompiledDomain, tsconfigParsed *TsConfigParsed, packageJsonImports *PackageJsonImports) bool {
	if targetDomain == nil || targetDomain.Alias == "" {
		return false
	}

	// If domain has explicit alias, use the original validation logic
	if targetDomain.AliasExplicit {
		if targetDomain.Alias == "*" {
			// If alias is catch-all "*", import should match the domain path
			// e.g. alias "*" -> import "src/auth/utils" for domain "src/auth"
			return strings.HasPrefix(request, targetDomain.Path)
		}
		return strings.HasPrefix(request, targetDomain.Alias)
	}

	// For inferred aliases (catch-all "*"), check if there's a more specific alias available
	if targetDomain.Alias == "*" {
		// First check tsconfig aliases
		if tsconfigParsed != nil {
			for alias := range tsconfigParsed.aliases {
				if alias == "*" {
					continue // Skip the catch-all itself
				}

				// Remove wildcards for matching
				cleanAlias := strings.TrimSuffix(alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")

				// Check if the import starts with this more specific alias
				if strings.HasPrefix(request, cleanAlias) {
					// Import uses a more specific alias, which is correct
					return true
				}
			}
		}

		// Then check package.json imports
		if packageJsonImports != nil {
			for alias := range packageJsonImports.simpleImportTargetsByKey {
				// Remove wildcards for matching
				cleanAlias := strings.TrimSuffix(alias, "/*")
				cleanAlias = strings.TrimSuffix(cleanAlias, "/**")

				// Check if the import starts with this more specific alias
				if strings.HasPrefix(request, cleanAlias) {
					// Import uses a more specific alias from package.json, which is correct
					return true
				}
			}
		}
	}

	// Fall back to original validation for catch-all alias
	if targetDomain.Alias == "*" {
		// If alias is catch-all "*", import should match the domain path
		// e.g. alias "*" -> import "src/auth/utils" for domain "src/auth"
		return strings.HasPrefix(request, targetDomain.Path)
	}
	return strings.HasPrefix(request, targetDomain.Alias)
}

// CheckImportConventionsFromTree checks import conventions from dependency tree with early filtering
// Pre-filter: Only check files that belong to a domain
// Pre-filter: Only check UserModule and MonorepoModule imports
func CheckImportConventionsFromTree(
	minimalTree MinimalDependencyTree,
	files []string,
	parsedRules []ImportConventionRule,
	resolver *ModuleResolver,
	cwd string,
	autofix bool,
) []ImportConventionViolation {
	var violations []ImportConventionViolation

	var tsconfigParsed *TsConfigParsed
	var packageJsonImports *PackageJsonImports
	if resolver != nil {
		tsconfigParsed = resolver.tsConfigParsed
		packageJsonImports = resolver.packageJsonImports
	}

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
			// Paths are already absolute in our system
			domain := ResolveDomainForFile(file, compiledDomains)
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
				rule.Autofix && autofix,
				cwd,
				tsconfigParsed,
				packageJsonImports,
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
	autofix bool,
	cwd string,
	tsconfigParsed *TsConfigParsed,
	packageJsonImports *PackageJsonImports,
) []ImportConventionViolation {
	violations := []ImportConventionViolation{}

	// Skip if the source domain is disabled
	if fileDomain != nil && !fileDomain.Enabled {
		return violations
	}

	// Check if tsconfig has explicit catch-all alias by examining the original tsconfig content
	hasExplicitCatchAll := false
	if tsconfigParsed != nil {
		// Try to find and read the original tsconfig.json to check if "*" was explicitly defined
		tsconfigPath := filepath.Join(cwd, "tsconfig.json")
		if tsconfigBytes, err := os.ReadFile(tsconfigPath); err == nil {
			originalContent := string(tsconfigBytes)
			// Check if "*" is explicitly mentioned in the paths section
			hasExplicitCatchAll = strings.Contains(originalContent, `"*"`) ||
				strings.Contains(originalContent, `'*'`) ||
				strings.Contains(originalContent, "\"*\"")
		} else {
			// Fallback: if we can't read the original file, assume catch-all is available if it exists
			_, hasCatchAll := tsconfigParsed.aliases["*"]
			hasExplicitCatchAll = hasCatchAll
		}
	}

	for importIndex, dep := range imports {
		// ... (rest of the code remains the same)
		// Pre-filter: Only check UserModule and MonorepoModule imports
		if dep.ResolvedType != UserModule && dep.ResolvedType != MonorepoModule {
			continue // Skip NodeModule, BuiltInModule, etc.
		}

		// Check if import targets a domain using the resolved path (ID field)
		var targetDomain *CompiledDomain
		var resolvedPath string
		if dep.ID != nil {
			resolvedPath = *dep.ID
			targetDomain = ResolveDomainForFile(resolvedPath, compiledDomains)
		}

		// Check for violations based on the rule
		violation := checkImportForViolation(filePath, dep, fileDomain, targetDomain, resolvedPath, importIndex, autofix, cwd, tsconfigParsed, hasExplicitCatchAll, packageJsonImports)
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
	autofix bool,
	cwd string,
	tsconfigParsed *TsConfigParsed,
	hasExplicitCatchAll bool,
	packageJsonImports *PackageJsonImports,
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
			violation := &ImportConventionViolation{
				FilePath:        filePath,
				ImportRequest:   dep.Request,
				ImportIndex:     importIndex,
				ViolationType:   "should-be-relative",
				SourceDomain:    sourceDomain.Path,
				TargetDomain:    getTargetPath(targetDomain, resolvedPath),
				ExpectedPattern: "relative path (e.g., ./utils)",
				ActualPattern:   dep.Request,
			}

			if autofix && dep.RequestStart > 0 && resolvedPath != "" {
				// Generate relative path from source file to target file
				sourceDir := filepath.Dir(filePath)
				rel, err := filepath.Rel(sourceDir, resolvedPath)
				if err == nil {
					// Ensure it starts with ./ or ../
					if !IsRelativeImport(rel) {
						rel = "./" + rel
					}
					// Preserving the style (extension and /index) from original request
					rel = adjustImportPathStyle(rel, dep.Request)
					violation.Fix = &Change{
						Start: int32(dep.RequestStart),
						End:   int32(dep.RequestEnd),
						Text:  rel,
					}
				}
			}
			return violation
		}
		return nil // Valid intra-domain relative import
	}

	// Inter-domain import (outside the source domain)
	if isRelative {
		// Inter-domain import should be aliased
		expectedPattern := "alias path (e.g., @domain/utils)"
		if targetDomain != nil && targetDomain.Alias != "" {
			if targetDomain.Alias == "*" {
				expectedPattern = targetDomain.Path + "/*"
			} else {
				expectedPattern = targetDomain.Alias + "/*"
			}
		} else {
			expectedPattern = "alias path (target domain not configured)"
		}

		violation := &ImportConventionViolation{
			FilePath:        filePath,
			ImportRequest:   dep.Request,
			ImportIndex:     importIndex,
			ViolationType:   "should-be-aliased",
			SourceDomain:    sourceDomain.Path,
			TargetDomain:    getTargetPath(targetDomain, resolvedPath),
			ExpectedPattern: expectedPattern,
			ActualPattern:   dep.Request,
		}

		if autofix && dep.RequestStart > 0 {
			var fixedPath string

			if targetDomain != nil && targetDomain.Alias != "" {
				// Case 1: Target domain is configured - use its alias
				relPathInTarget, err := filepath.Rel(targetDomain.AbsolutePath, resolvedPath)
				if err == nil {
					fixedPath = targetDomain.Alias
					if targetDomain.Alias == "*" {
						// If alias is "*", use the domain path as base
						fixedPath = targetDomain.Path
					}

					if relPathInTarget != "." {
						// Preserving the style (extension and /index) from original request
						fixedPath = adjustImportPathStyle(filepath.Join(fixedPath, relPathInTarget), dep.Request)
					}
				}
			} else if tsconfigParsed != nil && resolvedPath != "" && hasExplicitCatchAll {
				// Case 2: Target domain not configured - try to infer catch-all alias
				// Only do this if user has explicitly configured catch-all alias
				relPath, err := filepath.Rel(cwd, resolvedPath)
				if err == nil {
					relPath = filepath.ToSlash(relPath)
					inferredAlias := InferAliasForDomain(relPath, tsconfigParsed, nil)

					// Only use inferred alias if it's a catch-all alias
					if inferredAlias == "*" {
						fixedPath = inferredAlias
						if inferredAlias == "*" {
							fixedPath = relPath
						}
						// Preserving the style (extension and /index) from original request
						fixedPath = adjustImportPathStyle(fixedPath, dep.Request)
					}
				}
			}

			if fixedPath != "" {
				violation.Fix = &Change{
					Start: int32(dep.RequestStart),
					End:   int32(dep.RequestEnd),
					Text:  fixedPath,
				}
			}
		}

		return violation
	}

	// Check if using correct alias (only if we have an explicit alias definition in config or catch-all alias)
	if targetDomain != nil && targetDomain.Alias != "" && (targetDomain.AliasExplicit || targetDomain.Alias == "*") {
		if !ValidateImportUsesCorrectAlias(dep.Request, targetDomain, tsconfigParsed, packageJsonImports) {
			violation := &ImportConventionViolation{
				FilePath:        filePath,
				ImportRequest:   dep.Request,
				ImportIndex:     importIndex,
				ViolationType:   "wrong-alias",
				SourceDomain:    sourceDomain.Path,
				TargetDomain:    targetDomain.Path,
				ExpectedPattern: targetDomain.Alias + "/*",
				ActualPattern:   dep.Request,
			}

			if targetDomain.Alias == "*" {
				violation.ExpectedPattern = targetDomain.Path + "/*"
			}

			if autofix && dep.RequestStart > 0 {
				// If it's already an alias but wrong one, and we know the target domain,
				// we can try to fix it.
				// This is slightly more complex as we need to find the relative part of the import.
				// For now, if we have the resolved path, we can regenerate the aliased path.
				if resolvedPath != "" {
					relPathInTarget, err := filepath.Rel(targetDomain.AbsolutePath, resolvedPath)
					if err == nil {
						fixedPath := targetDomain.Alias
						if targetDomain.Alias == "*" {
							fixedPath = targetDomain.Path
						}

						if relPathInTarget != "." {
							fixedPath = adjustImportPathStyle(filepath.Join(fixedPath, relPathInTarget), dep.Request)
						}
						violation.Fix = &Change{
							Start: int32(dep.RequestStart),
							End:   int32(dep.RequestEnd),
							Text:  fixedPath,
						}
					}
				}
			}
			return violation
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
		if after, found := strings.CutPrefix(resolvedPath, "/"); found {
			parts := strings.Split(after, "/")
			if len(parts) >= 2 {
				return strings.Join(parts[:2], "/")
			}
			return parts[0]
		}
		return resolvedPath
	}
	return "unknown"
}

// adjustImportPathStyle ensures that the generated fix path matches the style of the original import request
// regarding file extensions and /index suffixes.
func adjustImportPathStyle(newPath, originalRequest string) string {
	// 1. Check if original had any of these extensions
	hasExtension := false
	for _, ext := range SourceExtensions {
		if strings.HasSuffix(originalRequest, ext) {
			hasExtension = true
			break
		}
	}

	// 2. Check if original had /index suffix (with or without extension)
	hasIndex := strings.HasSuffix(originalRequest, "/index")
	if !hasIndex {
		for _, ext := range SourceExtensions {
			if strings.HasSuffix(originalRequest, "/index"+ext) {
				hasIndex = true
				break
			}
		}
	}

	result := newPath
	// If original didn't have extension, strip from result
	if !hasExtension {
		for _, ext := range SourceExtensions {
			if strings.HasSuffix(result, ext) {
				result = strings.TrimSuffix(result, ext)
				break
			}
		}
	}

	// If original didn't have /index, strip from result
	if !hasIndex {
		result = strings.TrimSuffix(result, "/index")
	}

	return result
}
