package module

import (
	"fmt"
	"strings"

	"github.com/gobwas/glob"
)

const AllBuiltInModulesPattern = "builtin:*"

const builtInKeywordPrefix = "builtin:"

type ModulePattern struct {
	glob        glob.Glob
	allBuiltIns bool
}

func ValidateModulePattern(pattern string) error {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == AllBuiltInModulesPattern {
		return nil
	}
	if strings.HasPrefix(trimmed, builtInKeywordPrefix) {
		return fmt.Errorf("'%s' is not supported: use '%s' for every built-in module, or a module name such as 'fs' or 'node:fs' for one", trimmed, AllBuiltInModulesPattern)
	}
	if _, err := glob.Compile(trimmed); err != nil {
		return fmt.Errorf("invalid glob pattern '%s': %v", trimmed, err)
	}
	return nil
}

func CompileModulePatterns(patterns []string) []ModulePattern {
	compiled := make([]ModulePattern, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			continue
		}
		if trimmed == AllBuiltInModulesPattern {
			compiled = append(compiled, ModulePattern{allBuiltIns: true})
			continue
		}
		if strings.HasPrefix(trimmed, builtInKeywordPrefix) {
			continue
		}
		matcher, err := glob.Compile(trimmed)
		if err != nil {
			continue
		}
		compiled = append(compiled, ModulePattern{glob: matcher})
	}
	return compiled
}

func MatchesAnyModulePattern(patterns []ModulePattern, request string) bool {
	name := GetNodeModuleName(request)
	for _, pattern := range patterns {
		if pattern.allBuiltIns {
			if IsBuiltInModule(request) {
				return true
			}
			continue
		}
		if pattern.glob.Match(name) || pattern.glob.Match(request) {
			return true
		}
	}
	return false
}
