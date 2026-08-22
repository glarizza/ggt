package main

import (
	"bytes"
	"strings"
	"testing"
)

// execCpr runs a fresh cpro command with the given stdin and args, returning
// combined stdout. Mirrors execTranspose.
func execCpr(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	cmd := newCProCmd()
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

// TestCprCapoKeepsCapoHeader: a bare --capo N emits {capo: N} and never the
// human "(Capo N)" body line.
func TestCprCapoKeepsCapoHeader(t *testing.T) {
	stdin := "C\nhello world\nF\nnext line\n"
	out := execCpr(t, stdin, "-", "--key", "E", "--capo", "4", "--title", "Test")
	if !strings.Contains(out, "{capo: 4}") {
		t.Errorf("bare --capo 4 should keep {capo: 4} header:\n%s", out)
	}
	if strings.Contains(out, "(Capo") {
		t.Errorf("the (Capo N) body line must never be emitted:\n%s", out)
	}
	// body is still in the playing key C (capo not removed)
	if !strings.Contains(out, "[C]hello") {
		t.Errorf("bare --capo must not shift the body (still C-shapes):\n%s", out)
	}
}

// TestCprRemoveCapoShiftsBody: --capo N --remove-capo shifts body +N to the
// sounding key, keeps {key: E}, and drops {capo}.
func TestCprRemoveCapoShiftsBody(t *testing.T) {
	stdin := "C\nhello world\nF\nnext line\n"
	out := execCpr(t, stdin, "-", "--key", "E", "--capo", "4", "--remove-capo")
	// key stays the declared sounding key
	if !strings.Contains(out, "{key: E}") {
		t.Errorf("{key: E} must be preserved:\n%s", out)
	}
	// body shifted +4: C->E, F->A (diatonic sharps spelling)
	if !strings.Contains(out, "[E]hello world") {
		t.Errorf("C must shift to E under --remove-capo 4:\n%s", out)
	}
	if !strings.Contains(out, "[A]next line") {
		t.Errorf("F must shift to A under --remove-capo 4:\n%s", out)
	}
	// {capo} must be gone -- the chart is the native no-capo version
	if strings.Contains(out, "{capo") {
		t.Errorf("--remove-capo must drop {capo: N} header:\n%s", out)
	}
	if strings.Contains(out, "(Capo") {
		t.Errorf("(Capo N) line must still never be emitted:\n%s", out)
	}
}

// TestCprRemoveCapoRequiresCapo: --remove-capo without --capo N errors.
func TestCprRemoveCapoRequiresCapo(t *testing.T) {
	cmd := newCProCmd()
	cmd.SetIn(bytes.NewBufferString("C\nhello\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-", "--key", "E", "--remove-capo"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error when --remove-capo is used without --capo N")
	}
}

// TestCprNoCapoNoHeaderFields: no --capo at all => no {capo} and no (Capo N).
func TestCprNoCapoNoHeaderFields(t *testing.T) {
	stdin := "C\nhello\n"
	out := execCpr(t, stdin, "-", "--key", "E")
	if strings.Contains(out, "{capo") || strings.Contains(out, "(Capo") {
		t.Errorf("no capo requested, so neither {capo} nor (Capo N):\n%s", out)
	}
}
