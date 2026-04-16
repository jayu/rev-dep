package rules

// BoundaryRule describes module boundary constraints.
type BoundaryRule struct {
	Name    string   `json:"name"`
	Pattern string   `json:"pattern"`        // Glob pattern for files in this boundary
	Allow   []string `json:"allow"`          // Glob patterns for allowed imports
	Deny    []string `json:"deny,omitempty"` // Glob patterns for denied imports (overrides allow)
}

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

// ImportConventionDomain represents a single domain definition.
type ImportConventionDomain struct {
	Path    string `json:"path,omitempty"`
	Alias   string `json:"alias,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

// ImportConventionRule represents a rule for import path conventions.
type ImportConventionRule struct {
	Rule    string
	Domains []ImportConventionDomain
	Autofix bool
}
