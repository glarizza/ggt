// rootCmd is the top-level `ggt` command and its subcommand registration.
//
// Calling `ggt` with no subcommand prints the help listing (Cobra default
// when no Run is set on the root and a subcommand is required via
// SilenceUsage: false). No stdin fallback — this was the UX hang bug
// in the original single-purpose binary.

package main

import (
	"github.com/spf13/cobra"
)

// rootCmd is the top-level `ggt` command.
// No Run set → Cobra prints subcommand help and exits 1 when called alone.
var rootCmd = &cobra.Command{
	Use:   "ggt",
	Short: "Gary's Guitar Tool",
	Long: "ggt — Gary's Guitar Tool\n\n" +
			"A multi-subcommand CLI for working with guitar chord charts and\n" +
			"BandHelper .chopro files.\n\n" +
			"Available subcommands:\n" +
			"  chopro   Convert a tab chart to .chopro format",
	SilenceUsage:  false,
	SilenceErrors: false,
}

func init() {
	rootCmd.AddCommand(newChoproCmd())
}
