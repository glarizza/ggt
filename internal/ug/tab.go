// Package ug — user-tab support (version-ID fallback).
//
// FetchTabByURL is the top-level entry point that handles both
// official tabs (direct pro_meta) and user tabs (HTML + version ID).
package ug

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

// versionIDPattern extracts the first version ID from UG HTML.
//
// User-tab pages embed version data as an HTML-escaped JSON string:
//
//	&amp;quot;versions&quot;:[{&amp;quot;id&quot;:1947541,...}]
//
// A second, plain-JSON fallback handles pages that don't HTML-escape
// the data.
var (
	htmlEscapedVersion = regexp.MustCompile(
				`&quot;versions&quot;:\s*\[\s*\{\s*&quot;id&quot;:\s*(\d{4,8})`)
	plainJSONVersion = regexp.MustCompile(
				`"versions"\s*:\s*\[\s*\{\s*"id"\s*:\s*(\d{4,8})`)
)

// FetchTabByURL fetches a UG tab by its web-page URL.
//
// Strategy:
//  1. Try a direct pro_meta call (fast path — official tabs).
//  2. On ErrNotFound, fetch the HTML page, extract the version ID,
//     and retry pro_meta with that ID.
//  3. If both fail, return a descriptive error.
//
// The returned UGMeta is fully populated — no difference to the caller
// between official tabs and version-ID fallback.
func FetchTabByURL(tabURL string) (*UGMeta, error) {
	// Strategy 1: direct pro_meta
	meta, err := FetchUGMeta(tabURL)
	if err == nil {
		return meta, nil
	}
	// Strategy 2: version-ID fallback
	if errors.Is(err, ErrNotFound) {
		return fetchUserTabFromHTML(tabURL)
	}
	// Any other error — return as-is
	return nil, err
}

// fetchUserTabFromHTML fetches the user-tab HTML page, extracts the
// version ID from embedded JSON metadata, and retries pro_meta with it.
func fetchUserTabFromHTML(tabURL string) (*UGMeta, error) {
	html, err := fetchHTML(tabURL)
	if err != nil {
		return nil, fmt.Errorf("ug: HTML fetch to get version ID failed: %w", err)
	}
	versionID := extractVersionID(html)
	if versionID == 0 {
		return nil, fmt.Errorf("ug: no version record found in HTML for "+
				"tab URL %s — this may be a user tab with no UG Pro version",
				tabURL)
	}
	// Re-fetch pro_meta with the version ID.
	// FetchUGMeta accepts a URL, so we build a synthetic URL with ?id=versionID.
	versionURL := fmt.Sprintf("https://tabs.ultimate-guitar.com/?id=%d", versionID)
	return FetchUGMeta(versionURL)
}

// fetchHTML fetches a UG web-page and returns the raw HTML body.
// Returns a *fetchError on HTTP non-200 so callers can distinguish
// "network/parse error" from "page not found."
func fetchHTML(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("User-Agent",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
				"AppleWebKit/605.1.15 (KHTML, like Gecko) Chrome/131.0.0.0 "+
				"Safari/605.1.15")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(body), nil
}

// extractVersionID extracts the first version ID from UG HTML.
// Returns 0 if not found.
func extractVersionID(html string) int {
	if m := htmlEscapedVersion.FindStringSubmatch(html); m != nil {
		return atoiSafe(m[1])
	}
	if m := plainJSONVersion.FindStringSubmatch(html); m != nil {
		return atoiSafe(m[1])
	}
	return 0
}

// atoiSafe returns the integer value of a digit string, or 0 on any error.
func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
			}
		n = n*10 + int(c-'0')
	}
	return n
}
