// Command ggt is Gary's Guitar Tool — a multi-subcommand CLI for
// working with guitar chord charts and .chopro files.
//
// ggt
//   chopro      Convert a tab chart to BandHelper .chopro format

package main

import (
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
