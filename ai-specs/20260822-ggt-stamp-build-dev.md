# ggt: stamp the default `make build` + local/release mode + semver discipline

**Status:** IMPLEMENTED, 2026-08-22.  See §9.
**Author:** AI assistant.
**Date:** 2026-08-22.
**Amends:** `20260821-ggt-version.md` — specifically its `§2.3` / `§9.3`
decision that **`make build` stays UN-stamped** (honest `dev`). That decision is
reversed: a default build should be *traceable*, not anonymous.

---

## 1. Problem statement

The user's use case, in their own words:

> if we do local builds with `make build` I STILL want to know what semantic
> version this local build is based on … a local build where VERSION is 0.1.0
> or later in time when we're at like 1.5.3 … I need some way to trace that
> back. It would also be nice to get a short commit of HEAD for a local build.
> The use case is me coming back after you've implemented a feature and not
> knowing whether the binary at `./bin/ggt` includes this new feature or not.
>
> Related — we should begin incrementing the minor version with every new
> feature and patch when we fix things. Let's start respecting semantic
> version a little bit while pre-1.0.0.

Two problems:

1. **`make build` stamps nothing.** Its recipe (`go build -o bin/ggt
   ./cmd/ggt`) has no `-ldflags`. The `VERSION` / `GITCOMMIT` / `BUILDDATE` /
   `LDFLAGS` vars are *computed* at the top of the Makefile but only the
   separate `build-stamp` target uses them. So `./bin/ggt` from `make build`
   reports `ggt dev (commit unknown, un-stamped)` — it cannot tell you which
   version era or commit it is, defeating the "does this binary have the new
   feature?" check.
2. **No versioning discipline.** The version-bump targets exist but nothing
   says *when* to bump. The user wants: **minor per new feature, patch per fix**,
   while pre-1.0.0 (minor is effectively "free" here). `--remove-capo`
   (commit `57f6076`) is a new feature that did not bump the version; VERSION
   is still `0.1.0`.

## 2. Approach

### 2.1 The default `make build` now stamps

`make build` becomes the stamped, dev-mode build. It injects the same three
`-X` fields `build-stamp` already used, **plus a new build-mode field**:

| build path | `Version` | `GitCommit` | `BuildDate` | `Build` (mode) |
|---|---|---|---|---|
| bare `go build ./cmd/ggt` (no make) | `dev` | `unknown` | `""` | `""` → *source (un-stamped)* |
| **`make build`** | `0.2.0` (from `VERSION` file) | `git rev-parse --short HEAD` | RFC3339 now | **`dev`** → *local build* |
| `goreleaser` / CI at a tag | `0.2.0` | tag short sha | date | **`release`** |

So:

- The honest "I did a raw source compile" path is **preserved** by bare
   `go build` (still `dev / unknown / source`); the makefile build is now the
   useful one for day-to-day work.
- `./bin/ggt` from `make build` reports `ggt 0.2.0 (commit 3f2a1b4, built
   2026-.., local build)` — **fully traceable**: you always know the version
   baseline AND the exact commit, so you can see whether it postdates a
   feature. The short commit is the strongest "is this current?" tell, and it
   was **already computed** (`GITCOMMIT` at line 9) — wiring it in is a
   one-word change to the `build` recipe, not the "bigger lift" The user suspected.
- We can retire the now-redundant `build-stamp` target (or keep it as an
   alias). Recommendation: **fold it into `build`** — one fewer target to
   remember.

### 2.2 A new build-mode field (the "release vs local" disambiguator)

Add a 4th stamp-able var. **Decision the doc locks:** the semantic version
string stays **pure** (`0.2.0`, no `-dev` suffix) — a `-dev` suffixed string
would corrupt the very SemVer value we're trying to preserve and would confuse
goreleaser's `{{ .Version }}`. Instead, the *mode* is a separate field
printed on its own `build:` line in `ggt version`, and the one-liner
(`-v`/`--version`) stays a clean `ggt 0.2.0 (commit …, built …)`.

```go
// Build-able. Overwritten by -X at build time.
var (
    Version   = "dev"      // semver (VERSION file); "dev" when source-built
    GitCommit = "unknown"  // short head sha; "unknown" when source-built
    BuildDate = ""          // RFC3339; "" when source-built
    Build     = "source"   // "source" | "dev" (make build) | "release" (goreleaser)
)
```

`stampNote()` maps the mode to human words:

| `Build` | `ggt version` `build:` line |
|---|---|
| `""` / `source` | `source (un-stamped)` |
| `dev` | `local build (VERSION baseline + HEAD commit)` |
| `release` | `release` |

`ggt version` after this change (a `make build` binary):

```
ggt 0.2.0
  commit:   3f2a1b4
  built:    2026-08-22T…Z
  go:       go1.26.3
  platform: darwin/arm64
  build:    local build (VERSION baseline + HEAD commit)
```

### 2.3 Semver discipline (pre-1.0.0)

Record the convention (The user's words: "a LITTLE BIT"):

- **New feature → bump minor** (…0.1.0 → 0.2.0 → 0.3.0…).  Cheap and safe
   while pre-1.0.0.
- **Bug fix → bump patch** (…0.2.0 → 0.2.1).
- **Major** when we break the public CLI contract — not expected pre-1.0.0, but
   `version-major` stays for the day it is.
- Bump at the same time as the code, **in the same commit**, so the version
   and the feature ship together and the stamp always points at what you
   actually ran.
- Bump command: `make version-minor` (feature) / `make version-patch` (fix);
   then `make build` stamps the new version.

**First application:** the `--remove-capo` feature just landed (commit
`57f6076`) without a bump. This change applies `make version-minor`, taking
VERSION `0.1.0 → 0.2.0`, in the same commit.

## 3. Files to touch

| Path | Change |
|---|---|
| `Makefile` | `build:` gains `-ldflags "$(LDFLAGS)"`; add `-X …Build=dev` to `LDFLAGS`; retarget help; retire/alias `build-stamp`; keep `build` as the default |
| `internal/version/version.go` | add `Build` var; `stampNote()` maps the three modes; keep version string pure |
| `internal/version/version_test.go` | add/adjust cases: source, dev(make), release; version string stays pure |
| `.goreleaser.yml` | add `-X …Build=release` to the ldflags |
| `VERSION` | `0.1.0` → `0.2.0` (the `--remove-capo` feature) |
| `AGENTS.md` (propose) | add a one-paragraph *Semver discipline* note so every future session bumps the right field |
| `ai-specs/20260822-ggt-stamp-build-dev.md` | this doc |

## 4. Decisions captured

- **Reverses** the original "keep `make build` un-stamped" choice
   (`20260821-ggt-version.md §2.3/§9.3`): traceability now beats anonymity for
   the day-to-day build; the honest source build is preserved by bare
   `go build` (no make, no ldflags).
- **Short commit IS cheap**: `GITCOMMIT` was already in the Makefile, just not
   wired into `build`. No git plumbing needed — it already has a `|| unknown`
   fallback for no-`.git` builds.
- **Pure version string**; the "local vs release" distinction lives in a
   separate `Build` mode field, not a SemVer suffix.
- **Semver discipline** = minor-per-feature / patch-per-fix, bumped in the same
   commit as the code, while pre-1.0.0.

## 5. Out of scope

- GitHub Actions / CI release wiring (deferred, per the original spec).
- Cross-compile matrix (deferred).
- `ggt about` (later).
- Enforcing the discipline in CI (lint gate that blocks an un-bumped
   VERSION); a person/agent runs `make version-minor|patch` by hand.

## 6. Risks

- **`Stamped()` semantics shift**: today `Stamped() = Version != "dev"`. After
   this, a `make build` returns `Stamped() == true` (it *is* stamped). The
   "release-ness" check should key off `Build == "release"`, not `Stamped()`.
   Verify no caller relies on `Stamped()` meaning "is a release".
- **A developer who forgets to `make build`** after a bump still runs a stale
   binary — the whole point is the stamp tells them; no programmatic guard.
- **`VERSION` with a trailing newline** — `LDFLAGS` uses `cat | tr -d '\n'`
   already; keep it.

## 7. Tests / acceptance

1. `make build && bin/ggt version` prints `0.2.0`, a **real short commit**,
   a real build date, and `build: local build …`.
2. **The stale-binary tell:** bump VERSION to `0.2.0`, run `make version-minor`
   to `0.3.0`, `make build`, and confirm the binary now reports `0.3.0` + the
   new commit — i.e. you can see the bump from the binary itself.
3. Bare `go build ./cmd/ggt && ./ggt version` still prints the honest
   `dev / unknown / source (un-stamped)` — the un-stamped path survives.
4. `goreleaser` config has `-X …Build=release` (grep-gate on `.goreleaser.yml`).
5. `go test ./internal/version/...` covers source / dev / release modes.
6. VERSION file reads `0.2.0`; grep-gate: the literal `0.1.0` should NOT
   remain in a tracked `.go` (only the test fixture that checks "0.1.0 doesn't
   leak" may mention it).
7. `go build / go vet / go test ./...` all green.

## 8. Verification log

(empty — pending implementation.)
