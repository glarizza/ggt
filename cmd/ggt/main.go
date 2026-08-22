// Command ggt is Gary's Guitar Tool — a multi-subcommand CLI for
// working with guitar chord charts and .cpro files.
//
// ggt
//   cpro      Convert a tab chart to BandHelper .cpro format

package main

import (
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
