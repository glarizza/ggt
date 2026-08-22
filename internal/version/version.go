// Package version holds the build-stamp fields and the helpers that format
// them.
//
// The three vars default to "unset"-style sentinels so that a source-built
// binary (`go build` with no -ldflags) honestly reports that it is NOT a
// stamped release, rather than lying with a zero/empty value.
//
// At release or CI build time the values are injected with -ldflags -X, for
// example:
//
//	go build -ldflags "-X ggt/internal/version.Version=0.1.0 \
//	                   -X ggt/internal/version.GitCommit=379be26 \
//	                   -X ggt/internal/version.BuildDate=2026-08-21T12:00:00Z"
package version

import "fmt"

// Stamp-able fields. Overwritten by -ldflags -X at build time.
var (
	Version   = "dev"     // semantic version (ggt VERSION file); "dev" when un-stamped
	GitCommit = "unknown" // git --short hash; "unknown" when unstamped
	BuildDate = ""        // RFC3339 build time; "" when un-stamped
)

// IsStamped reports whether this binary carries a real release stamp
// (i.e. its Version was not left at the default "dev").
func Stamped() bool { return Version != "dev" }

// builtString is BuildDate, or the sentinel "(un-stamped)" when it was not
// injected at build time.
func builtString() string {
	if BuildDate == "" {
		return "un-stamped"
	}
	return BuildDate
}

// stampNote is the human word for the build's mode.
func stampNote() string {
	if Stamped() {
		return "release/stamped"
	}
	return "local/unstamped"
}

// Short is the one-line identity, used by the -v / --version reports and by
// the root command's Version field (which Cobra prints for --version).
func Short() string {
	return fmt.Sprintf("ggt %s (commit %s, %s)", Version, GitCommit, builtString())
}

// Lines is the rich multi-line identity, printed by the `ggt version`
// subcommand. It includes the go runtime and platform so a bare binary is
// fully auditable without the source tree.  The caller supplies the
// runtime-derived values so this package stays free of an import on
// runtime and so tests can assert exact formatting.
func Lines(goVersion, goos, goarch string) []string {
	return []string{
		"ggt " + Version,
		"  commit:   " + GitCommit,
		"  built:    " + builtString(),
		"  go:       " + goVersion,
		"  platform: " + goos + "/" + goarch,
		"  build:    " + stampNote(),
	}
}
