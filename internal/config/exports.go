package config

// AnyRuleChecksForUnusedExports reports whether any rule in the config enables the unused-exports check.
func AnyRuleChecksForUnusedExports(config *RevDepConfig) bool {
	return anyRuleChecksForUnusedExports(config)
}
