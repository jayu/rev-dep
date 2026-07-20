package globutil

import (
	"fmt"
	"testing"
)

var benchPatterns = []string{
	"**/*.ts", "src/**/*.tsx", "**/node_modules/**", "dist/**",
	"src/pages/**/*.ts*", "**/__tests__/**", "src/**/*.stories.tsx", "**/*.d.ts",
}

var benchPaths []string

func init() {
	for i := 0; i < 400; i++ {
		benchPaths = append(benchPaths,
			fmt.Sprintf("/repo/src/components/dir%d/Button.tsx", i),
			fmt.Sprintf("/repo/src/pages/admin/x%d/index.ts", i),
			fmt.Sprintf("/repo/node_modules/pkg%d/lib/index.js", i),
			fmt.Sprintf("/repo/deep/a/b/c/d/e/f/file%d.ts", i),
			fmt.Sprintf("/repo/other/thing%d.json", i),
		)
	}
}

func BenchmarkMatchNoNegation(b *testing.B) {
	m := CreateGlobMatchers(benchPatterns, "/repo")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range benchPaths {
			MatchesAnyGlobMatcher(p, m, false)
		}
	}
}

func BenchmarkMatchWithNegation(b *testing.B) {
	pats := append(append([]string{}, benchPatterns...), "!src/vendor/**", "!**/keep.ts")
	m := CreateGlobMatchers(pats, "/repo")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range benchPaths {
			MatchesAnyGlobMatcher(p, m, false)
		}
	}
}

// Mirrors a real project .gitignore: mostly bare directory names plus a few no-slash
// wildcards. This is the shape that forces the ancestor walk for every path.
var benchGitignorePatterns = []string{
	"node_modules", "dist", "build", "coverage", ".next", ".turbo", ".cache",
	"*.log", "*.tmp", "*.swp", "*.orig", "*.bak", ".DS_Store", ".env", ".env.local",
	"*.tsbuildinfo", ".eslintcache", ".vscode", ".idea", "*.iml", "__snapshots__",
	"storybook-static", "playwright-report", "test-results", ".nyc_output", "lib-cov",
	"*.pid", "*.seed", "bower_components", "jspm_packages", "web_modules", ".parcel-cache",
	"out", ".nuxt", ".serverless", ".fusebox", ".dynamodb", ".tern-port",
}

func BenchmarkMatchGitignoreShape(b *testing.B) {
	m := CreateGlobMatchers(benchGitignorePatterns, "/repo")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range benchPaths {
			MatchesAnyGlobMatcher(p, m, false)
		}
	}
}
