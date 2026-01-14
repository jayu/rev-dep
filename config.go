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

type RevDepConfig struct {
	Path               string         `json:"path,omitempty"` // Working directory this config applies to (default: ".")
	ModuleBoundaries   []BoundaryRule `json:"module_boundaries"`
	EntryPoints        interface{}
	NodeModulesConfig  interface{}
	MissingNodeModules interface{}
	UnusedNodeModules  interface{}
}

var configFileName = "rev-dep.config.json"

// LoadConfig loads the rev-dep configuration from the specified path.
// configPath can be a specific file path or a directory containing rev-dep.config.json.
// Returns a slice of RevDepConfig, allowing multiple configurations in one file (array of objects).
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

	// Try to unmarshal as a list first
	var configs []RevDepConfig
	if err := json.Unmarshal(jsonc.ToJSON(content), &configs); err != nil {
		// If that fails, maybe it's a single object (backward compatibility or user error)?
		// Let's try single object
		var singleConfig RevDepConfig
		if err2 := json.Unmarshal(jsonc.ToJSON(content), &singleConfig); err2 == nil {
			configs = []RevDepConfig{singleConfig}
		} else {
			// If both fail, return original error
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	for i, config := range configs {
		for j, rule := range config.ModuleBoundaries {
			if err := validatePattern(rule.Pattern); err != nil {
				return nil, fmt.Errorf("config[%d].module_boundaries[%d].pattern: %w", i, j, err)
			}
			for k, p := range rule.Allow {
				if err := validatePattern(p); err != nil {
					return nil, fmt.Errorf("config[%d].module_boundaries[%d].allow[%d]: %w", i, j, k, err)
				}
			}
			for k, p := range rule.Deny {
				if err := validatePattern(p); err != nil {
					return nil, fmt.Errorf("config[%d].module_boundaries[%d].deny[%d]: %w", i, j, k, err)
				}
			}
		}
	}

	return configs, nil
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
