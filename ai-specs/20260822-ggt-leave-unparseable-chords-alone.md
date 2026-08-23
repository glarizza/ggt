# cpro: best-effort chord rows — leave un-parseable symbols alone for the human

- **Date:** 2026-08-22
- **Status:** IMPLEMENTED, 2026-08-23.  Version bumped 0.2.0 -> 0.3.0
(new capability, minor bump per the SemVer discipline).  See §8.
- **Touches:** `internal/cpro/parser.go` (tokenizer + `IsChordLine`),
   `internal/cpro/placer.go` (`PlaceChords`/`renderToken`),
   `internal/cpro/filter.go` (`wrapStandaloneChordLine`), plus tests
- **Companion to:** `ai-specs/20260819-ggt-design.md` (parser/placer model)

## 1. The problem (reported on *Better Together*)

The Chorus chord rows look like this in the cleaned tab:

```
F                   G                                (F# - F)
  Mmmm, it's always better when we're together
```

`F` and `G` are normal chords. `(F# - F)` is a "slide/grab from F# down to F"
notation that the parser does **not** model — it is not a single chord and not
a slash chord. (The real sheet carries the same construct on the interlude and
outro as `(F - F#)` — a slide the other way.)

Two compounding bugs meant the **entire** chord row was dropped and the lyrics
passed through raw:

1. `tokenizeWithColumns` splits on **any** whitespace, so the grouped symbol
   `(F# - F)` was shredded into the tokens `(F#`, `-`, `F)`. The bare `-` is
   neither a chord nor an allowed annotation (like `x2`, `|`, `N.C.`).
2. `IsChordLine` is **all-or-nothing**: *every* token must be a chord or an
   annotation, else the whole row is reclassified as a **lyric** line and emits
   verbatim. One un-parseable token therefore poisoned the line.

So all four chorus-1 lines and four chorus-2 lines came out raw.

## 2. The fix — three coordinated changes

### 2.1 Tokenizer groups a top-level `(...)` / `[...]`

`tokenizeWithColumns` now tracks paren/bracket depth. A top-level `(...)` or
`[...]` group — even with inner spaces like `"F# - F"` — is ONE token. The
fragmenting `(F#`, `-`, `F)` problem is gone; the whole construct is one
decision point, not a broken one.

### 2.2 `IsChordLine` becomes tolerant ("high chord ratio")

A line is **any** chord row if it has **at least one real symbol** (a chord, an
annotation, or a skippable symbol) AND **no prose word**. A single odd symbol
no longer reclassifies the row as a lyric. Prose is still rejected — this does
**not** turn lyric lines into chord lines. (This is the "skipped" half of the
old design; see §5 for why "skip" is now "leave alone.")

### 2.3 Un-parseable symbols are LEFT ALONE, not dropped

`renderToken` now emits a skippable symbol **verbatim** — `"(F# - F)"` out, not
bracketed and un-transposed. It is placed on the word its column lands on, using
the same targeting as any chord. Bare **separators** (`-`/`+`/`*`) that carry no
chord are the only tokens dropped; everything with musical intent survives.

The de-capo transposer (`TransposeCProBody`) rewrites only `[...]` bracketed
chords, so a left-alone `"(F# - F)"` passes through un-bracketed ⇒ the transposer
never touches it. Its source spelling is preserved for the human to fix — the
tool does **not** guess what the slide means.

### 2.4 `wrapStandaloneChordLine` matches

The standalone (no-lyric-beneath) path in `filter.go` gets the same treatment,
so interlude-style rows with an un-parseable symbol also leave it in place.

## 3. Result

```
   [A]Mmmm, it's always [B]better when we're (F# - F)together
   [A]Yeah, we'll look at the [B]stars and we're (F# - F)together
   [A]Well, it's always [B]better when we're (F# - F)together
   [A]Yeah, we're always [B]better when we're (F# - F)together
```

The valid chords (`F`-`A`, `G`-`B` after the +4 de-capo) are placed; the
`(F# - F)` slide is **left in place** — wrong on purpose, so it *sticks out* and
tells the person doing the import that a fix is needed. That is better than
deleting it, which would silently alter the tab and could be missed by anyone
not deeply familiar with the chart. The whole row still exists.

## 4. Risks & edge cases

- **Prose never masquerades as a chord row.** Still requires zero prose
  tokens and ≥1 symbol. Prose like `"together"` is never skippable (its first
  non-space letter is a pitch? `t` is not `A..G`, no separator → it is a prose
  token).
- **Bare separators still dropped.** `F - G` (no parens) → `F` and `G` placed,
  the lone `-` dropped — it carries no chord. Only **grouped** `(...)`/`[...]`
  constructs are left alone.
- **`(F# - F)` lands a touch run-on** (e.g. `...we're (F# - F)together`). That
  is the existing placement rule doing its job; the human fixes the exact spot.
- **Un-bracketed ⇒ un-transposed.** A left-alone symbol keeps its source
  spelling, which may look inconsistent next to the de-capo'd chords — by
  design, since we don't know its intended target.

## 5. The misstep we walked back

The first cut implemented "skip" as **delete** — the symbol vanished from the
output. Gary caught it: *"the `(F - F#)` was not skipped, it was removed. It
DID have a function to the song."* "Skip it" meant **"leave it alone for the
human to fix,"** not "remove it." This is why the design goal is *conversion,
not addition or subtraction of the tab's musical content*: when the meaning is
hard to convert, we leave the raw construct in place and let the human arbitrate,
instead of silently changing the tab. Corrected in the same commit.

## 6. Testing

- `TestSkippableSymbolClassification` — `isSkippable` true for a grouped
  un-parseable symbol (`"(F# - F)"`, `"(Em + C)"`, `"[F - G]"`); false for a
  clean chord (`"(Gm)"`, `"(Cadd9)"`, `F#m7`). `IsChordLine` tolerant of one
  such symbol; still rejects prose.
- `TestTokenizeGroupsParens` — `"(F# - F)"` tokenizes to ONE token.
- `TestPlaceChordsLeavesUnparseableChordRowAlone` — `F`/`G` placed,
  `"(F# - F)"` present **verbatim**, un-bracketed (no `"[F# - F]"`).
- `TestPlaceChordsConnectorNotBracketed` — bare `-` between two chords is
  dropped, not bracketed.

## 7. Gotcha that nearly masked the fix: `bin/ggt` can go stale

`go build ./...` compiles the packages for the test runner but does **not**
rewrite `bin/ggt` — only `make build` does. After the "leave it alone" edits, a
stale `bin/ggt` still printed the **old** (delete) behaviour, so an end-to-end
grep showed 0 `(F# - F)` while the in-binary `Convert` (exercised by the unit
tests) showed all 8. Lesson: after any code change, `make build` before
running the binary, or the end-to-end grep will test yesterday's build.

## 8. Verification log

Applied and corrected within the `feat: keep a chord row when only one symbol
is un-parseable` commit. All packages build/vet/test green.

```
   [A]Mmmm, it's always [B]better when we're (F# - F)together
   [A]Yeah, we'll look at the [B]stars and we're (F# - F)together
   ...
verbatim "(F# - F)" surviving in remove-capo output : 8
bracketed      "[F# - F]" (must be 0)               : 0
```

The de-capo transpose is confirmed to leave un-bracketed `(F# - F)` untouched
(`TransposeCProBody` matches only `[...]`).
