package ug

import (
	"regexp"
	"sort"
	"strings"
)

// ChordFrequency is a count of how many times each chord symbol appears.
// The slice is sorted by descending count.
type ChordFrequency []ChordCount

// ChordCount is a single (chord, count) pair.
type ChordCount struct {
	Chord string
	Count int
}

// AnalyzeChords extracts chord tokens from a CLEAN text file (not raw
// UG markup — must already have been through Clean()) and produces
// a frequency report.
//
// It matches bracketed chord tokens like [Am], [C], [G/Bb], [F/A] and
// bare chord tokens appearing at the start of a line.
func AnalyzeChords(cleanText string) ChordFrequency {
	freq := map[string]int{}

	// 1. Bracketed chord tokens: [Am], [C], [G/Bb], [F/A]
	// The inner group captures the chord name without the brackets.
	bareRe := regexp.MustCompile(`\[((?:[A-G][#b]?[m7]?)(?:/[A-G][#b]?[m7]?)*)\]`)
	matches := bareRe.FindAllStringSubmatch(cleanText, -1)

	for _, m := range matches {
		chord := m[1]
		if chord == "" {
			continue // empty bracket like [Intro]
		}
		if !isChordToken(chord) {
			continue // [Verse 1] etc — not a real chord
		}
		freq[chord]++
	}

	// 2. Bare chord tokens at start of line (handles "Am C D" style)
	lineRe := regexp.MustCompile(`^([A-G][#b]?[m7]?)\b`)
	for _, line := range strings.Split(cleanText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '[' {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m != nil {
			token := m[1]
			if isChordToken(token) {
				freq[token]++
			}
		}
	}

	// 3. Sort descending by count.
	result := make(ChordFrequency, 0, len(freq))
	for c, n := range freq {
		result = append(result, ChordCount{c, n})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

// isChordToken returns true if s looks like a chord name
// (Am, C, D, G/Bb, F/A, E7, Bm) and NOT a word or section marker.
func isChordToken(s string) bool {
	re := regexp.MustCompile(`^[A-G][#b]?[m7]?(?:/[A-G][#b]?[m7]?)?$`)
	return re.MatchString(s)
}

// TonalCandidate returns the most frequent chord as a starting
// "tonal centre" candidate. Cross-check protocol (SKILL.md) is needed
// to determine the actual key — this is only a flag.
func TonalCandidate(cleanText string) string {
	freq := AnalyzeChords(cleanText)
	if len(freq) == 0 {
		return "(no chord tokens found)"
	}
	return freq[0].Chord
}
