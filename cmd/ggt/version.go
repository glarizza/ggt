// The `ggt version` subcommand reports the binary's full, auditable identity.
//
// It prints the stamp-able fields from internal/version plus the go runtime
// and platform, so a bare binary answers "what version, what commit, was this
// a release or a local build?" without the source tree.
//
// The one-line report is served separately by -v / --version; see
// newVersionFlag in the package that registers it.

package main

import (
	"fmt"
	"runtime"

	cobra "github.com/spf13/cobra"

	"ggt/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Report the binary's version, commit, and build info",
		Long: "Print the binary's full identity: the semantic version, the " +
			"commit hash it was built from, the build time, the go runtime, and " +
			"the platform —  enough to distinguish an official release from a " +
			"local build.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for _, l := range version.Lines(
				runtime.Version(), runtime.GOOS, runtime.GOARCH) {
				fmt.Fprintln(out, l)
			}
			return nil
		},
	}
}
