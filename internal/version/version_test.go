package version

import (
	"strings"
	"testing"
)

func TestShort_Unstamped(t *testing.T) {
    Version, GitCommit, BuildDate = "dev", "unknown", ""
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
    Version, GitCommit, BuildDate = "0.1.0", "379be26", "2026-08-21T12:00:00Z"
    got := Short()
    want := "ggt 0.1.0 (commit 379be26, 2026-08-21T12:00:00Z)"
    if got != want {
        t.Errorf("Short() stamped:\n got %q\nwant %q", got, want)
     }
     if !Stamped() {
        t.Error("a non-dev version must report Stamped=true")
     }
}

func TestLines_IncludesAllFields(t *testing.T) {
    Version, GitCommit, BuildDate = "0.1.0", "379be26", "2026-08-21T12:00:00Z"
    lines := Lines("go1.26.2", "linux", "amd64")
    joined := strings.Join(lines, "\n")
    for _, want := range []string{
        "ggt 0.1.0",
        "commit:   379be26",
        "built:    2026-08-21T12:00:00Z",
        "go:       go1.26.2",
        "platform: linux/amd64",
        "build:    release/stamped",
      } {
        if !strings.Contains(joined, want) {
            t.Errorf("Lines() missing %q in:\n%s", want, joined)
         }
     }
}

func TestLines_UnstampedNote(t *testing.T) {
    Version, GitCommit, BuildDate = "dev", "unknown", ""
    joined := strings.Join(Lines("go1.26.2", "darwin", "arm64"), "\n")
    if !strings.Contains(joined, "build:    local/stamped") &&
        !strings.Contains(joined, "build:    local/un-stamped") &&
        !strings.Contains(joined, "local") {
        t.Errorf("unstamped Lines should mention local/unstamped:\n%s", joined)
     }
}
