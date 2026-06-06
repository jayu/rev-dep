package checks

import (
	"testing"

	"rev-dep-go/internal/rules"
)

// userDep is a small helper to build a resolved user-module dependency.
func userDep(id, request string) MinimalDependency {
	return MinimalDependency{ID: id, Request: request, ResolvedType: UserModule}
}

func TestCheckModuleBoundaries_MutuallyExclusive(t *testing.T) {
	cwd := "/repo"

	boundaries := []rules.BoundaryRule{
		{
			Name: "feature-isolation",
			MutuallyExclusive: []string{
				"src/modules/analytics/**",
				"src/modules/billing/**",
				"src/modules/reporting/**",
			},
		},
	}

	analytics := "/repo/src/modules/analytics/index.ts"
	billing := "/repo/src/modules/billing/index.ts"

	tree := MinimalDependencyTree{
		analytics: {
			userDep("/repo/src/modules/billing/service.ts", "../billing/service"), // cross-group -> violation
			userDep("/repo/src/modules/analytics/util.ts", "./util"),              // same group -> ok
			userDep("/repo/src/shared/log.ts", "../../shared/log"),                // unlisted -> ok
		},
		billing: {
			userDep("/repo/src/modules/analytics/util.ts", "../analytics/util"), // cross-group (reverse) -> violation
		},
	}

	files := []string{analytics, billing}

	violations := CheckModuleBoundariesFromTree(tree, files, boundaries, cwd)

	if len(violations) != 2 {
		t.Fatalf("expected 2 cross-group violations, got %d: %+v", len(violations), violations)
	}

	for _, v := range violations {
		if v.RuleName != "feature-isolation" {
			t.Errorf("expected RuleName 'feature-isolation', got %q", v.RuleName)
		}
		if v.ViolationType != "denied" {
			t.Errorf("expected ViolationType 'denied', got %q", v.ViolationType)
		}
	}

	// Violations are sorted by file path: analytics first, then billing.
	if violations[0].FilePath != analytics || violations[0].ImportPath != "/repo/src/modules/billing/service.ts" {
		t.Errorf("unexpected first violation: %+v", violations[0])
	}
	if violations[1].FilePath != billing || violations[1].ImportPath != "/repo/src/modules/analytics/util.ts" {
		t.Errorf("unexpected second violation: %+v", violations[1])
	}
}

// denyIgnore carves an exception out of deny: a denied import that also matches
// denyIgnore is exempt, while everything else outside deny stays unrestricted
// (default-open is preserved).
func TestCheckModuleBoundaries_DenyIgnore(t *testing.T) {
	cwd := "/repo"

	boundaries := []rules.BoundaryRule{
		{
			Name:       "ui-no-api-internals",
			Pattern:    "src/ui/**",
			Deny:       []string{"src/api/**"},
			DenyIgnore: []string{"src/api/dto/**"},
		},
	}

	ui := "/repo/src/ui/page.ts"
	tree := MinimalDependencyTree{
		ui: {
			userDep("/repo/src/api/internal/db.ts", "../api/internal/db"), // denied, not exempt -> violation
			userDep("/repo/src/api/dto/user.ts", "../api/dto/user"),       // denied but exempt -> ok
			userDep("/repo/src/widgets/button.ts", "../widgets/button"),   // not denied -> ok
		},
	}

	violations := CheckModuleBoundariesFromTree(tree, []string{ui}, boundaries, cwd)

	if len(violations) != 1 {
		t.Fatalf("expected exactly 1 violation, got %d: %+v", len(violations), violations)
	}
	v := violations[0]
	if v.ImportPath != "/repo/src/api/internal/db.ts" {
		t.Errorf("expected the non-exempt api/internal import to be the only violation, got %+v", v)
	}
	if v.ViolationType != "denied" {
		t.Errorf("expected ViolationType 'denied', got %q", v.ViolationType)
	}
}

// A single mutuallyExclusive group must never flag an import that stays inside
// the same group, nor one to a path not listed in the group at all.
func TestCheckModuleBoundaries_MutuallyExclusive_NoFalsePositives(t *testing.T) {
	cwd := "/repo"

	boundaries := []rules.BoundaryRule{
		{
			Name: "feature-isolation",
			MutuallyExclusive: []string{
				"src/modules/analytics/**",
				"src/modules/billing/**",
			},
		},
	}

	analytics := "/repo/src/modules/analytics/index.ts"
	tree := MinimalDependencyTree{
		analytics: {
			userDep("/repo/src/modules/analytics/util.ts", "./util"),  // same group
			userDep("/repo/src/shared/log.ts", "../../shared/log"),    // unlisted (shared)
			userDep("/repo/src/modules/analytics/sub/deep.ts", "./x"), // same group, nested
		},
	}

	violations := CheckModuleBoundariesFromTree(tree, []string{analytics}, boundaries, cwd)

	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(violations), violations)
	}
}
