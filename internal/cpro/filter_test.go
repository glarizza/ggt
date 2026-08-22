package cpro

import "testing"

func TestStripParens(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Gm", "Gm"},
		{"(Gm)", "Gm"},
		{"(Dsus2/C)", "Dsus2/C"},
		{"((G))", "(G)"},
		{"", ""},
		{"(", "("},
		{")", ")"},
	}
	for _, c := range cases {
		got := stripParens(c.in)
		if got != c.want {
			t.Errorf("stripParens(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsDropLineX(t *testing.T) {
	drops := []string{"X", "X    ", "    X", " X"}
	for _, l := range drops {
		if !isDropLine(l) {
			t.Errorf("expected isDropLine(%q) = true, got false", l)
		}
	}
	noDrops := []string{"xx", "Xx", "XYZ", "abcX", "X abc"}
	for _, l := range noDrops {
		if isDropLine(l) {
			t.Errorf("expected isDropLine(%q) = false, got true", l)
		}
	}
}

func TestIsDropLineInstrumental(t *testing.T) {
	drops := []string{"(Instrumental)", "   (Instrumental)   "}
	for _, l := range drops {
		if !isDropLine(l) {
			t.Errorf("expected isDropLine(%q) = true, got false", l)
		}
	}
	noDrops := []string{"(Intro)", "(Verse 1)", "Instrumental", "instrumental"}
	for _, l := range noDrops {
		if isDropLine(l) {
			t.Errorf("expected isDropLine(%q) = false, got true", l)
		}
	}
}

func TestMaybeSectionBreak(t *testing.T) {
	// empty output → no-op
	var out []string
	maybeSectionBreak(&out)
	if len(out) != 0 {
		t.Errorf("expected no-op on empty out, got: %v", out)
	}

	// single content line → appends blank
	out = []string{"hello"}
	maybeSectionBreak(&out)
	if len(out) != 2 || out[1] != "" {
		t.Errorf("expected blank appended, got: %v", out)
	}

	// last line already blank → no-op
	out = []string{"hello", ""}
	maybeSectionBreak(&out)
	if len(out) != 2 {
		t.Errorf("expected no-op when last blank, got: %v", out)
	}
}
