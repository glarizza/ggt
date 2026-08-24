// Package ug — Tab-level tests for FetchTabByURL and extractVersionID.
package ug

import (
	"testing"
)

// TestExtractVersionID_HTMLEscaped verifies that the HTML-escaped JSON
// pattern UG actually serves is parsed correctly.
//
// UG serves the version metadata as an HTML-escaped JSON in a JS variable.
// The entities are &quot; (single-escaped, NOT double-escaped).
func TestExtractVersionID_HTMLEscaped(t *testing.T) {
	// This is the exact pattern UG serves in the tab page source:
	//
	//   &quot;versions&quot;:[{&quot;id&quot;:1947541,...}
	//
	// The test HTML uses the real UG HTML-escape (single &quot; not
	// double-&amp;quot;).
	const html = "<script>var x = &quot;versions&quot;:[{&quot;id&quot;:1947541," +
			"&quot;song_id&quot;:15747};</script>"
	got := extractVersionID(html)
	if got != 1947541 {
	t.Errorf("extractVersionID = %d, want 1947541", got)
	}
}

// TestExtractVersionID_PlainJSON verifies the plain-JSON fallback works
// (for pages that JSON.parse() the version data instead of inline-escaping).
func TestExtractVersionID_PlainJSON(t *testing.T) {
	// Plain JSON in a script tag (UG occasionally uses this form)
	const html = `<script>var data = {"versions":[{"id":5555555}]};</script>`
	got := extractVersionID(html)
	if got != 5555555 {
		t.Errorf("extractVersionID = %d, want 5555555", got)
	}
}

// TestExtractVersionID_NotPresent verifies 0 is returned when no versions
// field is in the HTML (e.g. Mraz user tabs that UG has not promoted).
func TestExtractVersionID_NotPresent(t *testing.T) {
	const html = `<html><body><p>No version data here.</p></body></html>`
	got := extractVersionID(html)
	if got != 0 {
		t.Errorf("extractVersionID = %d, want 0 for HTML with no versions field", got)
	}
}
