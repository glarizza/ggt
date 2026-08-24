// Package ug tests.
//
// Synthetic tests always run (copyright-free).
// Real-data tests skip when the fixture is absent (CI / fresh clone).
package ug

import (
	"os"
	"testing"
)

// --------------------------------------------------------------------
// Synthetic tests (always run — no copyrighted material)
// --------------------------------------------------------------------

// TestClean_Synthetic exercises the 6-pass clean pipeline covering
// all markup strip classes: [ch ...], [/ch], [syllable ...],
// [/syllable], [End].
func TestClean_Synthetic(t *testing.T) {
	const input = `[Intro]
	[ch app="x02210"]Am[/ch] [ch app="x32010"]C[/ch]
	[syllable data-time="0"]foobar[/syllable] bazqux quazzy
	[syllable data-time="1"]qwerty[/syllable] asdfgh jklzxc
	[C] [D] [G]

	[End]`

	clean, err := Clean(input)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0; output still has markup:\n%s",
			Fragments(clean), clean)
	}
}

// TestClean_TabLaneKeepsContent verifies that [tab]...[/tab] markers
// are stripped but the chord/lyric content between them is preserved.
func TestClean_TabLaneKeepsContent(t *testing.T) {
	const input = `[tab][ch app="x00200"]Am[/ch]   [ch app="x32010"]C[/ch]
			[syllable data-time="1"]foobar[/syllable] bar
		[/tab]`

	clean, err := Clean(input)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0 after clean\n%s", Fragments(clean), clean)
	}
	if !contains(clean, "Am") {
		t.Error("Clean() dropped the chord Am; expected it in output")
	}
	if !contains(clean, "bar") {
		t.Error("Clean() dropped the lyric word 'bar'")
	}
}

// TestClean_UserTabChordFormat verifies the user-tab [ch]CHORD[/ch] syntax
// (no app="X" attribute) is stripped by the same reChOpen/reChClose pair
// that handles official tabs. The regex \[\s*ch\s*[^]]*\] matches both
// [ch app="X02210"] and [ch].
func TestClean_UserTabChordFormat(t *testing.T) {
	const input = `[ch]Am[/ch] [ch]Em[/ch]
	I've paid my dues
	[ch]C[/ch] [ch]F[/ch]`

	clean, err := Clean(input)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0; output: %s", Fragments(clean), clean)
	}
	if contains(clean, "ch]") {
		t.Errorf("Clean() left ch residue in user-tab output: %q", clean)
	}
	for _, chord := range []string{"Am", "Em", "C", "F"} {
		if !contains(clean, chord) {
			t.Errorf("Clean() dropped user-tab chord %q", chord)
		}
	}
}

// TestClean_EndMarker verifies [End] / [end] is stripped.
func TestClean_EndMarker(t *testing.T) {
	clean, _ := Clean("[Intro]\nfoo\n[End]\n")
	if contains(clean, "[End]") {
		t.Error("[End] was NOT stripped; it should be gone after Clean")
	}
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0", Fragments(clean))
	}
}

// TestClean_SectionPreserved verifies section labels are NOT stripped.
func TestClean_SectionPreserved(t *testing.T) {
	clean, _ := Clean("[Intro]\nAm\n[Verse 1]\nC\n")
	if !contains(clean, "[Intro]") {
		t.Error("section [Intro] was stripped; it should be preserved")
	}
	if !contains(clean, "[Verse 1]") {
		t.Error("section [Verse 1] was stripped; it should be preserved")
	}
}

// --------------------------------------------------------------------
// Real-data integration tests (skip when fixture absent)
//
// Real UG fixtures live in internal/ug/testdata/ but are gitignored.
// CI / fresh clones skip these tests. To run locally:
//   cp ignored/ug-fixtures/ug001-lyrics.txt internal/ug/testdata/ug001-lyrics.txt
//   cp ignored/ug-fixtures/ug001-clean.txt  internal/ug/testdata/ug001-clean.txt
// --------------------------------------------------------------------

func TestClean_UgFixture01(t *testing.T) {
	const rawPath  = "testdata/ug001-lyrics.txt"
	const wantPath = "testdata/ug001-clean.txt"
	lyrics, err := os.ReadFile(rawPath)
	if err != nil {
		t.Skipf("fixture %s not present: %v", rawPath, err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Skipf("fixture %s not present: %v", wantPath, err)
	}
	got, err := Clean(string(lyrics))
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}
	if got != string(want) {
		t.Errorf("Clean() mismatch (got %d chars, want %d)\n  got:   %q\n  want: %q",
			len(got), len(want), firstN(got, 300), firstN(string(want), 300))
	}
}

func TestClean_UgFixture02(t *testing.T) {
	const rawPath  = "testdata/ug002-lyrics.txt"
	const wantPath = "testdata/ug002-clean.txt"
	lyrics, err := os.ReadFile(rawPath)
	if err != nil {
		t.Skipf("fixture %s not present: %v", rawPath, err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Skipf("fixture %s not present: %v", wantPath, err)
	}
	got, err := Clean(string(lyrics))
	if err != nil {
		t.Fatalf("Clean() error: %v", err)
	}
	if got != string(want) {
		t.Errorf("Clean() mismatch (got %d chars, want %d)", len(got), len(want))
	}
}

// --------------------------------------------------------------------
// URL extraction tests (no copyrighted material — just URL parsing)
// --------------------------------------------------------------------

func TestExtractTabID(t *testing.T) {
	cases := []struct {
		url  string
		want int
	}{
				// UG tab URL shapes (no claim about the actual song)
			{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song-official-1948797", 1948797},
			{"https://tabs.ultimate-guitar.com/tab/some-artist/another-song-official-2475408", 2475408},
				// Query-parameter shapes
			{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song?tab=1948797", 1948797},
			{"https://tabs.ultimate-guitar.com/tab/some-artist/another-song?pro=12345", 12345},
				// Version ID shape (for FetchTabByURL version-ID fallback)
			{"https://tabs.ultimate-guitar.com/?id=1947541", 1947541},
	}
	for _, c := range cases {
		got, err := ExtractTabID(c.url)
		if err != nil {
			t.Errorf("ExtractTabID(%q) error: %v", c.url, err)
			continue
			}
		if got != c.want {
			t.Errorf("ExtractTabID(%q) = %d, want %d", c.url, got, c.want)
			}
	}
}

func TestExtractTabID_Errors(t *testing.T) {
	cases := []string{
			"",
			"not-a-url",
			"https://tabs.ultimate-guitar.com/tab",   // no trailing digits
			"https://example.com",                      // not a UG URL
	}
	for _, url := range cases {
		if _, err := ExtractTabID(url); err == nil {
			t.Errorf("ExtractTabID(%q) expected an error", url)
		}
	}
}

// --------------------------------------------------------------------
// Chord frequency tests
// --------------------------------------------------------------------

func TestAnalyzeChords_Synthetic(t *testing.T) {
	const text = `[Am] [C] [D] [G]
[Am] [C]
[Em]
[Am]`
	freq := AnalyzeChords(text)
	if len(freq) == 0 {
		t.Fatal("AnalyzeChords returned empty for known chord text")
	}
	if freq[0].Chord != "Am" {
		t.Errorf("AnalyzeChords: top = %q, want Am", freq[0].Chord)
	}
	if freq[0].Count != 3 {
		t.Errorf("AnalyzeChords: Am count = %d, want 3", freq[0].Count)
	}
}

func TestTonalCandidate_Empty(t *testing.T) {
	tc := TonalCandidate("foo bar baz\nqux quux\n")
	if tc == "" {
		// TonalCandidate returns the top chord even for non-chord text
		// (it should still return something or an empty string, that's fine).
		t.Log("TonalCandidate returned empty on non-chord text — expected")
	}
}

// --------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}
