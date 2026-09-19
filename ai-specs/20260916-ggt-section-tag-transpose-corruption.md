# cpro/transpose: a section header that survives the stripper gets transposed as if it were a chord

**Status:** PROPOSED — not yet implemented
**Severity:** data corruption of output text; silent (no error, wrong-looking but not loud)
**First seen:** Pearl Jam — *Nothingman*, UG tab `1177677` (de-cap `--capo 5`)
**ggt version:** 0.5.1 (commit `2b2ad60`, `VERSION` = 0.5.1)
**Component:** `internal/music/music.go` (transpose) + `internal/cpro/parser.go` (section classification)
**Filed:** 2026-09-16

---

## 1. The problem

When converting a tab that has a **section header with trailing text** — e.g.

```
[Bridge] (All played as bar chords)
```

— and the run uses **de-capo** (`ggt cpro --capo N --remove-capo`, or a follow-up
`ggt transpose`), the header word is **transposed as if it were a chord** and the
bracket text is corrupted:

```
[Bridge] (All played as bar chords)     --input
        |  B is read as chord root; +5 semitones -> E
        v
[Eridge] (All played as bar chords)     --ggt output   <-- WRONG
```

`Bridge` → `Eridge`. In the *Nothingman* conversion (shape key C + capo 5, de-capped
+5) this bit the `[Bridge]` header and produced `[Eridge]`. It was caught by manual
audit and hand-stripped out of the final `.chopro`, but the corruption is **silent** —
ggt emits it with `exit 0` and no warning.

The same flaw corrupts **any** single-word header/annotation that starts on a pitch
letter `A–G`. It is not unique to "Bridge".

---

## 2. How to reproduce (synthetic, no copyrighted content)

### 2.1 Two-step pipe, the real failing path

```sh
printf '[Bridge] (All played as bar chords)\nC F G\nbridge lyric\n' > /tmp/bridge.txt
ggt cpro --title T --artist X --key C --capo 5 --remove-capo \
         --tempo 75 --time 4/4 --duration 1:00 -o /tmp/bridge.chopro /tmp/bridge.txt
grep '\[.*\]' /tmp/bridge.chopro
#   [Eridge] (All played as bar chords)   <-- corrupted
```

### 2.2 Direct, minimal, at the transpose layer

```sh
printf '{key: C}\n[Bridge] (x)\n' | ggt transpose --up 5   # Bridge B+5 -> Eridge
printf '{key: C}\n[Chorus] (x)\n' | ggt transpose --up 5   # Chorus  C+5 -> Fhorus
printf '{key: C}\n[Chorus] (x)\n' | ggt transpose --up 3   # Chorus  C+3 -> D#horus
```

Observed:

| bracket | shift | output | expected |
|---------|-------|--------|----------|
| `[Bridge]`  | +5 | `[Eridge]` | `[Bridge]` (untouched) |
| `[Bridge]`  | +3 | `[Dridge]` | `[Bridge]` |
| `[Chorus]`  | +5 | `[Fhorus]` | `[Chorus]` |
| `[Chorus]`  | +3 | `[D#horus]` | `[Chorus]` |
| `[Break]`   | +3 | `[Dreak]`   | `[Break]` |

### 2.3 Why *bare* tags don't show it (the trap)

A bare tag on its own line **is** handled correctly — `cpro` classifies
`[Bridge]` as a section header and drops it (inserting a section break). The bug
only appears when the header has **trailing text**, because then the line is *not*
classified as a section header and the bracket survives cpro into the transpose stage:

```sh
printf '[Bridge]\nC F G\nbridge lyric\n' | ggt cpro --capo 0 -  # bare -> stripped, fine
printf '[Bridge] (All played as bar chords)\n...' | ggt cpro --capo 5 --remove-capo -
#                                     ^ trailing text -> survives -> corrupted
```

So a test that only checks the *bare* `[Bridge]` case **passes today and misses the bug.**
The regression test *must* include trailing text.

---

## 3. Root cause

Two independent pieces both treat **"a word that starts on a pitch letter"** as a
chord, and they compound.

### 3.1 Upstream — `cpro` fails to strip a header that has trailing text

`internal/cpro/parser.go:14`:

```go
sectionRe = regexp.MustCompile(`^\s*\[[^\]]*\]\s*$`)
```

This matches a line that is **entirely** a bracket. `[Bridge]` matches (→ `Section`,
dropped with a section break). `[Bridge] (All played as bar chords)` does **not** match
(there is text after `]`), so `parser.go:202` does **not** classify it as a section
header — the `[Bridge]` token flows through as ordinary text into the cpro output.

### 3.2 Downstream — the transpose layer transposes it as a chord

`internal/music/music.go:60`:

```go
bracketRe = regexp.MustCompile(`\[([^\]]+)\]`)
```

`TransposeCProBody` (used by `--remove-capo`, `music.go:222`) runs `bracketRe` over every
body line and transposes the inner text of each match via `TransposeSymbol`.

`TransposeSymbol` (`music.go:175`) delegates to `parseChord` (`music.go:78`):

```go
func parseChord(sym string) (root, quality, bass string, ok bool) {
        s := strings.TrimSpace(sym)
        if s == "" || strings.Contains(s, " ") {   // ONLY guard: empty or whitespace
                return
        }
        m := rootRe.FindStringSubmatch(s)         // rootRe = ^([A-Ga-g])([#b]*)
        if m == nil {
                return
        }
        root = m[1] + m[2]
        rest := s[len(root):]
        ...
        quality = rest       // "ridge" from "Bridge"; "horus" from "Chorus"
        return root, quality, bass, true
}
```

The **only** refusal path is `s == ""` or a **space** in the token. "Bridge" has no
space, and `rootRe` greedily grabs `B` (a pitch letter) as the root, leaving
`quality = "ridge"`. `parseChord` returns `ok = true`, so `TransposeSymbol` shifts the
root: `shiftNote("B", +5) = "E"`, reassembled with the verbatim quality →
`"E" + "ridge" = "Eridge"`.

This is the *same* over-broad "A–G initial ⇒ chord" assumption that the
`standalone-lyric-word` bug (see `20260902-ggt-standalone-lyric-word-false-chord.md`)
flagged in the cpro *chord* recognizer — here it recurs in the **transpose** layer and
in the cpro *section* classifier, two different spots.

Note the parallel in the cpro recognizer too. `parser.go:18`:

```go
chordTokenRe = regexp.MustCompile(`^\(?[A-G][#b]?[A-Za-z0-9#b+\-]*(?:/[A-G][#b]?)?\*?\)?$`)
```

`chordTokenRe` matches `Bridge`, `Chorus`, `Break` as "chord tokens" — same flaw,
because a word whose first letter is a pitch letter satisfies the chord grammar.

---

## 4. Blast radius

**Any single-token bracket whose first run starts on `A–G` corrupts on transpose.**
Survey of common section/annotation words run through `ggt transpose --up 3`:

| header | starts on | up-3 output | corrupts? |
|--------|-----------|-------------|-----------|
| `Bridge`    | B (pitch) | `Dridge`      | yes |
| `Chorus`    | C (pitch) | `D#horus`     | yes |
| `Break`      | B (pitch) | `Dreak`        | yes |
| `Guitar`     | G (pitch) | `Auitar`       | yes   |
| `Riff`       | R (not pitch) | `Riff`        | no  |
| `Solo`       | S (not pitch) | `Solo`        | no  |
| `Intro`      | I (not pitch) | `Intro`       | no  |
| `Verse`      | V (not pitch) | `Verse`       | no  |
| `Outro`      | O (not pitch) | `Outro`       | no  |
| `Refrain`    | R (not pitch) | `Refrain`     | no  |
| `Tag`        | T (not pitch) | `Tag`         | no  |
| `PreChorus`  | P (not pitch) | `PreChorus`   | no  |
| `PostChorus` | P (not pitch) | `PostChorus`  | no  |
| `Verse 1`    | has **space** | `Verse 1`     | no  |

So the at-risk set = single-word headers starting on a pitch letter. Real-world
offenders seen in UG tabs: `Bridge`, `Chorus`, `Break`, `Guitar` (e.g. `[Guitar Solo]`
is *safe* because of the space, but a bare `[Guitar]` would corrupt). Multi-word headers
like `Verse 1`, `Intro 2`, `Guitar Solo`, `Post Chorus` are safe because the space trips
the only guard in `parseChord`.

**Note:** bare tags are stripped by cpro (3.1) so they rarely reach 3.2. The *live*
failure mode is **trailing-text headers** — which are common on UG user tabs (authors
annotate `Bridge`, `Chorus`, `Intro` with a parenthetical technique cue). These are the
ones to fix.

---

## 5. The fix (options, with a recommendation)

### Option A — Guard the transpose layer against non-chord tokens (narrow, targeted)

In `TransposeSymbol` / `TransposeCProBody` / `TransposeCProText`, refuse to transpose a
bracket whose inner content is not plausibly a chord. Cheapest robust variant: a small
**deny-list of section/annotation words** checked before the transpose:

```go
// Section/annotation headers that must never be transposed. Cased-insensitive,
// matched on the trimmed inner content of a bracket.
var staticTags = map[string]bool{
        "intro": true, "verse": true, "chorus": true, "bridge": true,
        "outro": true, "solo": true, "break": true, "refrain": true,
        "tag": true, "prechorus": true, "postchorus": true, "guitar": true,
        "riff": true, "fill": true, "fill-in": true, "instrumental": true,
        "keychange": true, "modulation": true, "coda": true, "interlude": true,
}
func isStaticTag(inner string) bool { return staticTags[strings.ToLower(strings.TrimSpace(inner))] }
```

Then in both `TransposeCProBody` and `TransposeCProText`, skip the bracket when
`isStaticTag(inner)`:

```go
lines[i] = bracketRe.ReplaceAllStringFunc(line, func(bracket string) string {
        inner := bracket[1 : len(bracket)-1]
        if isStaticTag(inner) {           // <-- NEW: never transpose a static tag
                return bracket
        }
        if t, ok := TransposeSymbol(inner, semis, style); ok {
                return "[" + t + "]"
        }
        return bracket
})
```

- **Pros:** small, self-contained, no risk to real chords; preserves the existing
  `[Am/G]`, `[D/F#]`, `[F# - F]` behaviour exactly.
- **Cons:** deny-list can miss a novel single-word header that starts on a pitch letter
  and isn't listed. Miss = silent corruption again, but the surface is now small and
  auditable, and the list is a single obvious diff to extend.

### Option B — Reject implausible chord "quality" suffixes in `parseChord` (principled)

`parseChord` currently returns `ok=true` for *any* `A–G`-initial token. Add a grammar
guard on the **quality** so a suffix that is not chord-plausible is refused:

```go
// qualityRe accepts the quality letters/digits/accidentals real chord names use:
// m, maj, 7, 6, 9, sus, dim, aug, add, +, -, b, #, /, digits.  NOT "ridge"/"horus".
var qualityRe = regexp.MustCompile(`^[0-9mbM#b+\-/a-z]*$` /* tighten as needed */)

func parseChord(sym string) (root, quality, bass string, ok bool) {
        ...
        quality = rest
        if quality != "" && !qualityRe.MatchString(quality) {
                return  // "ridge" / "horus" / "reak" are not a chord quality -> not a chord
        }
        return root, quality, bass, true
}
```

- **Pros:** fixes the class structurally; "Bridge", "Chorus", "Break", "Guitar" all
  fail the quality grammar and are left untouched. No deny-list maintenance.
- **Cons:** broader blast radius on the *chord* path — must be proven not to regress the
  existing `TestTransposeSymbol` cases (`Cadd9`, `Dm7b5`, `F#sus4`, `Cmaj7`, `C6/9`,
  slash chords). A too-tight `qualityRe` would start *rejecting real chords* (a worse,
  also-silent bug). Needs a corpus test (below) before merging.

### Option C — Fix upstream: classify trailing-text headers as sections in `cpro`

Widen `sectionRe` (or add a second pass) so `[Bridge] (All played as bar chords)` is
classified `Section` and dropped *before* it can reach transpose. Something like:

```go
// a section header followed by an optional parenthetical / free-text cue
sectionRe = regexp.MustCompile(`^\s*\[[^\]]*\](\s*\(.*\))?\s*$`)
// or: strip a standalone [tag] and keep any trailing text as prose
```

- **Pros:** addresses the *actual* first cause on *Nothingman*; the corrupted token never
  reaches transpose; preserves the technique cue text if desired.
- **Cons:** does **not** by itself fix the transpose layer's latent over-broadness; a
  future `[Bridge][F#]`-style collision or a direct `ggt transpose` call (a legit use
  case) would still corrupt. Should be a *complement* to A or B, not the whole fix.

### Recommendation

**A + C, with B as a future hardening.**

1. Do **Option C** (upstream) so trailing-text headers are handled as sections in cpro —
   this is what bites real UG tabs.
2. Add **Option A** (deny-list guard in the transpose layer) as cheap defense-in-depth
   so the transpose stage can't corrupt a surviving static word.
3. Optionally pursue **Option B** later, *gated by a chord corpus test* (§6.3), to close
   the structural gap without a growing deny-list.

A and B are mutually exclusive at the same call site (don't do both — the deny-list and
the quality-grammar would fight over "what is a chord"). Pick A now; revisit B when a
*real* false-chord that isn't a static word shows up.

---

## 6. Tests (regression + corpus)

**The most important constraint (§2.3): existing tests pass today and DO NOT cover this.
The new tests must use trailing-text headers, and must run through a *non-zero* shift.**

### 6.1 Unit — `internal/music/music_test.go` (primary guard, fastest feedback)

Add to the `TestTransposeSymbol` case table (note **no space**, **non-zero shift**):

```go
// section headers must NOT transpose, even though they start on a pitch letter
{"Bridge untouched +5", "Bridge", 5, StyleAuto, "Bridge", false},
{"Chorus untouched +5", "Chorus", 5, StyleAuto, "Chorus", false},
{"Break untouched +3", "Break",  3, StyleAuto, "Break",  false},
// and the file-level contract
```

Plus a `TransposeCProBody`/`TransposeCProText` case that locks the *surviving bracket*
path — use trailing text so §2.3 applies:

```go
func TestTransposeCProBodyLeavesSectionTags(t *testing.T) {
        in := "" +
                "{key: C}\n" +
                "[Bridge] (All played as bar chords)\n" +
                "[Dm7] lyric [F#] hi\n" +
                "[Chorus] (x3)\n" +
                "[F# - F] walkdown\n"
        out := TransposeCProBody(in, 5, StyleAuto)

        // the tag text must survive verbatim
        if !strings.Contains(out, "[Bridge] (All played as bar chords)") {
                t.Errorf("Bridge tag corrupted:\n%s", out)
        }
        if !strings.Contains(out, "[Chorus] (x3)") {
                t.Errorf("Chorus tag corrupted:\n%s", out)
        }
        // real chords must still move
        if !strings.Contains(out, "[F# - F] walkdown") {
                t.Errorf("walkdown should not move:\n%s", out)
        }
        // key header untouched on body transpose
        if got := KeyOf(out); got != "C" {
                t.Errorf("key must stay C: got %q", got)
        }
}
```

This test **fails on the current build** (`[Eridge]`, `[Gchorus]`) and passes once a /
b / c lands.

### 6.2 CLI regression — `cmd/ggt/transpose_test.go`

Mirror the house style of `execTranspose`:

```go
func TestTransposeDoesNotCorruptSectionTags(t *testing.T) {
        stdin := "{key: C}\n[Bridge] (All played as bar chords)\nC F G\nbridge lyric\n"
        out := execTranspose(t, stdin, "-", "--up", "5")
        if strings.Contains(out, "Eridge") || strings.Contains(out, "ridge") {
                t.Errorf("Bridge header was transposed:\n%s", out)
        }
        if !strings.Contains(out, "[Bridge]") {
                t.Errorf("Bridge tag should survive verbatim:\n%s", out)
        }
}
```

Optionally a `cpro_test.go` end-to-end case (§2.1) that runs `ggt cpro --capo 5
--remove-capo` on a tab containing `[Bridge] (…) ` and asserts the tag is either
dropped (Option C) or, at minimum, **never** appears corrupted.

### 6.3 Corpus / property test for Option B only

If Option B is ever pursued, add a property test: for a fixed shift, a corpus of real
quality-bearing chord names (`Dm7`, `F#9`, `Cadd9`, `G/B`, `E7b9`, `C6/9`, `Am/G`,
`Bsus4`, `D7alt`, `E+`, `Fm(b5)`) must each **transpose** (ok=true), while a corpus of
section/annotation words (`Bridge`, `Chorus`, `Break`, `Guitar`, `Riff`, `Coda`,
`Interlude`) must each be **refused** (ok=false). Run both at shifts {0, +3, +5, -4}
acce. `--flats` and `--sharps`. This guards the quality grammar from silently starting
to reject real chords.

### Verification log (to fill in when implemented)

```
$ ggt test ./internal/music/ ./cmd/ggt/
ok   ggt/internal/music
ok   ggt/cmd/ggt         (with the 3 new tests green)
# and manually:
$ printf '{key: C}\n[Bridge] (All played as bar chords)\n' | ggt transpose --up 5
{key: C}
[Bridge] (All played as bar chords)        # unchanged
```

---

## 7. Scope, effort, sequencing

- **Option C (cpro sectionRe)** — ~5 lines + 1 test in `internal/cpro`. Low risk. Do first.
- **Option A (transpose deny-list guard)** — ~10 lines + table in `internal/music`
  + 1 CLI test. Low risk. Do second.
- **Option B (quality grammar)** — needs the corpus property test and a careful review of
  every quality in the existing test table; medium risk. Defer.
- Update `internal/cpro/chart_test.go` / `parser_test.go` if `sectionRe` widening changes
  which lines are treated as sections (it may stop bracketing a *real* `[ch ...]`-style
  token — verify `chordTokenRe` is unaffected).
- Build the binary *after* editing and re-run the §2.1 repro against the fresh `bin/ggt`
  (the stale-binary gotcha noted in `20260822-ggt-leave-unparseable-chords-alone.md`
  §7: `bin/ggt` can silently lag the source).

---

## 8. Related

- `20260902-ggt-standalone-lyric-word-false-chord.md` — the *cpro recognizer* version of
  the same "A–G initial ⇒ chord" over-broadness, on the isolate-lyric-word path.
- `20260822-ggt-leave-unparseable-chords-alone.md` — the philosophy of "leave ambiguous
  tokens for a human" this fix should honour (don't *invent* a chord; don't *corrupt* a
  non-chord).
- `bin/ggt` staleness gotcha: `20260822-ggt-leave-unparseable-chords-alone.md` §7.
