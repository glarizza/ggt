// cpro subcommand: reads a tab chart file (or stdin via `-`) and
// outputs Chord Pro formatted data to stdout or a file.

package main

import (
	"fmt"
	"io"
	"os"

	"ggt/internal/cpro"
	"ggt/internal/music"

	"github.com/spf13/cobra"
)

func newCProCmd() *cobra.Command {
	// flag vars
	var (
		output     string
		title      string
		artist     string
		key        string
		capo       int
		removeCapo bool
		tempo      int
		timeSig    string
		duration   string
	)

	cmd := &cobra.Command{
		Use:   "cpro [INPUT]",
		Short: "Converts a tab chart to a ChordPro format .chopro file",
		Long: "cpro reads a plain-text guitar tab chart and reformats it\n" +
			"into a ChordPro-format .chopro file with inline chord placement.\n\n" +
			"Chords placed immediately before the word they align with.\n" +
			"Section headers ([Verse 1], [Chorus], etc.) insert a blank line\n" +
			"between sections. Standalone chord lines are bracketed in place.\n" +
			"Outer parens on chord tokens are stripped: (Gm) → [Gm].\n" +
			"End-of-song markers (X) and (Instrumental) lines are dropped.\n\n" +
			"INPUT is a file path, or '-' to read from stdin.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw []byte
			var err error
			switch args[0] {
			case "-":
				raw, err = io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			default:
				raw, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("reading %q: %w", args[0], err)
				}
			}

			if removeCapo && capo == 0 {
				return fmt.Errorf("--remove-capo requires --capo N (need a capo fret number to de-cap by)")
			}

			opts := cpro.HeaderOpts{
				Title:      title,
				Artist:     artist,
				Key:        key,
				Capo:       capo,
				RemoveCapo: removeCapo,
				Tempo:      tempo,
				Time:       timeSig,
				Duration:   duration,
			}

			result := cpro.Convert(string(raw), opts)

			// --remove-capo de-caps the body: shift chord shapes up by the
			// --capo N semitones so they match {key:} already in the header,
			// producing the native-key, no-capo chart for import. {capo: N}
			// is already suppressed by cpro when RemoveCapo is set.
			if removeCapo {
				result = music.TransposeCProBody(result, capo, music.StyleAuto)
			}

			if output != "" {
				if err := os.WriteFile(output, []byte(result), 0o644); err != nil {
					return fmt.Errorf("writing %q: %w", output, err)
				}
				return nil
			}

			if _, err := cmd.OutOrStdout().Write([]byte(result)); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	cmd.Flags().StringVar(&title, "title", "", "song title for {title:} header")
	cmd.Flags().StringVar(&artist, "artist", "", "artist for {artist:} header")
	cmd.Flags().StringVar(&key, "key", "", "song key for {key:} header")
	cmd.Flags().IntVar(&capo, "capo", 0, "capo fret N: adds a {capo: N} header line (suppressed when --remove-capo is set)")
	cmd.Flags().BoolVar(&removeCapo, "remove-capo", false, "de-cap the body: shift chord shapes up by --capo N so they match {key:} and drop {capo: N} (requires --capo)")
	cmd.Flags().IntVar(&tempo, "tempo", 0, "tempo in BPM for {tempo: N} header")
	cmd.Flags().StringVar(&timeSig, "time", "", "time signature for {time: T} header")
	cmd.Flags().StringVar(&duration, "duration", "", "duration for {duration: T} header")

	return cmd
}
