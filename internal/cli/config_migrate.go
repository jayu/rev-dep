package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/config"
	"rev-dep-go/internal/emoji"
	"rev-dep-go/internal/pathutil"
)

// migrationDocsURL is the v3 upgrade/breaking-changes guide.
const migrationDocsURL = "https://rev-dep.com/docs/upgrade-guides/v3-breaking-changes"

// legacyConfigError is returned by commands that refuse to run a pre-v3 config, pointing the
// user at `config migrate` and the upgrade guide.
func legacyConfigError(version string) error {
	v := ""
	if version != "" {
		v = " (configVersion " + version + ")"
	}
	return fmt.Errorf("this config uses the v2 schema%s, which rev-dep v3 no longer runs.\n\n"+
		"Upgrade it automatically:\n    rev-dep config migrate\n\n"+
		"Guide: %s", v, migrationDocsURL)
}

var migrateConfigCwd string

var configMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Upgrade a v2 config to the v3 (2.0) schema",
	Long: `Upgrade a (.)rev-dep.config.json(c) from the v2 schema to v3 (config version 2.0).

It applies the safe, unambiguous changes in place (renaming the top-level 'rules' array to
'workspaces', bumping 'configVersion' to 2.0, and removing the discontinued 'algorithm'
option from circular-imports detectors), preserving all comments and formatting. Review the
change with git before committing.

It then lists what it could NOT change for you: glob patterns whose match set may have
shifted under v3's stricter, gitignore-aligned rules, and behavior changes that no config
edit can address. Review those manually — see the v3 breaking-changes guide.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()
		cwd := pathutil.ResolveAbsoluteCwd(migrateConfigCwd)

		configPath, err := config.FindConfigFile(cwd)
		if err != nil {
			return fmt.Errorf("Could not find a config in %s: %v", cwd, err)
		}
		original, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("Could not read %s: %v", configPath, err)
		}

		res, err := config.MigrateConfig(original)
		if err != nil {
			return fmt.Errorf("Could not migrate %s: %v", configPath, err)
		}

		relTo := func(p string) string {
			if r, err := filepath.Rel(cwd, p); err == nil {
				return r
			}
			return p
		}
		fmt.Printf("%s Migrating %s\n", emoji.Search, relTo(configPath))

		if res.Changed {
			if err := os.WriteFile(configPath, res.Migrated, 0o644); err != nil {
				return fmt.Errorf("Could not write %s: %v", configPath, err)
			}
			fmt.Printf("\n%s  Applied automatically:\n", emoji.Fix)
			for _, c := range res.AppliedChanges {
				fmt.Printf("    - %s\n", c)
			}
		} else {
			fmt.Printf("\n%s Nothing to change automatically — the config is already on the 2.0 schema.\n", emoji.Success)
		}

		printMigrateReviews(res)
		printMigrateLintSummary(cwd)

		fmt.Printf("\n%s Full v3 upgrade guide: %s\n", emoji.File, migrationDocsURL)
		fmt.Printf("\n%s  Done in %dms.\n", emoji.Done, time.Since(startTime).Milliseconds())
		return nil
	},
}

func printMigrateReviews(res *config.MigrateResult) {
	if len(res.PatternReviews) > 0 {
		fmt.Printf("\n%s  Review these glob patterns — v3's glob bug fixes changed how they match files.\n", emoji.Warning)
		fmt.Printf("    legend:\n")
		fmt.Printf("    - matching: how this glob matches files changed in v3 — its match set may differ\n")
		fmt.Printf("    - sibling:  may have matched files in sibling workspaces, now it's matching files only in workspace where it is defined\n")
		printGroupedReviews(res.PatternReviews)
	}

	if len(res.ResultNotes) > 0 {
		fmt.Printf("\n%s Behavior changes (no config edit needed, but results may shift):\n", emoji.File)
		for _, n := range res.ResultNotes {
			fmt.Printf("    - %s\n", n)
		}
	}

	if len(res.CINotes) > 0 {
		fmt.Printf("\n%s CI / scripts (nothing to change in the config file):\n", emoji.File)
		for _, n := range res.CINotes {
			fmt.Printf("    - %s\n", n)
		}
	}
}

// printGroupedReviews prints the reviews as workspace → field → aligned rows, so a long field
// key is written once per group and the item/pattern columns line up.
func printGroupedReviews(reviews []config.PatternReview) {
	var wsOrder []string
	byWs := map[string]map[string][]config.PatternReview{}
	for _, r := range reviews {
		if byWs[r.Workspace] == nil {
			byWs[r.Workspace] = map[string][]config.PatternReview{}
			wsOrder = append(wsOrder, r.Workspace)
		}
		byWs[r.Workspace][r.Field] = append(byWs[r.Workspace][r.Field], r)
	}

	for _, ws := range wsOrder {
		label := ws
		if ws == "." || ws == "./" {
			label = ws + " (root)"
		}
		fmt.Printf("\n    %s\n", label)

		fields := byWs[ws]
		fieldNames := make([]string, 0, len(fields))
		for f := range fields {
			fieldNames = append(fieldNames, f)
		}
		sort.Strings(fieldNames) // stable order (detector map iteration is random)

		for _, field := range fieldNames {
			rows := fields[field]
			fmt.Printf("      %s\n", field)

			itemW, patW := 0, 0
			quoted := make([]string, len(rows))
			for i, r := range rows {
				quoted[i] = fmt.Sprintf("%q", shortenGlob(r.Pattern))
				itemW = max(itemW, len(r.Item))
				patW = max(patW, len(quoted[i]))
			}
			for i, r := range rows {
				fmt.Printf("        %-*s  %-*s  %s\n", itemW, r.Item, patW, quoted[i], strings.Join(r.Reasons, ", "))
			}
			fmt.Println()
		}
	}
}

// shortenGlob collapses the middle of a long, deep pattern for display, keeping the leading
// segment(s) and the filename: `**/app/a/b/c/File.tsx` -> `**/app/[..]/File.tsx`. The item
// index still locates the exact pattern in the config, so this is a readable hint only.
func shortenGlob(p string) string {
	const maxLen = 50
	if len(p) <= maxLen {
		return p
	}
	segs := strings.Split(p, "/")
	head := 1
	if segs[0] == "*" || segs[0] == "**" {
		head = 2 // a bare wildcard alone is uninformative — keep the next segment too
	}
	if len(segs) < head+3 { // need >=2 middle segments to be worth collapsing
		return p
	}
	kept := append(append([]string{}, segs[:head]...), "[..]", segs[len(segs)-1])
	return strings.Join(kept, "/")
}

// printMigrateLintSummary nudges the user to run config lint next. Non-fatal on any error —
// the migration already succeeded.
func printMigrateLintSummary(cwd string) {
	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		return
	}
	result, err := config.LintConfig(&cfg, cwd, config.AllLintRules)
	if err != nil {
		return
	}
	if errors, warnings := countLintFindings(result, false); errors > 0 || warnings > 0 {
		fmt.Printf("\n%s  Config lint found %d error(s) and %d warning(s) that could be improved — run `rev-dep config lint` for details (add `--fix` to apply).\n",
			emoji.Warning, errors, warnings)
	}
}

func init() {
	configMigrateCmd.Flags().StringVarP(&migrateConfigCwd, "cwd", "c", currentDir, "Working directory")
	configCmd.AddCommand(configMigrateCmd)
}
