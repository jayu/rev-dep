package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobwas/glob"
)

type ImportConventionViolation struct {
	FilePath      string  // Path to the file with the violation
	ImportRequest string  // The original import string (e.g., "@auth/utils")
	ImportIndex   int     // Index of this import in the source file (0-based)
	ViolationType string  // "should-be-relative" | "should-be-aliased" | "wrong-alias"
	Fix           *Change // Optional autofix change
}

type CompiledDomain struct {
	Path             string // Original path from config (e.g., "src/auth")
	AbsolutePath     string // Full absolute path for prefix matching
	EnforcedAlias    string // e.g., "@auth"
	AliasReplacement string
	AliasPathPrefix  string
	HasMatchingAlias bool
	CheckEnabled     bool
}

func IsRelativeImport(request string) bool {
	return strings.HasPrefix(request, "./") ||
		strings.HasPrefix(request, "../") ||
		request == "." ||
		request == ".."
}

func compileDomains(domains []ImportConventionDomain, compiledAliases []CompiledAlias, cwd string) []CompiledDomain {
	/**
	Alias replacement rules
	AliasPathPrefix - always must have trailing '/'
	AliasReplacement - always must have trailing '/'
		- except for empty string (we only cut prefix). This is handled by compile aliases
	*/
	var compiledDomains []CompiledDomain
	for _, domain := range domains {
		// should expand glob patterns
		if strings.HasSuffix(domain.Path, "*") {
			dirname := filepath.Dir(domain.Path)
			base := filepath.Base(domain.Path)
			dirNameMatcher := glob.MustCompile(base)
			baseDir := filepath.Join(cwd, dirname)

			directories, err := os.ReadDir(DenormalizePathForOS(baseDir))

			if err != nil {
				fmt.Printf("Failed to read domain directory: %e\n", err)
				continue
			}
			for _, directory := range directories {
				if directory.IsDir() && dirNameMatcher.Match(directory.Name()) {
					absolutePath := StandardiseDirPath(NormalizePathForInternal(filepath.Join(baseDir, directory.Name())))
					matches, aliasReplacement, aliasPathPrefix := getMatchingAlias(compiledAliases, absolutePath)

					compiledDomains = append(compiledDomains, CompiledDomain{
						Path:             domain.Path,
						AbsolutePath:     absolutePath,
						EnforcedAlias:    "", // glob domain does not have `enabled` config flag
						AliasReplacement: aliasReplacement,
						AliasPathPrefix:  aliasPathPrefix,
						HasMatchingAlias: matches,
						CheckEnabled:     domain.Enabled,
					})
				}
			}
		} else {
			// domain was defined as an object, path does not contains wildcard
			absolutePath := StandardiseDirPathInternal(NormalizePathForInternal(filepath.Join(cwd, domain.Path)))
			aliasReplacement := ""
			aliasPathPrefix := ""
			hasMatchingAlias := false

			// Try get alias defined in config
			if domain.Alias != "" {
				aliasReplacement = StandardiseDirPathInternal(domain.Alias) // user defined alias might not have trailing `/`
				aliasPathPrefix = absolutePath
				hasMatchingAlias = true
			}
			// Try match alias if not in config
			if !hasMatchingAlias {
				matches, aliasReplacementLocal, aliasPathPrefixLocal := getMatchingAlias(compiledAliases, absolutePath)
				if matches {
					aliasReplacement = aliasReplacementLocal
					aliasPathPrefix = aliasPathPrefixLocal
					hasMatchingAlias = true
				}
			}
			compiledDomains = append(compiledDomains, CompiledDomain{
				Path:             domain.Path,
				AbsolutePath:     absolutePath,
				EnforcedAlias:    domain.Alias,
				AliasReplacement: aliasReplacement,
				AliasPathPrefix:  aliasPathPrefix,
				HasMatchingAlias: hasMatchingAlias,
				CheckEnabled:     domain.Enabled,
			})
		}
	}
	return compiledDomains
}

type CompiledAlias struct {
	AliasReplacement string
	PathPrefix       string
	PathPrefixExact  bool
}
type AliasMapping struct {
	Alias  string
	Target string
}

func compileAliases(tsConfigParsed *TsConfigParsed, packageJsonImports *PackageJsonImports, cwd string) []CompiledAlias {
	var aliasMappings []AliasMapping
	for aliasKey, aliasValue := range tsConfigParsed.aliases {
		if strings.HasSuffix(aliasValue, "*") {
			aliasMappings = append(aliasMappings, AliasMapping{
				Alias:  aliasKey,
				Target: aliasValue,
			})
		}
	}

	sort.Slice(aliasMappings, func(a, b int) bool {
		targetAHasWildcard := strings.Contains(aliasMappings[a].Target, "*")
		targetBHasWildcard := strings.Contains(aliasMappings[b].Target, "*")

		// If one has wildcard and other doesn't, put the one without wildcard first
		if targetAHasWildcard && !targetBHasWildcard {
			return false
		}
		if !targetAHasWildcard && targetBHasWildcard {
			return true
		}

		// If both have wildcards or both don't, sort by alias length (longer first)
		if len(aliasMappings[a].Target) > len(aliasMappings[b].Target) {
			return true
		}
		if len(aliasMappings[a].Target) < len(aliasMappings[b].Target) {
			return false
		}
		return strings.Compare(aliasMappings[a].Target, aliasMappings[b].Target) < 0
	})

	compiledAliases := make([]CompiledAlias, 0, len(aliasMappings))

	for _, aliasMapping := range aliasMappings {
		absoluteTarget := NormalizePathForInternal(filepath.Join(cwd, aliasMapping.Target)) // on Windows filepath.Join will add wrong slash
		compiledAliases = append(compiledAliases, CompiledAlias{
			AliasReplacement: strings.TrimSuffix(aliasMapping.Alias, "*"),
			PathPrefix:       strings.TrimSuffix(absoluteTarget, "*"),
			PathPrefixExact:  !strings.Contains(aliasMapping.Target, "*"),
		})
	}

	return compiledAliases
}

func getMatchingAlias(compiledAliases []CompiledAlias, absolutePath string) (bool, string, string) {
	for _, alias := range compiledAliases {
		if alias.PathPrefixExact {
			if absolutePath == alias.PathPrefix {
				return true, alias.AliasReplacement, alias.PathPrefix
			}
		} else {
			if strings.HasPrefix(absolutePath, alias.PathPrefix) {
				return true, alias.AliasReplacement, alias.PathPrefix
			}
		}
	}
	return false, "", ""
}

func matchDomainToAbsolutePath(compiledDomains []CompiledDomain, absolutePath string) (bool, CompiledDomain) {
	for _, domain := range compiledDomains {
		if strings.HasPrefix(absolutePath, domain.AbsolutePath) {
			return true, domain
		}
	}
	return false, CompiledDomain{}
}

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

func CheckImportConventionsFromTree(
	minimalTree MinimalDependencyTree,
	files []string,
	parsedRules []ImportConventionRule,
	resolver *ModuleResolver,
	cwd string,
	autofix bool,
) ([]ImportConventionViolation, bool) {
	// TODO tests for current functionality

	shouldWarnAboutImportConventionWithPJsonImports := false
	if len(resolver.packageJsonImports.imports) > 0 {
		shouldWarnAboutImportConventionWithPJsonImports = true
	}

	var violations []ImportConventionViolation
	compiledAliases := compileAliases(resolver.tsConfigParsed, resolver.packageJsonImports, cwd)

	for _, importConventionRule := range parsedRules {
		compiledDomains := compileDomains(importConventionRule.Domains, compiledAliases, cwd)

		for filePath, imports := range minimalTree {
			fileMatches, fileDomain := matchDomainToAbsolutePath(compiledDomains, filePath)
			if fileMatches && fileDomain.CheckEnabled {
				for impIdx, imp := range imports {
					if imp.ResolvedType == UserModule || imp.ResolvedType == MonorepoModule {
						importFilePath := imp.ID
						isSameDomain := strings.HasPrefix(importFilePath, fileDomain.AbsolutePath)
						isRelative := IsRelativeImport(imp.Request)
						if isSameDomain {
							if !isRelative {
								newRequest, err := filepath.Rel(filepath.Dir(filePath), importFilePath)

								if err == nil {
									if !strings.HasPrefix(newRequest, "./") && !strings.HasPrefix(newRequest, "../") {
										newRequest = "./" + newRequest
									}
									newRequest = adjustImportPathStyle(newRequest, imp.Request)
									violations = append(violations, ImportConventionViolation{
										FilePath:      filePath,
										ImportRequest: imp.Request,
										ImportIndex:   impIdx,
										ViolationType: "should-be-relative",
										Fix: &Change{
											Start: int32(imp.RequestStart),
											End:   int32(imp.RequestEnd),
											Text:  newRequest,
										},
									})
								}
							}
						} else {
							importMatches, importDomain := matchDomainToAbsolutePath(compiledDomains, importFilePath)
							if isRelative {
								var fix *Change
								if importMatches && importDomain.HasMatchingAlias {
									fix = &Change{
										Start: int32(imp.RequestStart),
										End:   int32(imp.RequestEnd),
										Text:  adjustImportPathStyle(importDomain.AliasReplacement+strings.TrimPrefix(imp.ID, importDomain.AliasPathPrefix), imp.Request),
									}
								}
								violations = append(violations, ImportConventionViolation{
									FilePath:      filePath,
									ImportRequest: imp.Request,
									ImportIndex:   impIdx,
									ViolationType: "should-be-aliased",
									Fix:           fix,
								})
							} else {
								if importMatches && importDomain.EnforcedAlias != "" {
									if !strings.HasPrefix(imp.Request, importDomain.EnforcedAlias) {
										newRequest := importDomain.AliasReplacement + strings.TrimPrefix(imp.ID, importDomain.AliasPathPrefix)
										newRequest = adjustImportPathStyle(newRequest, imp.Request)
										violations = append(violations, ImportConventionViolation{
											FilePath:      filePath,
											ImportRequest: imp.Request,
											ImportIndex:   impIdx,
											ViolationType: "wrong-alias",
											Fix: &Change{
												Start: int32(imp.RequestStart),
												End:   int32(imp.RequestEnd),
												Text:  newRequest,
											},
										})
									}
								}
							}
						}
					}
				}
			}
		}

	}

	return violations, shouldWarnAboutImportConventionWithPJsonImports
}

/**

Tests

== STEP 1 ==

Config 1
- three domains defined using normal object-based config
  - path + alias in config (without trailing `/`) + enabled
	- path + no alias in config, but alias in tsconfig - basepath alias + enabled
	- path + alias in config (with trailing `/`) + disabled
Config 2
- two domains defined using simplified string-based config
  - one with wildcard - resolved into two directories, one dir has ts alias eg @consultants, another one has only basepath url alias
	- one without wildcard - resolved into existing dir, has alias in ts config

Test cases (all asserts violations and autofix)
- should-be-relative + persist extension
- should-be-aliased
  - target alias defined in config
	- target alias resolving to non-empty string eg @consultants
	- target alias resolving to baseUrl wildcard ("*": ["./*"],)
- wrong-alias
	- current alias is baseUrl wildcard alias ("*": ["./*"],)
	- current alias is non-empty string alias eg @consultants

For Config 1 Make sure to add all the above in the cases in disabled domain to verify it's actually disabled

== STEP 2 ==

Once the above 2 config test suits are implemented, copy-paste it but use tsconfig without basepath url alias, make different assertions
*/
