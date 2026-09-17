package main

import (
	"bytes"
	"strings"
	"testing"
)

// execTranspose runs a fresh transpose command with the given stdin and
// args, returning combined stdout.
func execTranspose(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	cmd := newTransposeCmd()
	cmd.SetIn(bytes.NewBufferString(stdin))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return buf.String()
}

func TestTransposeCmdToKey(t *testing.T) {
	stdin := "{key: E}\n[C] a [G] b [F#7] c\n"
	out := execTranspose(t, stdin, "-", "--to-key", "G")
	if !strings.Contains(out, "{key: G}") {
		t.Errorf("key header not rewritten to G:\n%s", out)
	}
	if !strings.Contains(out, "[A7] c") {
		t.Errorf("F#7 did not transpose to A7:\n%s", out)
	}
}

func TestTransposeCmdRefusesToKeyWithoutHeader(t *testing.T) {
	stdin := "[C] a [G] b\n"
	cmd := newTransposeCmd()
	cmd.SetIn(bytes.NewBufferString(stdin))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-", "--to-key", "G"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when --to-key has no {key:} header")
	}
}

func TestTransposeCmdMutualExclusion(t *testing.T) {
	stdin := "{key: E}\n[C]\n"
	cmd := newTransposeCmd()
	cmd.SetIn(bytes.NewBufferString(stdin))
	cmd.SetOut(&bytes.Buffer{})
	args := []string{"-", "--up", "2", "--down", "2"}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when --up and --down are both set")
	}
}

// TestTransposeDoesNotCorruptSectionTags is the §2.3 CLI regression: a bracketed
// section header carrying trailing text, run through the real transpose command
// under a nonzero shift, must survive verbatim -- not be mangled into "Eridge".
// The bare-tag case in TestTransposeCProBody passes today and misses this bug,
// so it deliberately includes the trailing cue and a nonzero shift.
func TestTransposeDoesNotCorruptSectionTags(t *testing.T) {
	stdin := "{key: C}\n[Bridge] (All played as bar chords)\nC F G\nbridge lyric\n"
	out := execTranspose(t, stdin, "-", "--up", "5")
	// +5 sends the B-root of "Bridge" to E, so the corruption would read
	// "Eridge" -- not the bare fragment "ridge", which also appears in "Bridge".
	if strings.Contains(out, "Eridge") {
		t.Errorf("Bridge header was transposed into Eridge:\n%s", out)
	}
	if !strings.Contains(out, "[Bridge] (All played as bar chords)") {
		t.Errorf("Bridge tag should survive verbatim:\n%s", out)
	}
}
