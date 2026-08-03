package config

import (
	"strconv"
	"strings"

	"rev-dep-go/internal/shell"
)

// Produces the command line for a duplicated-code invocation.
// If snapshot file is provided, params are skipped and taken from the snapshot instead.
func DuplicatedCodeCommand(opts *DuplicatedCodeOptions, rulePath string, globalIgnore, processIgnored []string) string {
	parts := []string{"rev-dep", "duplicated-code"}

	if dir := cwdArgForRule(rulePath); dir != "" {
		parts = append(parts, "--cwd", shell.Quote(dir))
	}

	if opts == nil {
		return strings.Join(parts, " ")
	}

	if opts.SnapshotPath != "" {

		return strings.Join(append(parts, "--snapshot", shell.Quote(opts.SnapshotPath)), " ")
	}

	if opts.BlindIdentifiers {
		parts = append(parts, "--blind-identifiers")
	}
	if opts.BlindStrings {
		parts = append(parts, "--blind-strings")
	}
	if opts.BlindNumbers {
		parts = append(parts, "--blind-numbers")
	}
	if opts.SkipObjects {
		parts = append(parts, "--skip-objects")
	}
	for _, flag := range []struct {
		name  string
		value int
	}{
		{"--min-tokens", opts.MinTokens},
		{"--min-lines", opts.MinLines},
		{"--min-depth", opts.MinDepth},
		{"--min-statements", opts.MinStatements},
		{"--min-duplicates", opts.MinDuplicates},
	} {
		if flag.value > 0 {
			parts = append(parts, flag.name, strconv.Itoa(flag.value))
		}
	}
	// Both sets of patterns simply remove files here, so they collapse into one flag.
	seen := map[string]bool{}
	for _, pattern := range append(append([]string{}, globalIgnore...), opts.IgnoreFiles...) {
		if pattern == "" || seen[pattern] {
			continue
		}
		seen[pattern] = true
		parts = append(parts, "--ignore-files", shell.Quote(pattern))
	}
	for _, pattern := range processIgnored {
		if pattern == "" {
			continue
		}
		parts = append(parts, "--process-ignored-files", shell.Quote(pattern))
	}
	return strings.Join(parts, " ")
}

// cwdArgForRule returns the --cwd value for a rule, or "" when the rule covers the directory
// the command would run in anyway. Deliberately not config.normalizeRulePath, which maps root
// to "." because its callers need a path; here root means "omit the flag".
func cwdArgForRule(rulePath string) string {
	trimmed := strings.TrimSpace(rulePath)
	trimmed = strings.TrimSuffix(trimmed, "/")
	switch trimmed {
	case "", ".", "./":
		return ""
	}
	return trimmed
}
