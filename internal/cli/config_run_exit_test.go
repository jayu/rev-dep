package cli

import (
	"testing"

	"rev-dep-go/internal/checks"
	"rev-dep-go/internal/config"
	"rev-dep-go/internal/sourceedit"
)

func TestShouldConfigRunExitNonZero(t *testing.T) {
	t.Run("returns false with fix when all issues are fixable", func(t *testing.T) {
		result := &config.ConfigProcessingResult{
			HasFailures: true,
			RuleResults: []config.RuleResult{
				{
					OrphanFiles:            []string{"src/orphan.ts"},
					OrphanFilesAutofixable: []string{"src/orphan.ts"},
					ImportConventionViolations: []checks.ImportConventionViolation{
						{
							ViolationType: "should-be-relative",
							Fix: &sourceedit.Change{
								Start: 0,
								End:   1,
								Text:  ".",
							},
						},
					},
					UnusedExports: []checks.UnusedExport{
						{
							ExportName: "unusedThing",
							Fix: &sourceedit.Change{
								Start: 0,
								End:   1,
								Text:  "",
							},
						},
					},
				},
			},
		}

		if shouldConfigRunExitNonZero(result, true) {
			t.Fatal("expected zero exit when --fix covers all found issues")
		}
	})

	t.Run("returns true with fix when any issue is unfixable", func(t *testing.T) {
		result := &config.ConfigProcessingResult{
			HasFailures: true,
			RuleResults: []config.RuleResult{
				{
					ImportConventionViolations: []checks.ImportConventionViolation{
						{
							ViolationType: "should-be-relative",
							Fix: &sourceedit.Change{
								Start: 0,
								End:   1,
								Text:  ".",
							},
						},
					},
					CircularDependencies: [][]string{{"a.ts", "b.ts", "a.ts"}},
				},
			},
		}

		if !shouldConfigRunExitNonZero(result, true) {
			t.Fatal("expected non-zero exit when --fix leaves unfixable issues")
		}
	})

	t.Run("without fix mirrors failure state", func(t *testing.T) {
		if shouldConfigRunExitNonZero(&config.ConfigProcessingResult{HasFailures: false}, false) {
			t.Fatal("expected zero exit when no failures were found")
		}
		if !shouldConfigRunExitNonZero(&config.ConfigProcessingResult{HasFailures: true}, false) {
			t.Fatal("expected non-zero exit when failures were found")
		}
	})
}
