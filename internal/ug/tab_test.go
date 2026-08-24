// Package ug — HTML-extraction tests for the user-tab fetch path.
//
// These tests use synthetic HTML that mimics UG's embedded JSON structure.
// No copyrighted material is involved — all content is test-only.
package ug

import (
	"strings"
	"testing"
)

// ====================================================================
//  HTML content extraction
// ====================================================================

// TestExtractContent_PlanJSON verifies the "content" field is found
// and returned in plain-JSON format.
func TestExtractContent_PlanJSON(t *testing.T) {
	html := `<script>var tabData = {
   "content": "Capo on 3rd Fret\\r\\n\\r\\n[Intro]\\r\\n[ch]Am[/ch] [ch]C[/ch]\\r\\n\\r\\n[Chorus]\\r\\n[ch]G[/ch]\\r\\n",
   "tab": {"id": 5555555, "song_name": "some song", "artist_name": "some artist", "username": "test_user", "votes": 42},
   "meta": {"capo": 3, "tuning": {"value": "E A D G B E", "index": 1, "name": "Standard"}, "difficulty": "intermediate"}
};</script>`
	content, found := extractContent(html)
	if !found {
		t.Fatal("extractContent found nothing when content field present")
	}
	// Verify \r\n was converted to \n
	if !strings.Contains(content, "[Intro]") {
		t.Errorf("extracted content missing [Intro]: %q", content[:200])
	}
	if strings.Contains(content, `\r`) {
		t.Errorf("content still has \\r after normalisation: %q",
			strings.TrimSpace(content)[:100])
	}
}

// TestExtractContent_HTMLEscaped verifies the "content" field is found
// in HTML-escaped format (&quot;...&quot;).
func TestExtractContent_HTMLEscaped(t *testing.T) {
	html := `<script>
   var data = "content&quot;:&quot;Capo on 7th Fret\\r\\n\\r\\n[Intro]\\r\\n[ch]Em[/ch] [ch]Cadd9[/ch]\\r\\n&quot;";
</script>`
	// Construct a realistic HTML-escaped blob for the content field.
	htmlEscaped := `&quot;content&quot;:&quot;Capo on 7th Fret\\r\\n\\r\\n[Intro]\\r\\n[ch]Em[/ch] [ch]Cadd9[/ch]\\r\\n&quot;`
	_, found := extractContent(string([]byte(html + htmlEscaped)))
	if !found {
		t.Error("extractContent failed on HTML-escaped content field")
	}
	// Just verify we get something; the actual extraction for HTML-escaped
	// uses reContentHTML.
}

// TestExtractFromHTML_Capo verifies that meta.capo is extracted.
func TestExtractFromHTML_Capo(t *testing.T) {
	html := `<script>
   var tabData = {
		"content": "\\r\\n[Intro]\\r\\n[ch]Am[/ch]\\r\\n",
		"tab": {
			"id": 5555555,
			"song_name": "some song",
			"artist_name": "some artist",
			"username": "test_user",
			"votes": 42,
			"type": "Chords"
		},
		"meta": {
			"capo": 7,
			"tuning": {"value": "E A D G B E", "index": 1, "name": "Standard"},
			"difficulty": "intermediate"
		}
	};
</script>`

	d, err := extractFromHTML(5555555, "https://tabs.ultimate-guitar.com/tab/unknown/unknown-chords-5555555", html)
	if err != nil {
		t.Fatalf("extractFromHTML error: %v", err)
	}
	if d.Capo != 7 {
		t.Errorf("Capo = %d, want 7", d.Capo)
	}
	if d.Source != "user-tab-html" {
		t.Errorf("Source = %q, want user-tab-html", d.Source)
	}
	if d.TabID != 5555555 {
		t.Errorf("TabID = %d, want 5555555", d.TabID)
	}
}

// TestExtractFromHTML_Tuning verifies the tuning value is extracted.
func TestExtractFromHTML_Tuning(t *testing.T) {
	html := `<script>
   var tabData = {
		"content": "\\r\\nfoo\\r\\n",
		"tab": {"id": 1111, "song_name": "x", "artist_name": "y", "username": "z", "votes": 1},
		"meta": {"capo": 0, "tuning": {"value": "F A# D G B E", "index": 2, "name": "Dropped D"}, "difficulty": "easy"}
	};
</script>`
	d, err := extractFromHTML(1111, "https://tabs.ultimate-guitar.com/tab/x/y-chords-1111", html)
	if err != nil {
		t.Fatalf("extractFromHTML error: %v", err)
	}
	if d.Tuning != "F A# D G B E" {
		t.Errorf("Tuning = %q, want F A# D G B E", d.Tuning)
	}
	if d.Difficulty != "easy" {
		t.Errorf("Difficulty = %q, want easy", d.Difficulty)
	}
}

// TestExtractFromHTML_CapoFromContentLine verifies that when meta.capo
// is 0 but the content starts with "Capo on Nth Fret", the fret number
// is extracted from the content line.
func TestExtractFromHTML_CapoFromContentLine(t *testing.T) {
	html := `<script>
   var tabData = {
		"content": "Capo on 7th Fret\\r\\n\\r\\n[Intro]\\r\\n[ch]Em[/ch]\\r\\n",
		"tab": {"id": 2222, "song_name": "x", "artist_name": "y", "username": "z", "votes": 1},
		"meta": {"capo": 0, "tuning": {"value": "E A D G B E", "index": 1}, "difficulty": "easy"}
	};
</script>`
	d, err := extractFromHTML(2222, "https://tabs.ultimate-guitar.com/tab/x/y-chords-2222", html)
	if err != nil {
		t.Fatalf("extractFromHTML error: %v", err)
	}
	// meta.capo=0, but content starts with "Capo on 7th Fret" — should find 7.
	if d.Capo != 7 {
		t.Errorf("Capo = %d, want 7 (extracted from content instruction)", d.Capo)
	}
}

// TestExtractFromHTML_UsernameAndVotes verifies user-tab fields.
func TestExtractFromHTML_UsernameAndVotes(t *testing.T) {
	html := `<script>
   var tabData = {
		"content": "\\r\\nfoo",
		"tab": {"id": 3333, "song_name": "x", "artist_name": "y", "username": "test_user", "votes": 311, "type": "Chords"},
		"meta": {"capo": 0, "tuning": {"value": "E A D G B E", "index": 1}, "difficulty": "intermediate"}
	};
</script>`
	d, err := extractFromHTML(3333, "https://tabs.ultimate-guitar.com/tab/x/y-chords-3333", html)
	if err != nil {
		t.Fatalf("extractFromHTML error: %v", err)
	}
	if d.Username != "test_user" {
		t.Errorf("Username = %q, want test_user", d.Username)
	}
	if d.Votes != 311 {
		t.Errorf("Votes = %d, want 311", d.Votes)
	}
}

// TestExtractFromHTML_NoContent verifies an error is returned when no
// content field exists.
func TestExtractFromHTML_NoContent(t *testing.T) {
	html := `<html><body>No tab data here.</body></html>`
	_, err := extractFromHTML(9999, "https://tabs.ultimate-guitar.com/tab/x/y-chords-9999", html)
	if err == nil {
		t.Error("expected error when no content field present")
	}
	if !strings.Contains(err.Error(), "no content field") {
		t.Errorf("error = %q, expected 'no content field'", err.Error())
	}
}

// ====================================================================
//  Clean step on HTML-extracted content
// ====================================================================

// TestClean_HTMLTabContent verifies that content extracted from HTML
// (with UG [ch]...[/ch] markup) passes through Clean() with 0 fragments.
func TestClean_HTMLTabContent(t *testing.T) {
	// Simulate a user-tab HTML content field after extractFromHTML.
	// \r\n has been converted to \n by normalizeUserTabContent.
	const content = `Capo on 7th Fret

[Intro]
[ch]Em[/ch] [ch]Cadd9[/ch] [ch]G[/ch] [ch]D[/ch]

[Verse 1]
[tab][ch]Em[/ch]                     [ch]Cadd9[/ch]
some lyric text here[/tab]
[tab][ch]G[/ch]                [ch]D[/ch]
more lyric text[/tab]

[Chorus]
[tab]([ch]Em[/ch])           [ch]Cadd9[/ch]
chorus lyric[/tab]

(Instrumental)

[Outro]
[X]`

	// First run the HTML normalisation step.
	normalised := normalizeUserTabContent(content)

	// Then clean.
	clean, err := Clean(normalised)
	if err != nil {
		t.Fatalf("Clean() returned error: %v", err)
	}

	// 0 fragments in output.
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0; output:\n%s",
			Fragments(clean), clean)
	}

	// Key chords should be present.
	for _, chord := range []string{"Em", "Cadd9", "G", "D"} {
		if !strings.Contains(clean, chord) {
			t.Errorf("Clean() dropped chord %q from HTML content", chord)
		}
	}

	// "Capo on 7th Fret" instruction should be stripped.
	if strings.Contains(clean, "Capo on") {
		t.Error("Capo instruction line was not stripped from output")
	}

	// [ch] and [/ch] should not be in output.
	if strings.Contains(clean, "[ch]") || strings.Contains(clean, "[/ch]") {
		t.Error("chord markup not stripped from output")
	}
}

// TestClean_CapoInstructionStripped verifies that "Capo on Nth Fret"
// at the top of content is removed by normalizeUserTabContent.
func TestClean_CapoInstructionStripped(t *testing.T) {
	const content = "Capo on 7th Fret\n\n[Intro]\n[ch]Am[/ch]"
	normalised := normalizeUserTabContent(content)
	if strings.Contains(normalised, "Capo on 7th") {
		t.Error("Capo instruction not stripped by normalizeUserTabContent")
	}
}

// ====================================================================
//  ExtractTabID still works for URL parsing
// ====================================================================

// TestExtractTabID_UserTabURL verifies that -chords-NNNN URLs
// parse correctly to the right tab ID.
func TestExtractTabID_UserTabURL(t *testing.T) {
	cases := []struct {
		url  string
		want int
	}{
		{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song-chords-696128", 696128},
		{"https://tabs.ultimate-guitar.com/tab/some-artist/some-song-official-1948797", 1948797},
		{"https://tabs.ultimate-guitar.com/tab/some-artist/another-song-chords-5555555", 5555555},
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

// TestExtractTabID_NoVersionArrayNeeded verifies we don't need to
// extract a version ID from the versions[] array — the URL IS the identity.
func TestExtractTabID_NoVersionArrayNeeded(t *testing.T) {
	// The -chords-696128 URL should parse to 696128 regardless of
	// what versions[] array the HTML might contain.
	got, err := ExtractTabID("https://tabs.ultimate-guitar.com/tab/unknown/song-chords-696128")
	if err != nil {
		t.Fatalf("ExtractTabID error: %v", err)
	}
	if got != 696128 {
		t.Errorf("ExtractTabID = %d, want 696128 (from URL slug)", got)
	}
}
