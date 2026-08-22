package version

import (
	"strings"
	"testing"
)

// reset sets the four stamp vars to a known base so tests are order-independent.
func reset(ver, commit, built, mode string) {
	Version = ver
	GitCommit = commit
	BuildDate = built
	Build = mode
}

func TestShort_Unstamped(t *testing.T) {
	reset("dev", "unknown", "", "source")
	got := Short()
	want := "ggt dev (commit unknown, un-stamped)"
	if got != want {
		t.Errorf("Short() unstamped:\n got %q\nwant %q", got, want)
	}
	if Stamped() {
		t.Error("default build should not report Stamped=true")
	}
}

func TestShort_Stamped(t *testing.T) {
	reset("0.1.0", "379be26", "2026-08-21T12:00:00Z", "dev")
	got := Short()
	want := "ggt 0.1.0 (commit 379be26, 2026-08-21T12:00:00Z)"
	if got != want {
		t.Errorf("Short() stamped:\n got %q\nwant %q", got, want)
	}
	if !Stamped() {
		t.Error("a non-dev version must report Stamped=true")
	}
	// the short one-liner must NOT carry the build mode or a -dev suffix
	if strings.Contains(got, "dev") || strings.Contains(got, "local") || strings.Contains(got, "release") {
		t.Errorf("Short() should stay mode-free: got %q", got)
	}
}

func TestLines_RoleRelease(t *testing.T) {
	reset("0.2.0", "3f2a1b4", "2026-08-22T12:00:00Z", "release")
	joined := strings.Join(Lines("go1.26.3", "linux", "amd64"), "\n")
	for _, want := range []string{
		"ggt 0.2.0",
		"commit:",
		"built:",
		"go:",
		"platform:",
		"build:",
		"release",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("release Lines() should contain %q in:\n%s", want, joined)
		}
	}
}

func TestLines_LocalDevBuild(t *testing.T) {
	reset("0.2.0", "3f2a1b4", "2026-08-22T12:00:00Z", "dev")
	joined := strings.Join(Lines("go1.26.3", "darwin", "arm64"), "\n")
	if !strings.Contains(joined, "local build") {
		t.Errorf("dev-mode Lines() should say 'local build':\n%s", joined)
	}
	if strings.Contains(joined, "release\n") {
		t.Errorf("a dev build must not read as a release:\n%s", joined)
	}
}

func TestLines_SourceUnstamped(t *testing.T) {
	reset("dev", "unknown", "", "source")
	joined := strings.Join(Lines("go1.26.3", "darwin", "arm64"), "\n")
	if !strings.Contains(joined, "source (un-stamped)") {
		t.Errorf("source Lines() should say 'source (un-stamped)':\n%s", joined)
	}
}

// TestStampModes pins the human wording of each build mode so `ggt version`
// reads differently for a source build, a `make build`, and a release.
func TestStampModes(t *testing.T) {
	cases := []struct {
		mode    string
		VERSION string
		want    string
	}{
		{"release", "0.2.0", "release"},
		{"dev", "0.2.0", "local build"},
		{"source", "dev", "source (un-stamped)"},
		{"", "dev", "source (un-stamped)"},
	}
	for _, c := range cases {
		reset(c.VERSION, "3f2a1b4", "2026-08-22T12:00:00Z", c.mode)
		joined := strings.Join(Lines("go1.26.3", "linux", "amd64"), "\n")
		if !strings.Contains(joined, c.want) {
			t.Errorf("mode %q (version %s) wanted %q in:\n%s", c.mode, c.VERSION, c.want, joined)
		}
	}
}

// TestStampedDistinguishes pins that Stamped() keys on VERSION only, and that a
// stamped version with an un-marked mode reports honestly (not "release").
func TestStampedDistinguishes(t *testing.T) {
	reset("dev", "unknown", "", "source")
	if Stamped() {
		t.Error("source build (VERSION=dev) must not be Stamped")
	}
	reset("0.2.0", "3f2a1b4", "2026-08-22T12:00:00Z", "source")
	if !Stamped() {
		t.Error("a stamped version must report Stamped=true even without a mode")
	}
}
