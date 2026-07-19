package rules

// BoundaryRule describes module boundary constraints.
//
// A rule is one of two mutually exclusive shapes:
//   - an explicit boundary: Pattern selects the source files, Allow/Deny select
//     the import targets (directional, supports any allow/deny matrix); or
//   - a MutuallyExclusive group: a flat list of globs where a file matching one
//     glob may not import a file matching any other glob in the list (sibling
//     isolation). It is sugar that expands to one explicit boundary per glob.
type BoundaryRule struct {
	Name    string   `json:"name"`
	Pattern string   `json:"pattern,omitempty"` // Glob pattern for files in this boundary
	Allow   []string `json:"allow,omitempty"`   // Glob patterns for allowed imports
	Deny    []string `json:"deny,omitempty"`    // Glob patterns for denied imports (overrides allow)

	// DenyIgnore carves exceptions out of Deny: an import matched by Deny is not
	// reported if it is also matched by DenyIgnore. Only meaningful with Deny.
	DenyIgnore []string `json:"denyIgnore,omitempty"`

	// MutuallyExclusive is a flat list of globs that may not import across each
	// other. Mutually exclusive with Pattern/Allow/Deny on the same rule.
	MutuallyExclusive []string `json:"mutuallyExclusive,omitempty"`
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

// RestrictedImportersDetectionOptions configures the inverse of restricted imports: it whitelists
// which entry points may transitively reach (import) a set of files and/or node modules. Any entry
// point NOT matching allowedEntryPoints that reaches one of those targets is a violation.
//
// The policy verb sits on the entry points (allowedEntryPoints), so the targets stay bare (files /
// modules) - the mirror image of restrictedImportsDetection, which puts the verb on the targets
// (denyFiles / denyModules) and leaves entryPoints bare.
//
// (The "blacklist" direction - forbidding specific entry points from reaching a target - is covered
// by restrictedImportsDetection, so it is intentionally not duplicated here.)
type RestrictedImportersDetectionOptions struct {
	Enabled            bool     `json:"enabled"`
	Files              []string `json:"files,omitempty"`
	Modules            []string `json:"modules,omitempty"`
	AllowedEntryPoints []string `json:"allowedEntryPoints,omitempty"`
	GraphExclude       []string `json:"graphExclude,omitempty"`
	IgnoreMatches      []string `json:"ignoreMatches,omitempty"`
	IgnoreTypeImports  bool     `json:"ignoreTypeImports,omitempty"`
}

func (o *RestrictedImportersDetectionOptions) IsEnabled() bool { return o != nil && o.Enabled }

// RestrictedDirectImportersDetectionOptions configures a NON-transitive importer policy: for a set of
// target files XOR node modules, it constrains which files may DIRECTLY import them. Unlike
// restrictedImportersDetection (which reasons about transitive reachability from entry points and needs
// a dependency graph), this detector only inspects direct import edges, so it never builds a graph.
//
// The policy is one of two mutually exclusive shapes:
//   - AllowImporters (whitelist): only files matching AllowImporters may directly import a target; any
//     other direct importer is a violation.
//   - DenyImporters (blacklist): files matching DenyImporters may not directly import a target; such a
//     direct importer is a violation.
//
// Files and Modules are mutually exclusive; AllowImporters and DenyImporters are mutually exclusive.
type RestrictedDirectImportersDetectionOptions struct {
	Enabled           bool     `json:"enabled"`
	Files             []string `json:"files,omitempty"`
	Modules           []string `json:"modules,omitempty"`
	AllowImporters    []string `json:"allowImporters,omitempty"`
	DenyImporters     []string `json:"denyImporters,omitempty"`
	IgnoreMatches     []string `json:"ignoreMatches,omitempty"`
	IgnoreTypeImports bool     `json:"ignoreTypeImports,omitempty"`
}

func (o *RestrictedDirectImportersDetectionOptions) IsEnabled() bool { return o != nil && o.Enabled }

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
