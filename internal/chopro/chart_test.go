package chopro

import (
	"strings"
	"testing"
)

func TestDropsSectionHeadersAndBlankLines(t *testing.T) {
	// With section headers in the input, the output should have a
	// blank-line separator between sections (section break), not
	// sections running together.
	raw := "[Intro]\nG   D\n\nhello world\n\n[Chorus]\nAm   C\nfoo bar\n"
	got := Convert(raw, HeaderOpts{})

	if strings.Contains(got, "[Intro]") || strings.Contains(got, "[Chorus]") {
		t.Errorf("section headers should be dropped, got:\n%s", got)
	}
	// Body should contain the merged chord+lyric lines
	if !strings.Contains(got, "hello world") {
		t.Errorf("expected plain lyric in body, got:\n%s", got)
	}
	if !strings.Contains(got, "[Am]foo [C]bar") {
		t.Errorf("expected chorus line, got:\n%s", got)
	}
	// Blank-line section break between verse and chorus
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	foundSectionBreak := false
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" && lines[i-1] != "" && i+1 < len(lines) && lines[i+1] != "" {
			foundSectionBreak = true
		}
	}
	if !foundSectionBreak {
		t.Errorf("expected a blank-line section break in output, got: %v", lines)
	}
}

func TestStandaloneChordLineNoLyricBelow(t *testing.T) {
	// (Em) (Cadd9) (G) (D) on its own should produce [Em] [Cadd9] [G] [D]
	// after paren-stripping.
	raw := "(Em) (Cadd9) (G) (D)\n\n"
	got := strings.TrimSpace(Convert(raw, HeaderOpts{}))
	want := "[Em] [Cadd9] [G] [D]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStandaloneChordLineFollowedByAnotherChordLine(t *testing.T) {
	// Two consecutive standalone chord lines are on separate output lines
	raw := "G   Am\nF   C\n"
	got := Convert(raw, HeaderOpts{})
	lines := []string{}
	for _, l := range strings.Split(got, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) != 2 || lines[0] != "[G]   [Am]" || lines[1] != "[F]   [C]" {
		t.Errorf("got %v", lines)
	}
}

func TestHeaderEmission(t *testing.T) {
	raw := "G D\nhello world\n"
	got := Convert(raw, HeaderOpts{Title: "Test Song", Key: "G", Capo: 7})
	// Expected: {title: Test Song}\n{key: G}\n{capo: 7}\n(Capo 7)\n[body]\n
	if !strings.HasPrefix(got, "{title: Test Song}\n{key: G}\n{capo: 7}\n(Capo 7)\n") {
		t.Errorf("expected header+capo-line prefix, got:\n%q", got)
	}
}

func TestNoHeaderWhenOptsEmpty(t *testing.T) {
	raw := "G D\nhello world\n"
	got := Convert(raw, HeaderOpts{})
	if strings.Contains(got, "{") {
		t.Errorf("no header expected when opts empty, got:\n%s", got)
	}
}

func TestXLineDropped(t *testing.T) {
	raw := "G D\nhello world\nX\n"
	got := Convert(raw, HeaderOpts{})
	if strings.Contains(got, "X\n") || strings.Contains(got, "\nX") {
		t.Errorf("X end-marker should be dropped, got:\n%s", got)
	}
}

func TestInstrumentalLineDropped(t *testing.T) {
	raw := "G D\nhello world\n\n(Instrumental)\n\nAm C\nfoo bar\n"
	got := Convert(raw, HeaderOpts{})
	if strings.Contains(got, "Instrumental") {
		t.Errorf("(Instrumental) should be dropped, got:\n%s", got)
	}
}

func TestCapoBodyLine(t *testing.T) {
	cases := []struct {
		capo int
		want string
	}{
		{0, ""},
		{-1, ""},
		{4, "(Capo 4)"},
		{7, "(Capo 7)"},
	}
	for _, c := range cases {
		got := capoBodyLine(c.capo)
		if got != c.want {
			t.Errorf("capoBodyLine(%d) = %q, want %q", c.capo, got, c.want)
		}
	}
}

func TestEmitHeaderEmpty(t *testing.T) {
	if got := emitHeader(HeaderOpts{}); got != "" {
		t.Errorf("expected empty header, got %q", got)
	}
}

func TestEmitHeaderAllFields(t *testing.T) {
	opts := HeaderOpts{
		Title:    "Test",
		Artist:   "Band",
		Key:      "G",
		Capo:     4,
		Tempo:    120,
		Time:     "4/4",
		Duration: "3:30",
	}
	got := emitHeader(opts)
	want := "{title: Test}\n{artist: Band}\n{key: G}\n{capo: 4}\n{tempo: 120}\n{time: 4/4}\n{duration: 3:30}"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}
