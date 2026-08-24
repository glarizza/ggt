// Package ug tests.
//
// Real UG data lives in ignored/ (gitignored, not committed).
// The real-data tests skip if the fixture is absent (which is always true
// in CI / fresh clones). Synthetic tests always run.

package ug

import (
	"os"
	"testing"
)

// --------------------------------------------------------------------
// Synthetic tests (always run, no copyrighted material)
// --------------------------------------------------------------------

// TestClean_Synthetic exercises the 6-pass clean pipeline with a
// hand-crafted UG markup snippet that has no real lyrics. It covers
// all 5 strip classes: [ch ...], [/ch], [syllable ...],
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
	// The chord "Am" should survive (it's between the open/close ch tags).
	if !contains(clean, "Am") {
		t.Error("Clean() dropped the chord Am; expected it in output")
	}
	if !contains(clean, "bar") {
		t.Error("Clean() dropped the lyric word 'bar'")
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
// These use real UG pro_meta lyrics. The fixtures live in
// ignored/ug-fixtures/ (gitignored) and are NOT in the repo.
// CI / fresh clones skip them.
//
// To run locally:
//   python3 scripts/scramble_ug_fixtures.py && go test ./internal/ug/ -v
//
// OR just copy fixtures manually:
//   cp ignored/ug-fixtures/hurt-lyrics.txt internal/ug/testdata/hurt-lyrics.txt
// --------------------------------------------------------------------

func TestClean_UgFixture01(t *testing.T) {
	// UG fixture 001: official tab, no capo, minor key
	const rawPath   = "testdata/ug001-lyrics.txt"
	const wantPath  = "testdata/ug001-clean.txt"
	lyrics, err   := os.ReadFile(rawPath)
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
		t.Errorf("Clean() mismatch (got %d chars, want %d)\n  got:  %q\n  want: %q",
			len(got), len(want), firstN(got, 300), firstN(string(want), 300))
	}
}

func TestClean_UgFixture02(t *testing.T) {
	// UG fixture 002: official tab, capo 3, remove-capo to F major
	const rawPath   = "testdata/ug002-lyrics.txt"
	const wantPath  = "testdata/ug002-clean.txt"
	lyrics, err   := os.ReadFile(rawPath)
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
		t.Errorf("Clean() mismatch (got %d chars, want %d)",
			len(got), len(want))
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
			// Real UG URL shapes (just testing the parser; no claim about the song)
			{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song-official-1948797", 1948797},
			{"https://tabs.ultimate-guitar.com/tab/some-artist/another-song-official-2475408", 2475408},
			// Query-parameter shapes
			{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song?tab=1948797", 1948797},
			{"https://tabs.ultimate-guitar.com/tab/some-artist/another-song?pro=12345", 12345},
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
		"https://tabs.ultimate-guitar.com/tab",
		"https://example.com",
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
		t.Error("TonalCandidate returned empty on non-chord text")
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
