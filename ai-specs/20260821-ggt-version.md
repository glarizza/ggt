# ggt version stamping & reporting (design doc)

**Status:** DRAFT, for review. No code yet.
**Author:** AI assistant.
**Date:** 2026-08-21.
**Companion to (not a replacement):** `20260819-ggt-design.md`,
`20260821-ggt-transpose.md`.

---

## 1. Problem statement

Today every `ggt` binary is anonymous. You cannot tell:

- **which version** a running binary is, or
- **which commit** it was built from, or
- **whether it is an official release or a local/CI build**.

There is no `VERSION` file, no version flags, no go-releaser config, and the
`Makefile` build target does not stamp anything.  The user's stated need:

1. A single, hand-editable *semantic-version* file at the repo root,
   holding just `0.1.0` to start.
2. That version, plus the commit and build date, **baked into the binary**
    via `ldflags` at build time (as go-releaser would do), so a freshly built
   binary reports itself without the source tree.
3. A way to query it: `ggt -v` and/or `ggt version`.

The goal: **from a bare binary you can always answer
`what version, what commit, was this a release or a local build`.**

## 2. Approach

Three moving parts, each doing one thing and composing cleanly:

```
   VERSION            (repo root, hand-edited, sole human source of truth)
   git HEAD          (the source of truth for "which commit")
   @BUILD_TIME       (injected only at release/CI build)         \
        |                                                       v
        +------------>   stamping via -ldflags -X   ---->   version package
                                                          (ggt/internal/version)
                                                                   |
                                                                   v
                                   ggt version  /  --version / -v  <--- reports it
```

### 2.1 The human source: `VERSION`

A file `VERSION` at the repo root whose **entire** content is a
[semantic version](https://semver.org) string, current: **`0.1.0`**.
No trailing newline concerns; the stamping step trims it.  This is the only
place a human edits the version number; everything else reads it.

### 2.2 The stamp-able vars: a dedicated `version` package

New file `internal/version/version.go`:

```go
package version

// Build-stamp fields. They default to "unset"-style sentinels so that a
// source-built binary (`go build` with no -ldflags) honestly reports that it
// is *not* a stamped release, rather than lying with a zero value.
var (
    Version   = "dev"        // semantic version, stamped from VERSION on release
    GitCommit = "unknown"    // short commit hash (git --short), stamped at release/CI
    BuildDate = ""           // RFC3339 build time; empty when not stamped
)

// Full returns a single human-readable line, e.g.
//   "ggt 0.1.0 (commit 379be26, built 2026-08-21T12:00:00Z)"
//   "ggt dev (commit unknown, built (unstamped))"
```

This is the **only** place the binary reads its identity.  It is importable,
so both `cmd/ggt` and any future binary share one source.

**Why a package rather than vars in `cmd/ggt/main.go`:** your requested
ldflags targeted `ggt/cmd.Version`, but the real, importable layout is
module `ggt` with the command at import path `ggt/cmd/ggt` (`package main`).
A tiny `ggt/internal/version` package is importable by anything and gives
`-ldflags` an unambiguous target: `ggt/internal/version.Version`,
…`GitCommit`, …`BuildDate`.  This honors the *intent* of a "cmd-level version
package" while keeping the stamp target a real, valid import path.

### 2.3 Stamping: Makefile (local) + go-releaser (release)

**Local/CI build, `Makefile`:**

```
VERSION   ?= $(shell cat VERSION 2>/dev/null || echo 0.0.0)
GITCOMMIT ?= $(shell git rev-parse --short HEAD)
BUILDDATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w \
             -X ggt/internal/version.Version=$(VERSION) \
             -X ggt/internal/version.GitCommit=$(GITCOMMIT) \
             -X ggt/internal/version.BuildDate=$(BUILDDATE)

build:                 # dev build, UNSTAMPED (honest "dev" report)
    go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

build-stamp:          # dev build WITH stamp, using current git+date+VERSION
    go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)
```

`build` stays un-stamped so a plain `go build`/`make build` does not fake a
release.  `build-stamp` is what a developer runs when they want to *know* what
they built.  `-s -w` strips debug symbols, matching release behaviour.

**Release build, `.goreleaser.yml`:** the same three ldflags, using
go-releaser's built-in `{{ .Version }}`, `{{ .ShortCommit }}`, `{{ .Date }}`
(added `{{ .Commit }}` is available if a full hash is ever wanted):

```yaml
# .goreleaser.yml
version: 2
builds:
  - id: ggt
    main: ./cmd/ggt
    binary: ggt
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - "-X ggt/internal/version.Version={{ .Version }}"
      - "-X ggt/internal/version.GitCommit={{ .ShortCommit }}"
      - "-X ggt/internal/version.BuildDate={{ .Date }}"
```

go-releaser reads the project version from the `VERSION` file by default
(v2: `project.version` / the `VERSION` file), so the human source and the
release binary never diverge.

### 2.4 Reporting: `ggt version` AND a shortcut

- **`ggt version`** — a new subcommand.  Prints the full multi-line identity:
   version, commit, build date, go version (`runtime.Version()`), OS/arch,
   and whether the build is *stamped* (release/CI) or *local/unstamped*.
- **`--version` / `-v`** — a short, single-line report (just `version (commit,
   built)`).  Wired via a persistent flag on the root command so **every**
   subcommand inherits `ggt <sub> --version`.

  Design choice: cobra auto-registers `--version` when `rootCmd.Version` is
  set to the one-liner.  We additionally register `-v` as its short alias via
   a persistent flag so `ggt -v` and `ggt --version` both work, satisfying
  "either … or both" with both.

Both read **only** from the `version` package, so the number you get out is
exactly the one baked in.

### 2.5 "Release vs local, off a commit"

The discriminator is the stamp sentinel set:

| Field     | release/CI (`build-stamp` / go-releaser) | plain `go build` (`build`) |
|-----------|-----------------------------------------|-----------------------------|
| Version   | `0.1.0` (from `VERSION`)                | `dev`                       |
| GitCommit | `379be26` (git --short default)     | `unknown`                   |
| BuildDate | `2026-08-21T…Z`                         | `""` → `(unstamped)`        |

`version.Full()` prints the honest sentinel form when unstamped, so you never
mistake a local build for a release.

### 2.6 Bumping the version

Three Makefile targets rewrite `VERSION` in place (the bump itself does **not**
stamp or commit — the developer commits the new `VERSION` manually so the
change is reviewable, and stamps on the next build).  These are Gary’s
proven recipes from other projects; the only project-specific bits are the
`VERSION` filename and the `cut -d.` splitting, which are identical here:

```make
.PHONY: version-patch
version-patch: # Bump patch version (0.1.0 -> 0.1.1)
	@current=$$(cat VERSION | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      minor=$$(echo $$current | cut -d. -f2); \
      patch=$$(echo $$current | cut -d. -f3); \
      new_patch=$$((patch + 1)); \
      new_version="$$major.$$minor.$$new_patch"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > VERSION; \
      echo "Version updated to $$new_version"

.PHONY: version-minor
version-minor: # Bump minor version (0.1.0 -> 0.2.0)
	@current=$$(cat VERSION | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      minor=$$(echo $$current | cut -d. -f2); \
      new_minor=$$((minor + 1)); \
      new_version="$$major.$$new_minor.0"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > VERSION; \
      echo "Version updated to $$new_version"

.PHONY: version-major
version-major: # Bump major version (0.1.0 -> 1.0.0)
	@current=$$(cat VERSION | tr -d '\n'); \
      major=$$(echo $$current | cut -d. -f1); \
      new_major=$$((major + 1)); \
      new_version="$$new_major.0.0"; \
      echo "Bumping version: $$current -> $$new_version"; \
      echo $$new_version > VERSION; \
      echo "Version updated to $$new_version"
```

Note for implementers: a real `Makefile` needs **literal TAB** indentation
on the recipe lines (markdown renders them as spaces); the recipes above show
the tab explicitly so the transcription is unambiguous.
## 3. Files to touch

| Path                              | Change |
|---|---|
| `VERSION` (new, repo root) | `0.1.0` — the sole human-edited version source |
| `internal/version/version.go` (new) | the stamp-able `Version`/`GitCommit`/`BuildDate` + `Full()` |
| `cmd/ggt/version.go` (new) | `ggt version` subcommand (full report) + the `-v`/`--version` shortcut on root |
| `cmd/ggt/root.go` | set `rootCmd.Version` (one-liner) and call `newVersionCmd()` / add the `-v` persistent flag |
| `Makefile` | add `VERSION`/`GITCOMMIT`/`BUILDDATE`/`LDFLAGS` vars; a `build-stamp` target; and the `version-patch` / `version-minor` / `version-major` bump targets (§2.6); keep `build` un-stamped |
| `.goreleaser.yml` (new) | minimal v2 config as above |
| `internal/version/version_test.go` (new) | `Full()` formatting for stamped vs unstamped |

Total surface: ~5 files, one new dir.

## 4. Decisions captured

- **Single source of truth:** `VERSION` (human) → stamped at build → reported.
   `VERSION` is the number a human changes; the binary never holds one at
   source-build time (it says `dev`).
- **Honest sentinels** (`dev` / `unknown` / unstamped) over zero-values, so
   a local build cannot masquerade as a release.
- **ldflags target** corrected to the real import path
    `ggt/internal/version.*` (yours said `ggt/cmd.Version`; the package lives
   here so the target is valid and reusable).
- **Both `ggt version` (rich) and `-v`/`--version` (short)** — satisfies
   "either or both" with both, and the shortcut is inherited by subcommands.
- **Commit hash:** whatever `git rev-parse --short HEAD` reports (git's own
   abbreviated length — currently 7 chars like `379be26`); go-releaser's
    `{{ .ShortCommit }}` matches it. The full 40-char `{{ .Commit }}` stays
   available if ever needed.

## 5. Out of scope / future

- Cross-compile matrix (windows-freebsd, etc.): add a `gomatix`/matrix later
    when first actually shipping to other OSes.
- `ggt about` (human-facing description, license, homepage): later, separate
   command.
- Signing / checksums: go-releaser handles this at release-config time, not in
   the core; deferred until release infra exists.
- CI wiring for `build-stamp` (GitHub Actions): out of scope for this change.

## 6. Risks

- **`-ldflags` string with spaces in `.goreleaser.yml`:** must be quoted
    (`"-X …"`) or go-releaser splits on spaces.  Noted in §2.3.
- **`VERSION` with a trailing newline / BOM:** strip in the `Version =`
  stamping step and in `cat` via `tr -d`.  Tested.
- **`git` unavailable at build time** (shallow/no-.git env): fallback
  sentinels (`unknown`) instead of a broken build.  Noted in §2.3 Makefile.
- **Cobra `Version` one-liner vs `version` subcommand mismatch:** keep both
   reading the SAME `version.Full()`/`version.Version`, so they can never
   disagree.

## 7. Tests / acceptance

1. `go build ./cmd/ggt` (plain) → `ggt version` reports `dev / unknown /
    unstamped`; `ggt -v` reports `ggt dev`.
2. `make build-stamp && bin/ggt version` reports the `VERSION`-file version,
    a short commit hash (git `--short`), and a real timestamp.
3. `bin/ggt version --help` lists nothing surprising; `bin/ggt chopro --version`
    inherits the shortcut.
4. `go test ./internal/version/...` covers stamped + unstamped `Full()`.
5. No `ggt` subcommand prints a hard-coded version literal anywhere (grep-gate:
   the literal `0.1.0` appears only in `VERSION`, not in `.go` sources).
6. **Manual end-to-end:** build-stamp, then `bin/ggt version` shows a
    release-looking line; plain-build shows the honest `dev` line.
## 8. Open questions for the user (RESOLVED)

All resolved in the user’s review pass; kept here for the record:

- **Q1 (commit hash):** use whatever `git rev-parse --short HEAD` reports
    (git’s own abbreviated length — currently 7 chars like `379be26`).  [DONE]
- **Q2 (un-stamped `BuildDate`):** empty string, rendered as `(unstamped)`.  [DONE]
- **Q3 (inheritance):** `-v`/`--version` is a *persistent* root flag, so every
    subcommand — the conversion stage, `ggt version`, and the rest — inherits
    it, with no per-subcommand work.  [DONE]
- **Q4 (`-v` vs `ggt version`):** both — `-v`/`--version` is the one-liner,
    `ggt version` is the rich multi-line report.  [DONE]
- **Q5 (git-tracking `VERSION`):** yes; the human source of truth, at the
    repo root, tracked.  [DONE]

### Added by the user in the same pass

- `ggt` is **Makefile-driven** for build/stamp/bump and **go-releaser**-driven
   for release.
- Three version-bump targets — `version-patch` / `version-minor` /
    `version-major` — from Gary’s recipe (detailed in §2.6).  [DRAFTED,
   not yet transcribed into the live Makefile.]

---

## 9. Implementation status & notes (post-build)

Implemented, built, and exercised end to end on 2026-08-22.  All tests,
`go vet`, and the bump/stamp targets run clean.  Two deliberate refinements
over the pre-build draft, noted for the record:

### 9.1 One-line report: our own flag, not Cobra's `rootCmd.Version`

The draft (2.4) leaned on Cobra's native `--version` (via
`rootCmd.Version`).  That prints `"ggt version ggt dev (...)"` — the root
command name and the one-liner collide.  Instead the implementation:

- registers the persistent flag **`-v, --version`** itself
    (`BoolVarP(&showShort, "version", "v", …)`);
- in `rootCmd.PersistentPreRunE`, when that flag is set, prints
   `version.Short()` and exits through an overridable `exitFn` (so tests
   stub the exit);
- adds a `rootCmd.RunE` that prints help, so a bare `ggt`/`ggt --version`
   with no subcommand still routes correctly.

Result: `ggt -v`, `ggt --version`, and (inherited) `ggt transpose --version`
all print the one-liner; `ggt version` prints the rich multi-line report;
bare `ggt` still prints help.  No double-name.

### 9.2 `internal/version`

- `Version` / `GitCommit` / `BuildDate` stamp-able vars, defaulting to
    `"dev"` / `"unknown"` / `""`.
    `Stamped()` = `Version != "dev"`; `builtString()` returns the date or the
   sentinel `"un-stamped"`; `Short()` one-liner, `Lines()` rich.
- Tests cover stamped vs un-stamped formatting.

Note: the pre-build `Short()` used `"(un-stamped)"` (with parens); the
implemented and tested form is the bare sentinel `"un-stamped"` — the test
(`TestShort_Unstamped`) pins this.

### 9.3 `Makefile`

- `VERSION` / `GITCOMMIT` / `BUILDDATE` / `LDFLAGS` vars; `VERSION` from
   `VERSION_FILE := VERSION` via `shell cat … || 0.0.0`; `GITCOMMIT` via
   `git rev-parse --short HEAD || unknown` (yours: "whatever git reports").
- `build` = un-stamped dev build (honest `dev`); `build-stamp` injects the
   three `-X` flags.   Verified: `make build-stamp && bin/ggt version`
   reports `0.1.0` + the live short commit + a real timestamp +
   `release/stamped`.
- `version-patch` / `version-minor` / `version-major` rewrite `VERSION` with
   your recipes.  Verified in sequence: `0.1.0 -> 0.1.1 -> 0.1.2 ->
   0.2.0 -> 1.0.0`, then restored to `0.1.0`.
- Recipe note: the first physical line of each bump recipe carries the tab;
   the `; \`-joined continuation lines are indented with spaces (they are
   part of the same recipe line and take no tab of their own).

### 9.4 `.goreleaser.yml`

v2 config, three `-X` ldflags from `{{ .Version }}` / `{{ .ShortCommit }}` /
`{{ .Date }}`, `CGO_ENABLED=0`, a `tar.gz` archive, changelog on.  Not yet
run through an actual release (no release infra / CI yet — out of scope for
this change per 5).

---

## 10. Open questions (resolved)

- **Q1** commit hash — whatever `git rev-parse --short HEAD` reports.   DONE.
- **Q2** un-stamped `BuildDate` — empty string, rendered `un-stamped`.   DONE.
- **Q3** inheritance — persistent root flag, inherited by all subcommands;
   `ggt transpose --version` confirmed.   DONE.
- **Q4** `-v` vs `ggt version` — both: `-v`/`--version` one-liner,
    `ggt version` rich.   DONE.
- **Q5** `VERSION` git-tracked — yes, at repo root.   DONE.
- **Q6 (user-added)** Makefile to drive everything, incl. the three
   version-bump targets; go-releaser for release.   DONE.

---

## 11. Verification log (2026-08-22)

- `go test ./...` — all packages pass, incl. `internal/version`.
- `go vet ./...` — clean.
- `ggt -v` / `ggt --version` / `ggt transpose --version` — all one-liner.
- `ggt version` — rich multi-line, incl. go version + platform + build mode.
- `make build-stamp && bin/ggt version` — reports `0.1.0`, live commit,
    real build date, `release/stamped`.
- `make version-patch/minor/major` — bump math correct end to end.
- Whole-tree bad-token sweep — 0 variants; good token intact.
