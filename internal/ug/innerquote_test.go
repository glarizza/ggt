// Package ug — regression tests for HTML-escaped content extraction.
//
// UG tab 1219927 (Some Fantastic) embeds a quoted phrase in its prose
// (e.g. \&quot;bathroom session\&quot;). UG renders an inner JSON escape \"
// as the 7-char sequence backslash-&-q-u-o-t-; (\&quot;). The old non-greedy
// reContentHTML stopped at that first inner &quot; and truncated the value to
// the intro note, dropping the entire chord body. These tests guard that.
//
// No copyrighted material: all content below is synthetic.
package ug

import (
	"strings"
	"testing"
)

// extractContent with an inner escaped quote must NOT truncate the value.
// Text AFTER the quoted phrase (the chord body) must survive.
func TestExtractContent_HTMLEscaped_InnerQuote(t *testing.T) {
	// Real UG escaping: inner quote = backslash + &quot;; CRLF = \r\n (4 chars).
	raw := `&quot;content&quot;:&quot;This is how I play it, recommend watching the \&quot;bathroom session\&quot; version on\r\n\r\nCapo 3\r\n\r\n[Intro]\r\n[ch]G[/ch] [ch]Am7[/ch] [ch]Bm[/ch]\r\n[Chorus]\r\n[ch]D/F#[/ch]\r\n&quot;`

	content, found := extractContent(raw)
	if !found {
		t.Fatal("extractContent found nothing for HTML-escaped content with inner escaped quote")
	}
	// Everything below the inner quote must survive (this is what the old
	// non-greedy regex dropped).
	for _, want := range []string{"bathroom session", "[Intro]", "[ch]D/F#[/ch]"} {
		if !strings.Contains(content, want) {
			t.Errorf("content truncated: missing %q; got: %q", want, content)
		}
	}
}

// End-to-end regression through extractFromHTML: a real-shape HTML-escaped
// object whose content has an inner escaped quote must yield a body that
// survives Clean() with 0 fragments and the chords intact.
func TestExtractFromHTML_InnerEscapedQuoteBody(t *testing.T) {
	html := `&quot;content&quot;:&quot;solo acoustic, recommend the \&quot;bathroom session\&quot; version\r\n\r\nCapo 3\r\n\r\n[Intro]\r\n[ch]G[/ch] [ch]Am7[/ch] [ch]Bm[/ch]\r\n[Chorus]\r\n[tab][ch]D/F#[/ch] [ch]A7[/ch]\r\nsome fake lyric line here[/tab]\r\n&quot;` +
		`&quot;revision_id&quot;:2,&quot;user_id&quot;:12345,&quot;username&quot;:&quot;sourctest&quot;,&quot;date&quot;:1643836256,` +
		`&quot;meta&quot;:{&quot;capo&quot;:3,&quot;tuning&quot;:{&quot;value&quot;:&quot;E A D G B E&quot;}}`

	if got := strings.Count(html, "\\&quot;bathroom session"); got != 1 {
		t.Fatalf("fixture setup error: expected 1 inner escaped quote, got %d", got)
	}

	d, err := extractFromHTML(9500001, "https://tabs.ultimate-guitar.com/tab/x/y-chords-9500001", html)
	if err != nil {
		t.Fatalf("extractFromHTML error: %v", err)
	}
	if !strings.Contains(d.Content, "[Chorus]") {
		t.Errorf("body truncated before [Chorus]; Content: %q", d.Content)
	}
	if !strings.Contains(d.Content, "[ch]D/F#[/ch]") {
		t.Errorf("body truncated before [ch]D/F#[/ch]; Content: %q", d.Content)
	}

	clean, cerr := Clean(d.Content)
	if cerr != nil {
		t.Fatalf("Clean() returned error: %v", cerr)
	}
	if Fragments(clean) != 0 {
		t.Errorf("Fragments() = %d, want 0; clean output:\n%s", Fragments(clean), clean)
	}
	if !strings.Contains(clean, "D/F#") || !strings.Contains(clean, "A7") {
		t.Errorf("cleaned output missing body chords: %q", clean)
	}
}

// Bare "Capo 3" instruction form (no ordinal, no "Fret" word) — the form UG
// 1219927 uses — must be both detected as a fret and stripped from content.
func TestCapoInstruction_BareCapo3(t *testing.T) {
	const content = "Capo 3\n\n[Intro]\n[ch]Am[/ch]"

	normalised := normalizeUserTabContent(content)
	if strings.Contains(normalised, "Capo 3") {
		t.Errorf("bare 'Capo 3' instruction not stripped from content: %q", normalised)
	}
	if !strings.Contains(normalised, "[Intro]") {
		t.Errorf("stripping 'Capo 3' dropped following content: %q", normalised)
	}
	if fret, ok := extractCapoFromInstruction(content); !ok || fret != 3 {
		t.Errorf("extractCapoFromInstruction = %d,%v; want 3,true", fret, ok)
	}
}
