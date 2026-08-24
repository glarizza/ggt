// Package ug — low-level HTTP helpers.
//
// fetchHTML is the UG user-agent HTTP client.
// atoiSafe is a nil-safe string→int parser used by HTML extraction.
// Top-level tab-finding logic lives in tabdata.go; pro_meta API in meta.go.
package ug

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// fetchHTML fetches a UG web-page and returns the raw HTML body.
// Returns an error (not an empty string) on any HTTP non-200 status.
func fetchHTML(pageURL string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	req.Header.Set("User-Agent",
				"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
				"AppleWebKit/605.1.15 (KHTML, like Gecko) " +
				"Chrome/131.0.0.0 Safari/605.1.15")
	req.Header.Set("Accept",
				"text/html,application/xhtml+xml,application/xml")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, pageURL)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	return string(body), nil
}

// atoiSafe parses a non-negative integer string, returning 0 on any
// error or empty string. Used by HTML extraction for ID, capo, votes.
func atoiSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
			}
		n = n*10 + int(c-'0')
	}
	return n
}
