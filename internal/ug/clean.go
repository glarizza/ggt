package ug

import (
	"fmt"
	"regexp"
	"strings"
)

// Clean takes UG pro-reader markup (the "lyrics" field from UG's
// pro_meta API) and produces a plain-text ChordPro-ready file that
// ggt cpro can consume.
//
// It strips 6 classes of UG-specific markup:
//  1. [ch app="XXXXXX"]  open chord tag    →  (removes tag, keeps chord name after it)
//  2. [/ch]               close chord tag   →  (removes tag entirely)
//  3. [tab] ... [/tab]    tab-lane blocks   →  keeps content, strips the markers
//  4. [syllable ...]      open lyric tag    →  removes tag, keeps the word after it
//  5. [/syllable]         close lyric tag   →  removes tag entirely
//  6. [End] / [end]       end-of-chart marker →  removes entirely
//
// It preserves section labels ([Intro], [Verse 1], [Chorus], [Outro])
// for ggt cpro's section-break handling, and never touches the lyric
// text itself — only the UG markup around it.
//
// A post-condition check runs at the end: the output must match zero
// of 6 fragment patterns. A non-zero fragment count means a strip
// pattern missed; that would silently corrupt the data, so it is a
// hard error, NOT a warning.
func Clean(lyrics string) (string, error) {
	// 1. Chord open tags: [ch app="XXXXXX"] → ""
     //    The chord NAME follows the ] and is a plain word (Am, C, D, F/A, G/Bb).
	//    We strip only the tag itself.
	s := reChOpen.ReplaceAllString(lyrics, "")

	// 2. Chord close tags: [/ch] → ""
	s = reChClose.ReplaceAllString(s, "")

	// 3. Tab lane markers: keep the content, strip [tab] and [/tab].
	//    This is the KEY fix vs. the old Python trail — don't drop the
	//    whole block, just the markers.
	s = reTabOpen.ReplaceAllString(s, "")
	s = reTabClose.ReplaceAllString(s, "")

	// 4. Syllable open tags: [syllable data-...="..."] → ""
	s = reSylOpen.ReplaceAllString(s, "")

	// 5. Syllable close tags: [/syllable] → ""
	s = reSylClose.ReplaceAllString(s, "")

	// 6. End-of-chart markers: [End] / [end] → ""
	s = reEndMarker.ReplaceAllString(s, "")
	// Also strip X-only lines (end-of-section markers that appear as X alone
	// on a line) and standalone "(Instrumental)" lines.
	s = reXOnlyLine.ReplaceAllString(s, "")
	s = reInstrumentalLine.ReplaceAllString(s, "")

	// Collapse 3+ blank lines to 1.
	s = reThreePlusBlanks.ReplaceAllString(s, "\n\n")

	// Strip leading blank lines.
	s = strings.TrimLeft(s, "\n")

	// Post-condition check: zero fragment tags allowed in output.
	frag := Fragments(s)
	if frag != 0 {
		return "", fmt.Errorf("clean: post-condition failed — %d fragment tag(s) remaining in output", frag)
	}
	return s, nil
}

// Fragments returns the count of fragment tag occurrences in text.
// A post-condition check must return 0 for clean output.
func Fragments(text string) int {
	n := 0
	for _, re := range fragmentPatterns {
		n += len(re.FindAllString(text, -1))
	}
	return n
}

// The 6 fragment regexes — the post-condition patterns that must
// all produce zero matches after a successful Clean().
var fragmentPatterns = []*regexp.Regexp{
	reChOpen,       // [ch ...]
	reChClose,       // [/ch]
	reTabOpen,       // [tab]
	reTabClose,      // [/tab]
	reSylOpen,      // [syllable ...]
	reSylClose,     // [/syllable]
}

// Regex patterns (compiled once at package init).
var (
	// [ch app="XXXXXX"] — open chord tag (with or without a space after [ch)
	reChOpen = regexp.MustCompile(`\[\s*ch\s*[^]]*]`)
	// [/ch] — close chord tag
	reChClose = regexp.MustCompile(`\[\s*/\s*ch\s*\]`)
	// [tab] — open tab-lane marker
	reTabOpen = regexp.MustCompile(`\[\s*tab\s*\]`)
	// [/tab] — close tab-lane marker
	reTabClose = regexp.MustCompile(`\[\s*/\s*tab\s*\]`)
	// [syllable data-...="..."] — open syllable tag (with data attributes)
	reSylOpen = regexp.MustCompile(`\[\s*syllable[^\]]*\]`)
	// [/syllable] — close syllable tag
	reSylClose = regexp.MustCompile(`\[\s*/\s*syllable\s*\]`)
	// [End] / [end] — end-of-chart marker
	reEndMarker = regexp.MustCompile(`\[\s*[Ee]nd\s*\]`)
	// X-only lines: a line that is just "X" (end-of-section marker)
	reXOnlyLine = regexp.MustCompile(`(?m)^X\s*$`)
	// Standalone "(Instrumental)" lines
	reInstrumentalLine = regexp.MustCompile(`(?m)^\(Instrumental\)\s*$`)
	// 3+ consecutive blank lines
	reThreePlusBlanks = regexp.MustCompile(`\n\s*\n\s*\n+`)
)
