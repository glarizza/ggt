# cpro: a standalone lyric word starting on a pitch letter is bracketed as a chord

- **Date:** 2026-09-02
- **Status:** PROPOSED — not yet implemented. This doc is the design gate; see
    `AGENTS.md → Build & Test`, bump **`0.5.1 → 0.5.2`** (patch — a bug fix) when
    shipped.
- **Touches (planned):** `internal/cpro/parser.go` (`chordTokenRe`,
    `IsChordToken`, `IsChordLine`, `kindOf`), optionally
    `internal/cpro/filter.go`, plus a regression fixture + tests.
- **Companion to:** `ai-specs/20260822-ggt-leave-unparseable-chords-alone.md`
    (this bug is its **mirror image**: that one made a *real* un-parseable chord
    *survive*; this one lets a *non-chord* word *become* a chord).
- **Copyright note:** every repro below is **synthetic** (fake English words).
    The repo forbids real lyrics in tracked files (`AGENTS.md → Copyright Policy`);
    `ai-specs/` is tracked, `ignored/` is not. Real-world evidence lives in the
    gitignored `ignored/best-of-you.*` set (see §1).

## 1. The problem (a real user-submitted capo tab; real text kept in gitignored `ignored/best-of-you.*`)

Converting a real user-submitted capo chart produced a phantom chord. The real
lyric text is redacted below and lives in the gitignored `ignored/best-of-you.*`
set — the repo forbids real lyrics in tracked files (`AGENTS.md → Copyright
Policy`); `Confess` appears only as the illustrative token:

```
<first chorus line — real text kept in gitignored ignored/best-of-you.*>
[F#] [Confess]          <-- "Confess" is a LYRIC word, bracketed as if it were a chord
<later chorus line — real text kept in gitignored ignored/best-of-you.*>
```

`Confess` is the English word sung on the `F#`. ggt turned it into the chord
token `[Confess]`. In that chart `Confess` hit three times (the three choruses).
The bug is not specific to that word — **any standalone lyric word whose first
letter is a pitch class (`A–G`) is mis-parsed as a chord**.

**Provenance without inlining lyrics.** The live evidence is kept in the
gitignored working dir, not this repo:

- `ignored/best-of-you.ggt-native.chproj` — ggt output *with* the bug
  (backup taken before the hand de-junk that currently ships the chart).
- `ignored/best-of-you.chopro` — the chart as actually imported (the `[Confess]`
  → `Confess` de-junk applied by hand as a stopgap; **this doc removes the need
  for that stopgap**).
- `ignored/repro-standalone-word.txt` / `ignored/repro-2.txt` — the synthetic
  inputs used for §2, committed-safe.

## 2. How to reproduce (synthetic, no copyrighted content)

The bug is in the parser's chord-token recognizer; it needs no real song. Run on
the committed binary:

```bash
# input: one English word per line; the pitch-initial ones get eaten
printf 'C\nChosen\nG\nF\nFallen\nA\nC\n' | ./bin/ggt cpro - --tempo 100
```

Output (observed, `bin/ggt`, 2026-09-02; `VERSION` 0.5.1, mode `dev`):

```
{tempo: 100}
[C]
[Chosen]            <-- "Chosen" (C + "hosen") became a chord
[G]
[F]
[Fallen]            <-- "Fallen" (F + "allen") became a chord
[Chosen...] [A]
[C]
```

A second probe confirms the discriminating feature is "starts on `A–G`":

```bash
printf 'This\nGone\nday\nFeel\nAway\n' | ./bin/ggt cpro - --tempo 100
```

```
{tempo: 100}
This                <-- "This" (T) is NOT a pitch letter → correctly stays a lyric
[Gone]day           <-- "Gone" (G + "one") eaten; "day" glued as its lyric
[Feel]              <-- "Feel" (F + "eel") eaten
[Away]              <-- "Away" (A + "way") eaten
```

**Trigger = a token that (a) starts with `A–G` and (b) is otherwise a run of
letters/digits**, sitting alone on a line (or as a chord-position word). Words
starting on a non-pitch letter (`This`, `day`, …) are unaffected.

> **Stale-binary gotcha (inherited from the companion spec §7).**
> `go build ./...` does not rewrite `bin/ggt`. After any change, `make build`
> *before* the end-to-end repro, or it tests yesterday's binary.

## 3. Root cause

### 3.1 The over-broad recognizer

`internal/cpro/parser.go:19`

```go
chordTokenRe = regexp.MustCompile(`^\(?[A-G][#b]?[A-Za-z0-9#b+\-]*(?:/[A-G][#b]?)?\*?\)?$`)
```

The post-root group `[A-Za-z0-9#b+\-]*` is **unbounded**: after a pitch root +
optional accidental, it greedily accepts *any* length of letters/digits. `IsChordToken`
(`:36`) is exactly `chordTokenRe.MatchString`:

```text
"Confess" -> C  (root, matches [A-G])  +  "onfess" (matches the * extension group)   => true
"Gone"    -> G  +  "one"                                                       => true
"Cadd9"   -> C  +  "add9"                                                      => true   (correct)
"C/D"     -> C  +  /D                                                          => true   (correct)
```

So the recognizer cannot tell `C` **onfess** from `C`**add**`9`: both look like
"pitch root + extension characters."

### 3.2 Why it only shows as an *isolated* bracket

`"Confess"` survives to the output only because of how the line classifier
combines things:

- `IsChordLine` (`:156`, made **tolerant** in the companion spec 20260822)
  returns true if the line has ≥1 symbol and **no prose word**.
- `ClassifyLine` (`:185`) → `Chord` for such a line.
- `PlaceChords`→`renderToken` (`:placer.go`) renders any chord token as
  `[stripParens(t)]`, so `"Confess"` becomes the literal `[Confess]`.

A single-word-chord row (`IsChordLine` = true for `"Confess"` alone) is exactly
what surfaces this: a *lone* token on its own line, where the only "prose test"
(`IsChordLine` → any prose word fails the row) is the token itself, which the
over-broad recognizer already cleared as a chord. Words embedded in a real lyric
line (`"…getting the [B]best…"`) never reach `PlaceChords` as a *row* and are
harmless — which is why, in *Best of You*, only the standalone `Confess` hit.

### 3.3 Blast radius of the mis-detection

Any English word that is pitch-initial and "chordish" is at risk when it sits
alone on a line or in a chord position: `Feel`, `Gone`, `Away`, `Best`,
`Break`, `Fallen`, `Confess`, `Broken`, `Goodbye`, `Chosen`, … plus the genuine
one-letter chords `C D E F G A B`, which are **ambiguous by nature** and cannot
be disambiguated from a 1-letter lyric — out of scope here (§4).

## 4. The fix (options, with a recommendation)

The companion spec established the governing principle: **convert musical
content, never add or subtract meaning we cannot parse; when unsure, leave the
symbol for a human.** The fix must let the *non-chord* fall through to the lyric
path **without** re-adding the failure the companion spec removed (a single bad
token reclassifying the whole row as a lyric — that was the *original* bug this
spec must not reintroduce).

### Option A — Tighten `chordTokenRe` to a real chord grammar (recommended target)

Replace the unbounded tail with a **structured chord-qualifier grammar** so the
extension after the root must be a legitimate sequence (quality letters
`m/M/maj/min/dim/aug/o/sus`, degree digits with optional `# b` and `add`/`sus`,
slash bass note, voicing `*`), e.g. conceptually:

```
root := [A-G][#b]*
qual := "maj"|"min"|"dim"|"aug"|"dom"|"m"|"M"|"o"|"sus"|"m(maj)" | (digit+ "add"?) | ...
chord := root qual? ("/"[A-G][#b]*)? "*"?
```

Then `"Confess"` (`C` + unparseable `"onfess"`) fails `chord`, `IsChordToken`
→ false, `IsChordLine` ("Confess" alone has **no** symbol) → the row is a lyric
and `Confess` flows to the lyric line, producing `[F#]Confess` with *no*
bracket — the desired result.

- **Pros:** correct; closes the whole class (not just the 6-letter ones).
- **Cons:** a hand-written chord grammar is the hard part — risk of **false
    negatives** that *drop* a valid but exotic chord (`C13#11`, `Cm(maj7)`,
    `F#9b5`, `C6/9`). Must be covered by a chord-corpus test (§6) and, to stay
    safe, should **fall through to `isSkippable`** (leave the symbol verbatim)
    rather than silently treat it as prose, so a genuinely odd chord never
    vanishes — mirroring the 20260822 philosophy.

### Option B — Heuristic prose guard on the extension (fast interim)

Keep `chordTokenRe` as the *candidate* test, then add a **guard**: after the
root + accidentals, if the remaining tail is **all letters (no digit, no
`# b + * - /`)** and **length ≥ 4**, classify the token as prose.

- Real quality abbreviations never run longer than 3 pure letters
  (`m`, `maj`, `min`, `dim`, `sus`, `aug`, `dom`), so a 4+-letter pure
  extension is essentially always a word.
- **Catches:** `Confess`(6), `Fallen`(5), `Broken`(5), `Goodbye`(6),
    `Best`(4? `est`=3 → **no**), `Chosen`(`hosen`=5 → yes).
- **Misses:** short pitch-initial words with a 3-letter tail — `Feel`, `Gone`,
    `Away`, `Best`, `Day` — these stay mis-detected.
- **Pros:** tiny, low-blast-radius edit; no grammar to maintain; can ship in
    `0.5.2` immediately.
- **Cons:** magic threshold (4); incomplete; the short-word survivors must be
    handled by Option A later.

### Recommendation

**Ship Option B as the patch `0.5.2`** (immediate closure of the long-word
class, minimal regression surface), **then Option A as the proper follow-up**
(the grammatical fix that closes the short-word class) with a full chord corpus.
Both keep the `isSkippable`/leave-alone path intact for genuine but odd chords.
The guard is gated to *prose-only* tokens so it cannot reclassify a mixed
chord+lyric row — the failure mode the companion spec fixed is not reintroduced.

## 5. Risks & edge cases

- **Do not regress 20260822.** The guard must reject *only* bare
  word-shaped tokens, and only on the path that makes a standalone token a
  chord. A mixed row (`"F  C  (F# - F)"`, `"A  Em  hello"`) keeps its real
  chords; it is never demoted to a lyric because of the guard.
- **Genuine one-letter chords (`C D E F G A B`).** Ambiguous with 3-letter and
  shorter pitch-initial words by construction. Option B (length ≥ 4) leaves them
  untouched — **no change in behavior**, hence no new risk. Disambiguating a
  lone 1-letter `C` from a lyric `C` is undecidable without melody/position
  context and is explicitly out of scope.
- **Real long chords with a long pure-letter extension are (nearly)
  nonexistent**, but not *impossible* (some tabbers write `Cmaj7` as `Cmaj` +
  separate `7`, or `Cmin13`). The guard's threshold must be conservative;
  §6's corpus pins the boundary.
- **Skippable constructs still survive.** `isSkippable` (`"(F# - F)"`,
    `"(Em + C)"`, `"[F - G]"`) is keyed on a *grouped* paren/bracket with a
    separator and is unrelated to the bare-word guard, so the 20260822
    leave-alone behavior is untouched.
- **`N.C.` / `x2` / `|` annotations** are unaffected (handled by
  `annotationTokenRe` before the guard).

## 6. Tests (regression + corpus)

Mirror the table-driven style in `internal/cpro/*_test.go`
(`TestStandaloneChordLineNoLyricBelow`, `TestSkippableSymbolClassification`).

1. **`TestIsChordTokenRejectsProseWords`** — `IsChordToken` is *false* for
   the fake-word fixtures: `Confess`, `Chosen`, `Fallen`, `Broken`, `Goodbye`,
   `Away`, `Feel`, `Gone` (Option B: assert the 4+-letter set is now `false`;
   mark the 3-letter survivors explicitly as "still-chordish, Option A").
2. **`TestIsChordTokenStillAcceptsRealChords`** — *no false negatives*:
   `C D E F G A B`, `C7`, `C#m`, `F#m9`, `Cmaj7`, `Cadd9`, `C7#11`,
   `Cm(maj7)`, `F#9b5`, `C6/9`, `C/D`, `(Gm)`, `C*` all remain `true`.
3. **`TestStandaloneProseWordNotBracketed`** (end-to-end) — the §2 input
   `C\nChosen\n…` no longer yields `[Chosen]` / `[Fallen]`; the words flow to
   the lyric path. Build a committed-scrambled fixture
   `sample_08_standalone_words.txt` via `scripts/scramble_lyrics.py` (fake
   words only — never real lyrics) and the matching expected output.
4. **Chord corpus (for Option A)** — a table of ~30 real chord symbols that
   must survive; add to `internal/cpro/parser_test.go` as the grammar lands.
5. **Companion-behavior guard** — assert the 20260822 `"(F# - F)"` /
   `"hello world"` tests still pass unchanged (the new guard did not disturb
   them).

### Verification log

After implementing and `make build`, re-run both §2 probes; expected:

```
$ printf 'C\nChosen\nG\nF\nFallen\nA\nC\n' | ./bin/ggt cpro - --tempo 100
{tempo: 100}
... no [Chosen], no [Fallen] ...   (and Option B: 'Feel' may still show [Feel])
$ printf 'This\nGone\nday\nFeel\nAway\n' | ./bin/ggt cpro - --tempo 100
{tempo: 100}
... only the 3-letter survivors (Feel/Away/Gone) bracketed under Option B ...
```

Full suite `make test` green; bump `VERSION` to `0.5.2` in the same commit
(`make version-patch`), with a conventional-commit title e.g.
`fix: stop bracketing standalone pitch-initial lyric words as chords`.

## 7. Scope, effort, sequencing

- **Scope:** one recognizer in `internal/cpro/parser.go` (Option B) plus tests;
    a second commit for Option A. No change to the public CLI contract.
- **Effort:** Option B ~a dozen lines + tests. Option A a small grammar + corpus.
- **Sequencing:** Option B now (`0.5.2`, fixes the *Confess*-class that
    actually bit us); Option A next when the chord corpus is in place.
- **Manual stopgap removed:** the hand `[Confess]` → `Confess` de-junk that
    currently ships *Best of You* needs no longer be done by hand; the tool
    produces that output natively.
