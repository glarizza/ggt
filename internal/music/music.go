// Package music provides the single missing piece in the ggt pipeline:
// native-key transposition of a chord chart that has already been run
// through ggt cpro.
//
// The chromatic (semitone) shift and the accidental spelling are
// delegated to github.com/brettbuddin/musictheory.  What this package
// owns is the thin glue no general library provides: a compact
// chord-SYMBOL transposition that parses "Dm7/F#" into root + quality
// + bass, preserves the quality through verbatim, shifts both halves
// of a slash chord, and reassembles the output the way BandHelper
// expects — "[Bm7/D#]".
//
// It also transposes the {key: …} header of a cpro output and
// leaves section headers, walk-downs (F# - F), and annotation tokens
// (x2, N.C.) completely untouched to be handled by a human after the
// fact.
//
// ggt transpose is deliberately a pure mechanical shift: it performs
// the semitone math and never decides whether a key is "correct."
// That judgment belongs to the AI orchestrating the pipeline, not to
// the tool.

package music

import (
	"regexp"
	"strings"

	"ggt/internal/sections"

	mt "github.com/brettbuddin/musictheory"
)

// Style selects the accidental spelling of transposed notes.
type Style int

const (
	// StyleAuto inherits from the input symbol's own spelling: a chord
	// that arrives with a flat (Db, Bb) shifts in flat land; a chord
	// that arrives with a sharp (F#, C#) shifts in sharp land; a
	// chord with no accidental (C, E) shifts in sharp land, which
	// matches the common "F# is always F#" convention.
	StyleAuto Style = iota

	// StyleSharps forces sharps (brett's AscNames).
	StyleSharps

	// StyleFlats forces flats (brett's DescNames).
	StyleFlats
)

// diatonicFor maps a natural note letter (A–G) to brett's 1-indexed
// diatonic position.
var diatonicFor = map[byte]int{
	'C': 1, 'D': 2, 'E': 3, 'F': 4, 'G': 5, 'A': 6, 'B': 7,
}

// rootRe captures a leading root note letter and any following
// accidentals (# or b, possibly multiple).
var rootRe = regexp.MustCompile(`^([A-Ga-g])([#b]*)`)

// bracketRe finds cpro's inline bracketed chord tokens.
var bracketRe = regexp.MustCompile(`\[([^\]]+)\]`)

// Section/annotation words that must NEVER be transposed (Bridge, Chorus,
// Break, Guitar, ...) live in the shared sections package. The transpose layer
// consults sections.IsWord (defense-in-depth behind the upstream cpro drop,
// Option C) so that a bracket holding a static section word is left verbatim
// rather than shifted into a garbage string like "Eridge". The set is a single,
// shared, auditable source of truth rather than a per-package copy.
// keyLineRe matches the leading {key: …} header of a cpro output and
// captures the value.
var keyLineRe = regexp.MustCompile(`^(\{key:\s*)([^}]*?)(\s*\})$`)

// parseChord splits a chord symbol such as "Dm7/F#" into its root,
// quality, and (optional) bass, reporting whether the symbol is in
// fact a single chord.
//
//	Walkdowns ("F# - F") and anything with a space return ok=false so
//	callers can leave them for human post-review.
//
//	A "/" in a quality (e.g. "C6/9") is NOT a slash chord — the
//	half after it is not a lone note — so the whole thing stays in
//	quality.  A real slash bass is the second half of a Dm7/F#-style
//	split where the right side is a lone note.
func parseChord(sym string) (root, quality, bass string, ok bool) {
	s := strings.TrimSpace(sym)
	if s == "" || strings.Contains(s, " ") {
		return
	}
	m := rootRe.FindStringSubmatch(s)
	if m == nil {
		return
	}
	root = m[1] + m[2]
	rest := s[len(root):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		cand := rest[i+1:]
		// A genuine slash bass is a lone note; "6/9", "7/9", "13/11"
		// etc. all have quality digits on the right.
		if bm := rootRe.FindStringSubmatch(cand); bm != nil && len(bm[1]+bm[2]) == len(cand) {
			quality = rest[:i]
			bass = cand
		} else {
			quality = rest
		}
	} else {
		quality = rest
	}
	return root, quality, bass, true
}

// shiftNote transposes a single note name (e.g. "F#", "Bb", "C") by
// semis, returning it in the brett spelling strategy dictated by
// style, with no octave.
func shiftNote(name string, semis int, style Style, rootForAuto string) string {
	d, mod, ok := noteClass(name)
	if !ok {
		return name
	}
	p := mt.NewPitch(d, mod, 0)
	p = p.Transpose(mt.Semitones(semis))
	out := p.Name(stratFor(style, rootForAuto))
	// brett renders scientific-pitch notation (e.g. "C#0"); no chord
	// name ends in a digit, so strip trailing octave digits.
	return strings.TrimRight(out, "0123456789")
}

// stratFor picks the brett ModifierStrategy for a Style.
func stratFor(style Style, rootForAuto string) mt.ModifierStrategy {
	switch style {
	case StyleSharps:
		return mt.AscNames
	case StyleFlats:
		return mt.DescNames
	default: // StyleAuto
		if strings.IndexByte(rootForAuto, 'b') >= 0 {
			return mt.DescNames
		}
		return mt.AscNames
	}
}

// noteClass converts a note name to brett's (diatonic, modifier) pair.
func noteClass(name string) (diatonic, modifier int, ok bool) {
	if name == "" {
		return 0, 0, false
	}
	d, ok := diatonicFor[up(name[0])]
	if !ok {
		return 0, 0, false
	}
	mod := 0
	for _, ch := range name[1:] {
		switch ch {
		case '#':
			mod++
		case 'b':
			mod--
		}
	}
	return d, mod, true
}

// up lowercases a note letter to uppercase for the diatonic lookup.
func up(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// TransposeSymbol shifts a single chord symbol by semis semitones,
// preserving its quality and both halves of any slash chord.  It
// returns the symbol unchanged and false when the symbol is not a
// single chord (walkdowns, x2, N.C., ...).
//
//	Dm7     -> C#m (down 1)
//	Dm7b5   -> Gbm7b5  (quality verbatim)
//	D/F#    -> C# / E#  (both halves)
//	Cadd9   -> Badd9
//	x2      -> x2       (unchanged, not a chord)
func TransposeSymbol(sym string, semis int, style Style) (string, bool) {
	// A static section/annotation word (Bridge, Chorus, Break, Guitar, ...) starts
	// on a pitch letter but is NOT a chord: it must be left verbatim, never
	// transposed into "Eridge". This is the single chokepoint, so the file-level
	// body/text transposers inherit the guard by calling through here.
	if sections.IsWord(sym) {
		return sym, false
	}
	root, quality, bass, ok := parseChord(sym)
	if !ok {
		return sym, false
	}
	out := shiftNote(root, semis, style, root) + quality
	if bass != "" {
		out += "/" + shiftNote(bass, semis, style, bass)
	}
	return out, true
}

// TransposeCProText transposes every bracketed chord token in a
// cpro output by semis semitones, rewrites the leading {key: V}
// header to the transposed tonic, and leaves everything else
// (section headers, walk-downs, annotation tokens, {capo: N}, …)
// exactly as it is.
func TransposeCProText(text string, semis int, style Style) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if m := keyLineRe.FindStringSubmatch(line); m != nil {
			trimmed := strings.TrimSpace(m[2])
			if t, ok := TransposeSymbol(trimmed, semis, style); ok {
				lines[i] = m[1] + t + m[3]
			}
			// A {key:} whose value is not a bare note (e.g. "E
			// major") is left unchanged.
			continue
		}
		lines[i] = bracketRe.ReplaceAllStringFunc(line, func(bracket string) string {
			inner := bracket[1 : len(bracket)-1]
			if t, ok := TransposeSymbol(inner, semis, style); ok {
				return "[" + t + "]"
			}
			return bracket
		})
	}
	return strings.Join(lines, "\n")
}

// TransposeCProBody is de-capo / body-only transposition. It is the same
// bracket shift as TransposeCProText but it IGNORES every {key:value}
// metadata line (key, capo, title, ...), leaving the header exactly as it
// was. The caller pre-fills {key: <sounding key>}; this shifts the body
// chord shapes up N semitones so they match that key — the native-key,
// no-capo representation. Walk-downs and annotation tokens are left to be
// fixed by hand afterwards, matching TransposeCProText's philosophy.
func TransposeCProBody(text string, semis int, style Style) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			continue // metadata block — leave key/capo/title/etc. untouched
		}
		lines[i] = bracketRe.ReplaceAllStringFunc(line, func(bracket string) string {
			inner := bracket[1 : len(bracket)-1]
			if t, ok := TransposeSymbol(inner, semis, style); ok {
				return "[" + t + "]"
			}
			return bracket
		})
	}
	return strings.Join(lines, "\n")
}

// naturalChromatic maps each natural note letter to its semitone position,
// C=0, with the black keys filling the gaps.  A diatonic index does not
// give a pitch class (it skips the black keys), which is why the old
// diatonic-minus-one was wrong.
var naturalChromatic = map[byte]int{
	'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11,
}

// pitchClassOf returns the chromatic pitch class (0=C, 11=B) of a bare
// note name, folding in accidentals (# = +1, b = -1, repeatable for
// double sharps / flats).  Pure arithmetic, independent of the library's
// internal pitch model -- this is the reference SemitonesTo uses.
func pitchClassOf(name string) (int, bool) {
	if name == "" {
		return 0, false
	}
	base, ok := naturalChromatic[up(name[0])]
	if !ok {
		return 0, false
	}
	mod := 0
	for _, c := range name[1:] {
		switch c {
		case '#':
			mod++
		case 'b':
			mod--
		}
	}
	return (((base + mod) % 12) + 12) % 12, true
}

// KeyOf returns the value of the leading {key: V} header, or "" if
// there isn't one.
func KeyOf(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if m := keyLineRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[2])
		}
	}
	return ""
}

// SemitonesTo returns how many semitones to shift a source key so that
// its tonic becomes the target key.  The result is in the range 1–12
// (a "move of 12" means no movement); a source or target that is not a
// bare note name returns false.
func SemitonesTo(from, to string) (int, bool) {
	pcFrom, ok1 := pitchClassOf(from)
	pcTo, ok2 := pitchClassOf(to)
	if !ok1 || !ok2 {
		return 0, false
	}
	return (pcTo - pcFrom + 12) % 12, true
}
