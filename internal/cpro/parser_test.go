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
