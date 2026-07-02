package telemetry

import "rev-dep-go/internal/config"

// Metrics describes the anonymous usage shape collected for a `config run`. Every field is a count
// - there are no free-form strings - so nothing here can identify a user or a project.
type Metrics struct {
	WorkspaceCount int `json:"workspaceCount"` // number of rules (== monorepo package count)
	FileCount      int `json:"fileCount"`      // files processed in this run

	// Per-detector: the maximum number of ENABLED definitions across all workspaces. A single
	// detector object counts as 1, an array counts its enabled entries, and a detector that is
	// absent or disabled in every workspace is 0 - so 0 doubles as "feature unused in this project".
	CircularImports           int `json:"circularImports"`
	OrphanFiles               int `json:"orphanFiles"`
	UnusedNodeModules         int `json:"unusedNodeModules"`
	MissingNodeModules        int `json:"missingNodeModules"`
	UnusedExports             int `json:"unusedExports"`
	UnresolvedImports         int `json:"unresolvedImports"`
	DevDepsUsageOnProd        int `json:"devDepsUsageOnProd"`
	RestrictedImports         int `json:"restrictedImports"`
	RestrictedImporters       int `json:"restrictedImporters"`
	RestrictedDirectImporters int `json:"restrictedDirectImporters"`
	ModuleBoundaries          int `json:"moduleBoundaries"`
	ImportConventions         int `json:"importConventions"`

	// Root-level option usage: 1 when the option is set to a non-default value, else 0.
	UsesNearestPackageResolution int `json:"usesNearestPackageResolution"`
	UsesIncludeDevDepsFromRoot   int `json:"usesIncludeDevDepsFromRoot"`
	UsesProcessIgnoredFiles      int `json:"usesProcessIgnoredFiles"`
	UsesIgnoreFiles              int `json:"usesIgnoreFiles"`
	UsesCustomAssetExtensions    int `json:"usesCustomAssetExtensions"`
	UsesConditionNames           int `json:"usesConditionNames"`
}

type enabledOption interface{ IsEnabled() bool }

// countEnabled returns the number of enabled definitions in a detector's slice. A disabled or nil
// entry is not counted, so a detector defined but disabled contributes 0.
func countEnabled[T enabledOption](items []T) int {
	n := 0
	for _, item := range items {
		if item.IsEnabled() {
			n++
		}
	}
	return n
}

// BuildMetrics derives the anonymous usage metrics from a parsed config. It is pure, in-memory, and
// fast - safe to call on the hot path before dispatching telemetry.
func BuildMetrics(cfg *config.RevDepConfig, fileCount int) Metrics {
	m := Metrics{
		WorkspaceCount: len(cfg.Rules),
		FileCount:      fileCount,
	}

	for i := range cfg.Rules {
		rule := &cfg.Rules[i]
		m.CircularImports = max(m.CircularImports, countEnabled(rule.CircularImportsDetections))
		m.OrphanFiles = max(m.OrphanFiles, countEnabled(rule.OrphanFilesDetections))
		m.UnusedNodeModules = max(m.UnusedNodeModules, countEnabled(rule.UnusedNodeModulesDetections))
		m.MissingNodeModules = max(m.MissingNodeModules, countEnabled(rule.MissingNodeModulesDetections))
		m.UnusedExports = max(m.UnusedExports, countEnabled(rule.UnusedExportsDetections))
		m.UnresolvedImports = max(m.UnresolvedImports, countEnabled(rule.UnresolvedImportsDetections))
		m.DevDepsUsageOnProd = max(m.DevDepsUsageOnProd, countEnabled(rule.DevDepsUsageOnProdDetections))
		m.RestrictedImports = max(m.RestrictedImports, countEnabled(rule.RestrictedImportsDetections))
		m.RestrictedImporters = max(m.RestrictedImporters, countEnabled(rule.RestrictedImportersDetections))
		m.RestrictedDirectImporters = max(m.RestrictedDirectImporters, countEnabled(rule.RestrictedDirectImportersDetections))
		// ModuleBoundaries and ImportConventions are presence-based (no enabled flag).
		m.ModuleBoundaries = max(m.ModuleBoundaries, len(rule.ModuleBoundaries))
		m.ImportConventions = max(m.ImportConventions, len(rule.ImportConventions))
	}

	if cfg.UsesNearestPackage() {
		m.UsesNearestPackageResolution = 1
	}
	if cfg.IncludeDevDepsFromRoot() {
		m.UsesIncludeDevDepsFromRoot = 1
	}
	if len(cfg.ProcessIgnoredFiles) > 0 {
		m.UsesProcessIgnoredFiles = 1
	}
	if len(cfg.IgnoreFiles) > 0 {
		m.UsesIgnoreFiles = 1
	}
	if len(cfg.CustomAssetExtensions) > 0 {
		m.UsesCustomAssetExtensions = 1
	}
	if len(cfg.ConditionNames) > 0 {
		m.UsesConditionNames = 1
	}

	return m
}

// asMeasurements flattens Metrics into the numeric map Application Insights expects.
func (m Metrics) asMeasurements() map[string]float64 {
	return map[string]float64{
		"workspaceCount":               float64(m.WorkspaceCount),
		"fileCount":                    float64(m.FileCount),
		"circularImports":              float64(m.CircularImports),
		"orphanFiles":                  float64(m.OrphanFiles),
		"unusedNodeModules":            float64(m.UnusedNodeModules),
		"missingNodeModules":           float64(m.MissingNodeModules),
		"unusedExports":                float64(m.UnusedExports),
		"unresolvedImports":            float64(m.UnresolvedImports),
		"devDepsUsageOnProd":           float64(m.DevDepsUsageOnProd),
		"restrictedImports":            float64(m.RestrictedImports),
		"restrictedImporters":          float64(m.RestrictedImporters),
		"restrictedDirectImporters":    float64(m.RestrictedDirectImporters),
		"moduleBoundaries":             float64(m.ModuleBoundaries),
		"importConventions":            float64(m.ImportConventions),
		"usesNearestPackageResolution": float64(m.UsesNearestPackageResolution),
		"usesIncludeDevDepsFromRoot":   float64(m.UsesIncludeDevDepsFromRoot),
		"usesProcessIgnoredFiles":      float64(m.UsesProcessIgnoredFiles),
		"usesIgnoreFiles":              float64(m.UsesIgnoreFiles),
		"usesCustomAssetExtensions":    float64(m.UsesCustomAssetExtensions),
		"usesConditionNames":           float64(m.UsesConditionNames),
	}
}
