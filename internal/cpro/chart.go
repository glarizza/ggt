// Package cpro drives the full conversion: walks a classified chart
// line by line, dispatching chord+lyric pairs to PlaceChords,
// standalone chord lines to wrapStandaloneChordLine, and tab-chart
// section markers to maybeSectionBreak (which inserts a single blank
// line as a separator rather than dropping it silently).
//
// Section headers like [Verse 1], [Chorus], [Intro], [Outro] are NOT
// emitted as text — they are dropped. A blank-line separator is inserted
// at each section boundary so verse/chorus/bridge structure is preserved
// in the .chopro output.
//
// End-of-song markers (X alone on a line) and standalone
// "(Instrumental)" lines are dropped entirely.
//
// When HeaderOpts is non-trivial, a {key: value} header block is
// prepended to the output. If --capo N is set (and --remove-capo is not), the
// header gains a single `{capo: N}` line; no human (Capo N) body line is ever emitted.

package cpro

import "strings"

// wrapStandaloneChordLine lives in filter.go.

// Convert reads the full text of a tab chart and returns .chopro output.
// If opts has any fields set, a {key: value} header is prepended.
// RemoveCapo is handled in cmd/ggt (body de-capo); cpro only emits {capo: N}.
func Convert(text string, opts HeaderOpts) string {
	rawLines := strings.Split(text, "\n")
	classified := make([]ClassifiedLine, len(rawLines))
	for i, l := range rawLines {
		classified[i] = ClassifyLine(l)
	}

	var out []string

	n := len(classified)
	for i := 0; i < n; {
		cl := classified[i]

		switch cl.Kind {
		case Section:
			maybeSectionBreak(&out)
			i++
		case Blank:
			i++
		case Chord:
			if i+1 < n && classified[i+1].Kind == Lyric {
				out = append(out, PlaceChords(cl.Text, classified[i+1].Text))
				i += 2
			} else {
				out = append(out, wrapStandaloneChordLine(cl.Text))
				i++
			}
		case Lyric:
			if !isDropLine(cl.Text) {
				out = append(out, cl.Text)
			}
			i++
		}
	}

	// trim trailing blank lines (can happen if chart ends with a section header)
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}

	// assemble final output
	var buf strings.Builder

	header := emitHeader(opts)
	if header != "" {
		buf.WriteString(header)
		buf.WriteByte('\n')
	}

	body := strings.Join(out, "\n")
	if body != "" {
		buf.WriteString(body)
		buf.WriteByte('\n')
	}

	return buf.String()
}
