# ggt — Gary's Guitar Tool — Design Doc
## Phase 1: `chopro` Subcommand Migration

**Date:** 2026-08-19
**Status:** Approved — implementing
**Repo:** `~/src/ggt`
**Module:** `ggt`
**Scope:** Migrate existing tab→chordpro logic into a Cobra CLI tool. Transpose is out of scope for this phase — it will be scoped out separately as `ggt transpose`.

---

## Problem Statement

The existing `chordpro-reformatter` tool does one job: it converts a plain-text
tab chart (chord-line-above-lyric-line) into inline ChordPro format. It builds
and all 10 unit tests pass, but:

1. **UX bug** — when invoked with no args it falls through to reading stdin and
     *hangs* instead of printing a usage message.
2. **No header** — it produces only the chord+lyric body. Real `.chopro` files
    that BandHelper consumes have a `{key: value}` header block with song metadata.
3. **Tab-chart conventions leak through** — end-of-song `X` markers and
     `(Instrumental)` lines pass through as raw lyric text.
4. **Parenthesized chords double-bracket** — `(Em)` in a chord line produces
     `[(Em)]` instead of `[Em]`.
5. **Sections run together** — because section headers and blank lines are both
    dropped, the verse/chorus/bridge boundary is lost entirely.
6. **No capo handling** — no way to record capo information.
7. **Architecture gap** — single-purpose binary can't host additional
     subcommands (e.g. the future transpose feature).

---

## Findings from Inspecting Real `.chopro` Files

Inspected ~50 files in `~/Downloads/*.chopro` (BandHelper's native format). Key
observations:

### 1. Header Format — BandHelper's `{key: value}` Format

```
{title: I'm Yours}
{artist: Jason Mraz}
{album: We Sing. We Dance. We Steal Things.}
{year: 2008}
{key: B}
{capo: 4}
{time: 4/4}
{tempo: 151}
{duration: 4:03}
```

Format: `{key: value}` — curly-bracket, colon-separated, ONE per line, NO space
after the colon. This is NOT ChordPro `!directive!` syntax.
`title` and `key` are nearly universal; others are optional.

One outlier file (`Babylon_David_Gray.chopro`) uses `[tag] value` instead.
Target the dominant `{key: value}` format.

### 2. Section Markers in Real Files

- `{start_of_verse}` — appears frequently; BandHelper-specific, separates song sections
- `{start_of_tab}` — rare; marks a tab notation section
- `(x2)`, `x2`, `x4`, `(x2 over bass solo)` — inline repeat annotations

The `[Verse 1]` / `[Chorus]` tab-chart section headers should be **dropped entirely**.
Even though BandHelper has a `{chorus}`-style capability, it doesn't support it in
practice — confirmed by the user's real files.

**However, section boundaries must still be present in the output.** All `.chopro`
files that the user maintains have a single blank line between sections (verse/chorus/
bridge boundary). See §5.

### 3. Standalone Chord Lines

Real `.chopro` files consistently use these for intros/progressions:
```
[D] [Dmaj7] [Bsus2/D]
[D] [Dmaj7]
```
This matches the current tool's "chord line with no lyric beneath" path.
No changes needed.

### 4. Complex Chord Names — Existing Regex Handles All

Tested the current `chordTokenRe` against every distinct chord found across the
50 real files. **All pass the regex**, including:

```
Bsus2/D   G6/B     Dsus2/C   Dsus2/Bb   B7sus4
F6add11   Emaj7    A7sus4    Dm9        F#/A#
Cadd2/B   Gmaj7-5/D G#9/C    D#m/C#     G#m7
```

No changes needed to the chord token regex.

### 5. Parenthesized Chords in Tab Charts

Chords wrapped in parens, e.g. `(Gm)`, appear in real tab charts and mean:
- "This chord is still ringing from the previous phrase" (sustain/hold), or
- "A reminder: you're still playing this chord here"

Neither meaning requires the parens in `.chopro` output. **Strip outer parens
from all chord tokens.** `(Gm)` → `[Gm]`, `(Dsus2/C)` → `[Dsus2/C]`.
Only the OUTERMOST paren pair is stripped — chord names themselves don't
contain meaningful parens.

### 6. Tab-Chart Conventions That Must NOT Appear in Output

| Input pattern | Current behaviour | Desired output |
|---|---|---|
| `X` alone on a line | passes through as lyric | **drop** |
| `(Instrumental)` | passes through as lyric | **drop** |
| `(Gm)` chord wrapping | `[(Gm)]` double-bracket bug | **`[Gm]`** (strip parens) |
| `[Section]` headers | dropped entirely | **dropped** but insert blank line between sections (see §5) |
| `x2` / `x4` / `(x2)` inline | handled by annotation regex ✓ | keep as-is |
| `Capo on Nth Fret` line | *(not expected in user's tabs)* | **not handled**; user won't copy this line |

---

## Approach

### 1. Project and Module Rename

| From | To |
|---|---|
| `~/src/chordpro-reformatter-go/` | `~/src/ggt/` |
| module `chordpro-reformatter` | module `ggt` |
| binary `chordpro-reformat` | binary `ggt` |
| `cmd/chordpro-reformat/main.go` | `cmd/ggt/main.go` |

### 2. Cobra CLI Structure

```
ggt                            (no subcommand → prints help + subcommand list,
                               exits non-zero, does NOT read stdin — fixes hang bug)
     └── chopro      [INPUT]    [--output OUTPUT]    [--title ...] [--artist ...]
                                    [--key ...] [--capo N] [--tempo N] [--time T]
                                    [--duration T]    [flag: full names only in V1]
               tab chart → .chopro file
```

Phase 1: `chopro` fully implemented. `transpose` is NOT part of this phase.

### 3. `ggt chopro` CLI Interface

```bash
# minimal: body only, just bracket reformatting
ggt chopro input.txt
ggt chopro input.txt --output song.chopro

# with header fields
ggt chopro input.txt \
    --title "Sample Song" \
    --artist "Some Band" \
    --key Em \
    --capo 7 \
    --tempo 120 \
    --time 4/4 \
    --output song.chopro
```

**Flags (full names only in V1 — short forms deferred until CLI stabilises):**
- `--output` / `-o` — write to file; default stdout
- `--title` → `{title: ...}` in header if non-empty
- `--artist` → `{artist: ...}` in header if non-empty
- `--key` → `{key: ...}` in header if non-empty
- `--capo N` → adds `{capo: N}` to header **AND** `(Capo N)` as the very first
   content line in the body (bandHelper personal-transpose visual cue — see §4)
- `--tempo N` → `{tempo: N}` in header if non-zero
- `--time T` → `{time: T}` in header if non-empty
- `--duration T` → `{duration: T}` in header if non-empty

**No stdin fallback.** With no positional arg and no `-`, exits with usage error.
With `-` as the positional arg, reads stdin explicitly.

### 4. Capo Handling

When `--capo N` is provided, TWO things happen:

1. **Header directive** — `{capo: N}` is emitted in the `{key: value}` header
   block in the standard position (after `key`, before `tempo`).
2. **First content line** — `(Capo N)` is prepended as the first line of the
   chord body. The parens are literal: open-paren, `Capo`, space, number,
    close-paren. This is the user's own visual cue for BandHelper personal
    transpose — not a functional instruction, just a reminder at the top of the song.

Example (full output for `--title "Sample Song" --key Em --capo 7`):
```
{title: Sample Song}
{key: Em}
{capo: 7}

(Capo 7)
[Em] [Cadd9] [G] [D]
[Em]You come out at [Cadd9]night
...
```

Note: `Capo on Nth Fret` **text lines are NOT auto-detected** — the user will
not include them when copying tabs. Only the `--capo N` flag drives capo output.

### 5. Section Boundary Blank Lines

**Current behaviour:** section headers dropped, blank lines dropped → verse and
chorus run together with no visual or structural separation.

**New behaviour:** When a `Section` line is encountered in the input, if the last
output line (in the running output builder) is NOT blank, push a single blank line.
This gives one clean blank line as a separator wherever a tab-chart section marker
was seen, without requiring any blank lines in the original tab.

**De-duplication:** if the last output line is already blank, do NOT push another
one. Multiple consecutive section headers (e.g. `[Intro]\n[Verse 1]`) collapse to
one blank line.

**Implementation:** in `chart.Convert`, when `cl.Kind == Section`, instead of
`i++; continue`, call a small helper `maybeSectionBreak(&out)` that inspects the
last element of `out` and appends `""` if needed.

**Effect on current tests:** `TestDropsSectionHeadersAndBlankLines` must be
updated — the expected output now has a blank line between the verse and chorus
lines, not none.

### 6. `[(Em)]` Double-Bracket Fix

In `renderToken` (in `placer.go`) and `wrapStandaloneChordLine` (in `chart.go`):
strip the OUTERMOST paren pair from a chord token before wrapping in brackets.

```
(Gm)        → [Gm]
(Dsus2/C)   → [Dsus2/C]
((G))       → [G]         (only one level stripped; inner parens are unusual)
```

Implementation: `strings.TrimPrefix(tok, "(")` + `strings.TrimSuffix(inner, ")")`
only when the token starts with `(` AND ends with `)`. This is safe because no
standard chord name contains a meaningful `(` or `)` character.

### 7. Line Filters (applied BEFORE bracket wrapping, in `filter.go`)

| Filter | Action |
|---|---|
| Line is `X` (with optional whitespace) — `^\s*X\s*$` | **drop** |
| Line matches `^\s*\(Instrumental\)\s*$` | **drop** |
| `[Section]` header (existing regex) | Call `maybeSectionBreak(out)` then advance |
| Blank line | **drop** (existing behaviour — section breaks handled by §5 instead) |
| `x2` / `x4` / `(x2)` inside a chord line | Pass through as-is (existing regex ✓) |

### 8. Header Emission

A new function `emitHeader(opts HeaderOpts) string` in `filter.go` produces the
`{key: value}` block. Only non-empty fields are emitted. Fixed field order:

```
title    →  if --title non-empty
artist   →  if --artist non-empty
key      →  if --key non-empty
capo     →  if --capo > 0
tempo    →  if --tempo > 0
time     →  if --time non-empty
duration →  if --duration non-empty
```

The header is emitted BEFORE the body. If `--capo N` is set, the `(Capo N)` line
goes immediately after the header block (before the first chord/lyric content
line). If no flag is provided that would produce any header field, the body starts
immediately with no header.

### 9. Code Structure

```
ggt/
  cmd/ggt/
    main.go             thin entry: root.Execute()
    root.go            root + chopro subcommand registration (Cobra)
    chopro.go         chopro subcommand wiring; flag definitions
  internal/
    chopro/             was: internal/chart + internal/parser + internal/placer
      chart.go         orchestrate: walk lines, pair chord+lyric, apply filters
      chart_test.go
      parser.go          ← moved from internal/parser
      parser_test.go
      placer.go          ← moved from internal/placer
      placer_test.go
      filter.go        NEW: line filters + header emission + section-break logic
      filter_test.go
  testdata/
    sample_02_input.txt            saved from this session
    sample_02_expected.chopro         expected output fixture (with --capo 7,
                                       --key Em, --title "Sample Song")
  ai-specs/
    20260819-ggt-design.md            this file
  go.mod
  Makefile
  README.md
```

**Rationale:** Merging the three packages into one `chopro` package keeps the
"unit of work" together. `filter.go` is the new layer that sits between
`parser` (classifies lines) and `chart` (orchestrates), handling
drop/filters, header emission, and section-break insertion.

---

## Files to Touch

**New:**
- `cmd/ggt/main.go` — replaces `cmd/chordpro-reformat/main.go`
- `cmd/ggt/root.go` — root Cobra command + subcommand registration
- `cmd/ggt/chopro.go` — chopro subcommand flags (`--title`, `--artist`, `--key`,
     `--capo`, `--tempo`, `--time`, `--duration`, `--output`)
- `internal/chopro/filter.go` — NEW: `HeaderOpts` struct, `emitHeader()`,
     `maybeSectionBreak()`, `isDropLine()`, `stripParens()`
- `internal/chopro/filter_test.go` — tests for all new logic
- `testdata/sample_02_input.txt` — saved from this session
- `testdata/sample_02_expected.chopro` — expected output fixture

**Moved (package path changes; logic mostly same, minor edits for new filters):**
- `internal/chart/chart.go` → `internal/chopro/chart.go`
     - `Convert()` gains `HeaderOpts` parameter
     - Section handling changes to call `maybeSectionBreak()`
     - `wrapStandaloneChordLine` calls `stripParens()` before bracketing
- `internal/chart/chart_test.go` → `internal/chopro/chart_test.go`
     - `TestDropsSectionHeadersAndBlankLines` — update expected output (now has
        section boundary blank line)
     - `TestStandaloneChordLineNoLyricBelow` — update expected `[(Em)]` → `[Em]`
- `internal/parser/parser.go` → `internal/chopro/parser.go`
     - No functional changes
- `internal/parser/parser_test.go` → `internal/chopro/parser_test.go`
     - No changes
- `internal/placer/placer.go` → `internal/chopro/placer.go`
     - `renderToken` calls `stripParens()` before bracketing
- `internal/placer/placer_test.go` → `internal/chopro/placer_test.go`
     - Add test for paren-stripping: `(Gm)` + "hi" → `[Gm]hi`

**Modified:**
- `go.mod` — module `ggt`, add `github.com/spf13/cobra`
- `Makefile` — update binary name, paths, add `chopro` run target
- `README.md` — new docs for `ggt chopro` interface

**Deleted:**
- `cmd/chordpro-reformat/` (directory)
- `internal/parser/`, `internal/placer/`, `internal/chart/` (empty after move)

---

## Risks

| Risk | Mitigation |
|---|---|
| Existing tests break after import path change | `go test ./...` after each move; logic unchanged, just update import paths in the moved packages and `chart.go` |
| `maybeSectionBreak` double-blanking | Guard: only push `""` if last output line is NOT already `""` |
| `stripParens` strips parens from non-chord tokens | `stripParens` is NOT used for annotations (`x2`, `N.C.`, `\|`, etc.) — only for actual chord tokens. Annotations still go through `IsAnnotationToken` path unchanged |
| `--capo N` with N=0 or negative | Cobra `Int` flag defaults to 0 → no capo emitted. Negative not meaningful; guard with `if capo > 0` |
| Section break inserts blank at very start of output | Skip the leading blank: when `maybeSectionBreak` triggers and the output slice is empty, don't append |
| `testdata/sample_02_expected.chopro` fixture gets stale | Regenerate with `ggt chopro testdata/sample_02_input.txt --title "Sample Song" --key Em --capo 7 -o testdata/sample_02_expected.chopro` after any filter change; diff in test |

---

## Testing Plan

**Unit level** (`go test ./...`):
- All existing 10 tests pass after move (with expected-output updates noted above)
- New `filter_test.go`:
     - `X` alone → dropped (returns `true` from `isDropLine`)
     - `(Instrumental)` → dropped
     - `stripParens("(Gm)")` → `"Gm"`
     - `stripParens("Gm")` → `"Gm"` (unchanged, not a paren-wrapped token)
     - `emitHeader` with all flags → produces correct `{key: value}` block
     - `emitHeader` with no flags → produces empty string
     - `maybeSectionBreak` when last line is `""` → no-op
     - `maybeSectionBreak` when last line is content → appends `""`
     - `maybeSectionBreak` when output is empty → no-op (no leading blank)
- Updated `chart_test.go`:
     - `TestDropsSectionHeadersAndBlankLines` → expects section break blank line
     - `TestStandaloneChordLineNoLyricBelow` → expects `[Em]` not `[(Em)]` (after stripParens)
- Updated `placer_test.go`:
     - New test: `PlaceChords("(Gm)", "hi")` → `[Gm]hi`

**Integration level** (manual smoke test):
```bash
ggt chopro testdata/sample_02_input.txt \
    --title "Sample Song" --key Em --capo 7 \
    --output /tmp/out.chopro
diff /tmp/out.chopro testdata/sample_02_expected.chopro    # no output = PASS

ggt                 # prints help, exits 1, does NOT hang
ggt chopro          # prints usage, exits 1, does NOT hang
ggt chopro -        # reads stdin (no positional = error; - = stdin)
```

---

## Out of Scope (This Phase)

- `ggt transpose` subcommand — scope separately in a future design doc
- Auto-detect song key from chord progression
- Parsing `.gp5` guitar-pro files
- Short-form flags (`-t` for `--title`, etc.) — deferred until CLI shape is stable
- BandHelper `{start_of_verse}` markers (user is happy to add manually in BandHelper)
- Auto-detecting `Capo on Nth Fret` from tab text — user won't copy it
