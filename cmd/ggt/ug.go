// ug subcommand: fetch UG official Pro tabs, clean the UG-lyric markup,
// and produce a clean text file plus a fact sheet for use with ggt cpro.

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

// ug flags are declared in newUGCmd and shared between the top-level
// `ggt ug` command (file mode) and the `ggt ug fetch` subcommand
// (URL mode).
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
//   ggt ug <meta.json>        clean a previously saved UG pro_meta JSON
func newUGCmd() *cobra.Command {
	flags := &ugFlagSet{}

	cmd := &cobra.Command{
		Use:   "ug",
		Short: "Process UG Pro tab data: fetch, clean, and analyse",
		Long:  "ggt ug fetches a UG official Pro tab, cleans the UG pro-reader\n" +
						"markup, and produces a clean text file for ggt cpro.\n\n" +
						"Modes:\n" +
						"  ggt ug fetch <url>     fetch from UG API and clean\n" +
						"  ggt ug <meta.json>     clean a saved UG pro_meta JSON\n",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUGFromFile(cmd, args, flags)
		},
	}

	flags.register(cmd)
	cmd.AddCommand(newUGFetchCmd(flags))
	return cmd
}

// newUGFetchCmd handles `ggt ug fetch <url>`.
func newUGFetchCmd(shared *ugFlagSet) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch <tab-url>",
		Short: "Fetch a UG tab from its URL and produce a clean text output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
				// 1. Fetch the UG pro_meta JSON.
			meta, err := ug.FetchUGMeta(args[0])
			if err != nil {
				return err
				}

				// 2. Fill defaults from meta where the user didn't supply them.
			applyMetaDefaults(shared, meta)

				// 3. Clean the lyrics.
			clean, err := ug.Clean(meta.Lyrics)
			if err != nil {
				return fmt.Errorf("ug fetch: %w", err)
				}

				// 4. Optional info to stderr.
			maybePrintFacts(cmd, meta, clean, shared)
			maybePrintChords(cmd, clean, shared)

				// 5. Output: if --key is set, produce .chopro; else print clean text.
			if shared.key != "" {
				result := cproConvert(clean, shared)
				writeOutput(cmd, result, shared.output)
				} else {
				if shared.output != "" {
						if err := os.WriteFile(shared.output, []byte(clean), 0o644); err != nil {
							return fmt.Errorf("writing %q: %w", shared.output, err)
						}
				} else {
					if _, err := cmd.OutOrStdout().Write([]byte(clean)); err != nil {
						return fmt.Errorf("writing output: %w", err)
						}
				}
			}
			return nil
		},
	}
	// Register flags on the fetch subcommand too (shares the same struct fields).
	shared.register(cmd)
	return cmd
}

// --- core helpers ---

// applyMetaDefaults fills in any unset flag fields from the UG meta data.
func applyMetaDefaults(f *ugFlagSet, meta *ug.UGMeta) {
	if f.title == "" {
		f.title = strings.TrimSpace(meta.Name)
	}
	if f.artist == "" {
		f.artist = strings.TrimSpace(meta.Artist)
	}
	if f.capo == 0 && meta.Meta.Capo > 0 {
		f.capo = meta.Meta.Capo
	}
	if f.tempo == 0 {
		f.tempo = meta.Tempo
	}
	if f.duration == "" {
		f.duration = meta.DurationString()
	}
}

// runUGFromFile handles `ggt ug <meta.json>` — process a saved JSON file.
func runUGFromFile(cmd *cobra.Command, args []string, f *ugFlagSet) error {
	facts,        _  := cmd.Flags().GetBool("facts")
	chordsFlag,   _  := cmd.Flags().GetBool("chords")
	removeCapo,   _  := cmd.Flags().GetBool("remove-capo")

	// Fill in output, key, tempo, time, duration, title, artist from flags
	output,    _ := cmd.Flags().GetString("output")
	key,       _ := cmd.Flags().GetString("key")
	tempo,     _ := cmd.Flags().GetInt("tempo")
	timeSig,   _ := cmd.Flags().GetString("time")
	duration,  _ := cmd.Flags().GetString("duration")
	title,     _ := cmd.Flags().GetString("title")
	artist,    _ := cmd.Flags().GetString("artist")
	capo,      _ := cmd.Flags().GetInt("capo")

	meta, err := loadSavedMeta(args[0])
	if err != nil {
		return fmt.Errorf("ug: %w", err)
	}

	// Overwritten by applyMetaDefaults but only if the flag wasn't explicitly set.
	// For file mode, just use what we got and fill in from meta.
	if title == "" {
		title = strings.TrimSpace(meta.Name)
	}
	if artist == "" {
		artist = strings.TrimSpace(meta.Artist)
	}
	if capo == 0 && meta.Meta.Capo > 0 {
		capo = meta.Meta.Capo
	}
	if tempo == 0 {
		tempo = meta.Tempo
	}
	if duration == "" {
		duration = meta.DurationString()
	}

	clean, err := ug.Clean(meta.Lyrics)
	if err != nil {
		return fmt.Errorf("ug: %w", err)
	}

	if facts {
		printFactSheet(cmd, meta, clean, title, artist,
			key, capo, tempo, timeSig, duration, removeCapo)
	}
	if chordsFlag {
		printChords(cmd, clean)
	}

	if key != "" {
		result := cproConvertDirect(clean, title, artist, key,
			capo, removeCapo, tempo, timeSig, duration)
		if output != "" {
			if err := os.WriteFile(output, []byte(result), 0o644); err != nil {
				return fmt.Errorf("writing %q: %w", output, err)
					}
		} else {
			if _, err := cmd.OutOrStdout().Write([]byte(result)); err != nil {
				return fmt.Errorf("writing output: %w", err)
					}
		}
	} else {
		if output != "" {
			if err := os.WriteFile(output, []byte(clean), 0o644); err != nil {
				return fmt.Errorf("writing %q: %w", output, err)
					}
		} else {
			if _, err := cmd.OutOrStdout().Write([]byte(clean)); err != nil {
				return fmt.Errorf("writing output: %w", err)
					}
		}
	}
	return nil
}

// --- helper functions ---

// loadSavedMeta parses a saved JSON file that may be in one of two formats.
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

	// Fall back to trail format: {"meta": "<inner JSON string>"}
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

// cproConvert runs ggt's cpro conversion on the clean text.
func cproConvert(cleanText string, f *ugFlagSet) string {
	return cproConvertDirect(cleanText, f.title, f.artist, f.key,
		f.capo, f.removeCapo, f.tempo, f.timeSig, f.duration)
}

// cproConvertDirect runs ggt's cpro conversion with explicit params.
func cproConvertDirect(cleanText, title, artist, key string,
	capo int, removeCapo bool, tempo int, timeSig, duration string) string {
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
	result := cpro.Convert(cleanText, opts)
	if removeCapo {
		result = music.TransposeCProBody(result, capo, music.StyleAuto)
	}
	return result
}

func maybePrintFacts(cmd *cobra.Command, meta *ug.UGMeta, cleanText string, f *ugFlagSet) {
	if f.facts {
		printFactSheet(cmd, meta, cleanText, f.title, f.artist,
			f.key, f.capo, f.tempo, f.timeSig, f.duration, f.removeCapo)
		}
}

func maybePrintChords(cmd *cobra.Command, cleanText string, f *ugFlagSet) {
	if f.chords {
		printChords(cmd, cleanText)
		}
}

func writeOutput(cmd *cobra.Command, data string, output string) {
	if output != "" {
		if err := os.WriteFile(output, []byte(data), 0o644); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "writing %q: %v\n", output, err)
			return
			}
		return
	}
	if _, err := cmd.OutOrStdout().Write([]byte(data)); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "writing output: %v\n", err)
	}
}

// printFactSheet writes a metadata summary to cmd's stderr.
func printFactSheet(cmd *cobra.Command,
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
		fmt.Fprintf(w, "Strumming BPM:       (no data)\n")
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

// printChords writes the chord frequency report to cmd's stderr.
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
