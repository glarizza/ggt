// Package ug — TabData: unified fetch result for PRO and user tabs.
//
// FetchTabByURL returns *TabData for both paths. HTML extraction for
// user tabs lives here; the pro_meta types stay in meta.go.
package ug

import (
	"fmt"
	"regexp"
	"strings"
)

// --------------------------------------------------------------------
//  FetchTabByURL — top-level dispatch
// --------------------------------------------------------------------

// FetchTabByURL fetches a UG tab (official Pro or user-submitted) and
// returns a fully-populated *TabData.
//
// Strategy (two distinct UG pathways):
//  1. PRO tab (official / behind paywall): pro_meta API returns 200.
//     Source = "pro_meta". All fields populated including Tempo,
//     StrumBPM, DurationMS, Tracks.
//  2. User tab (community, not in pro_meta): pro_meta returns 404.
//     Fall through to HTML extraction. Source = "user-tab-html".
//     Tempo/StrumBPM/DurationMS/Tracks are zero/nil — cross-check.
//
// The URL slug distinguishes the two:
//
//	...-official-NNNNNN  → PRO tab  (pro_meta fast path)
//	...-chords-NNNNM     → user tab (HTML extraction)
//
// No "try pro_meta then version-ID fallback" logic — that approach was
// the root cause of the capo mismatch bug (versions[0].id grabbed the
// wrong tab). User tabs are fetched from HTML directly.
func FetchTabByURL(tabURL string) (*TabData, error) {
	id, err := ExtractTabID(tabURL)
	if err != nil {
		return nil, fmt.Errorf("ug: extracting tab ID from URL %q: %w", tabURL, err)
	}

	// Try pro_meta (PRO tab fast path).
	meta, err := FetchUGMeta(tabURL)
	if err == nil {
		return UGMetaToTabData(meta, id), nil
	}
	if isNotFound(err) {
		// User tab: extract from HTML, no second API call.
		html, ferr := fetchHTML(tabURL)
		if ferr != nil {
			return nil, fmt.Errorf("ug: fetching HTML for user tab %d: %w", id, ferr)
		}
		return extractFromHTML(id, tabURL, html)
	}
	return nil, err
}

// isNotFound returns true when err wraps the ErrNotFound sentinel.
// The pro_meta error message includes "HTTP 404" in the error string;
// this is used as a string match (errors.Is would require ErrNotFound
// from FetchUGMeta, which wraps it differently).
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "404")
}

// --------------------------------------------------------------------
//  TabData — unified struct for both fetch paths
// --------------------------------------------------------------------

// TabData is the unified representation returned by FetchTabByURL.
//
// Source field:
//
//	"pro_meta"        — fetched via the official pro_meta API
//	"user-tab-html"   — extracted from the HTML page (no pro_meta available)
//
// For user tabs, Tempo/DelayurationMS/StrumBPM/Tracks are zero/nil —
// the HTML does not serve these. The fact sheet shows "not available"
// rather than placeholder values.
type TabData struct {
	// Common to both paths.
	TabID   int
	Name    string // song title
	Artist  string // performer name
	Tuning  string // string tuning (e.g. "E A D G B E")
	Capo    int    // capo fret (0 = no capo)
	Content string // UG-markup tab content; needs Clean()

	// HTML-only fields (user-tab path).
	Username   string // UG user who submitted the tab
	Votes      int    // user-tab vote count
	Difficulty string // "easy" | "intermediate" | "advanced"

	// pro_meta-only fields (PRO path).
	Tempo      int // UG top-level tempo (often 120 — unreliable)
	StrumBPM   int // from strummingPatterns[].bpm (closer to real)
	DurationMS int // duration in milliseconds (0 = not available)
	Tracks     []Track

	// Source tag.
	Source string // "pro_meta" or "user-tab-html"
}

// DurationOrPlaceholder returns mm:ss, or "n/a" if not available.
func (d *TabData) DurationOrPlaceholder() string {
	if d.DurationMS == 0 {
		return "n/a (not in this tab source)"
	}
	secs := d.DurationMS / 1000
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// --------------------------------------------------------------------
//  HTML extraction regexes
//
// UG embeds tab data as JSON in <script> blocks; two escaping levels:
//   1. HTML-escaped:  &quot;key&quot;:&quot;value&quot;
//   2. Plain JSON:    "key":"value"
//
// Each field has both patterns; firstMatch tries plain first, then
// HTML-escaped.
// --------------------------------------------------------------------

var (
	// "content" — the full chord+lyric body with UG markup.
	// Plain JSON first (faster common case), then HTML-escaped.
	reContentPlain = regexp.MustCompile(
		`"content"\s*:\s*"((?:\\.|[^"\\])*)"`)
	// (HTML-escaped "content" is extracted by extractContentHTMLEscaped,
	// a manual scanner — a non-greedy regex would truncate the body at the
	// first inner \&quot; quote UG renders. See that function.)

	// "tab":{"id":N,...} — the main tab's ID.
	reTabIDPlain = regexp.MustCompile(
		`"tab"\s*:\s*\{\s*"id"\s*:\s*(\d{4,8})`)
	reTabIDHTML = regexp.MustCompile(
		`&quot;tab&quot;:\{&quot;id&quot;:\s*(\d{4,8})`)

	// "meta":{"capo":N,...} — capo from the embedded metadata block.
	reCapoPlain = regexp.MustCompile(
		`"meta"\s*:\s*\{\s*"capo"\s*:\s*(\d+)`)
	reCapoHTML = regexp.MustCompile(
		`&quot;meta&quot;:\{&quot;capo&quot;:\s*(\d+)`)

	// "tuning":{"value":"E A D G B E",...}
	reTuningPlain = regexp.MustCompile(
		`"tuning"\s*:\s*\{[^}]*"value"\s*:\s*"([^"]*)"`)
	reTuningHTML = regexp.MustCompile(
		`&quot;tuning&quot;:\{[^}]*&quot;value&quot;:&quot;([^&]+)&quot;`)

	// "song_name":"..." — canonical UG song slug.
	reSongNamePlain = regexp.MustCompile(
		`"song_name"\s*:\s*"([^"]*)"`)
	reSongNameHTML = regexp.MustCompile(
		`&quot;song_name&quot;:&quot;([^&]+)&quot;`)

	// "artist_name":"..." — canonical UG artist slug.
	reArtistNamePlain = regexp.MustCompile(
		`"artist_name"\s*:\s*"([^"]*)"`)
	reArtistNameHTML = regexp.MustCompile(
		`&quot;artist_name&quot;:&quot;([^&]+)&quot;`)

	// "username":"..." — the tab author's UG handle.
	// Try user_iq adjacency first (most specific to the tab entry);
	// fall back to bare "username" for flat JSON structures.
	reUserNamePlain = regexp.MustCompile(
		`"user_iq"\s*:\s*\d+\s*,\s*"username"\s*:\s*"([^"]*)"`)
	reUserNamePlain2 = regexp.MustCompile(
		`"username"\s*:\s*"([^"]*)"`)
	reUserNameHTML = regexp.MustCompile(
		`&quot;user_iq&quot;\s*:\s*\d+\s*,&quot;username&quot;\s*:\s*&quot;(.*?)&quot;`)
	reUserNameHTML2 = regexp.MustCompile(
		`&quot;username&quot;\s*:\s*&quot;(.*?)&quot;`)

	// "tab":{...,"votes":N,...}
	reVotesPlain = regexp.MustCompile(
		`"votes"\s*:\s*(\d+)`)
	reVotesHTML = regexp.MustCompile(
		`&quot;tab&quot;:\{[^}]*&quot;votes&quot;:\s*(\d+)`)

	// "difficulty" — direct field match (not nested-block).
	// Nested-block regexes with [^}]* stop at the first inner } (tuning
	// sub-object), breaking the match. A direct match is simpler and
	// robust.
	reDifficultyPlain = regexp.MustCompile(
		`"difficulty"\s*:\s*"([a-z]+)"`)
	reDifficultyHTML = regexp.MustCompile(
		`&quot;difficulty&quot;:&quot;([a-z]+)&quot;`)
)

// --------------------------------------------------------------------
//  extractFromHTML — user-tab extraction
// --------------------------------------------------------------------

// extractFromHTML extracts a *TabData from a UG HTML page that embeds
// tab data as JSON. This is the user-tab path, used when pro_meta
// returns 404.
func extractFromHTML(tabID int, tabURL, html string) (*TabData, error) {
	d := &TabData{
		TabID:  tabID,
		Source: "user-tab-html",
	}

	// 1. Content field (the full chord+lyric body).
	content, found := extractContent(html)
	if !found {
		return nil, fmt.Errorf("ug: no content field found in HTML for tab %d", tabID)
	}
	// 2. Metadata fields — plain-JSON first, then HTML-escaped fallback.
	d.Name = strings.TrimSpace(firstMatch(html, reSongNamePlain, reSongNameHTML))
	d.Artist = strings.TrimSpace(firstMatch(html, reArtistNamePlain, reArtistNameHTML))
	d.Username = htmlUnescape(strings.TrimSpace(firstMatch(html,
		reUserNamePlain, reUserNamePlain2,
		reUserNameHTML, reUserNameHTML2)))
	d.Votes = atoiSafe(firstMatch(html, reVotesPlain, reVotesHTML))
	d.Difficulty = firstMatch(html, reDifficultyPlain, reDifficultyHTML)
	d.Capo = atoiSafe(firstMatch(html, reCapoPlain, reCapoHTML))
	d.Tuning = strings.TrimSpace(firstMatch(html, reTuningPlain, reTuningHTML))

	// 3. Cross-check: if meta.capo=0, try extracting from a "Capo on Nth
	//    Fret" instruction in the RAW content (before normalisation strips it).
	if d.Capo == 0 {
		if capoFromLine, ok := extractCapoFromInstruction(content); ok {
			d.Capo = capoFromLine
		}
	}

	// 4. Now normalise content (JSON escapes, HTML entities, strip capo instr.).
	d.Content = normalizeUserTabContent(content)

	// 4. Fallback: extract artist/name from URL slug if HTML data absent.
	if d.Name == "" || d.Artist == "" {
		uArtist, uName := slugFromURL(tabURL)
		if d.Name == "" {
			d.Name = uName
		}
		if d.Artist == "" {
			if uArtist != "" {
				d.Artist = uArtist
			} else {
				d.Artist = "unknown"
			}
		}
	}

	return d, nil
}

// --------------------------------------------------------------------
//  Content extraction and normalisation
// --------------------------------------------------------------------

// extractContent tries the plain-JSON pattern first, then the
// HTML-escaped pattern. Returns the content string and found flag.
func extractContent(html string) (string, bool) {
	var raw string
	var ok bool
	if m := reContentPlain.FindStringSubmatch(html); m != nil {
		raw, ok = m[1], true
	} else if v, found := extractContentHTMLEscaped(html); found {
		raw, ok = v, true
	}
	if !ok {
		return "", false
	}
	// Both paths decode the same way: JSON escapes, then HTML entities.
	v := unescapeJSONString(raw)
	v = htmlUnescape(v)
	return v, true
}

// extractContentHTMLEscaped locates the HTML-escaped `content` value and returns
// its FULL value, treating an inner `\&quot;` (a JSON `"` rendered as the
// `&quot;` HTML entity, i.e. backslash + `&quot;`) as part of the value and a
// BARE `&quot;` as the closing delimiter.
//
// This replaces the old non-greedy reContentHTML regex, which stopped at the
// FIRST inner &quot; and truncated the value at the first quoted phrase in the
// tab prose — silently dropping the entire chord+lyric body after it (the bug
// found on UG 1219927 / Some Fantastic, whose intro quotes a \"bathroom
// session\").
//
// Go's regexp has no lookbehind, so we scan manually: when we hit a backslash
// we skip the escaped token it guards — either an HTML entity (&...;) or a
// single character — so the &quot; after \&quot; can never look like the close.
func extractContentHTMLEscaped(raw string) (string, bool) {
	const open = `&quot;content&quot;:&quot;`
	i := strings.Index(raw, open)
	if i < 0 {
		return "", false
	}
	i += len(open)
	start := i
	for i < len(raw) {
		if raw[i] == '\\' {
			// Escaped token: skip it. If it targets an HTML entity, skip the
			// whole `&...;`; otherwise skip the single escaped char.
			if i+1 < len(raw) {
				if raw[i+1] == '&' {
					if j := strings.IndexByte(raw[i+1:], ';'); j >= 0 {
						i += j + 1
					} else {
						return raw[start:], true // unterminated entity; best effort
					}
				} else {
					i += 2
				}
			}
			continue
		}
		if strings.HasPrefix(raw[i:], `&quot;`) {
			return raw[start:i], true // bare closing delimiter
		}
		i++
	}
	return raw[start:], true // unterminated; return what we have
}

// extractCapoFromInstruction checks if content starts with a
// "Capo on N(th/rd) Fret" or "Capo N(th/rd)" instruction.
func extractCapoFromInstruction(content string) (int, bool) {
	re := regexp.MustCompile(
		`(?i)^capo\s+(?:on\s+)?(\d+)\s*(?:rd|th)?\s*(?:fret)?\b`)
	if m := re.FindStringSubmatch(content); m != nil {
		return atoiSafe(m[1]), true
	}
	return 0, false
}

// normalizeUserTabContent handles HTML-extracted content artefacts:
// 1. JSON escape sequences (\r\n, \n, \t as literal 2-char sequences).
// 2. HTML entities that may survive into the content.
// 3. A leading "Capo on Nth Fret" instruction line.
//
// This runs BEFORE the clean step (which handles UG markup tags).
func normalizeUserTabContent(s string) string {
	s = unescapeJSONString(s)
	s = htmlUnescape(s)
	s = stripCapoInstruction(s)
	return s
}

// stripCapoInstruction removes "Capo on Nth Fret\n" or "Capo N\n"
// from the start of a string.
func stripCapoInstruction(s string) string {
	re := regexp.MustCompile(
		`(?i)^capo\s+(?:on\s+)?\d+\s*(?:rd|th)?\s*(?:fret)?\s*\n?`)
	if m := re.FindString(s); m != "" {
		return strings.TrimSpace(s[len(m):])
	}
	return s
}

// --------------------------------------------------------------------
//  HTML extraction helpers
// --------------------------------------------------------------------

// firstMatch tries each regex in order, returning the capture group
// from the first one that matches. Returns "" if none match.
func firstMatch(html string, patterns ...*regexp.Regexp) string {
	for _, p := range patterns {
		if m := p.FindStringSubmatch(html); m != nil {
			return m[1]
		}
	}
	return ""
}

// htmlUnescape decodes common HTML entities.
func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

// unescapeJSONString unescapes standard JSON string escape sequences
// that appear as literal 2-character text in the extracted content.
func unescapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\r\n`, "\n")
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\t`, "\t")
	s = strings.ReplaceAll(s, `\r`, "\n")
	return s
}

// slugFromURL extracts (artist, song_name) from a UG tab URL slug.
//
// URL path: /tab/<artist-slug>/<song-slug>-chords|official-NNNN
// Returns best-effort title-case "Artist" and "Song Name".
func slugFromURL(tabURL string) (artist, name string) {
	idx := strings.Index(tabURL, "/tab/")
	if idx < 0 {
		return "", ""
	}
	path := tabURL[idx+5:] // skip "/tab/"
	// Split into artist segment and rest.
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		return "", ""
	}
	artist = toTitle(parts[0])
	songSlug := parts[1]
	// Strip trailing "-chords-NNNN" / "-official-NNNN" / "-guitar-NNNN".
	songSlug = reStripSlugSuffix.ReplaceAllString(songSlug, "")
	name = toTitle(songSlug)
	return artist, name
}

// reStripSlugSuffix strips "-chords|official|guitar-NNNN" from a slug.
var reStripSlugSuffix = regexp.MustCompile(`-(?:chords|official|guitar|ukulele|bass)-\d+$`)

// toTitle converts a hyphenated slug to "Title Case" words.
func toTitle(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(string(p[0])) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// --------------------------------------------------------------------
//  ProMeta adapter: *UGMeta → *TabData
// --------------------------------------------------------------------

// UGMetaToTabData converts a fetched *UGMeta (from pro_meta API) into
// a *TabData struct. HTML-only fields (Username, Votes, Difficulty)
// stay empty. This is the adapter used by `ggt ug <file>` mode.
func UGMetaToTabData(m *UGMeta, tabID int) *TabData {
	d := &TabData{
		TabID:      tabID,
		Name:       m.Name,
		Artist:     m.Artist,
		Tuning:     m.Meta.Tuning,
		Capo:       m.Meta.Capo,
		Content:    m.Lyrics,
		Tempo:      m.Tempo,
		DurationMS: m.Meta.Duration,
		Tracks:     m.Tracks,
		Source:     "pro_meta",
	}
	// Compute StrumBPM from strumming patterns.
	for _, p := range m.StrummingPatterns {
		if p.BPM > d.StrumBPM {
			d.StrumBPM = p.BPM
		}
	}
	return d
}
