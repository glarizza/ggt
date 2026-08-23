package cpro

import (
	"strings"
	"testing"
)

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

// TestPlaceChordsSkipsUnparseableChordRow is the crux of the "don't drop the
// whole line for one bad symbol" fix: F and G are placed, and the un-parseable
// (F# - F) is skipped rather than polluting the lyric.
// TestPlaceChordsLeavesUnparseableChordRowAlone is the crux of "convert, don't
// add/subtract musical content": F and G are placed, and the un-parseable
// "(F# - F)" slide is LEFT ALONE -- emitted verbatim so a human can fix it --
// rather than being silently deleted, which would hide that a chord lived there.
func TestPlaceChordsLeavesUnparseableChordRowAlone(t *testing.T) {
	chords := "F                    G                                (F# - F)"
	lyric := "  Mmmm, it's always better when we're together"
	got := PlaceChords(chords, lyric)

	// The two valid chords ARE placed, so the row is kept, not dropped.
	if !strings.Contains(got, "[F]") {
		t.Errorf("expected F to be placed, got %q", got)
	}
	if !strings.Contains(got, "[G]") {
		t.Errorf("expected G to be placed, got %q", got)
	}
	// The un-parseable symbol is LEFT ALONE: present, verbatim, un-bracketed,
	// and un-transposed (its spelling is not guessed at).
	if !strings.Contains(got, "(F# - F)") {
		t.Errorf("(F# - F) must be left in place verbatim, got %q", got)
	}
	if strings.Contains(got, "[F# - F]") {
		t.Errorf("the un-parseable symbol must not be bracketed as a chord, got %q", got)
	}
	// The lyric text still surrounds it (the leading fragment at least).
	if !strings.Contains(got, "Mmmm, it's always") {
		t.Errorf("leading lyric fragmented, got %q", got)
	}
}

// TestPlaceChordsConnectorNotBracketed checks a bare connector is skipped, not
// bracketed.
func TestPlaceChordsConnectorNotBracketed(t *testing.T) {
	got := PlaceChords("F - G", "one two three")
	if strings.Contains(got, "[-]") || strings.Contains(got, "[ - ]") {
		t.Errorf("connector must not be bracketed, got %q", got)
	}
	if !strings.Contains(got, "[F]") || !strings.Contains(got, "[G]") {
		t.Errorf("both chords should be placed, got %q", got)
	}
}
