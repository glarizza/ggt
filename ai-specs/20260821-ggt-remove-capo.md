# ggt cpro `--remove-capo` — de-capo the chord shapes to their sounding key

**Status:** DRAFT — pending approval before implementation.
**Date:** 2026-08-21
**Supersedes part of:** `20260821-ggt-transpose.md` (the transpose tool
stays a *separate* step; this adds de-capo *inside* the cpro formatter).

---

## 0. Prerequisite — the `cpro` rename  (DONE)

Already merged (commit `ada8139`).  The formatter subcommand, its command
file, its `ggt/internal/cpro` package, its exported prefix `CPro`, and the
`.cpro` extension are all the 4-byte name `cpro` (hex `63 70 72 6f`,
c-p-r-o — byte-stable and typeable).  This doc is written for that
post-rename world.  **No further work needed here.**

---

## 1. Problem

Running `cpro` on a web-cleaned tab gives an import whose **key field is
correct but the chord shapes are in the playing key, not the sounding key**:

```
ggt cpro clean.txt --key E --capo 4 -o out.cpro
```

produces

```
{key: E}
{capo: 4}
[C]There's no [C/B]combination of words
[Am]put on the back of an [Am/G]old card
[F]No song that [C/E]I could sing
...
```

The key is **E** (correct; the original sounding key), but the shapes are in
**C** — the *playing* key under capo 4 (C + 4 semitones = E).  There is no
`ggt` flag that fixes a chart like this:

- `ggt transpose --to-key E` sees `{key: E}` and does nothing (key already E).
- `ggt transpose --down 4` shifts *everything* down a 4th (key **and**
    body) — the wrong direction.  The correct fix is to raise the body
    **up** 4 semitones to land it on the already-declared key.
- That "up-N de-capo" reasoning is exactly what the user does by hand, and
    it currently means a second, separate command on an intermediate file.

### 1.1 The one-shot workflow the user wants

> "download the raw data, clean it, make a cpro file in ONE go that has been
> transformed and is ready to be imported."

So de-capo belongs **inside the cpro formatter**, driven by
`--capo N --remove-capo`, not as a post-hoc `ggt transpose`.

---

## 2. Approach

### 2.1 The two capo modes (decided by the user)

The user gave the exact behaviour, which collapses the old open question:

- **`--capo N` alone → keep `{capo: N}`.**  This is the *correct*
    representation: "the chart is in key E, played with a capo 4" — the
    BandHelper `{capo: N}` metadata field is technically right, so it stays.
    No body transposition.

- **`--capo N --remove-capo` → the native, no-capo chart.**  The user
    "provides the number of semitones needed to raise the existing chords
    in order to achieve the `--key` provided, and so the resulting chart is
    the native key, without any capos."  Concretely the cpro shifts every
    body chord **up N semitones** so the *playing* shapes land on the
    *sounding* ones, and at that time **removes the `{capo: N}` header.**
    With `--key E --capo 4 --remove-capo` the C-shapes become E-shapes,
    matching `{key: E}`, and `{capo: 4}` is gone.

**Decision (from §5): `--remove-capo` drops `{capo: N}` entirely.**
Rationale (confirmed by user): the resulting chart no longer uses a capo,
the whole point of `--remove-capo` is "the shapes already ARE the sound; I
play them as-is."  Leaving `{capo: 4}` on an E-shape body would make
BandHelper re-apply a capo the user just removed.

### 2.2 What `--capo N --remove-capo` does and does not touch

| Thing in the file | `--capo N` alone | `--capo N --remove-capo` |
|---|---|---|
| Body shapes `[C]` `[Am]` `[C/B]` `[Am/G]` walk-down `[F# - F]` | unchanged | **shifted +N** → E-shapes etc. |
| `{key: E}` header | **untouched** | **untouched** (already the target the shapes move *toward*) |
| `{capo: N}` header | **kept** | **removed** |
| `(Capo N)` human body line | **never emitted** | **never emitted** |
| Section headers, `{title:}`, `{artist:}`, `{tempo:}`, `{time:}`, `{duration:}` | unchanged | unchanged |

`--remove-capo` **requires `--capo N`** — there is nothing to remove and no
N to shift by otherwise, so `--remove-capo` alone is an error.

The shift reuses the existing `music` machinery (`TransposeSymbol`); the new
work is *wiring*: it applies to **body chord brackets (+ walk-down tokens)
only** and leaves the `{…}` metadata header and section headers alone.  The
existing `TransposeChopperText` also rewrites the `{key: …}` line, which is
wrong here (key must stay E), so a body-only transposer is needed:
`music.TransposeChopperBody(text string, semis int, style Style)` or a
`skipHeader bool` flag on the existing function.

### 2.3 The `(Capo N)` body line is eliminated — unconditionally

`--capo N` today emits **two** things: the `{capo: N}` metadata header
*and* a human-readable `(Capo N)` line at the top of the body.

Decision (confirmed by user): **the `(Capo N)` line is dropped in ALL
cases**, regardless of `--remove-capo`.  It is BandHelper-specific human
decoration — the thing the user adds *by hand* inside BandHelper when doing
a personal transposition — and it does not belong in the imported file.
Only the metadata field's presence is gated (kept without `--remove-capo`,
dropped with it, per §2.1); the human line is gone full stop.

The seam is wherever `emitHeader` / the `capoBodyLine` helper injects
`(Capo N)` in `cpro.Convert` / `filter.go` — that emission is removed.

---

## 3. Files to touch

| File | Change |
|---|---|
| `cmd/ggt/cpro.go` | Add `--remove-capo` bool flag; thread through `HeaderOpts`; error if set without `--capo N`; when set, run the body-only transposer (+N) on the converted result and drop the `{capo: N}` header |
| `ggt/internal/cpro/cpro.go` / `chart.go` | Thread `RemoveCapo bool` (and the semitone count N, derived from the capo int) through; apply body transposition; stop emitting the `(Capo N)` line unconditionally |
| `ggt/internal/cpro/filter.go` | Delete the `(Capo N)` line emission entirely; drop `{capo: N}` from the header when `--remove-capo` is set (keep it otherwise) |
| `ggt/internal/music/music.go` | Add `TransposeChopperBody(text, N, style)` that shifts bracketed chords + walk-down tokens by N but leaves `{…}` metadata lines and section headers untouched |
| tests | body-only transpose; `--capo N` keeps `{capo}`; `--capo N --remove-capo` shifts body +N **and** drops `{capo}`; `--remove-capo` without `--capo` errors; `(Capo N)` line never emitted in any mode |

### 3.1 Why `TransposeChopperText` is not enough as-is

`TransposeChopperText` shifts every bracketed token **and** rewrites the
`{key: …}` line.  `--remove-capo` needs the inverse relationship: move the
**body**, hold the **key** fixed (it already equals the target).  Hence a
body-only transposer that skips the `{key: …}` / metadata lines.  The shift
is +N (up), +4 for capo 4, applied only to bracket chords and walk-down
`F# - F` tokens.

---

## 4. Decisions made / locked

- `--remove-capo` is a **cpro flag**, not a new subcommand; it composes on
    the existing formatter.  It **requires `--capo N`** (error otherwise).
- The shift is **+N (up)** because the playing key is N semitones *below* the
    sounding key the `{key: …}` field already declares.  This is *not* a
    `ggt transpose` down-N.
- **`{capo: N}` is kept without `--remove-capo` and removed with it** (user
    decision; this resolves the old §5 open question in favour of "drop it").
- The human `(Capo N)` body line is **eliminated in all modes**.
- `ggt transpose` is **untouched**; it keeps `--up/--down/--to-key` and
    operates on already-de-capped output.

---

## 5. Resolved — `{capo: N}` under `--remove-capo`

Resolved by the user's clarification: under `--capo N --remove-capo` the
chart is the **native, no-capo** version, so **`{capo: N}` is removed.**
Without `--remove-capo`, `{capo: N}` is kept because the representation
("key E, capo 4") is technically correct.  No further decision needed.

---

## 6. Out of scope (this commit)

- Changing the `ggt transpose` subcommand.
- Adding `--capo` to `ggt transpose` (previously rejected).
- Auto-detecting the playing key from the shapes.
- The `cpro` rename (§0, already done).

---

## 7. Risks

- **Wrong-direction trap.** `--remove-capo` must say loudly in `--help` that
    it shifts **up N (the de-capo)**, not down-N.  A wrong sign silently
    produces a 4th-shown chart.
- **Body-only transposition.** Reusing `TransposeChopperText` as-is would
    also move `{key: …}` and turn E into G; the new path must skip the
    metadata header (and section headers).
- **Walk-downs / raw-text annotations are NOT transposed by design.**
     `parseChord` rejects any symbol with a space, so `F# - F` passes through
     untouched (same as `ggt transpose`); the human fixes it later. Single/slash
     chords like `[Am]`, `[Am/G]`, `[C/B]` DO shift (root + bass, plus N).
    `F# - F` chorus walk-down in Better Together is the known stress case.
- **`(Capo N)` removal must be global** — verify no code path still emits it
    in either mode.
- **`--remove-capo` requires `--capo N`** — error clearly, no silent no-op.

---

## 8. Acceptance (verified)

- `cpro clean.txt --key E --capo 4` → body in C (`[C] [Am] [C/B] …`),
    `{key: E}`, `{capo: 4}` present, no `(Capo 4)` line.
- `cpro clean.txt --key E --capo 4 --remove-capo` → body in E
    (diatonic E-sharps: `[E] [C#m] [F#m] [A] [B]`, slash `[E/D#] [C#m/B] [E/G#]`), `{key: E}` **unchanged**,
    **no `{capo}` field**, no `(Capo 4)` line.
     Spelling is diatonic-in-E: the root of Am +4 -> C#m,
     not the flat enharmonic Dm (matches `ggt transpose`'s auto/sharp convention).
- `cpro clean.txt --key E --remove-capo` (no `--capo`) → **error**.
- Walk-downs / raw-text annotations (`F# - F`, `x2`, `N.C.`) pass through
     unchanged under `--remove-capo`: `parseChord` rejects symbols with a space
     (same as `ggt transpose`); fixed by hand afterwards. Single/slash chords shift.
- Section headers unchanged in both modes; no metadata line moves except the
    de-capo-affecting ones.
- Full-suite test passes; `make build`/`make test` green.

---

## 9. Verification log

### 2026-08-21 implementation landed

- `go build ./...`, `go vet ./...`, `go test ./...`: all packages green
  (cmd/ggt, internal/cpro, internal/music, internal/version).
- End-to-end on `ignored/better-together-clean.txt`:
   - MODE 1 `--key E --capo 4`: C-shapes, `{capo: 4}`, 0 `(Capo N)` lines.
   - MODE 2 `--key E --capo 4 --remove-capo`: E-shapes `[E] [C#m] [F#m] ...`,
     `{key: E}` kept, `{capo}` absent, 0 `(Capo N)` lines.
   - MODE 3 `--key E --remove-capo`: error, exit 1.
   - walk-down `(F# - F)` passes through unchanged.
- cpro.go output switched from `fmt.Print` to `cmd.OutOrStdout()` so
  `cmd/ggt/cpro_test.go` (execCpr) can capture output (mirrors transpose).
- `TransposeCProBody` ignores `{key:...}`/metadata, shifts brackets +N;
  covered by `TestTransposeCProBody`.
