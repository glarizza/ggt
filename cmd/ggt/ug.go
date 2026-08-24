// ug subcommand: fetch and clean UG tabs. The ChordPro conversion
// (ggt cpro) is a separate command — ggt ug fetch is intentionally a
// pure clean step so that cpro stays decoupled and composable.
//
// Two data sources:
//   PRO tab   (URL has -official-):  pro_meta API.
//    User tab  (URL has -chords-):   HTML extraction.
//
// Two modes:
//   ggt ug fetch <url>       fetch from UG, clean, output clean text
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
func newUGCmd() *cobra.Command {
	flags := &ugFlagSet{}

	cmd := &cobra.Command{
		Use:   "ug",
		Short: "Process UG tab data: fetch, clean, and (optionally) cpro",
		Long:  "ggt ug fetches a UG tab (official Pro OR user-submitted),\n" +
			"cleans the UG pro-reader markup, and produces a clean text\n" +
			"file ready for ggt cpro.\n\n" +
			"Two data sources, one unified output:\n" +
			"  PRO tab   (URL -official-): pro_meta API, full metadata\n" +
			"  user tab  (URL -chords-):  HTML extraction, capo/tuning only\n\n" +
			"Two modes:\n" +
			"  ggt ug fetch <url>     fetch + clean (no cpro — pipe to cpro next)\n" +
			"  ggt ug \u003cmeta.json\u003e     clean a saved pro_meta JSON (+ optional cpro)\n",
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
//  ggt ug fetch   —  pure clean step (no cpro flags)
// --------------------------------------------------------------------

// ugFetchFlags — only --output, --facts, --chords.
// cpro metadata flags are deliberately absent — they belong to ggt cpro.
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
		Long:  "Fetches a UG tab — official Pro or user-submitted — and\ncleans the UG markup, writing a clean chart to stdout or a file.\n\n" +
			"Source auto-detected from URL:\n" +
			"  ...-official-NNNN   → pro_meta API (full metadata)\n" +
			"  ...-chords-NNNN      → HTML extraction (capo/tuning only)\n\n" +
			"For ChordPro conversion, pipe to ggt cpro:\n\n" +
			"  ggt ug fetch \"\u003cURL\u003e\" -o out.txt\n" +
			"  ggt cpro out.txt --key D --tempo 81 -o out.63686f70726f\n",
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			f.output, _   = cmd.Flags().GetString("output")
			f.facts, _    = cmd.Flags().GetBool("facts")
			f.chords, _   = cmd.Flags().GetBool("chords")
			return nil
			},
		RunE: func(cmd *cobra.Command, args []string) error {
				// 1. Fetch the UG tab (PRO or user, auto-detected).
			data, err := ug.FetchTabByURL(args[0])
			if err != nil {
				return err
				}

				// 2. Clean the UG pro-reader markup.
			clean, err := ug.Clean(data.Content)
			if err != nil {
				return fmt.Errorf("ug fetch: clean step: %w", err)
				}

				// 3. Optional diagnostic output to stderr.
			if f.facts {
				printFactSheetFetch(cmd, data, clean)
				}
			if f.chords {
				printChords(cmd, clean)
				}

				// 4. Write the clean text.
			return writeCleanOutput(cmd, clean, f.output)
			},
	}
	f.register(cmd)
	return cmd
}

// --------------------------------------------------------------------
//  ggt ug <file>   —  full-file mode with cpro pipeline
// --------------------------------------------------------------------

// runUGFromFile loads a saved pro_meta JSON file, converts to *TabData,
// and runs optional cpro conversion.
func runUGFromFile(cmd *cobra.Command, args []string, f *ugFlagSet) error {
	factsFlag,  _ := cmd.Flags().GetBool("facts")
	chordsFlag, _ := cmd.Flags().GetBool("chords")
	removeCapo, _ := cmd.Flags().GetBool("remove-capo")
	output,     _ := cmd.Flags().GetString("output")
	key,        _ := cmd.Flags().GetString("key")
	tempo,      _ := cmd.Flags().GetInt("tempo")
	timeSig,    _ := cmd.Flags().GetString("time")
	duration,   _ := cmd.Flags().GetString("duration")
	title,      _ := cmd.Flags().GetString("title")
	artist,     _ := cmd.Flags().GetString("artist")
	capo,       _ := cmd.Flags().GetInt("capo")

	// Load saved pro_meta JSON.
	meta, err := loadSavedMeta(args[0])
	if err != nil {
		return fmt.Errorf("ug: %w", err)
	}

	// Convert to *TabData — unified with the fetch path.
	tabID, _ := ug.ExtractTabID("https://tabs.ultimate-guitar.com/tab/unknown/unknown-official-0")
	_ = tabID
	data := ug.UGMetaToTabData(meta, 0)

	// Fill in from meta where user didn't supply explicit values.
	if title == ""  { title  = strings.TrimSpace(data.Name) }
	if artist == "" { artist = strings.TrimSpace(data.Artist) }
	if capo == 0    && data.Capo > 0           { capo = data.Capo }
	if tempo == 0   { tempo   = data.Tempo }
	if duration == "" { duration = data.DurationOrPlaceholder() }

	clean, err := ug.Clean(data.Content)
	if err != nil {
		return fmt.Errorf("ug: clean failed: %w", err)
	}

	if factsFlag {
		printFactSheetCpro(cmd, data, clean,
			title, artist, key, capo, tempo, timeSig, duration, removeCapo)
	}
	if chordsFlag {
		printChords(cmd, clean)
	}

	// If --key is set, run the cpro pipeline.
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
// (2) wrapped {"meta": "\u003cjson\u003e"} (what raw curl dumps produce).
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

// writeRawOutput writes a cpro result or the error to the output destination.
func writeRawOutput(cmd *cobra.Command, data, output string) error {
	if output != "" {
		return os.WriteFile(output, []byte(data), 0o644)
	}
	_, err := cmd.OutOrStdout().Write([]byte(data))
	return err
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
//  Fact sheets
//
//  printFactSheetFetch:  used by `ggt ug fetch` (pure clean, no cpro intent).
//    Shows what UG says. Flags user-tab limitations.
//
//  printFactSheetCpro:   used by `ggt ug \u003cfile\u003e` (full pipeline).
//    Shows both UG data and user-supplied cpro flag values.
// --------------------------------------------------------------------

// printFactSheetFetch shows UG-sourced metadata for `ggt ug fetch`.
// Works for both PRO and user-tab sources.
func printFactSheetFetch(cmd *cobra.Command,
	data *ug.TabData, cleanText string) {
	w := cmd.ErrOrStderr()

	title  := strings.TrimSpace(data.Name)
	artist := strings.TrimSpace(data.Artist)

	fmt.Fprintf(w, "Song:                 %s\n", title)
	fmt.Fprintf(w, "Artist:               %s\n", artist)
fmt.Fprintf(w, "Tab ID:               %d\n", data.TabID)
	fmt.Fprintf(w, "Source:               %s\n", data.Source)

	if data.Source == "user-tab-html" {
		if data.Username != "" {
			fmt.Fprintf(w, "UG user:              %s\n", data.Username)
		}
		fmt.Fprintf(w, "Votes:                %d\n", data.Votes)
		if data.Difficulty != "" {
			fmt.Fprintf(w, "Difficulty:           %s\n", data.Difficulty)
		}
	}

	fmt.Fprintf(w, "Tuning:               %s\n", data.Tuning)
	fmt.Fprintf(w, "Capo (UG data):       %d\n", data.Capo)

	tc := ug.TonalCandidate(cleanText)
	fmt.Fprintf(w, "Tonal candidate:      %s\n", tc)

	// Tempo — only available for PRO tabs.
	if data.Source == "pro_meta" {
		fmt.Fprintf(w, "UG tempo:             %d bpm  (UG default; cross-check)\n", data.Tempo)
		if data.StrumBPM > 0 {
			fmt.Fprintf(w, "Strumming BPM:        %d bpm  (closer to real tempo)\n", data.StrumBPM)
		}
	} else {
		fmt.Fprintf(w, "Tempo:                n/a  (user-tab HTML — cross-check externally)\n")
	}
	fmt.Fprintf(w, "Duration:             %s\n", data.DurationOrPlaceholder())

	if len(data.Tracks) > 0 {
		fmt.Fprintf(w, "Tracks:\n")
		for _, t := range data.Tracks {
			fmt.Fprintf(w, "  id=%-3d %-15s %-20s present=%v\n",
				t.ID, t.Kind, t.Name, t.Present)
		}
	}
}

// printFactSheetCpro shows UG data plus cpro flag values for the
// `ggt ug <file>` pipeline.
func printFactSheetCpro(cmd *cobra.Command,
	data *ug.TabData, cleanText string,
	title, artist, key string,
	capo, tempo int,
	timeSig, duration string,
	removeCapo bool,
) {
	w := cmd.ErrOrStderr()

	fmt.Fprintf(w, "Song:                 %s\n", title)
	fmt.Fprintf(w, "Artist:               %s\n", artist)
	fmt.Fprintf(w, "Source:               %s\n", data.Source)
	fmt.Fprintf(w, "Tuning:               %s\n", data.Tuning)
	fmt.Fprintf(w, "Capo:                 %d\n", capo)
	if removeCapo {
		fmt.Fprintf(w, "Capo mode:            --remove-capo applied\n")
	}

	tc := ug.TonalCandidate(cleanText)
	fmt.Fprintf(w, "Tonal candidate:      %s\n", tc)

	if key != "" {
		fmt.Fprintf(w, "Key (user):           %s\n", key)
	}

	if data.Source == "pro_meta" {
		fmt.Fprintf(w, "UG tempo:             %d bpm\n", data.Tempo)
		if data.StrumBPM > 0 {
			fmt.Fprintf(w, "Strumming BPM:        %d bpm\n", data.StrumBPM)
		}
	} else {
		fmt.Fprintf(w, "Tempo:                %d bpm (user-supplied; unverified)\n", tempo)
	}
	fmt.Fprintf(w, "Duration:             %s\n", duration)
	if timeSig != "" {
		fmt.Fprintf(w, "Time:                 %s\n", timeSig)
	}

	if len(data.Tracks) > 0 {
		fmt.Fprintf(w, "Tracks:\n")
		for _, t := range data.Tracks {
			fmt.Fprintf(w, "  id=%-3d %-15s %-20s present=%v\n",
				t.ID, t.Kind, t.Name, t.Present)
		}
	}
}

// printChords writes the chord frequency top-10 to cmd's stderr.
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
