package cli

import (
	"fmt"
	"slices"
	"strings"

	"rev-dep-go/internal/config"
)

// filterRunConfigRules narrows cfg to the workspaces named by --workspaces. Each requested path must
// be written exactly as the workspace path in the config: silently dropping a typo would run fewer
// checks than asked for while still reporting success.
func filterRunConfigRules(cfg *config.RevDepConfig, rules []string) error {
	if len(rules) == 0 {
		return nil
	}

	configured := make([]string, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		configured = append(configured, r.Path)
	}

	var unknown []string
	for _, r := range rules {
		if !slices.Contains(configured, r) {
			unknown = append(unknown, r)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("--workspaces: no workspace in config matches %s (configured workspaces: %s)",
			strings.Join(unknown, ", "), strings.Join(configured, ", "))
	}

	var filteredRules []config.Rule
	for _, r := range cfg.Rules {
		if slices.Contains(rules, r.Path) {
			filteredRules = append(filteredRules, r)
		}
	}

	cfg.Rules = filteredRules
	return nil
}
