# AGENTS.md — ggt (Gary's Guitar Tool)

## Project

`ggt` is a CLI tool for working with guitar chord charts and `.chopro` files
(used by BandHelper). It started as a single-purpose tab-to-chordpro
reformatter and is being expanded into a multi-subcommand tool.

---

## Commit Message Format

Every commit MUST begin with a **one-line conventional-commit title**
(`type: imperative summary`, e.g. `feat: add version stamping to make
build`). The title is the *first* line, stands alone, and is a hard
rule: it is what `git log --oneline` shows and the only thing most
people ever read, so **no commit ships without one.** After a blank line
comes the structured PROBLEM/SOLUTION/OUTCOME body:

```
<one-line title, e.g. "feat: stamp make build with version + commit">

PROBLEM:
<One or two sentences describing the actual problem being solved.
Not "refactored things" — the concrete issue that motivated this change.
If there is no problem (new capability, infrastructure), say so plainly.>

SOLUTION:
- <Bullet describing what was done>
- <Bullet describing what was done>
- ...

OUTCOME:
<What the world looks like after this change. What was gained, what
capability now exists, what the state of things is. Future-tense or
present-tense. This is what someone reading git log should be able to
reconstruct from OUTCOME alone.>
```

### Example

```
feat: add a {key: value} header block to the formatter

PROBLEM:
The formatter had no way to produce a BandHelper `{key: value}`
header block. All output was bare chord+lyric lines, making the
resulting .chopro files incomplete.

SOLUTION:
- Added `internal/cpro/filter.go` with `HeaderOpts` struct and
  `emitHeader()` function
- Wired `--title`, `--artist`, `--key`, `--capo`, `--tempo`, `--time`,
  `--duration` flags into `cmd/ggt/cpro.go`
- `--capo N` now also prepends `(Capo N)` as the first content line
- Added `filter_test.go` with unit tests for all header fields

OUTCOME:
`ggt cpro` can now produce complete, BandHelper-ready `.chopro` files
with full metadata header. All 25 tests pass. The
`sample_02_expected.chopro` fixture is a realistic end-to-end example.
```

---

## AI Specs Directory

All LLM/AI-generated spec files (design docs, implementation plans,
analysis) for this project live in `ai-specs/` at the repository root.

This overrides the global `~/ai-markdown/` convention **for this
repository only**. When an agent needs to read/write a design doc for
ggt, it uses `ai-specs/` here.

### Filename convention

`YYYYMMDD-<short-slug>.md`

Example: `ai-specs/20260819-ggt-design.md`

---

## Copyright Policy

**MUST NOT commit copyrighted material to this repository.**

Song lyrics, tab charts, chord progressions, and any other content
associated with a real song are copyrighted. They must NOT appear in
`sample-tabs/`, `testdata/`, or any other tracked file in this repo.
The entire git history of this repo must be clean of copyrighted
material — the repo may be published to GitHub.

### Where copyrighted material lives locally

A `ignored/` directory (gitignored) exists for keeping real song
tabs and lyrics on your machine without committing them:

```bash
ignored/my_song.txt
ignored/another_song.txt
```

You can run `ggt cpro ignored/my_song.txt` freely — the output
to stdout (or a local file) is never tracked by git.

### Scrambled test data

Test fixtures in `sample-tabs/` and `testdata/` use scrambled lyrics.
Two rules:

1. **Scramble the word content.** Run the scrambler to replace lyrics
   with same-length fake words:

   ```bash
   python3 scripts/scramble_lyrics.py ignored/my_song.txt
   # copies the scrambled result to sample-tabs/sample_07.txt
   ```

2. **Never use a real song name as a filename.** Test data files use
generic numeric names like `sample_01.txt`, `sample_02_capo.txt`,
`sample_04_expected.chopro`. Do NOT name them after songs or artists.

The scrambler preserves all chord-column positions, section structure,
blank lines, X markers, and (Instrumental) lines — only the lyric
words themselves are replaced. The replacement is deterministic
(seed 42), so the same input always produces the same output.

---

## Build & Test

```
make build                   # -> bin/ggt  (STAMPED: VERSION + HEAD commit
                              + build date + mode "dev"; ggt version reports it)
make test                    # go test ./...
make run FILE=path.txt       # build + run the converter on path
make build BUILDMODE=release # mint a release-flavoured binary locally
make version-minor           # bump 0.2.0 -> 0.3.0   (new feature)
make version-patch           # bump 0.2.0 -> 0.2.1   (bug fix)
make version-major           # bump 0.2.0 -> 1.0.0   (breaking CLI change)
```

## Versioning (SemVer, pre-1.0.0 discipline)

We respect semantic versioning **a little bit** while pre-1.0.0:

- **New capability / feature** → **bump minor**: `make version-minor`  (…0.2.0 → 0.3.0).   Minor is "free" until 1.0.0.
- **Bug fix** → **bump patch**: `make version-patch` (…0.2.0 → 0.2.1).
- **Breaking CLI change** → **bump major**: `make version-major` (rare before
  1.0.0; use it the day we break the public contract).

Rules of the road:

1. Bump `VERSION` (repo root — the sole human source of truth) **in the same
   commit as the code** it tracks, via a bump target, so the build stamps a
   version that matches reality; a mismatch would let `ggt version` lie.
2. Pre-1.0.0 the version string stays **pure** (no `-dev` suffix); the
   "local vs release" distinction lives in the `Build` stamp field that
   `ggt version` prints (`local build` vs `release`), not in the semver string.
3. A plain `go build` is deliberately UN-stamped and honestly reports
   `dev / unknown / source (un-stamped)`; the Makefile build is what traces a
   binary to a version + commit, answering "does ./bin/ggt have that feature?".
4. Official go-releaser / git-tag releases are **deferred** while features land;
   `Build=release` is already wired into `.goreleaser.yml` for when we get there.

---

## Conventions

- Module path: `ggt`
- CLI framework: `github.com/spf13/cobra`
- All subcommands live in `cmd/ggt/`
- Core logic lives in `internal/`
- Test fixtures live in `testdata/`
