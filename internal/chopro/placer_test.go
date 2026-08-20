package chopro

import "testing"

func TestSingleChordAtWordStart(t *testing.T) {
	got := PlaceChords("G", "hello there")
	want := "[G]hello there"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
func TestChordAlignedMidWordSnapsToWordNotSplit(t *testing.T) {
	chords := "         G"
	lyric := "hello world"
	got := PlaceChords(chords, lyric)
	want := "hello [G]world"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTwoChordsTwoWords(t *testing.T) {
	chords := "G        D"
	lyric := "one      two"
	got := PlaceChords(chords, lyric)
	want := "[G]one      [D]two"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestChordInWhitespaceAttachesToNextWord(t *testing.T) {
	chords := "  G"
	lyric := "a  bb"
	got := PlaceChords(chords, lyric)
	want := "a  [G]bb"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestChordPastEndOfLyricAttachesAfterLastWord(t *testing.T) {
	chords := "one         G"
	lyric := "one"
	got := PlaceChords(chords, lyric)
	want := "[one]one[G]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStackingTwoChordsSameWord(t *testing.T) {
	chords := "G D"
	lyric := "hi"
	got := PlaceChords(chords, lyric)
	want := "[G]hi[D]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStackingThreeChordsSameWord(t *testing.T) {
	chords := "G D A"
	lyric := "hi"
	got := PlaceChords(chords, lyric)
	want := "[G]hi[D][A]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLeadingWhitespacePreserved(t *testing.T) {
	chords := "  G"
	lyric := "  hello"
	got := PlaceChords(chords, lyric)
	want := "  [G]hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnnotationTokenPassthrough(t *testing.T) {
	chords := "G  x2"
	lyric := "hi"
	got := PlaceChords(chords, lyric)
	want := "[G]hix2"
	if got != want {
		t.Errorf("got %q, want %q (annotation passes through unbracketed)", got, want)
	}
}

func TestParenChordStrippedInPlaceChords(t *testing.T) {
	// (Gm) in a chord line should produce [Gm] not [(Gm)]
	got := PlaceChords("(Gm)", "hi")
	want := "[Gm]hi"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParenChordStrippedStandalone(t *testing.T) {
	// (Em) standalone chord line → [Em] not [(Em)]
	got := PlaceChords("(Em)", "")
	// Empty lyric → falls through to the standalone path
	if got != "[Em]" {
		t.Errorf("got %q, want %q", got, "[Em]")
	}
}
