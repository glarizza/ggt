package sections

import "testing"

func TestIsWord(t *testing.T) {
	// These are section/annotation words that can start on a pitch letter but
	// must never be treated as a chord. The set is the single source of truth
	// shared by the cpro parser and the transpose layer.
	known := []string{
		"Intro", "Verse", "Chorus", "Bridge", "Outro", "Solo", "Break",
		"refrain", "tag", "prechorus", "postchorus", "Guitar", "riff",
		"Fill", "fill-in", "instrumental", "interlude", "Coda",
		"keychange", "modulation",
	}
	for _, w := range known {
		if !IsWord(w) {
			t.Errorf("IsWord(%q) = false, want true", w)
		}
	}

	// Real chords and prose are NOT section words.
	not := []string{"Dm7", "G", "A#m", "F# - F", "x2", "Bridge x2", "Guitar x"}
	for _, w := range not {
		if IsWord(w) {
			t.Errorf("IsWord(%q) = true, want false", w)
		}
	}
}
