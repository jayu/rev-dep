package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tidwall/jsonc"
)

type BoundaryRule struct {
	Name    string   `json:"name"`
	Pattern string   `json:"pattern"`        // Glob pattern for files in this boundary
	Allow   []string `json:"allow"`          // Glob patterns for allowed imports
	Deny    []string `json:"deny,omitempty"` // Glob patterns for denied imports (overrides allow)
}

type CircularImportsOptions struct {
	Enabled           bool `json:"enabled"`
	IgnoreTypeImports bool `json:"ignoreTypeImports"`
}

type OrphanFilesOptions struct {
	Enabled           bool     `json:"enabled"`
	ValidEntryPoints  []string `json:"validEntryPoints,omitempty"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports,omitempty"`
	GraphExclude      []string `json:"graphExclude,omitempty"`
}

type UnusedNodeModulesOptions struct {
	Enabled                   bool     `json:"enabled"`
	IncludeModules            []string `json:"includeModules,omitempty"`
	ExcludeModules            []string `json:"excludeModules,omitempty"`
	PkgJsonFieldsWithBinaries []string `json:"pkgJsonFieldsWithBinaries,omitempty"`
	FilesWithBinaries         []string `json:"filesWithBinaries,omitempty"`
	FilesWithModules          []string `json:"filesWithModules,omitempty"`
	OutputType                string   `json:"outputType,omitempty"` // "list", "groupByModule", "groupByFile"
}

type MissingNodeModulesOptions struct {
	Enabled        bool     `json:"enabled"`
	IncludeModules []string `json:"includeModules,omitempty"`
	ExcludeModules []string `json:"excludeModules,omitempty"`
	OutputType     string   `json:"outputType,omitempty"` // "list", "groupByModule", "groupByFile"
}

// ImportConventionDomain represents a single domain definition
type ImportConventionDomain struct {
	Path    string `json:"path,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// ImportConventionRule represents a rule for import path conventions
type ImportConventionRule struct {
	Rule    string
	Domains []ImportConventionDomain
	Autofix bool
}

type Rule struct {
	Path                        string                     `json:"path"`                             // Required
	FollowMonorepoPackages      bool                       `json:"followMonorepoPackages,omitempty"` // Default: true
	ModuleBoundaries            []BoundaryRule             `json:"moduleBoundaries,omitempty"`
	CircularImportsDetection    *CircularImportsOptions    `json:"circularImportsDetection,omitempty"`
	OrphanFilesDetection        *OrphanFilesOptions        `json:"orphanFilesDetection,omitempty"`
	UnusedNodeModulesDetection  *UnusedNodeModulesOptions  `json:"unusedNodeModulesDetection,omitempty"`
	MissingNodeModulesDetection *MissingNodeModulesOptions `json:"missingNodeModulesDetection,omitempty"`
	ImportConventions           []ImportConventionRule     `json:"-"`
}

type RevDepConfig struct {
	ConfigVersion  string   `json:"configVersion"` // Required
	Schema         string   `json:"$schema,omitempty"`
	ConditionNames []string `json:"conditionNames,omitempty"`
	IgnoreFiles    []string `json:"ignoreFiles,omitempty"`
	Rules          []Rule   `json:"rules"`
}

var configFileName = "rev-dep.config.json"
var hiddenConfigFileName = ".rev-dep.config.json"
var configFileNameJsonc = "rev-dep.config.jsonc"
var hiddenConfigFileNameJsonc = ".rev-dep.config.jsonc"

// supportedConfigVersions lists config versions supported by this CLI release.
// Update this slice when adding or removing support for config versions.
var supportedConfigVersions = []string{"1.0", "1.1", "1.2"}

// validateConfigVersion returns an error when the provided config version
// is not in the supportedConfigVersions list.
func validateConfigVersion(version string) error {
	if slices.Contains(supportedConfigVersions, version) {
		return nil
	}
	return fmt.Errorf("unsupported configVersion '%s'. Supported versions: %v", version, supportedConfigVersions)
}

// findConfigFile looks for config files in the given directory
// It checks for .rev-dep.config.jsonc, .rev-dep.config.json, rev-dep.config.jsonc, and rev-dep.config.json
// Returns error if multiple files exist (ambiguous configuration)
func findConfigFile(dir string) (string, error) {
	hiddenConfigPathJsonc := filepath.Join(dir, hiddenConfigFileNameJsonc)
	hiddenConfigPath := filepath.Join(dir, hiddenConfigFileName)
	regularConfigPathJsonc := filepath.Join(dir, configFileNameJsonc)
	regularConfigPath := filepath.Join(dir, configFileName)

	var foundFiles []string

	// Check for all config file variants
	if _, err := os.Stat(hiddenConfigPathJsonc); err == nil {
		foundFiles = append(foundFiles, hiddenConfigPathJsonc)
	}
	if _, err := os.Stat(hiddenConfigPath); err == nil {
		foundFiles = append(foundFiles, hiddenConfigPath)
	}
	if _, err := os.Stat(regularConfigPathJsonc); err == nil {
		foundFiles = append(foundFiles, regularConfigPathJsonc)
	}
	if _, err := os.Stat(regularConfigPath); err == nil {
		foundFiles = append(foundFiles, regularConfigPath)
	}

	// Multiple files exist - ambiguous configuration
	if len(foundFiles) > 1 {
		return "", fmt.Errorf("multiple config files found in %s: %v - please use only one config file", dir, foundFiles)
	}

	// Return the one that exists
	if len(foundFiles) == 1 {
		return foundFiles[0], nil
	}

	return "", fmt.Errorf("no config file found in %s", dir)
}

// LoadConfig loads the rev-dep configuration from the specified path.
// configPath can be a specific file path or a directory containing rev-dep.config.json or rev-dep.config.jsonc.
// Returns a single RevDepConfig object.
func LoadConfig(configPath string) ([]RevDepConfig, error) {
	content, err := readConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	return ParseConfig(content)
}

// readConfigFile reads the config file content from the specified path.
// configPath can be a specific file path or a directory containing config files.
func readConfigFile(configPath string) ([]byte, error) {
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}

	if fileInfo.IsDir() {
		// Look for config files in the directory
		configFile, err := findConfigFile(configPath)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(configFile)
	}

	// Direct file path provided
	return os.ReadFile(configPath)
}

// ParseConfig parses the config content and returns a validated RevDepConfig.
func ParseConfig(content []byte) ([]RevDepConfig, error) {
	// First, parse into a generic map to validate field names and types
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(jsonc.ToJSON(content), &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate field names and structure
	if err := validateRawConfig(rawConfig); err != nil {
		return nil, err
	}

	// Use a temporary struct to unmarshal with generic types for normalization
	// We use this to capture the non-standard "domains" field (string or object)
	type rawImportConventionRule struct {
		Rule    string      `json:"rule"`
		Domains interface{} `json:"domains"`
		Autofix bool        `json:"autofix,omitempty"`
	}
	type rawRuleItems struct {
		ImportConventions []rawImportConventionRule `json:"importConventions"`
	}
	var rawRules struct {
		Rules []rawRuleItems `json:"rules"`
	}
	if err := json.Unmarshal(jsonc.ToJSON(content), &rawRules); err != nil {
		return nil, fmt.Errorf("failed to parse config for normalization: %w", err)
	}

	// Parse into final typed struct
	var config RevDepConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config into final structure: %w", err)
	}

	// Validate config
	if err := ValidateConfig(&config); err != nil {
		return nil, err
	}

	// Validate config version against supported versions for this CLI
	if err := validateConfigVersion(config.ConfigVersion); err != nil {
		return nil, err
	}

	// Set default values for optional fields and process import conventions
	for i := range config.Rules {
		// Default FollowMonorepoPackages to true if not explicitly set (zero value is false)
		if rawRules, ok := rawConfig["rules"].([]interface{}); ok && i < len(rawRules) {
			if ruleMap, ok := rawRules[i].(map[string]interface{}); ok {
				if _, exists := ruleMap["followMonorepoPackages"]; !exists {
					config.Rules[i].FollowMonorepoPackages = true
				}
			}
		}

		// Process and normalize import conventions
		if i < len(rawRules.Rules) {
			config.Rules[i].ImportConventions = make([]ImportConventionRule, len(rawRules.Rules[i].ImportConventions))
			for j, rawConv := range rawRules.Rules[i].ImportConventions {
				parsedDomains, err := parseImportConventionDomains(rawConv.Domains)
				if err != nil {
					return nil, fmt.Errorf("failed to parse import convention domains for rules[%d].importConventions[%d]: %w", i, j, err)
				}
				config.Rules[i].ImportConventions[j] = ImportConventionRule{
					Rule:    rawConv.Rule,
					Domains: parsedDomains,
					Autofix: rawConv.Autofix,
				}
			}
		}
	}

	return []RevDepConfig{config}, nil
}

// validateRulePath validates that a rule path is acceptable
func validateRulePath(path string) error {
	// Reject paths that try to go outside the project
	if strings.Contains(path, "../") {
		return fmt.Errorf("rule path '%s' contains '../' which is not allowed. Rule paths must be within the project directory", path)
	}

	// Normalize path by removing leading "./" if present
	normalizedPath := strings.TrimPrefix(path, "./")

	// Empty path is not allowed, except for "." which represents root
	if normalizedPath == "" && path != "./" && path != "." {
		return fmt.Errorf("rule path cannot be empty")
	}

	return nil
}

// normalizeRulePath normalizes a rule path by removing leading "./"
func normalizeRulePath(path string) string {
	// Remove leading "./" prefix
	normalized := strings.TrimPrefix(path, "./")

	// If the result is empty and the original was "./" or ".", return "." for root
	if normalized == "" && (path == "./" || path == ".") {
		return "."
	}

	return normalized
}

// validateRawConfig validates field names and basic structure before typed parsing
func validateRawConfig(raw map[string]interface{}) error {
	allowedRootFields := map[string]bool{
		"$schema":        true,
		"configVersion":  true,
		"conditionNames": true,
		"ignoreFiles":    true,
		"rules":          true,
	}

	for field := range raw {
		if !allowedRootFields[field] {
			return fmt.Errorf("unknown field '%s' in config root", field)
		}
	}

	rules, ok := raw["rules"]
	if !ok {
		return fmt.Errorf("rules field is required")
	}

	rulesArray, ok := rules.([]interface{})
	if !ok {
		return fmt.Errorf("rules must be an array, got %T", rules)
	}

	for i, rule := range rulesArray {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			return fmt.Errorf("rules[%d] must be an object, got %T", i, rule)
		}

		if err := validateRawRule(ruleMap, i); err != nil {
			return err
		}
	}

	return nil
}

// validateRawRule validates a single rule object
func validateRawRule(rule map[string]interface{}, index int) error {
	allowedRuleFields := map[string]bool{
		"path":                        true,
		"followMonorepoPackages":      true,
		"moduleBoundaries":            true,
		"circularImportsDetection":    true,
		"orphanFilesDetection":        true,
		"unusedNodeModulesDetection":  true,
		"missingNodeModulesDetection": true,
		"importConventions":           true,
	}

	for field := range rule {
		if !allowedRuleFields[field] {
			return fmt.Errorf("rules[%d]: unknown field '%s'", index, field)
		}
	}

	// Check required path field
	path, exists := rule["path"]
	if !exists {
		return fmt.Errorf("rules[%d].path is required", index)
	}
	pathStr, ok := path.(string)
	if !ok {
		return fmt.Errorf("rules[%d].path must be a string, got %T", index, path)
	}

	// Validate the path
	if err := validateRulePath(pathStr); err != nil {
		return fmt.Errorf("rules[%d].path: %v", index, err)
	}

	// Validate module boundaries if present
	if boundaries, exists := rule["moduleBoundaries"]; exists {
		if err := validateRawModuleBoundaries(boundaries, index); err != nil {
			return err
		}
	}

	// Validate detection options if present
	if circular, exists := rule["circularImportsDetection"]; exists {
		if err := validateRawCircularImportsDetection(circular, index); err != nil {
			return err
		}
	}

	if orphan, exists := rule["orphanFilesDetection"]; exists {
		if err := validateRawOrphanFilesDetection(orphan, index); err != nil {
			return err
		}
	}

	if unused, exists := rule["unusedNodeModulesDetection"]; exists {
		if err := validateRawUnusedNodeModulesDetection(unused, index); err != nil {
			return err
		}
	}

	if missing, exists := rule["missingNodeModulesDetection"]; exists {
		if err := validateRawMissingNodeModulesDetection(missing, index); err != nil {
			return err
		}
	}

	if conventions, exists := rule["importConventions"]; exists {
		if err := validateRawImportConventions(conventions, index); err != nil {
			return err
		}
	}

	return nil
}

// validateRawModuleBoundaries validates module boundaries structure
func validateRawModuleBoundaries(boundaries interface{}, ruleIndex int) error {
	boundariesArray, ok := boundaries.([]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].moduleBoundaries must be an array, got %T", ruleIndex, boundaries)
	}

	for i, boundary := range boundariesArray {
		boundaryMap, ok := boundary.(map[string]interface{})
		if !ok {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d] must be an object, got %T", ruleIndex, i, boundary)
		}

		allowedBoundaryFields := map[string]bool{
			"name":    true,
			"pattern": true,
			"allow":   true,
			"deny":    true,
		}

		for field := range boundaryMap {
			if !allowedBoundaryFields[field] {
				return fmt.Errorf("rules[%d].moduleBoundaries[%d]: unknown field '%s'", ruleIndex, i, field)
			}
		}

		// Check required fields
		if _, exists := boundaryMap["name"]; !exists {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].name is required", ruleIndex, i)
		}
		if _, exists := boundaryMap["pattern"]; !exists {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].pattern is required", ruleIndex, i)
		}

		// Check field types
		if name, ok := boundaryMap["name"]; !ok || name == nil {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].name cannot be null", ruleIndex, i)
		} else if _, ok := name.(string); !ok {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].name must be a string, got %T", ruleIndex, i, name)
		}

		if pattern, ok := boundaryMap["pattern"]; !ok || pattern == nil {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].pattern cannot be null", ruleIndex, i)
		} else if _, ok := pattern.(string); !ok {
			return fmt.Errorf("rules[%d].moduleBoundaries[%d].pattern must be a string, got %T", ruleIndex, i, pattern)
		}

		// Check optional array fields
		if allow, exists := boundaryMap["allow"]; exists && allow != nil {
			if _, ok := allow.([]interface{}); !ok {
				return fmt.Errorf("rules[%d].moduleBoundaries[%d].allow must be an array, got %T", ruleIndex, i, allow)
			}
		}

		if deny, exists := boundaryMap["deny"]; exists && deny != nil {
			if _, ok := deny.([]interface{}); !ok {
				return fmt.Errorf("rules[%d].moduleBoundaries[%d].deny must be an array, got %T", ruleIndex, i, deny)
			}
		}
	}

	return nil
}

// validateRawCircularImportsDetection validates circular imports detection structure
func validateRawCircularImportsDetection(circular interface{}, ruleIndex int) error {
	circularMap, ok := circular.(map[string]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].circularImportsDetection must be an object, got %T", ruleIndex, circular)
	}

	allowedFields := map[string]bool{
		"enabled":           true,
		"ignoreTypeImports": true,
	}

	for field := range circularMap {
		if !allowedFields[field] {
			return fmt.Errorf("rules[%d].circularImportsDetection: unknown field '%s'", ruleIndex, field)
		}
	}

	if _, exists := circularMap["enabled"]; !exists {
		return fmt.Errorf("rules[%d].circularImportsDetection.enabled is required", ruleIndex)
	}

	if enabled, ok := circularMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("rules[%d].circularImportsDetection.enabled cannot be null", ruleIndex)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("rules[%d].circularImportsDetection.enabled must be a boolean, got %T", ruleIndex, enabled)
	}

	if ignoreType, exists := circularMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("rules[%d].circularImportsDetection.ignoreTypeImports must be a boolean, got %T", ruleIndex, ignoreType)
		}
	}

	return nil
}

// validateRawOrphanFilesDetection validates orphan files detection structure
func validateRawOrphanFilesDetection(orphan interface{}, ruleIndex int) error {
	orphanMap, ok := orphan.(map[string]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].orphanFilesDetection must be an object, got %T", ruleIndex, orphan)
	}

	allowedFields := map[string]bool{
		"enabled":           true,
		"validEntryPoints":  true,
		"ignoreTypeImports": true,
		"graphExclude":      true,
	}

	for field := range orphanMap {
		if !allowedFields[field] {
			return fmt.Errorf("rules[%d].orphanFilesDetection: unknown field '%s'", ruleIndex, field)
		}
	}

	if _, exists := orphanMap["enabled"]; !exists {
		return fmt.Errorf("rules[%d].orphanFilesDetection.enabled is required", ruleIndex)
	}

	if enabled, ok := orphanMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("rules[%d].orphanFilesDetection.enabled cannot be null", ruleIndex)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("rules[%d].orphanFilesDetection.enabled must be a boolean, got %T", ruleIndex, enabled)
	}

	// Validate array fields
	if entryPoints, exists := orphanMap["validEntryPoints"]; exists && entryPoints != nil {
		if _, ok := entryPoints.([]interface{}); !ok {
			return fmt.Errorf("rules[%d].orphanFilesDetection.validEntryPoints must be an array, got %T", ruleIndex, entryPoints)
		}
	}

	if graphExclude, exists := orphanMap["graphExclude"]; exists && graphExclude != nil {
		if _, ok := graphExclude.([]interface{}); !ok {
			return fmt.Errorf("rules[%d].orphanFilesDetection.graphExclude must be an array, got %T", ruleIndex, graphExclude)
		}
	}

	if ignoreType, exists := orphanMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("rules[%d].orphanFilesDetection.ignoreTypeImports must be a boolean, got %T", ruleIndex, ignoreType)
		}
	}

	return nil
}

// validateRawUnusedNodeModulesDetection validates unused node modules detection structure
func validateRawUnusedNodeModulesDetection(unused interface{}, ruleIndex int) error {
	unusedMap, ok := unused.(map[string]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].unusedNodeModulesDetection must be an object, got %T", ruleIndex, unused)
	}

	allowedFields := map[string]bool{
		"enabled":                   true,
		"includeModules":            true,
		"excludeModules":            true,
		"pkgJsonFieldsWithBinaries": true,
		"filesWithBinaries":         true,
		"filesWithModules":          true,
		"outputType":                true,
	}

	for field := range unusedMap {
		if !allowedFields[field] {
			return fmt.Errorf("rules[%d].unusedNodeModulesDetection: unknown field '%s'", ruleIndex, field)
		}
	}

	if _, exists := unusedMap["enabled"]; !exists {
		return fmt.Errorf("rules[%d].unusedNodeModulesDetection.enabled is required", ruleIndex)
	}

	if enabled, ok := unusedMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("rules[%d].unusedNodeModulesDetection.enabled cannot be null", ruleIndex)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("rules[%d].unusedNodeModulesDetection.enabled must be a boolean, got %T", ruleIndex, enabled)
	}

	// Validate array fields
	arrayFields := []string{"includeModules", "excludeModules", "pkgJsonFieldsWithBinaries", "filesWithBinaries", "filesWithModules"}
	for _, field := range arrayFields {
		if value, exists := unusedMap[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("rules[%d].unusedNodeModulesDetection.%s must be an array, got %T", ruleIndex, field, value)
			}
		}
	}

	if outputType, exists := unusedMap["outputType"]; exists && outputType != nil {
		if _, ok := outputType.(string); !ok {
			return fmt.Errorf("rules[%d].unusedNodeModulesDetection.outputType must be a string, got %T", ruleIndex, outputType)
		}
	}

	return nil
}

// validateRawMissingNodeModulesDetection validates missing node modules detection structure
func validateRawMissingNodeModulesDetection(missing interface{}, ruleIndex int) error {
	missingMap, ok := missing.(map[string]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].missingNodeModulesDetection must be an object, got %T", ruleIndex, missing)
	}

	allowedFields := map[string]bool{
		"enabled":        true,
		"includeModules": true,
		"excludeModules": true,
		"outputType":     true,
	}

	for field := range missingMap {
		if !allowedFields[field] {
			return fmt.Errorf("rules[%d].missingNodeModulesDetection: unknown field '%s'", ruleIndex, field)
		}
	}

	if _, exists := missingMap["enabled"]; !exists {
		return fmt.Errorf("rules[%d].missingNodeModulesDetection.enabled is required", ruleIndex)
	}

	if enabled, ok := missingMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("rules[%d].missingNodeModulesDetection.enabled cannot be null", ruleIndex)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("rules[%d].missingNodeModulesDetection.enabled must be a boolean, got %T", ruleIndex, enabled)
	}

	// Validate array fields
	arrayFields := []string{"includeModules", "excludeModules"}
	for _, field := range arrayFields {
		if value, exists := missingMap[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("rules[%d].missingNodeModulesDetection.%s must be an array, got %T", ruleIndex, field, value)
			}
		}
	}

	if outputType, exists := missingMap["outputType"]; exists && outputType != nil {
		if _, ok := outputType.(string); !ok {
			return fmt.Errorf("rules[%d].missingNodeModulesDetection.outputType must be a string, got %T", ruleIndex, outputType)
		}
	}

	return nil
}

// ValidateConfig validates the RevDepConfig structure and required fields.
func ValidateConfig(config *RevDepConfig) error {
	if config.ConfigVersion == "" {
		return fmt.Errorf("configVersion is required")
	}

	for j, rule := range config.Rules {
		if rule.Path == "" {
			return fmt.Errorf("rules[%d].path is required", j)
		}

		// Validate module boundaries in rules
		for k, boundary := range rule.ModuleBoundaries {
			if err := validateBoundaryRule(&boundary, fmt.Sprintf("rules[%d].moduleBoundaries[%d]", j, k)); err != nil {
				return err
			}
		}

		// Validate circular imports detection options
		if rule.CircularImportsDetection != nil {
			if err := validateCircularImportsOptions(rule.CircularImportsDetection, fmt.Sprintf("rules[%d].circularImportsDetection", j)); err != nil {
				return err
			}
		}

		// Validate orphan files detection options
		if rule.OrphanFilesDetection != nil {
			if err := validateOrphanFilesOptions(rule.OrphanFilesDetection, fmt.Sprintf("rules[%d].orphanFilesDetection", j)); err != nil {
				return err
			}
		}

		// Validate unused node modules detection options
		if rule.UnusedNodeModulesDetection != nil {
			if err := validateUnusedNodeModulesOptions(rule.UnusedNodeModulesDetection, fmt.Sprintf("rules[%d].unusedNodeModulesDetection", j)); err != nil {
				return err
			}
		}

		// Validate missing node modules detection options
		if rule.MissingNodeModulesDetection != nil {
			if err := validateMissingNodeModulesOptions(rule.MissingNodeModulesDetection, fmt.Sprintf("rules[%d].missingNodeModulesDetection", j)); err != nil {
				return err
			}
		}

		// Validate import conventions
		if len(rule.ImportConventions) > 0 {
			// Additional validation can be added here if needed
			// The main validation is already done in validateRawImportConventions
		}
	}

	return nil
}

// validateBoundaryRule validates a single boundary rule
func validateBoundaryRule(boundary *BoundaryRule, prefix string) error {
	if err := validatePattern(boundary.Pattern); err != nil {
		return fmt.Errorf("%s.pattern: %w", prefix, err)
	}

	for l, p := range boundary.Allow {
		if err := validatePattern(p); err != nil {
			return fmt.Errorf("%s.allow[%d]: %w", prefix, l, err)
		}
	}

	for l, p := range boundary.Deny {
		if err := validatePattern(p); err != nil {
			return fmt.Errorf("%s.deny[%d]: %w", prefix, l, err)
		}
	}

	return nil
}

// validateCircularImportsOptions validates circular imports detection options
func validateCircularImportsOptions(opts *CircularImportsOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}
	// No additional validation needed for now
	return nil
}

// validateOrphanFilesOptions validates orphan files detection options
func validateOrphanFilesOptions(opts *OrphanFilesOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	// Validate valid entry points if provided
	for i, entryPoint := range opts.ValidEntryPoints {
		if entryPoint == "" {
			return fmt.Errorf("%s.validEntryPoints[%d]: cannot be empty", prefix, i)
		}
	}

	// Validate graph exclude patterns
	for i, pattern := range opts.GraphExclude {
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("%s.graphExclude[%d]: %w", prefix, i, err)
		}
	}

	return nil
}

// validateUnusedNodeModulesOptions validates unused node modules detection options
func validateUnusedNodeModulesOptions(opts *UnusedNodeModulesOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	// Validate output type
	if opts.OutputType != "" && opts.OutputType != "list" && opts.OutputType != "groupByModule" && opts.OutputType != "groupByFile" {
		return fmt.Errorf("%s.outputType: must be one of 'list', 'groupByModule', 'groupByFile', got '%s'", prefix, opts.OutputType)
	}

	return nil
}

// validateMissingNodeModulesOptions validates missing node modules detection options
func validateMissingNodeModulesOptions(opts *MissingNodeModulesOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	// Validate output type
	if opts.OutputType != "" && opts.OutputType != "list" && opts.OutputType != "groupByModule" && opts.OutputType != "groupByFile" {
		return fmt.Errorf("%s.outputType: must be one of 'list', 'groupByModule', 'groupByFile', got '%s'", prefix, opts.OutputType)
	}

	return nil
}

// validateRawImportConventions validates import conventions structure
func validateRawImportConventions(conventions interface{}, ruleIndex int) error {
	conventionsArray, ok := conventions.([]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].importConventions must be an array, got %T", ruleIndex, conventions)
	}

	if len(conventionsArray) == 0 {
		return fmt.Errorf("rules[%d].importConventions cannot be empty", ruleIndex)
	}

	for i, convention := range conventionsArray {
		conventionMap, ok := convention.(map[string]interface{})
		if !ok {
			return fmt.Errorf("rules[%d].importConventions[%d] must be an object, got %T", ruleIndex, i, convention)
		}

		allowedConventionFields := map[string]bool{
			"rule":    true,
			"domains": true,
			"autofix": true,
		}

		for field := range conventionMap {
			if !allowedConventionFields[field] {
				return fmt.Errorf("rules[%d].importConventions[%d]: unknown field '%s'", ruleIndex, i, field)
			}
		}

		// Check required fields
		if _, exists := conventionMap["rule"]; !exists {
			return fmt.Errorf("rules[%d].importConventions[%d].rule is required", ruleIndex, i)
		}
		if _, exists := conventionMap["domains"]; !exists {
			return fmt.Errorf("rules[%d].importConventions[%d].domains is required", ruleIndex, i)
		}

		// Validate rule field
		rule, ok := conventionMap["rule"].(string)
		if !ok {
			return fmt.Errorf("rules[%d].importConventions[%d].rule must be a string, got %T", ruleIndex, i, conventionMap["rule"])
		}

		if rule != "relative-internal-absolute-external" {
			return fmt.Errorf("rules[%d].importConventions[%d].rule: unknown rule '%s'. Only 'relative-internal-absolute-external' is supported", ruleIndex, i, rule)
		}

		// Validate autofix field (optional, defaults to false)
		if autofix, exists := conventionMap["autofix"]; exists && autofix != nil {
			if _, ok := autofix.(bool); !ok {
				return fmt.Errorf("rules[%d].importConventions[%d].autofix must be a boolean, got %T", ruleIndex, i, autofix)
			}
		}

		// Validate domains field
		if err := validateRelativeInternalAbsoluteExternalRule(conventionMap, ruleIndex, i); err != nil {
			return err
		}
	}

	return nil
}

// validateRelativeInternalAbsoluteExternalRule validates the specific rule
func validateRelativeInternalAbsoluteExternalRule(rule map[string]interface{}, ruleIndex int, convIndex int) error {
	domains := rule["domains"]

	// Check if domains is an array
	domainsArray, ok := domains.([]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].importConventions[%d].domains must be an array, got %T", ruleIndex, convIndex, domains)
	}

	if len(domainsArray) == 0 {
		return fmt.Errorf("rules[%d].importConventions[%d].domains cannot be empty", ruleIndex, convIndex)
	}

	// Check if all elements are strings or all are objects
	var hasStrings bool
	var hasObjects bool
	var parsedDomains []ImportConventionDomain

	for i, domain := range domainsArray {
		switch v := domain.(type) {
		case string:
			hasStrings = true
			if v == "" {
				return fmt.Errorf("rules[%d].importConventions[%d].domains[%d] cannot be empty string", ruleIndex, convIndex, i)
			}
			parsedDomains = append(parsedDomains, ImportConventionDomain{Path: v, Enabled: true})
		case map[string]interface{}:
			hasObjects = true
			domainMap := v

			// Check for required path field
			path, exists := domainMap["path"]
			if !exists {
				return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].path is required", ruleIndex, convIndex, i)
			}

			pathStr, ok := path.(string)
			if !ok {
				return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].path must be a string, got %T", ruleIndex, convIndex, i, path)
			}

			if pathStr == "" {
				return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].path cannot be empty", ruleIndex, convIndex, i)
			}

			// Check for optional alias field
			var alias string
			if aliasField, exists := domainMap["alias"]; exists {
				aliasStr, ok := aliasField.(string)
				if !ok {
					return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].alias must be a string, got %T", ruleIndex, convIndex, i, aliasField)
				}
				if aliasStr == "" {
					return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].alias cannot be empty", ruleIndex, convIndex, i)
				}
				alias = aliasStr
			}

			// Check for optional enabled field, default to true
			enabled := true
			if enabledField, exists := domainMap["enabled"]; exists {
				enabledBool, ok := enabledField.(bool)
				if !ok {
					return fmt.Errorf("rules[%d].importConventions[%d].domains[%d].enabled must be a boolean, got %T", ruleIndex, convIndex, i, enabledField)
				}
				enabled = enabledBool
			}

			parsedDomains = append(parsedDomains, ImportConventionDomain{Path: pathStr, Alias: alias, Enabled: enabled})
		default:
			return fmt.Errorf("rules[%d].importConventions[%d].domains[%d] must be a string or object, got %T", ruleIndex, convIndex, i, domain)
		}
	}

	// Mixed types are not allowed
	if hasStrings && hasObjects {
		return fmt.Errorf("rules[%d].importConventions[%d].domains cannot mix strings and objects", ruleIndex, convIndex)
	}

	// Validate no nested domains
	if err := validateNoNestedDomains(parsedDomains, ruleIndex, convIndex); err != nil {
		return err
	}

	return nil
}

// validateNoNestedDomains checks that no domain path is a prefix of another
func validateNoNestedDomains(domains []ImportConventionDomain, ruleIndex int, convIndex int) error {
	for i := 0; i < len(domains); i++ {
		for j := i + 1; j < len(domains); j++ {
			// Normalize paths for comparison
			path1 := filepath.Clean(domains[i].Path)
			path2 := filepath.Clean(domains[j].Path)

			// Check if one path is a prefix of the other
			if strings.HasPrefix(path1, path2) && (path1 == path2 || strings.HasPrefix(path1[len(path2):], string(filepath.Separator))) {
				return fmt.Errorf("rules[%d].importConventions[%d]: nested domains not allowed: '%s' and '%s'", ruleIndex, convIndex, domains[i].Path, domains[j].Path)
			}
			if strings.HasPrefix(path2, path1) && (path2 == path1 || strings.HasPrefix(path2[len(path1):], string(filepath.Separator))) {
				return fmt.Errorf("rules[%d].importConventions[%d]: nested domains not allowed: '%s' and '%s'", ruleIndex, convIndex, domains[i].Path, domains[j].Path)
			}
		}
	}
	return nil
}

// parseImportConventionDomains converts domains from interface{} to []ImportConventionDomain
func parseImportConventionDomains(domains interface{}) ([]ImportConventionDomain, error) {
	domainsArray, ok := domains.([]interface{})
	if !ok {
		return nil, fmt.Errorf("domains must be an array, got %T", domains)
	}

	var parsedDomains []ImportConventionDomain
	for i, domain := range domainsArray {
		switch v := domain.(type) {
		case string:
			if v == "" {
				return nil, fmt.Errorf("domains[%d] cannot be empty string", i)
			}
			parsedDomains = append(parsedDomains, ImportConventionDomain{Path: v, Enabled: true})
		case map[string]interface{}:
			domainMap := v

			// Check for required path field
			path, exists := domainMap["path"]
			if !exists {
				return nil, fmt.Errorf("domains[%d].path is required", i)
			}

			pathStr, ok := path.(string)
			if !ok {
				return nil, fmt.Errorf("domains[%d].path must be a string, got %T", i, path)
			}

			if pathStr == "" {
				return nil, fmt.Errorf("domains[%d].path cannot be empty", i)
			}

			if strings.Contains(pathStr, "*") {
				return nil, fmt.Errorf("domains[%d].path cannot contain wildcards", i)
			}

			// Check for optional alias field
			var alias string
			if aliasField, exists := domainMap["alias"]; exists {
				aliasStr, ok := aliasField.(string)
				if !ok {
					return nil, fmt.Errorf("domains[%d].alias must be a string, got %T", i, aliasField)
				}
				if aliasStr == "" {
					return nil, fmt.Errorf("domains[%d].alias cannot be empty", i)
				}
				alias = aliasStr
			}

			// Check for optional enabled field, default to true
			enabled := true
			if enabledField, exists := domainMap["enabled"]; exists {
				enabledBool, ok := enabledField.(bool)
				if !ok {
					return nil, fmt.Errorf("domains[%d].enabled must be a boolean, got %T", i, enabledField)
				}
				enabled = enabledBool
			}

			parsedDomains = append(parsedDomains, ImportConventionDomain{Path: pathStr, Alias: alias, Enabled: enabled})
		default:
			return nil, fmt.Errorf("domains[%d] must be a string or object, got %T", i, domain)
		}
	}

	return parsedDomains, nil
}

func validatePattern(pattern string) error {
	if len(pattern) >= 2 && pattern[0] == '.' && (pattern[1] == '/' || pattern[1] == '\\') {
		return fmt.Errorf("pattern '%s' starts with './' or '.\\', which is not allowed. Use paths that starts with file or directory name", pattern)
	}
	if len(pattern) >= 3 && pattern[0] == '.' && pattern[1] == '.' && (pattern[2] == '/' || pattern[2] == '\\') {
		return fmt.Errorf("pattern '%s' starts with '../' or '..\\', which is not allowed. Use paths that starts with file or directory name", pattern)
	}
	return nil
}
