# AGENTS.md — ggt (Gary's Guitar Tool)

## Project

`ggt` is a CLI tool for working with guitar chord charts and `.chopro` files
(used by BandHelper). It started as a single-purpose tab-to-chordpro
reformatter and is being expanded into a multi-subcommand tool.

---

## Commit Message Format

All commits MUST follow this structure:

```
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
PROBLEM:
The chopro formatter had no way to produce a BandHelper `{key: value}`
header block. All output was bare chord+lyric lines, making the
resulting .chopro files incomplete.

SOLUTION:
- Added `internal/chopro/filter.go` with `HeaderOpts` struct and
  `emitHeader()` function
- Wired `--title`, `--artist`, `--key`, `--capo`, `--tempo`, `--time`,
  `--duration` flags into `cmd/ggt/chopro.go`
- `--capo N` now also prepends `(Capo N)` as the first content line
- Added `filter_test.go` with unit tests for all header fields

OUTCOME:
`ggt chopro` can now produce complete, BandHelper-ready `.chopro` files
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

You can run `ggt chopro ignored/my_song.txt` freely — the output
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
make build    # → bin/ggt
make test     # go test ./...
make run FILE=path.txt   # build + run ggt chopro on path
```

---

## Conventions

- Module path: `ggt`
- CLI framework: `github.com/spf13/cobra`
- All subcommands live in `cmd/ggt/`
- Core logic lives in `internal/`
- Test fixtures live in `testdata/`
