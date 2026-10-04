package cli

import (
	"strings"
	"testing"

	"rev-dep-go/internal/config"
)

func emptyRuleResult(path string, failOnEmpty bool) config.RuleResult {
	return config.RuleResult{RulePath: path, FileCount: 0, FailOnEmptyWorkspace: failOnEmpty}
}

func TestJSONOutputReportsEmptyWorkspace(t *testing.T) {
	cases := []struct {
		name string
		rr   config.RuleResult
		want string
	}{
		{name: "failing", rr: emptyRuleResult("packages/a", true), want: "fail"},
		{name: "warning", rr: emptyRuleResult("packages/a", false), want: "warn"},
		{name: "has files", rr: config.RuleResult{RulePath: "packages/a", FileCount: 3, FailOnEmptyWorkspace: true}, want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildJSONRuleResult(c.rr, "/work", nil).EmptyWorkspace; got != c.want {
				t.Errorf("EmptyWorkspace = %q, want %q", got, c.want)
			}
		})
	}
}

// The issues list holds only what failed the run, so a warning-only empty workspace stays out.
func TestIssuesListReportsOnlyFailingEmptyWorkspaces(t *testing.T) {
	out := formatIssuesListOutput([]jsonRuleResult{
		buildJSONRuleResult(emptyRuleResult("packages/failing", true), "/work", nil),
		buildJSONRuleResult(emptyRuleResult("packages/warning", false), "/work", nil),
	})

	if !strings.Contains(out, "Empty Workspaces (1):") || !strings.Contains(out, "packages/failing (0 files)") {
		t.Errorf("failing empty workspace missing from the issues list:\n%s", out)
	}
	if strings.Contains(out, "packages/warning") {
		t.Errorf("warning-only empty workspace listed as an issue:\n%s", out)
	}
}

// --fix cannot add files to a workspace, so a failing empty workspace keeps the exit non-zero.
func TestFixRunStillFailsOnEmptyWorkspace(t *testing.T) {
	result := &config.ConfigProcessingResult{
		HasFailures: true,
		RuleResults: []config.RuleResult{emptyRuleResult("packages/a", true)},
	}
	if !shouldConfigRunExitNonZero(result, true) {
		t.Error("shouldConfigRunExitNonZero(fix) = false, want true for a failing empty workspace")
	}
}
