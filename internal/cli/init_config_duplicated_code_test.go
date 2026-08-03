package cli

import (
	"testing"

	"rev-dep-go/internal/config"
)

func TestInitConfiguresDuplicatedCodeOnceOnTheRootWorkspace(t *testing.T) {
	generate := func(ps projectStructure, standalone standalonePackages, sel standaloneSelection) []config.Rule {
		specs, meta := planRules(ps, standalone, sel)
		var rules []config.Rule
		for _, spec := range specs {
			rules = append(rules, materializeRule(spec, detectorsAll, carriesDuplicatedCode(spec, meta.rootRuleCreated)))
		}
		return rules
	}
	withDuplicatedCode := func(rules []config.Rule) []string {
		var paths []string
		for _, r := range rules {
			if len(r.DuplicatedCodeDetections) > 0 && r.DuplicatedCodeDetections[0].Enabled {
				paths = append(paths, r.Path)
			}
		}
		return paths
	}
	equal := func(t *testing.T, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("duplicated code configured on %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("duplicated code configured on %v, want %v", got, want)
			}
		}
	}

	t.Run("monorepo: the root rule only, never the packages", func(t *testing.T) {
		ps := projectStructure{
			cwd:                  "/repo",
			isMonorepo:           true,
			workspacePackageDirs: []string{"/repo/packages/a", "/repo/packages/b"},
		}
		rules := generate(ps, standalonePackages{}, standaloneNone)
		if len(rules) != 3 {
			t.Fatalf("expected a root rule plus two package rules, got %d", len(rules))
		}
		equal(t, withDuplicatedCode(rules), []string{"."})
	})

	t.Run("single package project: the root rule", func(t *testing.T) {
		ps := projectStructure{cwd: "/repo", rootHasPackageJson: true}
		equal(t, withDuplicatedCode(generate(ps, standalonePackages{}, standaloneNone)), []string{"."})
	})

	t.Run("root rule plus standalone packages: still only the root", func(t *testing.T) {
		ps := projectStructure{cwd: "/repo", rootHasPackageJson: true}
		standalone := standalonePackages{all: []string{"tools/cli"}, curated: []string{"tools/cli"}}
		equal(t, withDuplicatedCode(generate(ps, standalone, standaloneCurated)), []string{"."})
	})

	t.Run("no base project: each package keeps its own", func(t *testing.T) {
		standalone := standalonePackages{
			all:     []string{"apps/api", "apps/web"},
			curated: []string{"apps/api", "apps/web"},
		}
		rules := generate(projectStructure{cwd: "/repo"}, standalone, standaloneCurated)
		equal(t, withDuplicatedCode(rules), []string{"apps/api", "apps/web"})
	})
}
