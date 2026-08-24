// Package ug handles the UG official Pro tab pipeline:
// fetch from api-web.ultimate-guitar.com, clean the UG-lyric markup,
// extract metadata, and analyse the chord set.
//
// The pro_meta API is a PUBLIC endpoint — no auth, no login, no Cloudflare.
// A plain HTTP GET gets the full JSON for an official tab.
package ug

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrNotFound is the sentinel error returned by FetchUGMeta when the UG
// pro_meta API returns HTTP 404. Callers (e.g. FetchTabByURL in tab.go)
// check errors.Is(err, ErrNotFound) to decide whether to fall back to
// HTML scraping for the version ID.
var ErrNotFound = errors.New("ug: tab not found in pro_meta (HTTP 404)")

// UGMeta is the typed representation of UG's pro_meta API response.
// Fields are populated after Unmarshal, and can be accessed directly.
type UGMeta struct {
	// Name is the song title as UG knows it (e.g. "Hurt").
	Name string `json:"name"`
	// Artist is the performer name as UG knows it (e.g. "Johnny Cash").
	Artist string `json:"artist"`
	// Tempo is UG's reported BPM. For 4/5 of our trails this is 120 —
	// a UG "default" placeholder, NOT the real tempo. Treat as a
	// placeholder and cross-check against songbpm / musicstax / tunebat.
	Tempo int `json:"tempo"`
	// Meta holds the nested tuning/capo/duration details.
	Meta struct {
		// Tuning is the string tuning (e.g. "E A D G B E").
		Tuning string `json:"tuning"`
		// Capo is the capo fret number; 0 means no capo.
		Capo int `json:"capo"`
		// Duration is the song duration in milliseconds (e.g. 225400 = 3:45.4).
		Duration int `json:"duration"`
	} `json:"meta"`
	// Lyrics is UG's pro-reader markup, un-cleaned. See Clean() in clean.go
	// for the 6-pass strip that produces a clean text file.
	Lyrics string `json:"lyrics"`
	// StrummingPatterns is UG's strumming-pattern data, one entry per
	// pattern section. The .BPM field is often closer to the real tempo
	// than the top-level Tempo field.
	StrummingPatterns []struct {
		Part        string `json:"part"`
		Denominator int    `json:"denuminator"`
		BPM         int    `json:"bpm"`
		IsTriplet   int    `json:"is_triplet"`
	} `json:"strummingPatterns"`
	// Tracks is UG's track listing (chord / finger / lyrics / guitar tabs).
	Tracks []Track `json:"tracks"`
}

// Track is one entry in UG's track list.
type Track struct {
	Kind     string `json:"kind"`
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Present  bool   `json:"present"`
}

// DurationString formats UG's millisecond duration as mm:ss.
func (m *UGMeta) DurationString() string {
	secs := m.Meta.Duration / 1000
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
}

// UGTabAPIMeta is the raw JSON response from
// https://api-web.ultimate-guitar.com/v1/tab/pro/meta?id=X
//
// The API response is a single JSON object with UGMeta fields at the top level.
// Our old trail files wrapped it as {outer: {meta: "<json string>"}} —
// that was a bug in how the old trail files were saved, not the API format.
// The API itself returns a flat JSON object.
type UGTabAPIResponse = UGMeta

// FetchUGMeta calls the UG pro_meta API for the given tab URL and returns
// the parsed metadata.
//
// The URL is a UG tab URL like:
//   https://tabs.ultimate-guitar.com/tab/johnny-cash/hurt-official-1948797
//
// No authentication is required. The API is public as of 2026.
//
// Returns an error if the tab ID can't be extracted from the URL or the
// HTTP request fails.
func FetchUGMeta(tabURL string) (*UGMeta, error) {
	id, err := ExtractTabID(tabURL)
	if err != nil {
		return nil, fmt.Errorf("extracting tab ID from URL: %w", err)
	}

	apiURL := fmt.Sprintf("https://api-web.ultimate-guitar.com/v1/tab/pro/meta?id=%d", id)
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", apiURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: tab id %d may be a user tab, not official", ErrNotFound, id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, apiURL, strings.TrimSpace(string(body)))
	}

	var meta UGMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parsing pro_meta JSON: %w", err)
	}
	return &meta, nil
}

// ExtractTabID parses a UG tab URL and returns the numeric tab ID that the
// pro_meta API expects as ?id=X.
//
// UG URL formats:
//
//	https://tabs.ultimate-guitar.com/tab/johnny-cash/hurt-official-1948797
//	https://tabs.ultimate-guitar.com/tab/johnny-cash/hurt?tab=1948797
//
// In both cases the "id" is the trailing number.
func ExtractTabID(tabURL string) (int, error) {
	u, err := url.Parse(tabURL)
	if err != nil {
		return 0, err
	}

	// Prefer the ?pro=X / ?tab=X query parameter if present.
	if proQuery := u.Query().Get("pro"); proQuery != "" {
		return strconv.Atoi(proQuery)
	}
	if tabQuery := u.Query().Get("tab"); tabQuery != "" {
		return strconv.Atoi(tabQuery)
	}
	// UG also accepts ?id=N (used when extracting a version ID
	// for the FetchTabByURL fallback).
	if idQuery := u.Query().Get("id"); idQuery != "" {
		return strconv.Atoi(idQuery)
	}

	// Otherwise: extract trailing digits from the last path segment.
	// Path: /tab/johnny-cash/hurt-official-1948797 → "1948797"
	path := u.Path
	slash := strings.LastIndex(path, "/")
	seg := path
	if slash >= 0 {
		seg = path[slash+1:]
	}

	// Find the trailing digit sequence.
	end := len(seg)
	start := end
	for start > 0 && seg[start-1] >= '0' && seg[start-1] <= '9' {
		start--
	}
	if start == end {
		return 0, fmt.Errorf("no trailing digit found in path segment %q from URL %q", seg, tabURL)
	}
	return strconv.Atoi(seg[start:])
}
