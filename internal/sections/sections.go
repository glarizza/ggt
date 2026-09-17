// Package sections holds the single canonical set of section/annotation
// words -- Intro, Verse, Bridge, Chorus, Break, Guitar, ... -- that can appear
// as a bracketed chart header ([Bridge], [Chorus] (x3)) and must be treated as
// structure, never as a chord.
//
// The set lives here so that both the cpro parser (which must DROP such headers)
// and the transpose layer (which must NEVER transpose them) import ONE list,
// rather than each carrying a copy that can drift out of sync. When a new
// section word shows up, it is added in this one place. A full "is this a chord
// at all?" decision -- which would also drop chord-shaped section words like
// [Bridge] without any closed list -- is the deferred chord-quality grammar
// (Option B); until then this auditable closed set is the single source of
// truth. Matched on the trimmed inner content, case-insensitively.
package sections

import "strings"

// words is the closed, canonical set of section/annotation words.
var words = map[string]bool{
	"intro":        true,
	"verse":        true,
	"chorus":       true,
	"bridge":       true,
	"outro":        true,
	"solo":         true,
	"break":        true,
	"refrain":      true,
	"tag":          true,
	"prechorus":    true,
	"postchorus":   true,
	"guitar":       true,
	"riff":         true,
	"fill":         true,
	"fill-in":      true,
	"instrumental": true,
	"interlude":    true,
	"coda":         true,
	"keychange":    true,
	"modulation":   true,
}

// IsWord reports whether s, trimmed and lower-cased, is a known
// section/annotation word. Both the cpro parser and the transpose layer use
// this so the vocabulary cannot drift between them.
func IsWord(s string) bool {
	return words[strings.ToLower(strings.TrimSpace(s))]
}
