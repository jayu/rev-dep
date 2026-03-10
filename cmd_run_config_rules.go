package main

import (
	"fmt"
	"slices"
)

func filterRunConfigRules(config *RevDepConfig, rules []string) error {
	if len(rules) == 0 {
		return nil
	}

	var filteredRules []Rule
	for _, r := range config.Rules {
		if slices.Contains(rules, r.Path) {
			filteredRules = append(filteredRules, r)
		}
	}

	if len(filteredRules) == 0 {
		return fmt.Errorf("none of the requested rules %v found in config", rules)
	}

	config.Rules = filteredRules
	return nil
}
