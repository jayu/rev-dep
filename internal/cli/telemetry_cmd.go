package cli

import (
	"os"

	"github.com/spf13/cobra"

	"rev-dep-go/internal/telemetry"
)

var telemetryCheckOnly bool

// telemetryReporterCmd is the hidden subcommand the detached reporter process runs. It reads an
// anonymous telemetry payload from stdin and sends it. It is intentionally undocumented and hidden
// from `--help`; end users never invoke it directly.
//
// With --check it instead verifies that a telemetry connection string was baked into this build,
// exiting non-zero if not. The production build uses this to guarantee releases are not shipped with
// telemetry silently disabled.
var telemetryReporterCmd = &cobra.Command{
	Use:           "__telemetry",
	Hidden:        true,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if telemetryCheckOnly {
			if !telemetry.Configured() {
				os.Exit(1)
			}
			return
		}
		telemetry.RunReporter()
	},
}

func init() {
	telemetryReporterCmd.Flags().BoolVar(&telemetryCheckOnly, "check", false,
		"Exit non-zero unless a telemetry connection string is baked into this build")
	rootCmd.AddCommand(telemetryReporterCmd)
}
