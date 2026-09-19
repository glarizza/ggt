// ggt transpose — native-key transposition of a .chopro file that
// has already been run through ggt cpro.
//
// It is a pure mechanical chromatic shift of the bracket chords and
// a rewrite of the {key: V} header.  It never decides whether a key
// is "correct"; that judgment is the AI orchestrator's job, made
// against a published-key lookup and not this tool's.
//
// Input model: the .chopro file cpro emits is already bracketed
// ([C] lyric) and width-independent, so transposing it never shifts
// columns — that is why this step runs AFTER cpro.
//
// Three input forms, exactly one required:
//
//	--up N / --down N   shift every chord N semitones up / down
//	--to-key K         shift so the tonic lands on key K
//	                   (delta computed from the current {key:} header)
//
// The 80% case is  --down N  when the tab is capo N-shapes: the
// published (native) key is N semitones below the shape you see.
// There is no --capo flag: reading the capo out of a tab and deciding
// "down N" is an intellectual task for the AI, not a parameter this
// mechanical tool should decode.
//
// ggt transpose INPUT  --down N
// ggt transpose INPUT  --to-key C:
// ggt cpro --key E --capo 5 -o out.chopro IN.txt
// ggt transpose out.chopro --to-key C:   (native-key version)

package main

import (
	"fmt"
	"io"
	"os"

	"ggt/internal/music"
	"github.com/spf13/cobra"
)

func newTransposeCmd() *cobra.Command {
	var (
		up      int
		down    int
		toKey   string
		accStd  bool   // --sharps
		accFlt  bool   // --flats
		output  string // --output
		inplace bool   // --inplace
		input   string // --input
	)
	cmd := &cobra.Command{
		Use:   "transpose [INPUT]",
		Short: "Transpose a .chopro file to another key",
		Long: "transpose reads a .chopro file (cpro's output) and shifts " +
			"every bracket chord by a number of semitones, rewriting the " +
			"{key: V} header to the new tonic.  Section headers, walk-downs " +
			"(F# - F), annotations (x2, N.C.), {capo: N} and all other " +
			"header fields are left exactly as they are.\n\n" +
			"Exactly ONE of --up N, --down N, or --to-key K must be given:\n" +
			"  --up N / --down N    shift every chord N semitones\n" +
			"  --to-key K           transpose to land on key K\n\n" +
			"INPUT is a file path, or - to read from stdin.",
	}

	// Use the default flag set for the actual parse, because
	// Cobra does not support -i / positional-with-defaults
	// cleanly alongside its own string-int flags.
	cmd.Flags().IntVar(&up, "up", 0, "shift up N semitones")
	cmd.Flags().IntVar(&down, "down", 0, "shift down N semitones")
	cmd.Flags().StringVarP(&toKey, "to-key", "k", "", "transpose to land on key K")
	cmd.Flags().BoolVar(&accStd, "sharps", false, "spell accidentals with sharps")
	cmd.Flags().BoolVar(&accFlt, "flats", false, "spell accidentals with flats")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	cmd.Flags().BoolVar(&inplace, "inplace", false, "rewrite the source file in place")
	cmd.Flags().StringVarP(&input, "input", "i", "", "input file (alternative to the positional arg, or '-' for stdin)")

	// Exactly one of the three move modes must be present, and they
	// cannot co-exist.
	cmd.MarkFlagsMutuallyExclusive("up", "down", "to-key")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Exactly one of the three move modes may be set.  MarkFlagsMutuallyExclusive
		// already rejects >1; here we also enforce "at least one".
		nMove := 0
		if cmd.Flags().Changed("up") {
			nMove++
		}
		if cmd.Flags().Changed("down") {
			nMove++
		}
		if cmd.Flags().Changed("to-key") {
			nMove++
		}
		if nMove != 1 {
			return fmt.Errorf("exactly one of --up N, --down N, --to-key K must be set (got %d)", nMove)
		}

		// Accidental style.  Default auto (inherit input); --sharps/--flats
		// are mutually exclusive.
		if accStd && accFlt {
			return fmt.Errorf("--sharps and --flats are mutually exclusive")
		}
		style := music.StyleAuto
		switch {
		case accStd:
			style = music.StyleSharps
		case accFlt:
			style = music.StyleFlats
		}

		// Resolve how many semitones to shift.
		var semis int
		switch {
		case cmd.Flags().Changed("up"):
			semis = up
		case cmd.Flags().Changed("down"):
			semis = -down
			// --to-key: resolved below once we have read the header.
		}

		// Read the input: positional arg, --input flag, or - / stdin.
		src := input
		if src == "" && len(args) > 0 {
			src = args[0]
		}
		var raw []byte
		var err error
		if src == "" || src == "-" {
			raw, err = io.ReadAll(cmd.InOrStdin())
		} else {
			raw, err = os.ReadFile(src)
		}
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		text := string(raw)

		// Resolve the semitone count for --to-key mode.
		if cmd.Flags().Changed("to-key") {
			cur := music.KeyOf(text)
			if cur == "" {
				return fmt.Errorf("--to-key %q requires a {key: ...} header in the input", toKey)
			}
			d, ok := music.SemitonesTo(cur, toKey)
			if !ok {
				return fmt.Errorf("could not map %q to %q as a chromatic distance", cur, toKey)
			}
			semis = d
		}

		result := music.TransposeCProText(text, semis, style)

		switch {
		case inplace:
			if err := os.WriteFile(src, []byte(result), 0o644); err != nil {
				return fmt.Errorf("writing in place: %w", err)
			}
		case output != "":
			if err := os.WriteFile(output, []byte(result), 0o644); err != nil {
				return fmt.Errorf("writing %q: %w", output, err)
			}
		}
		if !inplace && output == "" {
			if _, err := cmd.OutOrStdout().Write([]byte(result)); err != nil {
				return fmt.Errorf("writing to stdout: %w", err)
			}
		}
		return nil
	}
	return cmd

}
