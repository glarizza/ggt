# ggt: release pipeline + README overhaul (final reconciled plan)

**Date:** 2026-09-17
**Status:** submitted for Until review (re-opened + re-submitted after Plan-check reconciliation).

Supersedes the earlier draft of this plan. This re-submission reconciles the plan
with the implementation that was actually shipped (the PR was implemented first,
then the user adjudicated every Plan-check difference). The decisions below are the
final, authoritative intent. The PR implements exactly this state.

## Context / why

Two separate, non-coupled workstreams, bundled here to avoid churning two plan
re-cycles (user override; they otherwise don't share code).

**Workstream A — release pipeline (0.5.1 → 0.5.2).** ggt currently has no
published release. A first cut publishes a 4-platform matrix of static Go
binaries to the GitHub Releases page, via a **human-driven `make release`**
(Goreleaser runs locally, not in CI). This first tag also captures the patch
already owed for two shipped bug fixes — PR #1 (`fix(cpro): do not transpose
section tags`, `0.5.0→0.5.1`) and the PR #2 `fix(cpro): stop bracketing
standalone lyric words as chords`, `0.5.1→0.5.2` — so 0.5.2 is the first
published release and VERSION lands at `0.5.2`. No new runtime behavior.

**Workstream B — README overhaul.** Rewrite `README.md` to document all four
subcommands (`ug`, `cpro`, `transpose`, `version`) with examples, rationale,
and a new install-from-GitHub-releases section.

**The `.chopro` extension correction (folds into B).** The real BandHelper
extension is `.chopro` (six bytes), *not* `.cpro` (four bytes); the committed
fixture `testdata/sample_02_expected.chopro` already uses it. This PR renames
the extension everywhere it actually appeared: `README.md` (33→0 `.cpro`
file-extension refs; subcommand `cpro` + package `internal/cpro` are KEPT, as
named), the `--help` help text + code comments across the `cmd`/`internal` Go
sources, `AGENTS.md`, and `scripts/README.md`. **No behavior change** — only
user-facing extension strings and comments.

**True origin (corrected in B).** ggt exists because a guitar player builds a
BandHelper setlist from online tab charts and ggt converts them to
BandHelper-ready `.chopro` files. It is *not* framed around a "metered /
Cloudflare / own-the-toolchain" story — that was a fabricated framing that has
been removed. `ug`'s network access is a normal part of the job, not a feature
to be defended.

## Invariants (all must hold; a failing check blocks merge)

1. No `deb`/`rpm`/Homebrew/S3/Vault/Doormat/dev-release-for-testing.
2. Go version in CI is `1.26` (matches `go.mod`).
3. No auto-release GitHub Actions workflow; no tag/Release from CI.
4. No `make release` target exists today — adding it is the change (human-run only).
5. No copyrighted material in tracked files. The 20260902 spec's earlier inline
   "Best of You" lyric excerpt was **redacted** (the recognizable lines replaced
   with redaction comments; the real input remains in the gitignored
   `ignored/best-of-you.*` set). `Confess` appears only as an illustrative token.
6. `--remove-capo` examples include the required `--capo N` (the command errors
   without it — confirmed at `cmd/ggt/cpro.go:59`).
7. Subcommand doc order is usage order: `ug` (incl. `ug fetch`) → `cpro`
   → `transpose` → `version`.
8. `.chopro` is the file extension everywhere; subcommand `cpro` + package
   `internal/cpro` are kept by name.

## Workstream A — release pipeline (code) — IMPLEMENTED

### 1. LICENSE
A standard MIT license file; `Copyright (c) 2026 Gary Larizza` is the **first
line**, followed by the standard MIT body (GitHub auto-detects MIT from the
body text). The `cpro.go`/package-name `cpro` are unchanged.

### 2. .goreleaser.yml
Narrowed to a fixed 4-platform matrix (goreleaser v2.9 syntax):
`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`. Each built with
`-tags release` and the git/VERSION stamps from the Makefile. `nolint`
annotations kept where required by `gci` (a `golangci-lint` tool in the
repo's own linters, not part of this change).

### 3. .github/workflows/ci.yml
PR-only, **no release job**. `on: pull_request:` with no `permissions:` other
than the write scopes the nudge job needs to post its updatable comment:
`contents: read, pull-requests: write, issues: write` (`issues: write` because
GitHub conflates PR-comment posting with the issues write scope — it is
required for the comment, not an expansion of blast radius).
- `path-filter` → `test`: runs `make test` (`go test ./...`) on
  `${{ needs.path-filter.outputs.run == 'true' }}`. The Go path filter is
  `**/cmd/**/*.go`, `**/internal/**/*.go`, `*.go`, `go.mod`, `go.sum` —
  **equivalent** to the earlier `**/*.go` intent (all Go in this repo lives
  under `cmd/`/`internal/`/root + the two module files; the precise form is
  stricter, not looser).
- `path-filter` → `version-nudge` (always, no path filter): posts/updatable a
  single marker comment `<!-- ggt-release -->` on every PR. Positive branch:
  `✅ VERSION bumped to \`$CURV\` — this PR is ready to cut a release with
  \`make release\` once it merges.` `CURV` is the raw `VERSION` file content
  (e.g. `0.5.2`); the `make release` target tags it as `v$CURV`. This message
  form is final.
- `path-filter` → `release` job: **does not exist** (out-of-scope; release is
  human-driven). No `GITHUB_TOKEN` for releases anywhere.

### 4. Makefile (new targets)
`release` (human-run only): read `VERSION` → `tag="v$(VERSION)"` → guard
(local + remote) → `git tag -a "$tag" -m "Release $tag"` → push →
`GITHUB_TOKEN=$(gh auth token) goreleaser release --clean`. The
`Releasing/Release/Released` messages print the single-`v` `$tag` (a cosmetic
`vv`-doubling bug was fixed in `release-dryrun`). Tag-existence guard is
mandatory; aborts if `v$(VERSION)` exists locally or on the remote.
`release-dryrun`: `goreleaser release --clean --snapshot` (no tag, no push, no
publish). An informational post-publish step (`gh release list --limit 1`)
confirms the latest release; a tag-specific assertion is a future hardening,
not required for 0.5.2.

### 5. VERSION: `0.5.1` → `0.5.2`.

## Workstream B — README overhaul + `.chopro` correction — IMPLEMENTED

### 1. Rewrite README.md
Subcommand table + "what it does" + "why it exists" per subcommand in usage
order: `ug` → `cpro` → `transpose` → `version`. Examples are illustrative (no
copyrighted lyrics — synthetic `la la la` / `Chosen`/`Fallen` placeholders).
Intentional simplifications (final, the user's decisions):
- **Flags**: the per-subcommand flag lists are *not* exhaustively documented;
  the README directs users to `ggt <subcommand> --help` for the full flag list.
  This is deliberate, not an omission to fix.
- **`ug`**: documents `fetch` + offline-clean only. It does **not** cover the
  UG "official Pro vs user-submitted" source distinction nor cite the user-tab
  design records (`20260823-ug-v2-user-tabs`, `20260825-…`) — the user
  considers those internal/implementation detail, not README-level material.
- **`cpro`**: documents the native-key (`.chopro`) conversion + `--capo`/
  `--remove-capo`; a concise description, not an exhaustive restatement of all
  internal cpro parsing rules.
- **[Bridge] protection**: the `transpose` section states that section headers
  (`[Bridge]`, `[Chorus]`, …) are detected and *never* rewritten (PR #1), tying
  the doc to the `20260916-…corruption.md` fix.
- **License**: a `## License` section points at the MIT `LICENSE`.
- **Stdin example**: the `transpose` stdin example was fixed — `cat in.chopro |
  ggt transpose --to-key F -o out.chopro` (stdin → new file) and a separate
  in-place form `ggt transpose in.chopro --to-key F --inplace`. The earlier
  `… --inplace in.chopro` variant silently read the named file and ignored the
  pipe (positional beats stdin); it is gone.
- **`cpro --capo 2` example**: shows the as-fretted playing-key body (correct —
  `--capo N` alone records `{capo: N}` without `--remove-capo`); `--remove-capo`
  is what lands the native key.
- **Origin**: the true purpose (convert online tab charts → BandHelper `.chopro`);
  the fabricated "metered / Cloudflare / own-the-toolchain vs rent" framing is
  removed; `ug`'s network access is a normal part of the job.
- **Prose is user-generic** ("you"/"a user"); the product name
  "Gary's Guitar Tool" is retained as a name (title, `--help` Short,
  AGENTS.md heading, LICENSE copyright). No third-person "Gary" in body prose.

### 2. `.chopro` extension correction (no behavior change)
The `--help` text + comments across `cmd/ggt/{cpro,transpose,main,root}.go` and
`internal/cpro/{chart,filter,placer}.go` were updated from `.cpro` to `.chopro`
because the **incorrect extension appeared in that user-facing text**. This is
why the PR touches `cmd`/`internal` despite WS1 originally scoping Go changes to
the Makefile — **the reason is the extension fix, documented here so it is an
acknowledged, intended part of the change.** Also updated: `AGENTS.md` and
`scripts/README.md` (`scripts/scramble_lyrics.py` already used `.chopro`).
Subcommand `cpro` and package `internal/cpro` are KEPT — that is not the
extension.

### 3. Historical ai-specs committed for history
`ai-specs/20260902-ggt-standalone-lyric-word-false-chord.md` and
`ai-specs/20260916-ggt-section-tag-transpose-corruption.md` are committed into
the PR tree so the design/bug-fixes trail is visible to readers (they were
written in `~/.until/plans` but not previously in the tracked repo). Both
already use `.chopro` and contain no cost/auth framing. The 20260902 spec's
"Best of You" lyric was **redacted** (invariant #5). These two files are
**in scope** for this change as the history record carried alongside the README
overhaul.

### 4. Finalized-plan copy (completion step)
At completion this plan is copied to
`ai-specs/20260917-ggt-release-pipeline-and-readme.md` (the spec trail), which
is also what the "missing-finalized-plan-copy" check expected.

## Files to touch

### Workstream A (code)
- `ggt/LICENSE` — MIT, `Copyright (c) 2026 Gary Larizza` first line.
- `ggt/.goreleaser.yml` — fixed 4-platform matrix (goreleaser v2.9 syntax).
- `ggt/.github/workflows/ci.yml` — `path-filter` → `test` (Go files) +
  `v-nudge` (always, write scopes for the comment). No release job.
- `ggt/Makefile` — `release` / `release-dryrun` / `version-{patch,minor,major}`
  targets; tag guards; single-`v` messages.
- `ggt/VERSION` — `0.5.1` → `0.5.2`.
- `ggt/internal/version/version.go` — no change.

### Workstream B (docs + extension correction)
- `ggt/README.md` — full rewrite per §Workstream B.
- `ggt/AGENTS.md` + `ggt/scripts/README.md` — `.cpro` → `.chopro` only.
- `ggt/cmd/ggt/{cpro,transpose,main,root}.go`,
  `ggt/internal/cpro/{chart,filter,placer}.go` — `.cpro` → `.chopro` in
  help text + comments (no executable-behavior change; verified by `make test`).
  **Touched for the extension fix — see §2, the reason is documented.**
- `ggt/ai-specs/20260902-ggt-standalone-lyric-word-false-chord.md` — new,
  committed; lyric redacted.
- `ggt/ai-specs/20260916-ggt-section-tag-transpose-corruption.md` — new,
  committed.
- `ggt/ai-specs/20260917-ggt-release-pipeline-and-readme.md` — new, copied from
  this finalized plan at completion.

## Risks
- **Go path filter too narrow**: resolved — `**/cmd/**`, `**/internal/**`,
  `*.go`, `go.mod`, `go.sum` cover every Go file in this repo (all live under
  cmd/internal/root); verified by the CI `test` job green on the PR.
- **Releases from CI**: none. `make release` is human-run; the workflow is
  `on: pull_request`, no release job, no `GITHUB_TOKEN` for releases.
- **`issues: write` scope**: required to post the updatable nudge comment;
  GitHub conflates PR-comment writing with the issues scope. It is comment-only,
  not a code/write expansion.
- **Tag-already-exists**: guarded on local + remote; aborts.
- **`make release` messages**: the single-`v` `$tag` form is used (the earlier
  `vv`-doubling was a cosmetic bug, fixed).
- **No auto-patch**: user must run `make version-patch` / `release` by hand.
- **Copyrighted lyric (20260902)**: redacted from the tracked spec (invariant
  #5); the real input stays in the gitignored `ignored/` set.

## Testing / verification (per change)
- **LICENSE**: present; first line `Copyright (c) 2026 Gary Larizza`; MIT body
  intact; GitHub still auto-detects MIT.
- **Goreleaser**: `make release-dryrun` lists 4 tarballs; no deb/rpm.
- **CI**: on the live PR — `path-filter` ✅, `test` ✅ (`make test`),
  `v-nudge` ✅; the `<!-- ggt-release -->` "✅ VERSION bumped to `0.5.2`" nudge
  is present and updatable.
- **Makefile**: `make test` green after the double-`v` fix; `release` recipe
  prints single-`v` messages; guards intact.
- **README**: 0 cost/auth/cloud language; `.chopro` used consistently, `.cpro`
  never as the file extension; subcommand order `ug→cpro→transpose→version`;
  `--capo 4 --remove-capo` example present; `[Bridge]` note present; License
  section present; transpose stdin example is the verified stdin→file form;
  provenance ("use `ggt <subcommand> --help`") replaces exhaustive flag lists.
- **`.chopro` correction**: `make test` green (help strings only); fixture
  `testdata/sample_02_expected.chopro` consistent with code + README.
- **ai-specs**: 20260902 lyric scrubbed (no recognizable excerpt); 20260916
  clean (synthetic `[Chords]`/`[Chorus]` block).
- **Plan-check**: reconciled via re-submission; remaining intentional items are
  acknowledged by slug on the PR.

## Out of scope (explicit, final)
- No auto-release CI job; no `GITHUB_TOKEN` for releases; deb/rpm/Homebrew/S3/
  Vault/Doormat/dev-release-for-testing.
- No auto-patch; no `VERSION` guard in CI. The `version-nudge` comment is the
  sole auto-signal, and it is informational.
- README does **not**: document the UG official-Pro-vs-user-submitted
  distinction, cite the UG design records, restate exhaustive cpro parsing
  rules, or list every subcommand flag. Those are intentional per the user's
  decisions (use `ggt <subcommand> --help`, keep UG detail out of the
  user-facing README). `.chopro` extension is correct and final; `.cpro`
  is not used as a file extension anywhere.
