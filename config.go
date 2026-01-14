package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/jsonc"
)

type BoundaryRule struct {
	Name    string   `json:"name"`
	Pattern string   `json:"pattern"` // Glob pattern for files in this boundary
	Allow   []string `json:"allow"`   // Glob patterns for allowed imports
	Deny    []string `json:"deny"`    // Glob patterns for denied imports (overrides allow)
}

type CircularImportsOptions struct {
	Enabled           bool `json:"enabled"`
	IgnoreTypeImports bool `json:"ignoreTypeImports"`
}

type OrphanFilesOptions struct {
	Enabled           bool     `json:"enabled"`
	ValidEntryPoints  []string `json:"validEntryPoints"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports"`
	GraphExclude      []string `json:"graphExclude"`
}

type UnusedNodeModulesOptions struct {
	Enabled                   bool     `json:"enabled"`
	IncludeModules            []string `json:"includeModules"`
	ExcludeModules            []string `json:"excludeModules"`
	PkgJsonFieldsWithBinaries []string `json:"pkgJsonFieldsWithBinaries"`
	FilesWithBinaries         []string `json:"filesWithBinaries"`
	FilesWithModules          []string `json:"filesWithModules"`
	OutputType                string   `json:"outputType"` // "list", "groupByModule", "groupByFile"
}

type MissingNodeModulesOptions struct {
	Enabled        bool     `json:"enabled"`
	IncludeModules []string `json:"includeModules"`
	ExcludeModules []string `json:"excludeModules"`
	OutputType     string   `json:"outputType"` // "list", "groupByModule", "groupByFile"
}

type Rule struct {
	Path                        string                     `json:"path"` // Required
	ModuleBoundaries            []BoundaryRule             `json:"moduleBoundaries,omitempty"`
	CircularImportsDetection    *CircularImportsOptions    `json:"circularImportsDetection,omitempty"`
	OrphanFilesDetection        *OrphanFilesOptions        `json:"orphanFilesDetection,omitempty"`
	UnusedNodeModulesDetection  *UnusedNodeModulesOptions  `json:"unusedNodeModulesDetection,omitempty"`
	MissingNodeModulesDetection *MissingNodeModulesOptions `json:"missingNodeModulesDetection,omitempty"`
}

type RevDepConfig struct {
	ConfigVersion  string   `json:"configVersion"` // Required
	ConditionNames []string `json:"conditionNames,omitempty"`
	IgnoreFiles    []string `json:"ignoreFiles,omitempty"`
	Rules          []Rule   `json:"rules"`
}

var configFileName = "rev-dep.config.json"

// LoadConfig loads the rev-dep configuration from the specified path.
// configPath can be a specific file path or a directory containing rev-dep.config.json.
// Returns a single RevDepConfig object.
func LoadConfig(configPath string) ([]RevDepConfig, error) {
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		return nil, err
	}

	actualPath := configPath
	if fileInfo.IsDir() {
		actualPath = filepath.Join(configPath, configFileName)
	}

	content, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, err
	}

	// Parse as single object
	var config RevDepConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if config.ConfigVersion == "" {
		return nil, fmt.Errorf("configVersion is required")
	}
	for j, rule := range config.Rules {
		if rule.Path == "" {
			return nil, fmt.Errorf("rules[%d].path is required", j)
		}
		// Validate module boundaries in rules
		for k, boundary := range rule.ModuleBoundaries {
			if err := validatePattern(boundary.Pattern); err != nil {
				return nil, fmt.Errorf("rules[%d].moduleBoundaries[%d].pattern: %w", j, k, err)
			}
			for l, p := range boundary.Allow {
				if err := validatePattern(p); err != nil {
					return nil, fmt.Errorf("rules[%d].moduleBoundaries[%d].allow[%d]: %w", j, k, l, err)
				}
			}
			for l, p := range boundary.Deny {
				if err := validatePattern(p); err != nil {
					return nil, fmt.Errorf("rules[%d].moduleBoundaries[%d].deny[%d]: %w", j, k, l, err)
				}
			}
		}
	}

	return []RevDepConfig{config}, nil
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
