package cli

import (
	"fmt"
	"slices"

	"rev-dep-go/internal/config"
)

func filterRunConfigRules(cfg *config.RevDepConfig, rules []string) error {
	if len(rules) == 0 {
		return nil
	}

	var filteredRules []config.Rule
	for _, r := range cfg.Rules {
		if slices.Contains(rules, r.Path) {
			filteredRules = append(filteredRules, r)
		}
	}

	if len(filteredRules) == 0 {
		return fmt.Errorf("none of the requested rules %v found in config", rules)
	}

	cfg.Rules = filteredRules
	return nil
}
