// Package version holds the build-stamp fields and the helpers that format
// them.
//
// The vars default to "unset"-style sentinels so that a source-built binary
// (`go build` with no -ldflags) honestly reports that it is NOT a release,
// rather than lying with a zero/empty value.
//
// At local or release build time the values are injected with -ldflags -X,
// for example a `make build` (dev mode) runs:
//
//	go build -ldflags "-X ggt/internal/version.Version=0.2.0 \
//	                    -X ggt/internal/version.GitCommit=3f2a1b4 \
//	                    -X ggt/internal/version.BuildDate=2026-08-22T12:00:00Z \
//	                    -X ggt/internal/version.Build=dev"
package version

import "fmt"

// Stamp-able fields. Overwritten by -ldflags -X at build time.
var (
	Version   = "dev"     // semantic version (VERSION file); "dev" when source-built
	GitCommit = "unknown" // git --short hash; "unknown" when source-built
	BuildDate = ""        // RFC3339 build time; "" when source-built
	Build     = "source"  // "source" | "dev" (make build) | "release" (goreleaser)
)

// Stamped reports whether this binary carries a real version stamp
// (i.e. its semantic VERSION was not left at the default "dev"). A `make build`
// binary IS stamped; a bare `go build` is not. Stamped() does NOT distinguish a
// release from a local build; that is the job of Build / stampNote()
// (design doc 20260822-ggt-stamp-build-dev.md).
func Stamped() bool { return Version != "dev" }

// builtString is BuildDate, or the sentinel "un-stamped" when it was not
// injected at build time.
func builtString() string {
	if BuildDate == "" {
		return "un-stamped"
	}
	return BuildDate
}

// stampNote classifies the build by its mode. The three ways ggt is built are:
//
//	"source"  a bare `go build` (no make / no ldflags)    -> "source (un-stamped)"
//	"dev"     `make build` (VERSION + HEAD commit + date)  -> "local build"
//	"release" goreleaser / CI at a tag                     -> "release"
//
// The mode lives in a separate field so the semantic version string can stay
// pure (no -dev suffix); the build line is how a human tells a local dev build
// apart from an official release at a glance.
func stampNote() string {
	switch Build {
	case "release":
		return "release"
	case "dev":
		return "local build"
	default:
		// "source" or an un-injected mode var. If a semantic version / commit
		// stamp got injected but the mode did not, say so honestly rather than
		// claiming "release".
		if Stamped() {
			return "stamped (unmarked mode)"
		}
		return "source (un-stamped)"
	}
}

// Short is the one-line identity, used by the -v / --version reports and by the
// root command's version field. It stays free of the build mode so the semantic
// version is never polluted with a -dev suffix.
func Short() string {
	return fmt.Sprintf("ggt %s (commit %s, %s)", Version, GitCommit, builtString())
}

// Lines is the rich multi-line identity, printed by the `ggt version`
// subcommand. It includes the go runtime and platform so a bare binary is fully
// auditable without the source tree, plus a build-mode line. The caller supplies
// the runtime-derived values so this package stays free of an import on
// `runtime` and so tests can assert exact formatting.
func Lines(goVersion, goos, goarch string) []string {
	return []string{
		"ggt " + Version,
		"  commit:      " + GitCommit,
		"  built:       " + builtString(),
		"  go:          " + goVersion,
		"  platform:    " + goos + "/" + goarch,
		"  build:       " + stampNote(),
	}
}
