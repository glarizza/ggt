// Package chopro classifies raw tab-chart lines and recognizes chord
// tokens vs. lyric text vs. section headers.
//
// It was originally three separate packages (parser, placer, chart) that
// have been merged into a single package for the ggt chopro subcommand.
package chopro

import "regexp"

var (
	sectionRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*$`)

	// chordTokenRe: root note A-G, optional accidental, optional quality
	// suffix, optional slash bass note, optional trailing "*" (voicing
	// footnote), optionally wrapped in outer parens.
	chordTokenRe = regexp.MustCompile(`^\(?[A-G][#b]?[A-Za-z0-9#b+\-]*(?:/[A-G][#b]?)?\*?\)?$`)

	// annotationTokenRe: tokens allowed on a chord line without being
	// chords themselves — repeat markers, bar dividers, no-chord markers.
	annotationTokenRe = regexp.MustCompile(`(?i)^(\|+|x\d+|\(x\d+\)|N\.C\.|\(N\.C\.\)|%)$`)

	// tokenRe: used to split a line into whitespace-delimited tokens.
	tokenRe = regexp.MustCompile(`\S+`)
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

// IsChordLine reports whether every token on line is a chord or an
// allowed annotation, and there's at least one token.
func IsChordLine(line string) bool {
	tokens := tokenRe.FindAllString(line, -1)
	if len(tokens) == 0 {
		return false
	}
	for _, tok := range tokens {
		if !IsChordToken(tok) && !IsAnnotationToken(tok) {
			return false
		}
	}
	return true
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

// ClassifyLine determines what kind of chart line a single line is.
func ClassifyLine(line string) ClassifiedLine {
	if isBlank(line) {
		return ClassifiedLine{Blank, line}
	}
	if sectionRe.MatchString(line) {
		return ClassifiedLine{Section, line}
	}
	if IsChordLine(line) {
		return ClassifiedLine{Chord, line}
	}
	return ClassifiedLine{Lyric, line}
}
