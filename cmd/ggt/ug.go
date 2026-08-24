// ug subcommand: fetch and clean UG Pro tabs. The ChordPro conversion
// (ggt cpro) is a separate command — ggt ug fetch is intentionally a pure
// clean step so that cpro stays decoupled and composable.
//
// Two modes:
//   ggt ug fetch <url>       fetch from UG API, clean, output clean text
//   ggt ug <meta.json>       process a saved UG pro_meta JSON file
//
// ggt ug fetch only exposes --output, --facts, and --chords (to stderr).
// The cpro metadata (--key, --tempo, --capo, etc.) belongs to ggt cpro.
// This separation is deliberate: "run one more command is not that hard."

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ggt/internal/cpro"
	"ggt/internal/music"
	"ggt/internal/ug"

	"github.com/spf13/cobra"
)

// --------------------------------------------------------------------
// ggt ug <file>  —  full-file mode with cpro pipeline
//
// This mode takes a saved UG pro_meta JSON file and optionally runs
// ggt cpro on the cleaned content. All cpro flags are exposed here.
// --------------------------------------------------------------------

type ugFlagSet struct {
	output     string
	facts      bool
	chords     bool
	removeCapo bool
	capo       int
	key        string
	tempo      int
	timeSig    string
	duration   string
	title      string
	artist     string
}

func (f *ugFlagSet) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "write result to file instead of stdout")
	cmd.Flags().BoolVarP(&f.facts, "facts", "f", false, "print fact sheet to stderr")
	cmd.Flags().BoolVarP(&f.chords, "chords", "c", false, "print chord frequency to stderr")
	cmd.Flags().StringVar(&f.title, "title", "", "title (default: from UG)")
	cmd.Flags().StringVar(&f.artist, "artist", "", "artist (default: from UG)")
	cmd.Flags().StringVar(&f.key, "key", "", "key for cpro conversion; triggers full cpro mode")
	cmd.Flags().IntVar(&f.capo, "capo", 0, "capo fret number")
	cmd.Flags().BoolVar(&f.removeCapo, "remove-capo", false, "de-cap body; requires --capo N")
	cmd.Flags().IntVar(&f.tempo, "tempo", 0, "tempo in BPM (default: from UG)")
	cmd.Flags().StringVar(&f.timeSig, "time", "", "time signature")
	cmd.Flags().StringVar(&f.duration, "duration", "", "duration mm:ss (default: from UG)")
}

// newUGCmd returns the top-level `ggt ug` command.
//
// Usage:
//   ggt ug fetch <tab-url>    fetch from UG API and clean
//   ggt ug <meta.json>        clean a previously saved UG pro_meta JSON (+ optional cpro)
func newUGCmd() *cobra.Command {
	flags := &ugFlagSet{}

	cmd := &cobra.Command{
		Use:   "ug",
		Short: "Process UG Pro tab data: fetch, clean, and (optionally) cpro",
		Long:  "ggt ug fetches a UG official Pro tab, cleans the UG pro-reader\n" +
		"markup, and produces a clean text file ready for ggt cpro.\n\n" +
		"Two modes:\n" +
		"  ggt ug fetch <url>     fetch + clean (no cpro — use ggt cpro next)\n" +
		"  ggt ug <meta.json>     clean a saved pro_meta JSON (+ optional cpro)\n",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUGFromFile(cmd, args, flags)
		},
	}

	flags.register(cmd)
	cmd.AddCommand(newUGFetchCmd())
	return cmd
}

// --------------------------------------------------------------------
//  ggt ug fetch  —  pure clean command (no cpro flags)
//
// The fetch subcommand exposes only --output, --facts, and --chords.
// No --key, --tempo, --capo, --remove-capo, --title, --artist flags —
// those belong to ggt cpro. This keeps the two commands decoupled and
// their responsibilities unambiguous.
// --------------------------------------------------------------------

// ugFetchFlags holds the pure-clean flag set for `ggt ug fetch`.
// No cpro fields — only the clean step.
type ugFetchFlags struct {
	output string
	facts  bool
	chords bool
}

func (f *ugFetchFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "write clean text to file instead of stdout")
	cmd.Flags().BoolVarP(&f.facts, "facts", "f", false, "print fact sheet to stderr")
	cmd.Flags().BoolVarP(&f.chords, "chords", "c", false, "print chord frequency to stderr")
}

func newUGFetchCmd() *cobra.Command {
	f := &ugFetchFlags{}

	cmd := &cobra.Command{
		Use:   "fetch <tab-url>",
		Short: "Fetch a UG tab from its URL and output cleaned text",
		Long:  "Fetches a UG Pro tab (official or user-submitted), cleans the UG\n" +
		"pro-reader markup, and writes the clean chart to stdout or to a file.\n\n" +
		"This is a pure clean step. For ChordPro conversion, pipe the output\n" +
		"to ggt cpro:\n\n" +
		"  ggt ug fetch '<url>' -o out.txt\n" +
		"  ggt cpro out.txt --key D --tempo 81 -o out.chopro\n",
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			f.output, _  = cmd.Flags().GetString("output")
			f.facts, _   = cmd.Flags().GetBool("facts")
			f.chords, _  = cmd.Flags().GetBool("chords")
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Fetch the UG tab via pro_meta API (with version-ID fallback
			//    for user tabs that don't respond to the tab URL ID).
			meta, err := ug.FetchTabByURL(args[0])
			if err != nil {
				return err
			}

			// 2. Clean the UG pro-reader markup.
			clean, err := ug.Clean(meta.Lyrics)
			if err != nil {
				return fmt.Errorf("ug fetch: clean step: %w", err)
			}

			// 3. Optional diagnostic output to stderr.
			if f.facts {
				printFactSheetFromMeta(cmd, meta, clean)
			}
			if f.chords {
				printChords(cmd, clean)
			}

			// 4. Write the clean text to a file or stdout.
			return writeCleanOutput(cmd, clean, f.output)
		},
	}
	f.register(cmd)
	return cmd
}

// --------------------------------------------------------------------
//  ggt ug <file>  —  full file mode with cpro pipeline
// --------------------------------------------------------------------

func runUGFromFile(cmd *cobra.Command, args []string, f *ugFlagSet) error {
	factsFlag,    _ := cmd.Flags().GetBool("facts")
	chordsFlag,   _ := cmd.Flags().GetBool("chords")
	removeCapo,   _ := cmd.Flags().GetBool("remove-capo")
	output,       _ := cmd.Flags().GetString("output")
	key,          _ := cmd.Flags().GetString("key")
	tempo,        _ := cmd.Flags().GetInt("tempo")
	timeSig,      _ := cmd.Flags().GetString("time")
	duration,     _ := cmd.Flags().GetString("duration")
	title,        _ := cmd.Flags().GetString("title")
	artist,       _ := cmd.Flags().GetString("artist")
	capo,         _ := cmd.Flags().GetInt("capo")

	meta, err := loadSavedMeta(args[0])
	if err != nil {
		return fmt.Errorf("ug: %w", err)
	}

	// Fill in from meta where the user didn't supply explicit values.
	if title == ""  { title  = strings.TrimSpace(meta.Name)  }
	if artist == "" { artist = strings.TrimSpace(meta.Artist) }
	if capo == 0    && meta.Meta.Capo > 0 { capo = meta.Meta.Capo }
	if tempo == 0   { tempo  = meta.Tempo }
	if duration == "" { duration = meta.DurationString() }

	clean, err := ug.Clean(meta.Lyrics)
	if err != nil {
		return fmt.Errorf("ug: clean failed: %w", err)
	}

	if factsFlag {
		printFactSheetWithCpro(cmd, meta, clean,
			title, artist, key, capo, tempo, timeSig, duration, removeCapo)
	}
	if chordsFlag {
		printChords(cmd, clean)
	}

	// If --key is set, run the cpro pipeline; otherwise output clean text.
	if key != "" {
		result := cproConvertDirect(clean, title, artist, key,
			capo, removeCapo, tempo, timeSig, duration)
		return writeRawOutput(cmd, result, output)
	}
	return writeCleanOutput(cmd, clean, output)
}

// --------------------------------------------------------------------
//  Shared helpers
// --------------------------------------------------------------------

// loadSavedMeta parses a saved JSON file that may be in one of two formats:
// (1) flat UGMeta object (what FetchUGMeta writes to disk), or
// (2) wrapped {"meta": "<json>"} (what raw curl dumps produce).
func loadSavedMeta(path string) (*ug.UGMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	// Try flat first.
	var flat ug.UGMeta
	if err := json.Unmarshal(raw, &flat); err == nil && flat.Name != "" {
		return &flat, nil
	}
	// Fall back to wrapped format.
	var wrapped struct {
		Meta string `json:"meta"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Meta != "" {
		var meta ug.UGMeta
		if err := json.Unmarshal([]byte(wrapped.Meta), &meta); err != nil {
			return nil, fmt.Errorf("parsing inner meta string: %w", err)
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("unrecognised JSON format in %s", path)
}

// writeCleanOutput writes clean text or the error to cmd's output destination.
func writeCleanOutput(cmd *cobra.Command, clean, output string) error {
	if output != "" {
		return os.WriteFile(output, []byte(clean), 0o644)
	}
	_, err := cmd.OutOrStdout().Write([]byte(clean))
	return err
}

// writeRawOutput writes a CPro result or the error to the output destination.
func writeRawOutput(cmd *cobra.Command, data, output string) error {
	if output != "" {
		return os.WriteFile(output, []byte(data), 0o644)
	}
	_, err := cmd.OutOrStdout().Write([]byte(data))
	return err
}

// cproConvert runs the UG clean text through ggt cpro with metadata.
func cproConvert(cleanText string, f *ugFlagSet) string {
	return cproConvertDirect(cleanText, f.title, f.artist, f.key,
		f.capo, f.removeCapo, f.tempo, f.timeSig, f.duration)
}

// cproConvertDirect runs ggt's cpro pipeline with explicit parameters.
func cproConvertDirect(cleanText, title, artist, key string,
	capo int, removeCapo bool, tempo int, timeSig, duration string) string {
	opts := cpro.HeaderOpts{
		Title:      title,
		Artist:     artist,
		Key:        key,
		Capo:        capo,
		RemoveCapo: removeCapo,
		Tempo:       tempo,
		Time:        timeSig,
		Duration:    duration,
	}
	result := cpro.Convert(cleanText, opts)
	if removeCapo {
		result = music.TransposeCProBody(result, capo, music.StyleAuto)
	}
	return result
}

// --------------------------------------------------------------------
//  Fact sheet: two variants
//
//  printFactSheetFromMeta:  used by `ggt ug fetch`
//    reads all fields from the meta struct — no user-supplied cpro flags.
//    Shows what UG says (title, artist, capo, tempo, duration from API).
//
//  printFactSheetWithCpro:  used by `ggt ug <file>`
//    Takes explicit cpro flag values as well, showing what will land
//    in the chopro header.
// --------------------------------------------------------------------

// printFactSheetFromMeta is the minimal fact sheet for `ggt ug fetch`.
// It reads all displayed values from the UG meta struct directly —
// no user-supplied cpro flags are involved.
func printFactSheetFromMeta(cmd *cobra.Command, meta *ug.UGMeta, cleanText string) {
	w := cmd.ErrOrStderr()

	title  := strings.TrimSpace(meta.Name)
	artist := strings.TrimSpace(meta.Artist)
	capo   := meta.Meta.Capo
	duration := meta.DurationString()

	fmt.Fprintf(w, "Song:                %s\n", title)
	fmt.Fprintf(w, "Artist:              %s\n", artist)
	fmt.Fprintf(w, "Tuning:              %s\n", meta.Meta.Tuning)
	fmt.Fprintf(w, "Capo (UG data):      %d\n", capo)

	tc := ug.TonalCandidate(cleanText)
	fmt.Fprintf(w, "Tonal candidate:     %s\n", tc)

	strumBPM := 0
	for _, p := range meta.StrummingPatterns {
		if p.BPM > strumBPM {
			strumBPM = p.BPM
		}
	}
	fmt.Fprintf(w, "UG tempo:            %d bpm  (UG default; cross-check before trusting)\n", meta.Tempo)
	if strumBPM > 0 {
		fmt.Fprintf(w, "Strumming BPM:       %d bpm  ← closer to real tempo\n", strumBPM)
	} else {
		fmt.Fprintf(w, "Strumming BPM:       (no strumming data in this tab)\n")
	}
	fmt.Fprintf(w, "Duration (UG):       %s\n", duration)

	if len(meta.Tracks) > 0 {
		fmt.Fprintf(w, "Tracks:\n")
		for _, t := range meta.Tracks {
			fmt.Fprintf(w, "  id=%-3d %-15s %-20s present=%v\n",
				t.ID, t.Kind, t.Name, t.Present)
		}
	}
}

// printFactSheetWithCpro shows all metadata including cpro flag values
// for the full pipeline (`ggt ug <file>` mode).
func printFactSheetWithCpro(cmd *cobra.Command,
	meta *ug.UGMeta, cleanText string,
	title, artist, key string,
	capo, tempo int,
	timeSig, duration string,
	removeCapo bool,
) {
	w := cmd.ErrOrStderr()

	fmt.Fprintf(w, "Song:                %s\n", title)
	fmt.Fprintf(w, "Artist:              %s\n", artist)
	fmt.Fprintf(w, "Tuning:              %s\n", meta.Meta.Tuning)
	fmt.Fprintf(w, "Capo:                %d\n", capo)
	if removeCapo {
		fmt.Fprintf(w, "Capo mode:           --remove-capo applied\n")
	}

	tc := ug.TonalCandidate(cleanText)
	fmt.Fprintf(w, "Tonal candidate:     %s\n", tc)

	if key != "" {
		fmt.Fprintf(w, "Key (user):          %s\n", key)
	}

	strumBPM := 0
	for _, p := range meta.StrummingPatterns {
		if p.BPM > strumBPM {
			strumBPM = p.BPM
		}
	}
	fmt.Fprintf(w, "UG tempo:            %d bpm\n", meta.Tempo)
	if strumBPM > 0 {
		fmt.Fprintf(w, "Strumming BPM:       %d bpm\n", strumBPM)
	} else {
		fmt.Fprintf(w, "Strumming BPM:       (no strumming data)\n")
	}
	fmt.Fprintf(w, "Duration:            %s\n", duration)
	if timeSig != "" {
		fmt.Fprintf(w, "Time:                %s\n", timeSig)
	}

	if len(meta.Tracks) > 0 {
		fmt.Fprintf(w, "Tracks:\n")
		for _, t := range meta.Tracks {
			fmt.Fprintf(w, "  id=%-3d %-15s %-20s present=%v\n",
				t.ID, t.Kind, t.Name, t.Present)
		}
	}
}

// printChords writes the chord frequency top-10 to cmd's stderr.
// Used by both fetch and file modes.
func printChords(cmd *cobra.Command, cleanText string) {
	w := cmd.ErrOrStderr()
	freq := ug.AnalyzeChords(cleanText)
	fmt.Fprintf(w, "Chord frequency:\n")
	limit := 10
	if len(freq) < limit {
		limit = len(freq)
	}
	for _, cc := range freq[:limit] {
		fmt.Fprintf(w, "  %-14s x%d\n", cc.Chord, cc.Count)
	}
	fmt.Fprintf(w, "Tonal candidate: %s\n", ug.TonalCandidate(cleanText))
}
