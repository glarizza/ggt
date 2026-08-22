package music

import (
	"strings"
	"testing"
)

// TestTransposeSymbol covers the per-symbol contract: uniform chromatic
// shift, quality preserved verbatim, both halves of a slash chord,
// non-chords untouched, and accidental-style selection.
//
// Every "expected" below was hand-computed on the chromatic circle and
// checked by ear — gary's "have it work" criterion.
func TestTransposeSymbol(t *testing.T) {
	cases := []struct {
		name   string
		sym    string
		semis  int
		style  Style
		want   string
		wantOK bool
	}{
		// basic single-chord shifts
		{"major down 1", "C", -1, StyleAuto, "B", true},
		{"minor down 1", "Dm", -1, StyleAuto, "C#m", true},
		{"major up", "G", +5, StyleAuto, "C", true},
		{"root only down 3", "E", -3, StyleAuto, "C#", true},

		// quality preserved verbatim
		{"m7 quality", "Dm7", -1, StyleAuto, "C#m7", true},
		{"m7b5 quality", "Dm7b5", -3, StyleAuto, "Bm7b5", true},
		{"add9 quality", "Cadd9", -1, StyleAuto, "Badd9", true},
		{"sus4 with sharp", "F#sus4", -2, StyleAuto, "Esus4", true},
		{"maj7 flats", "Cmaj7", -1, StyleFlats, "Bmaj7", true},
		{"dominant 7b9", "A7b9", +3, StyleSharps, "C7b9", true},
		{"slash 6/9 quality stays", "C6/9", -1, StyleAuto, "B6/9", true},

		// slash chords: both halves shift
		{"slash D/F# down 3 sharps", "D/F#", -3, StyleSharps, "B/D#", true},
		{"slash D/F# down 3 flats", "D/F#", -3, StyleFlats, "B/Eb", true},
		{"slash with quality", "Dm7/F#", -3, StyleSharps, "Bm7/D#", true},
		{"slash E/G# up 1", "E/G#", +1, StyleAuto, "F/A", true},

		// accidental selection
		{"F# stays F# in sharps", "F#", 0, StyleSharps, "F#", true},
		{"F# as Gb in flats", "F#", 0, StyleFlats, "Gb", true},
		{"auto sharp root inherits sharp", "F#m", 0, StyleAuto, "F#m", true},
		{"auto flat root inherits flat", "Bbm", 0, StyleAuto, "Bbm", true},
		{"A down 3 sharps = F#", "A", -3, StyleSharps, "F#", true},
		{"A down 3 flats = Gb", "A", -3, StyleFlats, "Gb", true},

		// non-chords left untouched / not transposed
		{"walkdown two-chords", "F# - F", -3, StyleAuto, "F# - F", false},
		{"x2 annotation", "x2", -3, StyleAuto, "x2", false},
		{"NC annotation", "N.C.", -3, StyleAuto, "N.C.", false},
		{"section header word", "Verse 1", -3, StyleAuto, "Verse 1", false},
		{"empty", "", -3, StyleAuto, "", false},

		// the "preserve degree" cases gary called out
		{"Dm7b5 untouched at 0", "Dm7b5", 0, StyleAuto, "Dm7b5", true},
		{"Cadd9 to Aadd9 down 3", "Cadd9", -3, StyleAuto, "Aadd9", true},
	}

	for _, tc := range cases {
		got, ok := TransposeSymbol(tc.sym, tc.semis, tc.style)
		if ok != tc.wantOK || got != tc.want {
			t.Errorf("%s: TransposeSymbol(%q, %d, %v) = (%q, %v); want (%q, %v)",
				tc.name, tc.sym, tc.semis, tc.style, got, ok, tc.want, tc.wantOK)
		}
	}
}

// TestTransposeCProText covers the file-level contract: brackets
// transposed, {key:} rewritten, everything else in a cpro output
// left alone.
func TestTransposeCProText(t *testing.T) {
	in := "" +
		"{key: E}\n" +
		"{capo: 3}\n" +
		"(Capo 3)\n" + // visual cue line — must pass through (not a bracket chord)
		"[G] [D]\n" +
		"[G]better [D]better\n" +
		"[F#7] a chord\n" +
		"[F# - F] a walkdown\n"
	out := TransposeCProText(in, -3, StyleSharps)

	// E->C, G->E, D->B, F#->D, F# - F -> unchanged (walkdown).
	if got := KeyOf(out); got != "C#" {
		t.Errorf("key: got %q want %q", got, "C#")
	}
	if !strings.Contains(out, "{capo: 3}") {
		t.Errorf("capo line dropped: %q", out)
	}
	if !strings.Contains(out, "(Capo 3)") {
		t.Errorf("(Capo 3) cue dropped (should pass through): %q", out)
	}
	if !strings.Contains(out, "[E] [B]") {
		t.Errorf("expected \"[E] [B]\" in output:\n%s", out)
	}
	if !strings.Contains(out, "[E]better [B]better") {
		t.Errorf("inline brackets not transposed:\n%s", out)
	}
	if !strings.Contains(out, "[D#7] a chord") {
		t.Errorf("F#7 not transposed to D#7 (sharps):\n%s", out)
	}
	if !strings.Contains(out, "[F# - F] a walkdown") {
		t.Errorf("walkdown was modified — should pass through:\n%s", out)
	}
}

// TestTransposeCProBody covers body-only de-capo transposition: the {key:}
// (and any {\u003cmetadata\u003e} line) is left untouched while bracketed chords
// shift by N; walk-downs with spaces pass through unchanged.
func TestTransposeCProBody(t *testing.T) {
	in := "" +
		"{key: E}\n" +
		"{capo: 4}\n" +
		"[C] a [G] b\n" +
		"[Am] lyric [Am/G]\n" +
		"[F# - F] walkdown\n"
	out := TransposeCProBody(in, 4, StyleSharps)

	// key stays E -- it is the target the shapes move toward, not shifted.
	if got := KeyOf(out); got != "E" {
		t.Errorf("key must stay E (body-only): got %q", got)
	}
	// body shifted +4 with sharps: C->E, G->B, Am->C#m, Am/G->C#m/B.
	if !strings.Contains(out, "[E] a [B] b") {
		t.Errorf("C/G not shifted +4:\n%s", out)
	}
	if !strings.Contains(out, "[C#m] lyric [C#m/B]") {
		t.Errorf("Am / Am/G not shifted +4:\n%s", out)
	}
	// the {capo: 4} metadata line passes through the body transpose untouched
	// (it lives in the header; cmd drops it separately on --remove-capo).
	if !strings.Contains(out, "{capo: 4}") {
		t.Errorf("{capo: 4} must pass through body transpose untouched:\n%s", out)
	}
	// walkdown with a space is not a parseChord symbol -> unchanged.
	if !strings.Contains(out, "[F# - F] walkdown") {
		t.Errorf("walkdown was modified -- should pass through:\n%s", out)
	}
}

// TestToKey covers --to-key distance math.
func TestToKey(t *testing.T) {
	cases := []struct {
		from, to string
		want     int
		wantOK   bool
	}{
		{"E", "F#", 2, true},  // E -> F# is up 4
		{"C", "G", 7, true},   // C -> G is up 7
		{"G", "G", 0, true},   // same key = no move
		{"F#", "E", 10, true}, // F# down 1 == up 11
		{"E", "not a key", 0, false},
		{"", "C", 0, false},
	}
	for _, c := range cases {
		got, ok := SemitonesTo(c.from, c.to)
		if ok != c.wantOK || got != c.want {
			t.Errorf("SemitonesTo(%q,%q) = (%d,%v); want (%d,%v)",
				c.from, c.to, got, ok, c.want, c.wantOK)
		}
	}
}

// TestRoundTrip up-then-down is identity.
func TestRoundTrip(t *testing.T) {
	for _, sym := range []string{"Dm7", "F#9", "Cadd9", "G/B", "E7"} {
		down, _ := TransposeSymbol(sym, -3, StyleSharps)
		back, _ := TransposeSymbol(down, +3, StyleSharps)
		if back != sym {
			t.Errorf("round-trip %s -> %s -> %s (want %s)", sym, down, back, sym)
		}
	}
}
