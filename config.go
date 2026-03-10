package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gobwas/glob"
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

func (o *CircularImportsOptions) IsEnabled() bool { return o != nil && o.Enabled }

type OrphanFilesOptions struct {
	Enabled           bool     `json:"enabled"`
	ValidEntryPoints  []string `json:"validEntryPoints,omitempty"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports,omitempty"`
	GraphExclude      []string `json:"graphExclude,omitempty"`
	Autofix           bool     `json:"autofix,omitempty"`
}

func (o *OrphanFilesOptions) IsEnabled() bool { return o != nil && o.Enabled }

type UnusedNodeModulesOptions struct {
	Enabled                   bool     `json:"enabled"`
	IncludeModules            []string `json:"includeModules,omitempty"`
	ExcludeModules            []string `json:"excludeModules,omitempty"`
	PkgJsonFieldsWithBinaries []string `json:"pkgJsonFieldsWithBinaries,omitempty"`
	FilesWithBinaries         []string `json:"filesWithBinaries,omitempty"`
	FilesWithModules          []string `json:"filesWithModules,omitempty"`
	OutputType                string   `json:"outputType,omitempty"` // "list", "groupByModule", "groupByFile"
}

func (o *UnusedNodeModulesOptions) IsEnabled() bool { return o != nil && o.Enabled }

type MissingNodeModulesOptions struct {
	Enabled        bool     `json:"enabled"`
	IncludeModules []string `json:"includeModules,omitempty"`
	ExcludeModules []string `json:"excludeModules,omitempty"`
	OutputType     string   `json:"outputType,omitempty"` // "list", "groupByModule", "groupByFile", "groupByModuleFilesCount"
}

func (o *MissingNodeModulesOptions) IsEnabled() bool { return o != nil && o.Enabled }

type UnusedExportsOptions struct {
	Enabled           bool               `json:"enabled"`
	ValidEntryPoints  []string           `json:"validEntryPoints,omitempty"`
	IgnoreTypeExports bool               `json:"ignoreTypeExports,omitempty"`
	GraphExclude      []string           `json:"graphExclude,omitempty"`
	Ignore            FileValueIgnoreMap `json:"ignore,omitempty"`
	IgnoreFiles       []string           `json:"ignoreFiles,omitempty"`
	IgnoreExports     []string           `json:"ignoreExports,omitempty"`
	Autofix           bool               `json:"autofix,omitempty"`
}

func (o *UnusedExportsOptions) IsEnabled() bool { return o != nil && o.Enabled }

type UnresolvedImportsOptions struct {
	Enabled       bool               `json:"enabled"`
	Ignore        FileValueIgnoreMap `json:"ignore,omitempty"`
	IgnoreFiles   []string           `json:"ignoreFiles,omitempty"`
	IgnoreImports []string           `json:"ignoreImports,omitempty"`
}

func (o *UnresolvedImportsOptions) IsEnabled() bool { return o != nil && o.Enabled }

type RestrictedDevDependenciesUsageOptions struct {
	Enabled           bool     `json:"enabled"`
	ProdEntryPoints   []string `json:"prodEntryPoints,omitempty"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports,omitempty"`
}

func (o *RestrictedDevDependenciesUsageOptions) IsEnabled() bool { return o != nil && o.Enabled }

type RestrictedImportsDetectionOptions struct {
	Enabled           bool     `json:"enabled"`
	EntryPoints       []string `json:"entryPoints,omitempty"`
	GraphExclude      []string `json:"graphExclude,omitempty"`
	DenyFiles         []string `json:"denyFiles,omitempty"`
	DenyModules       []string `json:"denyModules,omitempty"`
	IgnoreMatches     []string `json:"ignoreMatches,omitempty"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports,omitempty"`
}

func (o *RestrictedImportsDetectionOptions) IsEnabled() bool { return o != nil && o.Enabled }

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

type FollowMonorepoPackagesValue struct {
	FollowAll bool
	Packages  map[string]bool
}

func (f FollowMonorepoPackagesValue) IsEnabled() bool {
	return f.FollowAll || len(f.Packages) > 0
}

func (f FollowMonorepoPackagesValue) ShouldFollowAll() bool {
	return f.FollowAll
}

func (f FollowMonorepoPackagesValue) ShouldFollowPackage(name string) bool {
	if f.FollowAll {
		return true
	}

	return f.Packages[name]
}

type Rule struct {
	Path                         string                                   `json:"path"` // Required
	ProdEntryPoints              []string                                 `json:"prodEntryPoints,omitempty"`
	DevEntryPoints               []string                                 `json:"devEntryPoints,omitempty"`
	FollowMonorepoPackages       FollowMonorepoPackagesValue              `json:"-"`
	ModuleBoundaries             []BoundaryRule                           `json:"moduleBoundaries,omitempty"`
	CircularImportsDetections    []*CircularImportsOptions                `json:"-"`
	OrphanFilesDetections        []*OrphanFilesOptions                    `json:"-"`
	UnusedNodeModulesDetections  []*UnusedNodeModulesOptions              `json:"-"`
	MissingNodeModulesDetections []*MissingNodeModulesOptions             `json:"-"`
	UnusedExportsDetections      []*UnusedExportsOptions                  `json:"-"`
	UnresolvedImportsDetections  []*UnresolvedImportsOptions              `json:"-"`
	DevDepsUsageOnProdDetections []*RestrictedDevDependenciesUsageOptions `json:"-"`
	RestrictedImportsDetections  []*RestrictedImportsDetectionOptions     `json:"-"`
	ImportConventions            []ImportConventionRule                   `json:"-"`
}

func (r *Rule) getCircularImportsDetections() []*CircularImportsOptions {
	return r.CircularImportsDetections
}

func (r *Rule) getOrphanFilesDetections() []*OrphanFilesOptions {
	return r.OrphanFilesDetections
}

func (r *Rule) getUnusedNodeModulesDetections() []*UnusedNodeModulesOptions {
	return r.UnusedNodeModulesDetections
}

func (r *Rule) getMissingNodeModulesDetections() []*MissingNodeModulesOptions {
	return r.MissingNodeModulesDetections
}

func (r *Rule) getUnusedExportsDetections() []*UnusedExportsOptions {
	return r.UnusedExportsDetections
}

func (r *Rule) getUnresolvedImportsDetections() []*UnresolvedImportsOptions {
	return r.UnresolvedImportsDetections
}

func (r *Rule) getDevDepsUsageOnProdDetections() []*RestrictedDevDependenciesUsageOptions {
	return r.DevDepsUsageOnProdDetections
}

func (r *Rule) getRestrictedImportsDetections() []*RestrictedImportsDetectionOptions {
	return r.RestrictedImportsDetections
}

func parseOneOrManyObjects[T any](raw json.RawMessage) ([]*T, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var single T
	if err := json.Unmarshal(raw, &single); err == nil {
		item := new(T)
		*item = single
		return []*T{item}, nil
	}

	var many []T
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, err
	}

	result := make([]*T, 0, len(many))
	for i := range many {
		item := new(T)
		*item = many[i]
		result = append(result, item)
	}

	return result, nil
}

func marshalOneOrManyObjects[T any](items []*T) interface{} {
	if len(items) == 0 {
		return nil
	}

	if len(items) == 1 {
		return items[0]
	}

	values := make([]T, 0, len(items))
	for _, item := range items {
		if item != nil {
			values = append(values, *item)
		}
	}
	return values
}

func (r Rule) MarshalJSON() ([]byte, error) {
	type ruleWire struct {
		Path                        string                 `json:"path"`
		ProdEntryPoints             []string               `json:"prodEntryPoints,omitempty"`
		DevEntryPoints              []string               `json:"devEntryPoints,omitempty"`
		ModuleBoundaries            []BoundaryRule         `json:"moduleBoundaries,omitempty"`
		CircularImportsDetection    interface{}            `json:"circularImportsDetection,omitempty"`
		OrphanFilesDetection        interface{}            `json:"orphanFilesDetection,omitempty"`
		UnusedNodeModulesDetection  interface{}            `json:"unusedNodeModulesDetection,omitempty"`
		MissingNodeModulesDetection interface{}            `json:"missingNodeModulesDetection,omitempty"`
		UnusedExportsDetection      interface{}            `json:"unusedExportsDetection,omitempty"`
		UnresolvedImportsDetection  interface{}            `json:"unresolvedImportsDetection,omitempty"`
		DevDepsUsageOnProdDetection interface{}            `json:"devDepsUsageOnProdDetection,omitempty"`
		RestrictedImportsDetection  interface{}            `json:"restrictedImportsDetection,omitempty"`
		ImportConventions           []ImportConventionRule `json:"importConventions,omitempty"`
	}

	wire := ruleWire{
		Path:                        r.Path,
		ProdEntryPoints:             r.ProdEntryPoints,
		DevEntryPoints:              r.DevEntryPoints,
		ModuleBoundaries:            r.ModuleBoundaries,
		CircularImportsDetection:    marshalOneOrManyObjects(r.getCircularImportsDetections()),
		OrphanFilesDetection:        marshalOneOrManyObjects(r.getOrphanFilesDetections()),
		UnusedNodeModulesDetection:  marshalOneOrManyObjects(r.getUnusedNodeModulesDetections()),
		MissingNodeModulesDetection: marshalOneOrManyObjects(r.getMissingNodeModulesDetections()),
		UnusedExportsDetection:      marshalOneOrManyObjects(r.getUnusedExportsDetections()),
		UnresolvedImportsDetection:  marshalOneOrManyObjects(r.getUnresolvedImportsDetections()),
		DevDepsUsageOnProdDetection: marshalOneOrManyObjects(r.getDevDepsUsageOnProdDetections()),
		RestrictedImportsDetection:  marshalOneOrManyObjects(r.getRestrictedImportsDetections()),
		ImportConventions:           r.ImportConventions,
	}

	return json.Marshal(wire)
}

func (r *Rule) UnmarshalJSON(data []byte) error {
	type ruleWire struct {
		Path                        string          `json:"path"`
		ProdEntryPoints             []string        `json:"prodEntryPoints,omitempty"`
		DevEntryPoints              []string        `json:"devEntryPoints,omitempty"`
		ModuleBoundaries            []BoundaryRule  `json:"moduleBoundaries,omitempty"`
		CircularImportsDetection    json.RawMessage `json:"circularImportsDetection,omitempty"`
		OrphanFilesDetection        json.RawMessage `json:"orphanFilesDetection,omitempty"`
		UnusedNodeModulesDetection  json.RawMessage `json:"unusedNodeModulesDetection,omitempty"`
		MissingNodeModulesDetection json.RawMessage `json:"missingNodeModulesDetection,omitempty"`
		UnusedExportsDetection      json.RawMessage `json:"unusedExportsDetection,omitempty"`
		UnresolvedImportsDetection  json.RawMessage `json:"unresolvedImportsDetection,omitempty"`
		DevDepsUsageOnProdDetection json.RawMessage `json:"devDepsUsageOnProdDetection,omitempty"`
		RestrictedImportsDetection  json.RawMessage `json:"restrictedImportsDetection,omitempty"`
	}

	var wire ruleWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	circular, err := parseOneOrManyObjects[CircularImportsOptions](wire.CircularImportsDetection)
	if err != nil {
		return err
	}
	orphan, err := parseOneOrManyObjects[OrphanFilesOptions](wire.OrphanFilesDetection)
	if err != nil {
		return err
	}
	unusedNodeModules, err := parseOneOrManyObjects[UnusedNodeModulesOptions](wire.UnusedNodeModulesDetection)
	if err != nil {
		return err
	}
	missingNodeModules, err := parseOneOrManyObjects[MissingNodeModulesOptions](wire.MissingNodeModulesDetection)
	if err != nil {
		return err
	}
	unusedExports, err := parseOneOrManyObjects[UnusedExportsOptions](wire.UnusedExportsDetection)
	if err != nil {
		return err
	}
	unresolvedImports, err := parseOneOrManyObjects[UnresolvedImportsOptions](wire.UnresolvedImportsDetection)
	if err != nil {
		return err
	}
	devDeps, err := parseOneOrManyObjects[RestrictedDevDependenciesUsageOptions](wire.DevDepsUsageOnProdDetection)
	if err != nil {
		return err
	}
	restrictedImports, err := parseOneOrManyObjects[RestrictedImportsDetectionOptions](wire.RestrictedImportsDetection)
	if err != nil {
		return err
	}

	r.Path = wire.Path
	r.ProdEntryPoints = wire.ProdEntryPoints
	r.DevEntryPoints = wire.DevEntryPoints
	r.ModuleBoundaries = wire.ModuleBoundaries

	r.CircularImportsDetections = circular
	r.OrphanFilesDetections = orphan
	r.UnusedNodeModulesDetections = unusedNodeModules
	r.MissingNodeModulesDetections = missingNodeModules
	r.UnusedExportsDetections = unusedExports
	r.UnresolvedImportsDetections = unresolvedImports
	r.DevDepsUsageOnProdDetections = devDeps
	r.RestrictedImportsDetections = restrictedImports

	return nil
}

type RevDepConfig struct {
	ConfigVersion         string   `json:"configVersion"` // Required
	Schema                string   `json:"$schema,omitempty"`
	ConditionNames        []string `json:"conditionNames,omitempty"`
	CustomAssetExtensions []string `json:"customAssetExtensions,omitempty"`
	IgnoreFiles           []string `json:"ignoreFiles,omitempty"`
	Rules                 []Rule   `json:"rules"`
}

var configFileName = "rev-dep.config.json"
var hiddenConfigFileName = ".rev-dep.config.json"
var configFileNameJsonc = "rev-dep.config.jsonc"
var hiddenConfigFileNameJsonc = ".rev-dep.config.jsonc"

// supportedConfigVersions lists config versions supported by this CLI release.
// Update this slice when adding or removing support for config versions.
var supportedConfigVersions = []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5", "1.6"}

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
func LoadConfig(configPath string) (RevDepConfig, error) {
	content, err := readConfigFile(configPath)
	if err != nil {
		return RevDepConfig{}, err
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
func ParseConfig(content []byte) (RevDepConfig, error) {
	// First, parse into a generic map to validate field names and types
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(jsonc.ToJSON(content), &rawConfig); err != nil {
		return RevDepConfig{}, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate field names and structure
	if err := validateRawConfig(rawConfig); err != nil {
		return RevDepConfig{}, err
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
		return RevDepConfig{}, fmt.Errorf("failed to parse config for normalization: %w", err)
	}

	// Parse into final typed struct
	var config RevDepConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &config); err != nil {
		return RevDepConfig{}, fmt.Errorf("failed to parse config into final structure: %w", err)
	}

	// Validate config
	if err := ValidateConfig(&config); err != nil {
		return RevDepConfig{}, err
	}

	// Validate config version against supported versions for this CLI
	if err := validateConfigVersion(config.ConfigVersion); err != nil {
		return RevDepConfig{}, err
	}

	// Set default values for optional fields and followMonorepoPackages and process import conventions
	for i := range config.Rules {
		if rawRulesArray, ok := rawConfig["rules"].([]interface{}); ok && i < len(rawRulesArray) {
			ruleMap, isRuleMap := rawRulesArray[i].(map[string]interface{})
			if !isRuleMap {
				continue
			}

			rawFollow, exists := ruleMap["followMonorepoPackages"]
			if !exists {
				config.Rules[i].FollowMonorepoPackages = FollowMonorepoPackagesValue{FollowAll: true}
			} else {
				parsedFollow, err := parseFollowMonorepoPackagesValue(rawFollow)
				if err != nil {
					return RevDepConfig{}, err
				}
				config.Rules[i].FollowMonorepoPackages = parsedFollow
			}

			// Apply rule-level entry point inheritance for selected detectors.
			// Explicit detector arrays (including empty) override rule-level defaults.
			for _, orphanCfg := range config.Rules[i].getOrphanFilesDetections() {
				if orphanCfg.ValidEntryPoints == nil {
					orphanCfg.ValidEntryPoints = mergeAndDedupeEntryPoints(config.Rules[i].ProdEntryPoints, config.Rules[i].DevEntryPoints)
				}
			}

			for _, unusedExportsCfg := range config.Rules[i].getUnusedExportsDetections() {
				if unusedExportsCfg.ValidEntryPoints == nil {
					unusedExportsCfg.ValidEntryPoints = mergeAndDedupeEntryPoints(config.Rules[i].ProdEntryPoints, config.Rules[i].DevEntryPoints)
				}
			}

			for _, devDepsCfg := range config.Rules[i].getDevDepsUsageOnProdDetections() {
				if devDepsCfg.ProdEntryPoints == nil {
					devDepsCfg.ProdEntryPoints = cloneStringSlice(config.Rules[i].ProdEntryPoints)
				}
			}
		}

		// Process and normalize import conventions
		if i < len(rawRules.Rules) {
			config.Rules[i].ImportConventions = make([]ImportConventionRule, len(rawRules.Rules[i].ImportConventions))
			for j, rawConv := range rawRules.Rules[i].ImportConventions {
				parsedDomains, err := parseImportConventionDomains(rawConv.Domains)
				if err != nil {
					return RevDepConfig{}, fmt.Errorf("failed to parse import convention domains for rules[%d].importConventions[%d]: %w", i, j, err)
				}
				config.Rules[i].ImportConventions[j] = ImportConventionRule{
					Rule:    rawConv.Rule,
					Domains: parsedDomains,
					Autofix: rawConv.Autofix,
				}
			}
		}
	}

	return config, nil
}

func cloneStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	return append([]string(nil), input...)
}

func mergeAndDedupeEntryPoints(prodEntryPoints []string, devEntryPoints []string) []string {
	if len(prodEntryPoints) == 0 && len(devEntryPoints) == 0 {
		return nil
	}

	merged := make([]string, 0, len(prodEntryPoints)+len(devEntryPoints))
	seen := make(map[string]bool, len(prodEntryPoints)+len(devEntryPoints))

	for _, entryPoint := range prodEntryPoints {
		if !seen[entryPoint] {
			seen[entryPoint] = true
			merged = append(merged, entryPoint)
		}
	}

	for _, entryPoint := range devEntryPoints {
		if !seen[entryPoint] {
			seen[entryPoint] = true
			merged = append(merged, entryPoint)
		}
	}

	return merged
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
		"$schema":               true,
		"configVersion":         true,
		"conditionNames":        true,
		"customAssetExtensions": true,
		"ignoreFiles":           true,
		"rules":                 true,
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

	if customAssetExtensions, exists := raw["customAssetExtensions"]; exists && customAssetExtensions != nil {
		extensionsArray, ok := customAssetExtensions.([]interface{})
		if !ok {
			return fmt.Errorf("customAssetExtensions must be an array, got %T", customAssetExtensions)
		}
		for i, extension := range extensionsArray {
			if _, ok := extension.(string); !ok {
				return fmt.Errorf("customAssetExtensions[%d] must be a string, got %T", i, extension)
			}
		}
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
		"prodEntryPoints":             true,
		"devEntryPoints":              true,
		"followMonorepoPackages":      true,
		"moduleBoundaries":            true,
		"circularImportsDetection":    true,
		"orphanFilesDetection":        true,
		"unusedNodeModulesDetection":  true,
		"missingNodeModulesDetection": true,
		"unusedExportsDetection":      true,
		"unresolvedImportsDetection":  true,
		"devDepsUsageOnProdDetection": true,
		"restrictedImportsDetection":  true,
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

	if followMonorepoPackages, exists := rule["followMonorepoPackages"]; exists {
		if err := validateRawFollowMonorepoPackages(followMonorepoPackages, index); err != nil {
			return err
		}
	}

	for _, field := range []string{"prodEntryPoints", "devEntryPoints"} {
		if value, exists := rule[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("rules[%d].%s must be an array, got %T", index, field, value)
			}
		}
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

	if unusedExports, exists := rule["unusedExportsDetection"]; exists {
		if err := validateRawUnusedExportsDetection(unusedExports, index); err != nil {
			return err
		}
	}

	if unresolved, exists := rule["unresolvedImportsDetection"]; exists {
		if err := validateRawUnresolvedImportsDetection(unresolved, index); err != nil {
			return err
		}
	}

	if conventions, exists := rule["importConventions"]; exists {
		if err := validateRawImportConventions(conventions, index); err != nil {
			return err
		}
	}

	if restrictedDevDeps, exists := rule["devDepsUsageOnProdDetection"]; exists {
		if err := validateRawRestrictedDevDependenciesUsageDetection(restrictedDevDeps, index); err != nil {
			return err
		}
	}

	if restrictedImports, exists := rule["restrictedImportsDetection"]; exists {
		if err := validateRawRestrictedImportsDetection(restrictedImports, index); err != nil {
			return err
		}
	}

	return nil
}

func validateRawDetectionObjectOrArray(
	raw interface{},
	ruleIndex int,
	fieldName string,
	validateInstance func(map[string]interface{}, string) error,
) error {
	if detectionMap, ok := raw.(map[string]interface{}); ok {
		return validateInstance(detectionMap, fmt.Sprintf("rules[%d].%s", ruleIndex, fieldName))
	}

	detectionArray, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("rules[%d].%s must be an object or array of objects, got %T", ruleIndex, fieldName, raw)
	}

	for i, item := range detectionArray {
		detectionMap, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("rules[%d].%s[%d] must be an object, got %T", ruleIndex, fieldName, i, item)
		}

		if err := validateInstance(detectionMap, fmt.Sprintf("rules[%d].%s[%d]", ruleIndex, fieldName, i)); err != nil {
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
	return validateRawDetectionObjectOrArray(circular, ruleIndex, "circularImportsDetection", validateRawCircularImportsDetectionInstance)
}

func validateRawCircularImportsDetectionInstance(circularMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":           true,
		"ignoreTypeImports": true,
	}

	for field := range circularMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := circularMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := circularMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	if ignoreType, exists := circularMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("%s.ignoreTypeImports must be a boolean, got %T", prefix, ignoreType)
		}
	}

	return nil
}

// validateRawOrphanFilesDetection validates orphan files detection structure
func validateRawOrphanFilesDetection(orphan interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(orphan, ruleIndex, "orphanFilesDetection", validateRawOrphanFilesDetectionInstance)
}

func validateRawOrphanFilesDetectionInstance(orphanMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":           true,
		"validEntryPoints":  true,
		"ignoreTypeImports": true,
		"graphExclude":      true,
		"autofix":           true,
	}

	for field := range orphanMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := orphanMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := orphanMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	// Validate array fields
	if entryPoints, exists := orphanMap["validEntryPoints"]; exists && entryPoints != nil {
		if _, ok := entryPoints.([]interface{}); !ok {
			return fmt.Errorf("%s.validEntryPoints must be an array, got %T", prefix, entryPoints)
		}
	}

	if graphExclude, exists := orphanMap["graphExclude"]; exists && graphExclude != nil {
		if _, ok := graphExclude.([]interface{}); !ok {
			return fmt.Errorf("%s.graphExclude must be an array, got %T", prefix, graphExclude)
		}
	}

	if ignoreType, exists := orphanMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("%s.ignoreTypeImports must be a boolean, got %T", prefix, ignoreType)
		}
	}

	if autofix, exists := orphanMap["autofix"]; exists && autofix != nil {
		if _, ok := autofix.(bool); !ok {
			return fmt.Errorf("%s.autofix must be a boolean, got %T", prefix, autofix)
		}
	}

	return nil
}

// validateRawUnusedNodeModulesDetection validates unused node modules detection structure
func validateRawUnusedNodeModulesDetection(unused interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(unused, ruleIndex, "unusedNodeModulesDetection", validateRawUnusedNodeModulesDetectionInstance)
}

func validateRawUnusedNodeModulesDetectionInstance(unusedMap map[string]interface{}, prefix string) error {
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
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := unusedMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := unusedMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	// Validate array fields
	arrayFields := []string{"includeModules", "excludeModules", "pkgJsonFieldsWithBinaries", "filesWithBinaries", "filesWithModules"}
	for _, field := range arrayFields {
		if value, exists := unusedMap[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("%s.%s must be an array, got %T", prefix, field, value)
			}
		}
	}

	if outputType, exists := unusedMap["outputType"]; exists && outputType != nil {
		if _, ok := outputType.(string); !ok {
			return fmt.Errorf("%s.outputType must be a string, got %T", prefix, outputType)
		}
	}

	return nil
}

// validateRawMissingNodeModulesDetection validates missing node modules detection structure
func validateRawMissingNodeModulesDetection(missing interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(missing, ruleIndex, "missingNodeModulesDetection", validateRawMissingNodeModulesDetectionInstance)
}

func validateRawMissingNodeModulesDetectionInstance(missingMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":        true,
		"includeModules": true,
		"excludeModules": true,
		"outputType":     true,
	}

	for field := range missingMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := missingMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := missingMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	// Validate array fields
	arrayFields := []string{"includeModules", "excludeModules"}
	for _, field := range arrayFields {
		if value, exists := missingMap[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("%s.%s must be an array, got %T", prefix, field, value)
			}
		}
	}

	if outputType, exists := missingMap["outputType"]; exists && outputType != nil {
		if _, ok := outputType.(string); !ok {
			return fmt.Errorf("%s.outputType must be a string, got %T", prefix, outputType)
		}
	}

	return nil
}

// validateRawUnusedExportsDetection validates unused exports detection structure
func validateRawUnusedExportsDetection(unusedExports interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(unusedExports, ruleIndex, "unusedExportsDetection", validateRawUnusedExportsDetectionInstance)
}

func validateRawUnusedExportsDetectionInstance(unusedExportsMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":           true,
		"validEntryPoints":  true,
		"ignoreTypeExports": true,
		"graphExclude":      true,
		"ignore":            true,
		"ignoreFiles":       true,
		"ignoreExports":     true,
		"autofix":           true,
	}

	for field := range unusedExportsMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := unusedExportsMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := unusedExportsMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	// Validate array fields
	if entryPoints, exists := unusedExportsMap["validEntryPoints"]; exists && entryPoints != nil {
		if _, ok := entryPoints.([]interface{}); !ok {
			return fmt.Errorf("%s.validEntryPoints must be an array, got %T", prefix, entryPoints)
		}
	}

	if graphExclude, exists := unusedExportsMap["graphExclude"]; exists && graphExclude != nil {
		if _, ok := graphExclude.([]interface{}); !ok {
			return fmt.Errorf("%s.graphExclude must be an array, got %T", prefix, graphExclude)
		}
	}

	if ignore, exists := unusedExportsMap["ignore"]; exists && ignore != nil {
		ignoreMap, ok := ignore.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s.ignore must be an object, got %T", prefix, ignore)
		}
		for filePath, exportName := range ignoreMap {
			if strings.TrimSpace(filePath) == "" {
				return fmt.Errorf("%s.ignore contains empty file path", prefix)
			}
			switch v := exportName.(type) {
			case string:
			case []interface{}:
				for idx, item := range v {
					if _, ok := item.(string); !ok {
						return fmt.Errorf("%s.ignore['%s'][%d] must be a string, got %T", prefix, filePath, idx, item)
					}
				}
			default:
				return fmt.Errorf("%s.ignore['%s'] must be a string or array of strings, got %T", prefix, filePath, exportName)
			}
		}
	}

	if ignoreFiles, exists := unusedExportsMap["ignoreFiles"]; exists && ignoreFiles != nil {
		ignoreFilesArr, ok := ignoreFiles.([]interface{})
		if !ok {
			return fmt.Errorf("%s.ignoreFiles must be an array, got %T", prefix, ignoreFiles)
		}
		for i, v := range ignoreFilesArr {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s.ignoreFiles[%d] must be a string, got %T", prefix, i, v)
			}
		}
	}

	if ignoreExports, exists := unusedExportsMap["ignoreExports"]; exists && ignoreExports != nil {
		ignoreExportsArr, ok := ignoreExports.([]interface{})
		if !ok {
			return fmt.Errorf("%s.ignoreExports must be an array, got %T", prefix, ignoreExports)
		}
		for i, v := range ignoreExportsArr {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s.ignoreExports[%d] must be a string, got %T", prefix, i, v)
			}
		}
	}

	if ignoreType, exists := unusedExportsMap["ignoreTypeExports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("%s.ignoreTypeExports must be a boolean, got %T", prefix, ignoreType)
		}
	}

	if autofix, exists := unusedExportsMap["autofix"]; exists && autofix != nil {
		if _, ok := autofix.(bool); !ok {
			return fmt.Errorf("%s.autofix must be a boolean, got %T", prefix, autofix)
		}
	}

	return nil
}

// ValidateConfig validates the RevDepConfig structure and required fields.
func ValidateConfig(config *RevDepConfig) error {
	if config.ConfigVersion == "" {
		return fmt.Errorf("configVersion is required")
	}

	if err := validateCustomAssetExtensions(config.CustomAssetExtensions, "customAssetExtensions"); err != nil {
		return err
	}

	for j, rule := range config.Rules {
		if rule.Path == "" {
			return fmt.Errorf("rules[%d].path is required", j)
		}
		if err := validateRuleEntryPoints(&rule, fmt.Sprintf("rules[%d]", j)); err != nil {
			return err
		}
		packageIdx := 0
		for pattern := range rule.FollowMonorepoPackages.Packages {
			trimmedPattern := strings.TrimSpace(pattern)
			if trimmedPattern == "" {
				return fmt.Errorf("rules[%d].followMonorepoPackages[%d] cannot be empty", j, packageIdx)
			}
			packageIdx++
		}

		// Validate module boundaries in rules
		for k, boundary := range rule.ModuleBoundaries {
			if err := validateBoundaryRule(&boundary, fmt.Sprintf("rules[%d].moduleBoundaries[%d]", j, k)); err != nil {
				return err
			}
		}

		// Validate circular imports detection options
		for idx, detection := range rule.getCircularImportsDetections() {
			prefix := fmt.Sprintf("rules[%d].circularImportsDetection", j)
			if len(rule.getCircularImportsDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateCircularImportsOptions(detection); err != nil {
				return err
			}
		}

		// Validate orphan files detection options
		for idx, detection := range rule.getOrphanFilesDetections() {
			prefix := fmt.Sprintf("rules[%d].orphanFilesDetection", j)
			if len(rule.getOrphanFilesDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateOrphanFilesOptions(detection, prefix); err != nil {
				return err
			}
		}

		// Validate unused node modules detection options
		for idx, detection := range rule.getUnusedNodeModulesDetections() {
			prefix := fmt.Sprintf("rules[%d].unusedNodeModulesDetection", j)
			if len(rule.getUnusedNodeModulesDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateUnusedNodeModulesOptions(detection, prefix); err != nil {
				return err
			}
		}

		// Validate missing node modules detection options
		for idx, detection := range rule.getMissingNodeModulesDetections() {
			prefix := fmt.Sprintf("rules[%d].missingNodeModulesDetection", j)
			if len(rule.getMissingNodeModulesDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateMissingNodeModulesOptions(detection, prefix); err != nil {
				return err
			}
		}

		// Validate unused exports detection options
		for idx, detection := range rule.getUnusedExportsDetections() {
			prefix := fmt.Sprintf("rules[%d].unusedExportsDetection", j)
			if len(rule.getUnusedExportsDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateUnusedExportsOptions(detection, prefix); err != nil {
				return err
			}
		}

		// Validate unresolved imports detection options
		for idx, detection := range rule.getUnresolvedImportsDetections() {
			prefix := fmt.Sprintf("rules[%d].unresolvedImportsDetection", j)
			if len(rule.getUnresolvedImportsDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateUnresolvedImportsOptions(detection, prefix); err != nil {
				return err
			}
		}

		// Validate restricted dev dependencies usage detection options
		for idx, detection := range rule.getDevDepsUsageOnProdDetections() {
			prefix := fmt.Sprintf("rules[%d].devDepsUsageOnProdDetection", j)
			if len(rule.getDevDepsUsageOnProdDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateRestrictedDevDependenciesUsageOptions(detection, prefix); err != nil {
				return err
			}
		}

		for idx, detection := range rule.getRestrictedImportsDetections() {
			prefix := fmt.Sprintf("rules[%d].restrictedImportsDetection", j)
			if len(rule.getRestrictedImportsDetections()) > 1 {
				prefix = fmt.Sprintf("%s[%d]", prefix, idx)
			}
			if err := validateRestrictedImportsDetectionOptions(detection, prefix); err != nil {
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
func validateCircularImportsOptions(opts *CircularImportsOptions) error {
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
	if opts.OutputType != "" {
		switch opts.OutputType {
		case "list", "groupByModule", "groupByFile", "groupByModuleFilesCount":
			// allowed
		default:
			return fmt.Errorf("%s.outputType: must be one of 'list', 'groupByModule', 'groupByFile', 'groupByModuleFilesCount', got '%s'", prefix, opts.OutputType)
		}
	}

	return nil
}

// validateUnusedExportsOptions validates unused exports detection options
func validateUnusedExportsOptions(opts *UnusedExportsOptions, prefix string) error {
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

	normalizedIgnore, normalizedIgnoreExports, err := validateAndNormalizeIgnoreConfig(opts.Ignore, opts.IgnoreFiles, opts.IgnoreExports, prefix, "ignoreExports")
	if err != nil {
		return err
	}
	opts.Ignore = normalizedIgnore
	opts.IgnoreExports = normalizedIgnoreExports

	return nil
}

// validateRawUnresolvedImportsDetection validates raw unresolved imports detection option
func validateRawUnresolvedImportsDetection(unresolved interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(unresolved, ruleIndex, "unresolvedImportsDetection", validateRawUnresolvedImportsDetectionInstance)
}

func validateRawUnresolvedImportsDetectionInstance(unresolvedMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":       true,
		"ignore":        true,
		"ignoreFiles":   true,
		"ignoreImports": true,
	}

	for field := range unresolvedMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := unresolvedMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := unresolvedMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	if ignore, exists := unresolvedMap["ignore"]; exists && ignore != nil {
		ignoreMap, ok := ignore.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s.ignore must be an object, got %T", prefix, ignore)
		}

		for filePath, request := range ignoreMap {
			if strings.TrimSpace(filePath) == "" {
				return fmt.Errorf("%s.ignore contains empty file path", prefix)
			}
			switch v := request.(type) {
			case string:
			case []interface{}:
				for idx, item := range v {
					if _, ok := item.(string); !ok {
						return fmt.Errorf("%s.ignore['%s'][%d] must be a string, got %T", prefix, filePath, idx, item)
					}
				}
			default:
				return fmt.Errorf("%s.ignore['%s'] must be a string or array of strings, got %T", prefix, filePath, request)
			}
		}
	}

	if ignoreFiles, exists := unresolvedMap["ignoreFiles"]; exists && ignoreFiles != nil {
		ignoreFilesArr, ok := ignoreFiles.([]interface{})
		if !ok {
			return fmt.Errorf("%s.ignoreFiles must be an array, got %T", prefix, ignoreFiles)
		}
		for i, v := range ignoreFilesArr {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s.ignoreFiles[%d] must be a string, got %T", prefix, i, v)
			}
		}
	}

	if ignoreImports, exists := unresolvedMap["ignoreImports"]; exists && ignoreImports != nil {
		ignoreImportsArr, ok := ignoreImports.([]interface{})
		if !ok {
			return fmt.Errorf("%s.ignoreImports must be an array, got %T", prefix, ignoreImports)
		}
		for i, v := range ignoreImportsArr {
			if _, ok := v.(string); !ok {
				return fmt.Errorf("%s.ignoreImports[%d] must be a string, got %T", prefix, i, v)
			}
		}
	}

	return nil
}

// validateUnresolvedImportsOptions validates resolved options structure
func validateUnresolvedImportsOptions(opts *UnresolvedImportsOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	normalizedIgnore, normalizedIgnoreImports, err := validateAndNormalizeIgnoreConfig(opts.Ignore, opts.IgnoreFiles, opts.IgnoreImports, prefix, "ignoreImports")
	if err != nil {
		return err
	}
	opts.Ignore = normalizedIgnore
	opts.IgnoreImports = normalizedIgnoreImports

	return nil
}

// validateRawRestrictedDevDependenciesUsageDetection validates restricted dev dependencies usage detection structure
func validateRawRestrictedDevDependenciesUsageDetection(restrictedDevDeps interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(restrictedDevDeps, ruleIndex, "devDepsUsageOnProdDetection", validateRawRestrictedDevDependenciesUsageDetectionInstance)
}

func validateRawRestrictedDevDependenciesUsageDetectionInstance(restrictedDevDepsMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":           true,
		"prodEntryPoints":   true,
		"ignoreTypeImports": true,
	}

	for field := range restrictedDevDepsMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := restrictedDevDepsMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := restrictedDevDepsMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	if entryPoints, exists := restrictedDevDepsMap["prodEntryPoints"]; exists && entryPoints != nil {
		if _, ok := entryPoints.([]interface{}); !ok {
			return fmt.Errorf("%s.prodEntryPoints must be an array, got %T", prefix, entryPoints)
		}
	}

	if ignoreType, exists := restrictedDevDepsMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("%s.ignoreTypeImports must be a boolean, got %T", prefix, ignoreType)
		}
	}

	return nil
}

func validateAndNormalizeIgnoreConfig(ignore FileValueIgnoreMap, ignoreFiles []string, ignoreValues []string, prefix string, ignoreValuesFieldName string) (FileValueIgnoreMap, []string, error) {
	for i, pattern := range ignoreFiles {
		if err := validatePattern(pattern); err != nil {
			return nil, nil, fmt.Errorf("%s.ignoreFiles[%d]: %w", prefix, i, err)
		}
		if _, err := glob.Compile(NormalizeGlobPattern(pattern)); err != nil {
			return nil, nil, fmt.Errorf("%s.ignoreFiles[%d] has invalid glob pattern '%s': %v", prefix, i, pattern, err)
		}
	}

	normalizedIgnore := make(FileValueIgnoreMap, len(ignore))
	for configuredPath, valuePatterns := range ignore {
		normalizedPath := normalizeIgnoreFilePath(configuredPath)
		if normalizedPath == "" {
			return nil, nil, fmt.Errorf("%s.ignore contains empty file path", prefix)
		}

		if filepath.IsAbs(DenormalizePathForOS(normalizedPath)) {
			return nil, nil, fmt.Errorf("%s.ignore['%s'] must be a relative path", prefix, configuredPath)
		}

		if normalizedPath == ".." || strings.HasPrefix(normalizedPath, "../") {
			return nil, nil, fmt.Errorf("%s.ignore['%s'] must not traverse parent directories", prefix, configuredPath)
		}

		if _, err := glob.Compile(NormalizeGlobPattern(normalizedPath)); err != nil {
			return nil, nil, fmt.Errorf("%s.ignore['%s'] has invalid file glob pattern: %v", prefix, configuredPath, err)
		}

		if len(valuePatterns) == 0 {
			return nil, nil, fmt.Errorf("%s.ignore['%s'] cannot be empty", prefix, configuredPath)
		}

		normalizedValuePatterns := make([]string, 0, len(valuePatterns))
		for valueIdx, valuePattern := range valuePatterns {
			trimmedValuePattern := strings.TrimSpace(valuePattern)
			if trimmedValuePattern == "" {
				return nil, nil, fmt.Errorf("%s.ignore['%s'][%d] cannot be empty", prefix, configuredPath, valueIdx)
			}
			if _, err := glob.Compile(trimmedValuePattern); err != nil {
				return nil, nil, fmt.Errorf("%s.ignore['%s'][%d] has invalid value glob pattern '%s': %v", prefix, configuredPath, valueIdx, trimmedValuePattern, err)
			}
			normalizedValuePatterns = append(normalizedValuePatterns, trimmedValuePattern)
		}

		normalizedIgnore[normalizedPath] = normalizedValuePatterns
	}

	normalizedIgnoreValues := make([]string, 0, len(ignoreValues))
	for i, valuePattern := range ignoreValues {
		trimmedValuePattern := strings.TrimSpace(valuePattern)
		if trimmedValuePattern == "" {
			return nil, nil, fmt.Errorf("%s.%s[%d] cannot be empty", prefix, ignoreValuesFieldName, i)
		}
		if _, err := glob.Compile(trimmedValuePattern); err != nil {
			return nil, nil, fmt.Errorf("%s.%s[%d] has invalid glob pattern '%s': %v", prefix, ignoreValuesFieldName, i, trimmedValuePattern, err)
		}
		normalizedIgnoreValues = append(normalizedIgnoreValues, trimmedValuePattern)
	}

	return normalizedIgnore, normalizedIgnoreValues, nil
}

// validateRestrictedDevDependenciesUsageOptions validates restricted dev dependencies usage options structure
func validateRestrictedDevDependenciesUsageOptions(opts *RestrictedDevDependenciesUsageOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	// Validate production entry points if provided
	for i, entryPoint := range opts.ProdEntryPoints {
		if entryPoint == "" {
			return fmt.Errorf("%s.prodEntryPoints[%d]: cannot be empty", prefix, i)
		}
	}

	return nil
}

func validateRawRestrictedImportsDetection(restrictedImports interface{}, ruleIndex int) error {
	return validateRawDetectionObjectOrArray(restrictedImports, ruleIndex, "restrictedImportsDetection", validateRawRestrictedImportsDetectionInstance)
}

func validateRawRestrictedImportsDetectionInstance(restrictedImportsMap map[string]interface{}, prefix string) error {
	allowedFields := map[string]bool{
		"enabled":           true,
		"entryPoints":       true,
		"graphExclude":      true,
		"denyFiles":         true,
		"denyModules":       true,
		"ignoreMatches":     true,
		"ignoreTypeImports": true,
	}

	for field := range restrictedImportsMap {
		if !allowedFields[field] {
			return fmt.Errorf("%s: unknown field '%s'", prefix, field)
		}
	}

	if _, exists := restrictedImportsMap["enabled"]; !exists {
		return fmt.Errorf("%s.enabled is required", prefix)
	}

	if enabled, ok := restrictedImportsMap["enabled"]; !ok || enabled == nil {
		return fmt.Errorf("%s.enabled cannot be null", prefix)
	} else if _, ok := enabled.(bool); !ok {
		return fmt.Errorf("%s.enabled must be a boolean, got %T", prefix, enabled)
	}

	arrayFields := []string{"entryPoints", "graphExclude", "denyFiles", "denyModules", "ignoreMatches"}
	for _, field := range arrayFields {
		if value, exists := restrictedImportsMap[field]; exists && value != nil {
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("%s.%s must be an array, got %T", prefix, field, value)
			}
		}
	}

	if ignoreType, exists := restrictedImportsMap["ignoreTypeImports"]; exists && ignoreType != nil {
		if _, ok := ignoreType.(bool); !ok {
			return fmt.Errorf("%s.ignoreTypeImports must be a boolean, got %T", prefix, ignoreType)
		}
	}

	return nil
}

func validateRestrictedImportsDetectionOptions(opts *RestrictedImportsDetectionOptions, prefix string) error {
	if !opts.Enabled {
		return nil
	}

	if len(opts.EntryPoints) == 0 {
		return fmt.Errorf("%s.entryPoints is required when enabled", prefix)
	}

	if len(opts.DenyFiles) == 0 && len(opts.DenyModules) == 0 {
		return fmt.Errorf("%s: either denyFiles or denyModules must be provided when enabled", prefix)
	}

	for i, entryPoint := range opts.EntryPoints {
		if strings.TrimSpace(entryPoint) == "" {
			return fmt.Errorf("%s.entryPoints[%d]: cannot be empty", prefix, i)
		}
	}

	for i, pattern := range opts.DenyFiles {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("%s.denyFiles[%d]: cannot be empty", prefix, i)
		}
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("%s.denyFiles[%d]: %w", prefix, i, err)
		}
	}

	for i, pattern := range opts.IgnoreMatches {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("%s.ignoreMatches[%d]: cannot be empty", prefix, i)
		}
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("%s.ignoreMatches[%d]: %w", prefix, i, err)
		}
	}

	for i, pattern := range opts.GraphExclude {
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("%s.graphExclude[%d]: %w", prefix, i, err)
		}
	}

	for i, pattern := range opts.DenyModules {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			return fmt.Errorf("%s.denyModules[%d]: cannot be empty", prefix, i)
		}
		if _, err := glob.Compile(trimmed); err != nil {
			return fmt.Errorf("%s.denyModules[%d]: invalid glob pattern '%s': %v", prefix, i, trimmed, err)
		}
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

func validateRuleEntryPoints(rule *Rule, prefix string) error {
	for i, entryPoint := range rule.ProdEntryPoints {
		if entryPoint == "" {
			return fmt.Errorf("%s.prodEntryPoints[%d]: cannot be empty", prefix, i)
		}
	}

	for i, entryPoint := range rule.DevEntryPoints {
		if entryPoint == "" {
			return fmt.Errorf("%s.devEntryPoints[%d]: cannot be empty", prefix, i)
		}
	}

	return nil
}

func parseFollowMonorepoPackagesValue(rawValue interface{}) (FollowMonorepoPackagesValue, error) {
	switch v := rawValue.(type) {
	case bool:
		if v {
			return FollowMonorepoPackagesValue{FollowAll: true}, nil
		}
		return FollowMonorepoPackagesValue{}, nil
	case []interface{}:
		if len(v) == 0 {
			return FollowMonorepoPackagesValue{}, fmt.Errorf("followMonorepoPackages must be a boolean or array of strings: array cannot be empty")
		}

		patterns := make(map[string]bool, len(v))
		for i, item := range v {
			pattern, ok := item.(string)
			if !ok {
				return FollowMonorepoPackagesValue{}, fmt.Errorf("followMonorepoPackages[%d] must be a string, got %T", i, item)
			}

			trimmedPattern := strings.TrimSpace(pattern)
			if trimmedPattern == "" {
				return FollowMonorepoPackagesValue{}, fmt.Errorf("followMonorepoPackages[%d] cannot be empty", i)
			}

			patterns[trimmedPattern] = true
		}

		return FollowMonorepoPackagesValue{Packages: patterns}, nil
	default:
		return FollowMonorepoPackagesValue{}, fmt.Errorf("followMonorepoPackages must be a boolean or array of strings, got %T", rawValue)
	}
}

func validateRawFollowMonorepoPackages(followMonorepoPackages interface{}, ruleIndex int) error {
	switch v := followMonorepoPackages.(type) {
	case bool:
		return nil
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("rules[%d].followMonorepoPackages must be a boolean or array of strings: array cannot be empty", ruleIndex)
		}

		for i, item := range v {
			strValue, ok := item.(string)
			if !ok {
				return fmt.Errorf("rules[%d].followMonorepoPackages must be a boolean or array of strings: rules[%d].followMonorepoPackages[%d] is %T", ruleIndex, ruleIndex, i, item)
			}

			trimmedValue := strings.TrimSpace(strValue)
			if trimmedValue == "" {
				return fmt.Errorf("rules[%d].followMonorepoPackages[%d] cannot be empty", ruleIndex, i)
			}
		}

		return nil
	default:
		return fmt.Errorf("rules[%d].followMonorepoPackages must be a boolean or array of strings, got %T", ruleIndex, followMonorepoPackages)
	}
}
