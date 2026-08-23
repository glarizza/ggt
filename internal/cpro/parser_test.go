package cpro

import "testing"

func TestChordTokens(t *testing.T) {
	shouldMatch := []string{"G", "Am7", "Cadd9", "D/F#", "Bbm7", "A7sus4", "E7sus4add13", "(D/B)", "G*"}
	for _, tok := range shouldMatch {
		if !IsChordToken(tok) {
			t.Errorf("expected %q to be a chord token", tok)
		}
	}

	shouldNotMatch := []string{"Hello", "the"}
	for _, tok := range shouldNotMatch {
		if IsChordToken(tok) {
			t.Errorf("expected %q to NOT be a chord token", tok)
		}
	}
}

func TestChordLineDetection(t *testing.T) {
	chordLines := []string{
		"G   Am7   Fmaj7   G6",
		"G D/F#  Em7  A7sus4  x2",
		"| Am     | F      | Am     | F      |",
		"N.C.",
	}
	for _, line := range chordLines {
		if !IsChordLine(line) {
			t.Errorf("expected %q to be classified as a chord line", line)
		}
	}

	notChordLines := []string{
		"this is a plain lyric line",
		"",
	}
	for _, line := range notChordLines {
		if IsChordLine(line) {
			t.Errorf("expected %q to NOT be a chord line", line)
		}
	}
}

func TestClassifyLineKinds(t *testing.T) {
	cases := []struct {
		line string
		want LineKind
	}{
		{"[Verse 1]", Section},
		{"    [Chorus]   ", Section},
		{"G   Am   F   C", Chord},
		{"some placeholder lyric text", Lyric},
		{"", Blank},
		{"    ", Blank},
	}
	for _, c := range cases {
		got := ClassifyLine(c.line).Kind
		if got != c.want {
			t.Errorf("ClassifyLine(%q).Kind = %v, want %v", c.line, got, c.want)
		}
	}
}

// TestSkippableSymbolClassification locks in the "skip the un-parseable chord
// symbol, keep the rest" behaviour: wrapped symbols like (F# - F) are
// tolerated on a chord row, while clean chords and bare chords are not
// mis-skipped.
func TestSkippableSymbolClassification(t *testing.T) {
	shouldSkip := []string{"(F# - F)", "(Em + C)", "(C/B  G)", "[F - G]"}
	for _, tok := range shouldSkip {
		if !isSkippable(tok) {
			t.Errorf("expected %q to be skippable", tok)
		}
		if IsChordToken(tok) {
			t.Errorf("expected %q NOT to parse as a clean chord", tok)
		}
	}

	shouldNotSkip := []string{"(Gm)", "(C)", "(Cadd9)", "G", "F#m", "F#m7", "D/F#"}
	for _, tok := range shouldNotSkip {
		if isSkippable(tok) {
			t.Errorf("expected %q NOT to be skippable (clean chord)", tok)
		}
	}

	// A row of clean chords plus one un-parseable symbol is still a chord row.
	if !IsChordLine("F                   G                              (F# - F)") {
		t.Errorf("expected a chord row with one un-parseable symbol to be a chord line")
	}
	// Prose is still not a chord row.
	if IsChordLine("this is a plain lyric line") {
		t.Errorf("prose should not be a chord line")
	}
	// A bare chord with a connector between (no parens) is tolerated.
	if !IsChordLine("F - G") {
		t.Errorf("bare chord row with a connector should be a chord line")
	}
}

func TestTokenizeGroupsParens(t *testing.T) {
	toks := tokenizeWithColumns("F      G    (F# - F)")
	got := make([]string, len(toks))
	for i, t := range toks {
		got[i] = t.text
	}
	want := []string{"F", "G", "(F# - F)"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}
