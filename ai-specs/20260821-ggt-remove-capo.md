# ggt subcommand `--remove-capo` — de-capo the chord shapes to their
# sounding key

**Status:** DRAFT — pending approval before implementation.
**Date:** 2026-08-21
**Supersedes part of:** `20260821-ggt-transpose.md` (the transpose tool
stays as a *separate* step; this adds de-capo *inside* the formatter).

---

## 0. Prerequisite: rename the formatter token to `cpro`

This design describes the **post-rename** world, in which the formatter (former 6-byte-token)
subcommand, its command file, its `ggt/internal/` package, and its exported
symbols are all named **`cpro`** (4 chars, c-p-r-o; exported prefix
`CPro`).

**Why:** the current 6-byte subcommand token (hex `63686f70726f`) is what this
agent has repeatedly been *unable to type* from prose — every attempt
resolves to a 7-character variant and corrupts files. `cpro`/`CPro` are
byte-stable (verified: standalone, inside `func cproConvert()`, repeated)
and free of the English-word collision that makes `cpro` impossible to emit
reliably.

The rename is a separate, mechanical, single commit and needs no design
doc of its own.  It is a pure byte swap, performed by a script that
**reads the old token from the `good` file and never types it** (the agent
cannot emit it — proven by the very corruption this fix addresses).
Concretely, for every occurrence of the 6-byte token:

| old (6-byte token, hex `63686f70726f`) | new |
|---|---|
| subcommand flag + `Use` string | `cpro` |
| `cmd/ggt/<token>.go` | `cmd/ggt/cpro.go` |
| `ggt/internal/<token>` (dir + package) | `ggt/internal/cpro` |
| capitalized symbols (`<Token>` in ids) | `CPro` |
| test helpers / capitalized identifiers | `CPro` |

`good`/`bad` scratch files are deleted once the rename lands.

**This doc assumes that rename is already merged.**  All paths, command
names, and function names below use the new spelling (`cpro`/`CPro`).

---


## 1. Problem

Running the cpro on a web-cleaned tab gives a BandHelper import where
the **key field is correct but the chord shapes are wrong**:

```
ggt cpro clean.txt --key E --capo 4 --title "Better Together" \
   --artist "Jack Johnson" --duration "3:27" -o out.cpro

   {key: E}
   {capo: 4}
      [C]There's no [C/B]combination of words
      [Am]put on the back of a [Am/G]postcard
      [F]No song that [C/E]I could sing
      ...
```

The key is E (correct, that's the original sounding key), but the *shapes*
are in **C** — the **playing** key under capo 4 (C + 4 = E).  The chords
are "wrong" relative to the stated key, and there is no `ggt` flag that
fixes them:

- `ggt transpose --to-key E` on this file would see `{key: E}` and
    think it is *already* in E, so it does nothing.
- `ggt transpose --down 4` shifts the whole file down a 4th (key **and**
    body), which is the *wrong direction* — the user would have to reason
    that the playing key C must move **up** 4 to land the body on E.
- That reasoning ("capo 4 → sound up 4") is exactly what the user does by
    hand anyway, and it is a second, separate command on an intermediate
    file.

### 1.1 The one-shot workflow the user actually wants

> "download the raw data, clean it, make a .cpro file in ONE go that
> has been transformed and is ready to be imported."

So the de-capo step belongs **inside the cpro formatter itself**, driven
by `--capo N --remove-capo`, not as a post-hoc second `ggt transpose`
invocation.  The user is willing to run transpose separately *in BandHelper*
for a personal transpose later; the cpro's job is to hand over a
correct, ready-to-look-at chart.

---

## 2. Approach

### 2.1 New flag: `--remove-capo` on the cpro subcommand

- Requires `--capo N` to be set.  `--remove-capo` alone is an error:
   there is nothing to remove and no N to shift by.
- When `--capo N --remove-capo` is given, the cpro shifts **every chord
    shape in the body up N semitones**, so the *playing* shapes land on
  the *sounding* ones.  With `--key E --capo 4` the C-shapes become
   E-shapes, matching the `{key: E}` field that was already emitted.
- The shift reuses the **existing** `music.TransposeSymbol` /
     `TransposeCProText` machinery — no new music math.  What is new is
    *how* it is wired: the shift applies to **body chord brackets only**,
    and leaves the `{…}` metadata header and section headers untouched.

### 2.2 What `--remove-capo` does and does not touch

| Thing in the file | `--remove-capo` (with `--capo 4`) |
|---|---|
| Body chord shapes `[C]`, `[Am]`, `[C/B]`, `[Am/G]`, walk-downs `[F# - F]` | **shifted +4** → `[E]`, `[Dm]`, `[E/B]`, `[Dm/B]`, `[G# - G]` |
| `{key: E}` header | **untouched** — it is already the target the shapes move *toward*; it is E |
| `{capo: 4}` header | **decision point** (§5): drop it, or keep it as `{capo: 0}` |
| `(Capo 4)` human body line | **never emitted** — see §2.3 |
| Section headers `[Intro]`, `[Verse]` | **untouched** |
| `{title:}`, `{artist:}`, `{tempo:}`, `{time:}`, `{duration:}` | **untouched** |

This is the "transpose the playing shapes to the sound" operation.  It is
the inverse of what a *real* transpose means: `--remove-capo 4` is *not* a
down-4 (that would be the wrong direction); it is an up-4 of the shapes to
reach the already-declared sounding key.

### 2.3 The `(Capo N)` body line is eliminated, unconditionally

Current behavior: `--capo N` emits **two** things — the `{capo: N}` BandHelper
metadata header **and** a human-readable `(Capo N)` body line at the top of
the chart.

Decision: **the `(Capo N)` body line is dropped in all cases**, regardless
of `--remove-capo`.  It is BandHelper-specific human decoration that the
user wants to control manually when they do a *personal transposition*
inside BandHelper; it should not be baked into the import.  The
`{capo: N}` metadata header is still emitted (that is what BandHelper reads
for its own transposition); only the human line goes away.

So `emitHeader`/`capoBodyLine` (or wherever the `(Capo N)` line is added
in `cpro.Convert` / `cpro.filter`) should stop producing it, full stop —
not "only when `--remove-capo` is set".

---

## 3. Files to touch

| File | Change |
|---|---|
| `cmd/ggt/cpro.go` | Add `--remove-capo` flag; wire it through `HeaderOpts`; validate "requires --capo N"; call the body-only transposer on the result when set |
| `ggt/internal/cpro/cpro.go` / `chart.go` | Thread `RemoveCapo bool` through; apply body transposition; stop emitting the `(Capo N)` line |
| `ggt/internal/cpro/filter.go` | Drop the `(Capo N)` line emission entirely (the `capoBodyLine` / header seam); optionally drop/zero `{capo}` when `--remove-capo` (§5) |
| `ggt/internal/music/music.go` | Expose a **body-only** transposer: shift chord brackets (+ walk-down tokens) by N but leave the `{…}` metadata header *and* section headers untouched. The existing `TransposeCProText` also rewrites the `{key:}` line, which is wrong here (key must stay E), so it needs a variant or a flag: `TransposeCProBody(text, n, style)` or a `skipKey bool` |
| tests | body-only transposition, the up-N "de-capo" semantics, `--remove-capo` requires `--capo`, the `(Capo N)` line never appears |

### 3.1 Why `music.TransposeCProText` is not enough as-is

`TransposeCProText` shifts **every** bracketed token *and* rewrites the
`{key: …}` line.  For `--remove-capo` we want the **inverse** relationship:
move the **body** but **hold the key fixed** (the key is already the target
the shapes are moving *to*).  So:

- either add `TransposeCProBody(text string, semis int, style Style)` that
    skips the `{key: …}` (and other `{…}` metadata) lines, leaving them
   intact,
- or add a `skipHeader bool` parameter and have both callers
   pass it appropriately.

Either way, the shift is purely the bracket chords (+ walk-down `F# - F`
tokens), by +N.  With N = +4 the playing C-shapes become the sounding
E-shapes; the `{key: E}` line stays `E`.

---

## 4. Decisions made / locked

- `--remove-capo` is **a cpro flag**, not a new subcommand.  It composes
     onto the existing formatter and produces a ready-to-import file.
     It requires `--capo N` (error otherwise).
- `--remove-capo`'s shift is **+N** (up), because the playing key is N
     semitones *below* the sounding key the `{key: …}` field already
     declares.  This is distinct from `ggt transpose --up/--down`, which
     move the *whole* chart; `--remove-capo` only de-caps the *shapes*.
- `ggt transpose` keeps its `--up N` / `--down N` / `--to-key K` surface
     and works on files that already have their capos removed, untouched.
   This change does not alter the transpose subcommand at all.
- The `{capo: N}` header remains BandHelper metadata; the human `(Capo N)`
    body line is eliminated unconditionally (§2.3).

---

## 5. Open question — does `{capo: N}` stay or go under `--remove-capo`?

With `--capo 4 --remove-capo` the body is now in **E** (the sound), and the
`{key: E}` is correct, but the chart no longer *uses* a capo — the whole
point of `--remove-capo` is "the shapes ARE the sound; I play them as-is,
no capo."  So:

- **Option A (drop it):** `--remove-capo` emits **no** `{capo: N}` field at
    all.  Cleanest "this is the no-capo version."
- **Option B (zero it):** keep the field as `{capo: 0}` so BandHelper sees
    a capo slot and the user can later add `+4` for a personal transpose
    without editing a missing field.
- **Option C (keep it):** keep `{capo: 4}` as BandHelper metadata so
  BandHelper can re-derive the E-shapes itself; redundant but not wrong.

Recommend **Option A** (drop `{capo}`), because the user explicitly said the
`(Capo #)` *line* is the thing they add by hand in BandHelper — if the
`{capo}` field is what BandHelper actually acts on, leaving `{capo: 4}` on
an E-shape body would re-apply a capo the user already removed.

**Need the user's call on A/B/C before coding this.**

---

## 6. Out of scope (this commit)

- Changing the `ggt transpose` subcommand (it stays as is; it operates on
   already-de-capped files).
- Adding a `--capo` *to* `ggt transpose` (previously rejected; the human /
    agent decides the direction).
- Auto-detecting the playing key from the shapes.
- The `cpro`→`cpro` rename itself (a separate mechanical commit;
    this doc assumes it is done).

---

## 7. Risks

- **Wrong-direction trap is the original bug.** `--remove-capo` must
    *document loudly in `--help`* that it is "up N, the de-capo," not a
    down-N.  Getting this wrong silently produces a 4th-shown chart.
- **Body-only transposition.** Reusing `TransposeCProText` as-is would
    also move the `{key:}` field and turn E into G.  The new path must
    skip the header metadata lines.
- **Walk-downs / annotation chords** (`[F# - F]`, `[C/B]`, `[Am/G]`,
    `x2`) must transposition-follow as they already do in the transpose
   path; verify they move consistently (the F# - F walkdown in the
  Better Together chorus is the known stress case).
- **(Capo N) line removal is global**, not gated on `--remove-capo` — make
    sure no other code path still emits it.
- **`--remove-capo` without `--capo`** must error clearly.

---

## 8. Acceptance tests (proposed)

- `cpro … --key E --capo 4 --remove-capo` → body shapes in E
    (`[E]`, `[Dm]`, `[E/B]`, `[Dm/B]`), `{key: E}` **unchanged**, and the
   `(Capo 4)` line **absent**.
- `cpro … --key E --capo 4` (no `--remove-capo`) → body shapes in C
    (playing key), `{key: E}`, `{capo: 4}`, and the `(Capo 4)` line
    **absent** (the human line is gone unconditionally).
- `cpro … --remove-capo` with no `--capo` → **error**.
- Walk-downs `[F# - F]` transposition-follow correctly (+4 → `[G# - G]`).
- Section headers unchanged; no metadata line other than the
   de-capo-affecting ones is moved.
- (Per §5 decision) the `{capo: N}` field presence/absence under
   `--remove-capo` matches whatever the user picks.

---

## 9. Verification log

(empty — pending implementation, after the `cpro`→`cpro` rename and
 the §5 decision.)
