// Package chopro merges a chord line with the lyric line beneath it
// into a single inline-chord line (what BandHelper calls .chopro format).
//
// Placement rule (deliberately simple — no mid-word splitting):
//
// For each chord token, find the word in the lyric line that sits
// directly below the chord's FIRST character. Put the chord
// immediately before that word.
//
// If two or more chords land on the same word, stack them in
// left-to-right order: the first chord goes before the word, every
// chord after that goes after the word (and after each other), in the
// order they appeared on the chord line.
//
// A chord whose column falls in whitespace (not inside any word) is
// attached to the next word to its right. A chord past the end of the
// lyric line, or with no word to its right, attaches after the last
// word on the line.
//
// Outer parens on chord tokens are stripped before bracketing:
// (Gm) → [Gm]. Parens in tab notation mean "chord ringing from previous
// phrase" or "still holding this chord" — not semantically meaningful
// in .chopro output.
//
// "N.C." tokens are rendered as literal "(N.C.)" text rather than a
// bracketed chord, since it isn't a chord to be transposed or looked up.

package chopro

import "strings"

// token is one chord/annotation token and the column of its first
// character on the chord line.
type token struct {
	text string
	col  int
}

// word is one lyric word and its column span [start, end) on the lyric
// line.
type word struct {
	text  string
	start int
	end   int
}

func tokenizeWithColumns(line string) []token {
	var out []token
	inTok := false
	start := 0
	for i, r := range line {
		if r == ' ' || r == '\t' {
			if inTok {
				out = append(out, token{line[start:i], start})
				inTok = false
			}
			continue
		}
		if !inTok {
			start = i
			inTok = true
		}
	}
	if inTok {
		out = append(out, token{line[start:], start})
	}
	return out
}

func wordsWithColumns(line string) []word {
	toks := tokenizeWithColumns(line)
	words := make([]word, len(toks))
	for i, t := range toks {
		words[i] = word{t.text, t.col, t.col + len(t.text)}
	}
	return words
}

func renderToken(t token) string {
	upper := strings.ToUpper(t.text)
	if upper == "N.C." || upper == "(N.C.)" {
		return "(N.C.)"
	}
	if IsAnnotationToken(t.text) {
		return t.text // e.g. "x2", "|" — pass through as literal text
	}
	return "[" + stripParens(t.text) + "]"
}

// targetWordIndex returns the index into words that this chord's first
// character sits under, per the simple first-char-alignment rule.
// ok is false only if words is empty.
func targetWordIndex(chordCol int, words []word) (idx int, ok bool) {
	if len(words) == 0 {
		return 0, false
	}
	for i, w := range words {
		if w.start <= chordCol && chordCol < w.end {
			return i, true
			}
	}
	// in whitespace: attach to the next word to the right
	for i, w := range words {
		if w.start >= chordCol {
			return i, true
			}
	}
	// past the end of the line: attach to the last word
	return len(words) - 1, true
}

// PlaceChords merges a chord line into the lyric line beneath it.
// No transposition happens here — pass already-transposed chord text in
// if needed.
func PlaceChords(chordLine, lyricLine string) string {
	tokens := tokenizeWithColumns(chordLine)
	words := wordsWithColumns(lyricLine)

	if len(words) == 0 {
		parts := make([]string, len(tokens))
		for i, t := range tokens {
			parts[i] = renderToken(t)
		}
		return strings.Join(parts, " ")
	}

	// wordIndex -> ordered list of tokens landing on that word
	stacks := make(map[int][]token)
	for _, t := range tokens {
		idx, ok := targetWordIndex(t.col, words)
		if !ok {
			continue
		}
		stacks[idx] = append(stacks[idx], t)
	}

	var b strings.Builder
	b.WriteString(lyricLine[:words[0].start]) // leading whitespace

	for i, w := range words {
		assigned := stacks[i]
		if len(assigned) > 0 {
			b.WriteString(renderToken(assigned[0]))
			b.WriteString(w.text)
			for _, t := range assigned[1:] {
				b.WriteString(renderToken(t))
			}
		} else {
			b.WriteString(w.text)
		}
		if i < len(words)-1 {
			b.WriteString(lyricLine[w.end:words[i+1].start]) // preserve gap
		}
	}

	b.WriteString(lyricLine[words[len(words)-1].end:]) // trailing whitespace
	return b.String()
}
