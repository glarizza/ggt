// Package cpro classifies raw tab-chart lines and recognizes chord
// tokens vs. lyric text vs. section headers.
//
// It was originally three separate packages (parser, placer, chart) that
// have been merged into a single package for the ggt cpro subcommand.
package cpro

import (
	"regexp"
	"strings"

	"ggt/internal/sections"
)

var (
	// sectionRe matches a bare section header: a line that is just a [tag]
	// bracket. Unchanged from before -- any bracket alone is a section and gets
	// dropped with a section break.
	sectionRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*$`)

	// sectionCueRe matches a section header that ALSO carries a trailing
	// parenthetical cue -- e.g. "[Bridge] (all played as bar chords)", which UG
	// authors annotate on the line. Such a header is still a section and must be
	// dropped as such, BUT this shape also matches a real chord row such as
	// "[Dm7] (x3)" -- a chord plus a repeat/voicing annotation -- which MUST NOT
	// be dropped. A pure regex cannot tell "Bridge" from "Dm7", so the decision is
	// gated on isSectionWord (below): a cued header is a section when its bracket
	// content is a known section word (shared via the sections package) OR is not a
	// real chord at all (handles numbered heads like "[Verse 1] (quiet)"). The full
	// grammar-based version of this distinction is the deferred chord-quality work
	// (Option B).
	sectionCueRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*\(.+\)\s*$`)

	// bracketInnerRe captures the inner content of a leading [..] bracket, used to
	// read the section word out of a cued header for isSectionWord.
	bracketInnerRe = regexp.MustCompile(`^\s*\[([^\]]*)\]`)

	// chordTokenRe: root note A-G, optional accidental, optional quality
	// suffix, optional slash bass note, optional trailing "*" (voicing
	// footnote), optionally wrapped in outer parens.
	chordTokenRe = regexp.MustCompile(`^\(?[A-G][#b]?[A-Za-z0-9#b+\-]*(?:/[A-G][#b]?)?\*?\)?$`)

	// annotationTokenRe: tokens allowed on a chord line without being
	// chords themselves — repeat markers, bar dividers, no-chord markers.
	annotationTokenRe = regexp.MustCompile(`(?i)^(\|+|x\d+|\(x\d+\)|N\.C\.|\(N\.C\.\)|%)$`)

	// chordishRunRe: a run of only chordish characters — root notes,
	// accidentals, quality letters, digits, and the separators we tolerate
	// (including inner spaces). Tries to tell a hard-to-parse chord like
	// "F# - F" apart from a prose word.
	chordishRunRe = regexp.MustCompile(`^[A-Za-z0-9#b@+*/.,:\- ]+$`)
	// chordishSepRe: a chord separator — the thing that makes "F# - F"
	// un-parseable as one clean chord.
	chordishSepRe = regexp.MustCompile(`[+\-=*/:]`)
)

// IsChordToken reports whether tok looks like a chord symbol.
func IsChordToken(tok string) bool {
	return chordTokenRe.MatchString(tok)
}

// IsAnnotationToken reports whether tok is an allowed non-chord token
// that may still appear on a chord line (x2, |, N.C., ...).
func IsAnnotationToken(tok string) bool {
	return annotationTokenRe.MatchString(tok)
}

// Token kinds, ordered from "definitely a chord" to "a lyric word."
const (
	tokChord      int = iota // a clean chord or slash chord
	tokAnnotation            // x2, |, N.C., ...
	tokSkippable             // a hard-to-parse chord we skip, e.g. "(F# - F)"
	tokConnector             // bare noise, e.g. "-", "+", "."
	tokProse                 // a lyric word; disqualifies a chord line
)

// isConnector reports whether tok is pure chord-line noise: a run of only
// separator/connector characters that never stands alone as a chord, like the
// "-" between two chords, a "+", or a lone ".".
func isConnector(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		switch r {
		case '-', '+', '=', '*', '.', ',', ':', '|':
		default:
			return false
		}
	}
	return true
}

// stripOuter peels one pair of outer () or [] from tok. ok is false when
// there is no matched outer pair.
func stripOuter(tok string) (string, bool) {
	tok = strings.TrimSpace(tok)
	if len(tok) < 2 {
		return tok, false
	}
	switch tok[0] {
	case '(':
		if tok[len(tok)-1] == ')' {
			return tok[1 : len(tok)-1], true
		}
	case '[':
		if tok[len(tok)-1] == ']' {
			return tok[1 : len(tok)-1], true
		}
	}
	return tok, false
}

// isSkippable reports whether tok is a parenthesized or bracketed chord
// notation we do not model — a "difficult symbol" like "(F# - F)" or
// "(Em + C)". It is wrapped in one () / [] pair; the interior starts on a
// pitch note (A-G), contains the separator that makes it un-parseable, and is
// otherwise chordish (no prose letters). Prose words like "together" never
// qualify even when wrapped in parens. These are the symbols we LEAVE ALONE
// for the human (emit verbatim) rather than auto-model or drop — the design
// intent is to convert, not to add or subtract musical meaning we cannot parse.
func isSkippable(tok string) bool {
	inner, ok := stripOuter(tok)
	if !ok {
		return false
	}
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return false
	}
	// First non-space character must be a pitch note (A-G); reject anything
	// else, then fall through to the separator + chordish checks.
	found := false
	for _, r := range inner {
		if r == ' ' {
			continue
		}
		if r < 'A' || r > 'G' {
			return false
		}
		found = true
		break
	}
	if !found {
		return false
	}
	// Needs a separator — that's what makes it un-parseable as one chord.
	if !chordishSepRe.MatchString(inner) {
		return false
	}
	// Every character must be chordish, not a prose letter.
	return chordishRunRe.MatchString(inner)
}

// kindOf classifies a single token into a token-kind.
func kindOf(tok string) int {
	if IsChordToken(tok) {
		return tokChord
	}
	if IsAnnotationToken(tok) {
		return tokAnnotation
	}
	if isSkippable(tok) {
		return tokSkippable
	}
	if isConnector(tok) {
		return tokConnector
	}
	return tokProse
}

// IsChordLine reports whether line is a chord row. A line is a chord row if it
// (tokenised with paren/bracket grouping) has at least one real chord,
// annotation, or skippable symbol AND no prose word. Skippable symbols and
// bare connectors are tolerated, so a single un-parseable chord like "(F# - F)"
// on a row of otherwise-clean chords no longer reclassifies the whole row as a
// lyric line.
func IsChordLine(line string) bool {
	tokens := tokenizeWithColumns(line)
	seenSymbol := false
	for _, t := range tokens {
		switch kindOf(t.text) {
		case tokProse:
			return false
		case tokConnector:
			// tolerated noise, but a lone connector is not a "symbol"
		case tokChord, tokAnnotation, tokSkippable:
			seenSymbol = true
		}
	}
	return seenSymbol
}

// LineKind categorizes a single line of a tab chart.
type LineKind int

const (
	Blank LineKind = iota
	Section
	Chord
	Lyric
)

// ClassifiedLine pairs a LineKind with the original line text.
type ClassifiedLine struct {
	Kind LineKind
	Text string
}

func isBlank(line string) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

// isSectionWord reports whether a bracket's inner content is a section/annotation
// word, rather than a real chord. A cued section header is dropped as a section
// only when the bracket holds such a word, which keeps a real chord row like
// "[Dm7] (x3)" intact and lets arbitrary section heads like "[Verse 1] (quiet)"
// be recognized even though they are not in a closed list.
//
// Two cases make something a section word:
//   - it is NOT a parseable chord at all (e.g. "Verse 1", "Intro 2", "Quiet"), or
//   - it is a known static section word that merely LOOKS like a chord
//     (e.g. "Bridge", "Chorus", "Break") -- drawn from the shared sections
//     package so cpro and the transpose layer never drift apart.
//
// A chord like "Dm7" or "G" is parseable and not a known section word, so it is
// not a section. A chord-quality grammar (deferred Option B) would make the
// decision fully rigorous; until then the shared sections.IsWord set is the
// single extension point.
func isSectionWord(inner string) bool {
	t := strings.TrimSpace(inner)
	if sections.IsWord(t) {
		return true
	}
	return !IsChordToken(t)
}

// ClassifyLine determines what kind of chart line a single line is.
func ClassifyLine(line string) ClassifiedLine {
	if isBlank(line) {
		return ClassifiedLine{Blank, line}
	}
	if sectionRe.MatchString(line) {
		return ClassifiedLine{Section, line}
	}
	// A section header may also carry a trailing parenthetical cue -- e.g.
	// "[Bridge] (all played as bar chords)", "[Verse 1] (quiet)", "[Intro 2]
	// (fingering)". Drop it as a section, but ONLY when the bracket holds a
	// section word, so a real chord row such as "[Dm7] (x3)" keeps its chord
	// meaning instead of being dropped.
	if sectionCueRe.MatchString(line) {
		if m := bracketInnerRe.FindStringSubmatch(line); m != nil && isSectionWord(m[1]) {
			return ClassifiedLine{Section, line}
		}
	}
	if IsChordLine(line) {
		return ClassifiedLine{Chord, line}
	}
	return ClassifiedLine{Lyric, line}
}
