// rootCmd is the top-level `ggt` command and its subcommand registration.
//
// Calling `ggt` with no subcommand prints the help listing (Cobra default
// when no Run is set on the root and a subcommand is required via
// SilenceUsage: false). No stdin fallback — this was the UX hang bug
// in the original single-purpose binary.

package main

import (
	"fmt"
	"os"

	cobra "github.com/spf13/cobra"

	"ggt/internal/version"
)

// rootCmd is the top-level `ggt` command.
// No Run set → Cobra prints subcommand help and exits 1 when called alone.
var rootCmd = &cobra.Command{
	Use:   "ggt",
	Short: "Gary's Guitar Tool",
	Long: "ggt — Gary's Guitar Tool\n\n" +
		"A multi-subcommand CLI for working with guitar chord charts and\n" +
		"BandHelper .cpro files.\n\n" +
		"Available subcommands:\n" +
		"  cpro    Convert a tab chart to .cpro format\n" +
		"  ug      Process UG Pro tab data (fetch, clean, analyse)\n" +
		"  version Print the version and exit\n",
	SilenceUsage:  false,
	SilenceErrors: false,
}

// showShort drives the -v / --short one-line report.
var showShort bool

// exitFn is the process-exit hook, overridable in tests so the
// -v / --short reporting can be asserted without a real os.Exit.
var exitFn = os.Exit

func init() {
	// -v / --short: our own one-line report; also inherited, handled before
	// any subcommand runs.
	rootCmd.PersistentFlags().BoolVarP(
		&showShort, "version", "v", false, "print the one-line version and exit")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if showShort {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, version.Short())
			exitFn(0)
		}
		return nil
	}

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}

	rootCmd.AddCommand(newTransposeCmd())
	rootCmd.AddCommand(newCProCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newUGCmd())
}
