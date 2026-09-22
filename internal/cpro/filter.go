// Package cpro — filter and header-emission layer.
//
// This file provides the "pre-processing" layer that sits between
// parser (classifies lines) and chart (orchestrates the full walk).
// It handles:
//   - stripParens:     remove outer parens from chord tokens
//   - isDropLine:      detect lines that should be dropped entirely
//   - maybeSectionBreak: insert a blank line when a section boundary is hit
//   - HeaderOpts/emitHeader/capoBodyLine: BandHelper {key: value} header
//
// import "ggt/internal/cpro" — package cpro

package cpro

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// xLineRe matches a line that is exactly "X" (the standard tab-chart
	// end-of-song marker), optionally surrounded by whitespace.
	xLineRe = regexp.MustCompile(`^\s*X\s*$`)

	// instrRe matches a standalone "(Instrumental)" line.
	instrRe = regexp.MustCompile(`^\s*\(Instrumental\)\s*$`)
)

// isDropLine returns true if a lyric-class line should be dropped
// entirely (not emitted to the output).
func isDropLine(line string) bool {
	return xLineRe.MatchString(line) || instrRe.MatchString(line)
}

// stripParens removes the outermost paren pair from a chord token.
//
//	"(Gm)"   → "Gm"
//	"Gm"     → "Gm"   (unchanged — no outer parens)
//	"((G))"  → "(G)"  (only one level stripped)
//
// Safe because standard chord names never contain meaningful parens.
// Only the OUTERMOST pair is stripped, so "(D/B)" → "D/B" but "(A((B)))"
// → "(A((B)))" only has one level removed.
func stripParens(tok string) string {
	if len(tok) >= 2 && tok[0] == '(' && tok[len(tok)-1] == ')' {
		return tok[1 : len(tok)-1]
	}
	return tok
}

// maybeSectionBreak appends a "" (blank line) to out if the last
// element is not already blank and out is not empty.
func maybeSectionBreak(out *[]string) {
	if len(*out) == 0 {
		return // no leading blank
	}
	if (*out)[len(*out)-1] != "" {
		*out = append(*out, "")
	}
}

// capo in the header is omitted when RemoveCapo is set (the chart is the
// native-key, no-capo version). RemoveCapo also drives the body de-capo
// shift (wired in cmd/ggt), not the header itself.
// HeaderOpts holds optional fields for the {key: value} .chopro header.
type HeaderOpts struct {
	Title      string
	Artist     string
	Key        string
	Capo       int
	RemoveCapo bool
	Tempo      int
	Time       string
	Duration   string
}

// emitHeader produces the {key: value} header block.
// Only non-empty fields are emitted, in fixed order. Returns "" if no
// fields are set.
func emitHeader(o HeaderOpts) string {
	var lines []string
	if o.Title != "" {
		lines = append(lines, fmt.Sprintf("{title: %s}", o.Title))
	}
	if o.Artist != "" {
		lines = append(lines, fmt.Sprintf("{artist: %s}", o.Artist))
	}
	if o.Key != "" {
		lines = append(lines, fmt.Sprintf("{key: %s}", o.Key))
	}
	if o.Capo > 0 && !o.RemoveCapo {
		lines = append(lines, fmt.Sprintf("{capo: %d}", o.Capo))
	}
	if o.Tempo > 0 {
		lines = append(lines, fmt.Sprintf("{tempo: %d}", o.Tempo))
	}
	if o.Time != "" {
		lines = append(lines, fmt.Sprintf("{time: %s}", o.Time))
	}
	if o.Duration != "" {
		lines = append(lines, fmt.Sprintf("{duration: %s}", o.Duration))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// (the human-facing (Capo N) body line has been removed entirely; if you
// want a reminder that a personal capo transpose is needed, add it back by
// hand in BandHelper at import time.)

// wrapStandaloneChordLine brackets every real chord on a chord line that has
// no lyric beneath it, preserving the original inter-chord spacing. Bare
// connectors are skipped, and skippable chord symbols are LEFT ALONE (rendered
// verbatim), matching PlaceChords — a hard-to-parse chord on an otherwise-clean
// row is preserved for a human to fix, not dropped.
func wrapStandaloneChordLine(line string) string {
	tokens := tokenizeWithColumns(line)
	if len(tokens) == 0 {
		return line
	}
	var b strings.Builder
	prev := 0
	for _, t := range tokens {
		// Preserve the original gap from the previous emitted position.
		if t.col > prev {
			b.WriteString(line[prev:t.col])
		}
		// Skip bare connectors (they carry no chord); skippable symbols fall
		// through and are rendered verbatim, left alone for a human to fix.
		if isConnector(t.text) {
			prev = t.col + len(t.text)
			continue
		}
		b.WriteString(renderToken(t))
		prev = t.col + len(t.text)
	}
	if prev < len(line) {
		b.WriteString(line[prev:])
	}
	return b.String()
}
