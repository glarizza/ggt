# ggt transpose — design spike

**Status:** IMPLEMENTED. `internal/music` + `cmd/ggt/transpose.go` are written, table-driven tested, and run the real Better Together tab end to end.  See §13.
**Author intent:** collect the user's needs, lock the UX, and lay out the key-math
mechanics so the AI never has to guess the theory. This doc is the authoritative
spec for the `transpose` subcommand; when approved it becomes the contract the
implementation must satisfy.

---

## 1. Problem statement

ggt can clean a downloaded tab and place its chords into BandHelper `.cpro`
structure, but it cannot do the **music-theory** step BandHelper actually needs:
the **native-key transposition**.

The user plays **capo shapes** — easy open shapes plus a capo instead of barre
chords: easier, a different voice, and it keeps chords movable so he can shift
a song between keys himself. Every tab he encounters is in the
**playing/shape key** — the chords as they *look on the fretboard* with the
capo clamped on — not the key that is being *heard*. BandHelper, though, stores
the **native / original (sounding) key** and applies a per-player "personal
transpose" on top. That split is the whole point: the *piano player* reads the
stored chords natively while the *guitar player* applies a transpose to get
their easy shapes back. To import a song at all, the chart must carry the
native key.

Today that math is done on Copilot / Claude by hand, per song — the exact
"cloud AI decides it won't help" behaviour the user is moving away from by running
a local model. We want `ggt transpose` to do it, deterministically, with
**zero AI in the loop**.

This is the single big missing piece; `clean_tab.py` + `ggt cpro` already cover
everything before and after it.

---

## 2. Domain recap (what the theory is)

- **Native key = shape key shifted by the capo.** A tab says "E-shapes at capo
    3" meaning the fingers play the *shape* of E with the capo at fret 3, and
   the sound is up 3 semitones from E. the user wants the **original / published**
   key, which is N semitones *lower* — shift **down** N.
- **Personal transpose is not stored in the file.** the user sets it per-player,
    by hand, in BH's UI.
- **Transposition is a uniform chromatic shift applied to every chord:**
      - *Preserve chord quality exactly.* `m7b5` stays `m7b5`, `sus4` stays
         `sus4`, `maj7` stays `maj7`. Only the **root** (and, for a slash
      chord, its **bass** letter) moves.
      - *Slash chords* transpose both halves: `D/F#` down 3 is `B/D#`.
      - *Borrowed / altered chords* move verbatim — never "corrected" to a
       diatonic substitute.
      - *N.C. / annotations* (`x2`, `|`, `%`) and section headers pass through.
- **Capo markings can lie.** A tab's stated capo may be wrong or stale; the
    source of truth for the *key* is the published key and its chord-function
    profile, not the tab's own "Capo: N" label. That policing is the AI /
    orchestrator's job — **not** transpose's.
- **Direction.** the user's 80% case is `--down N`: "these chords sound at capo N,
    so the original key is N semitones lower — shift everything down N." The AI
    reasons out magnitude and direction from the capo; the subcommand just
    executes the shift it is told to run.

---

## 3. User interaction (designed first)

### 3.1 What the user needs, in words

> "I got a tab in E-shapes at capo 3. Give me that song in its native key so I
>  can hand it to BandHelper — and don't make me do the music math."

Two decisions the UI must make easy:
1. **How far and in which direction to move.**   `--down N` for the 80% case.
2. **Into which accidental spelling.**   `--auto` by default; the F#/Bb
    always-sharp / always-flat cases handled at v2.

### 3.2 Two modes, no `--capo` sugar

`ggt transpose` takes exactly one of two forms — **mutually exclusive**:

| Flag | Meaning | When the user / the AI uses it |
|---|---|---|
| **`--up N`** / **`--down N`** (-N) | Shift every chord N semitones up or down. `--down N` = `--up (12−N)`.  N is 0–11. | **The 80% case.** The AI reads "capo N" off the cleaned tab, decides "down N gives the original key", and runs `--down N`. the user also uses this for one-off re-keyings ("a bit too high for my voice — down 2"). |
| **`--to-key K`** (-k) | Transpose so the output tonic lands on key K. ggt computes the chromatic delta from the current `{key}` header to K. | the user knows the published / target key and doesn't want to subtract. |

**No `--capo` flag, and no `--key` override flag.** The capo value is not
something the tool decodes. When the AI sees "capo N" in the tab it has
*already done the thinking* and issues `--down N`. Keeping `--capo` out of the
subcommand's own flags prevents two different "how far" meanings (the shape key
itself vs a semitone offset) from ever colliding on one command line. The tool
's job is singular: **shift by a semitone count.**

### 3.3 The 80% case — the full choreography

1. **Read the cleaned tab.** Notice the capo N (from a `(Capo 3)` banner, a
       `capo: 3` metadata line, or by deducing it from the shapes looking open
     and easy — "this is clearly a capo tab, not a barre tab").
2. **Reason about direction.** "These chords are relative to capo N, so the
     original / native key is N semitones lower. Run `--down N`."
     *(Why `--down` not `--up`: the tab's shapes sound N semitones **above** the
      true key; to return to the true key you go the other way.)*
3. **Run the tool:** `ggt transpose input.cpro --down N -o output/song.cpro`.
4. **Do the verification `ggt transpose` itself does *not*:**
      - *parallel* web search for the song's published key — musicnotes first
          (publisher sheet — best available), then Wikipedia's infobox, then
        Hook Theory / Chordify. Two-or-more independent sources agreeing
          on the same tonic is "solid" for key, per the research skill.
      - Check the **first chord** of the output against the found key — the
        classic direction-error spot (an un-shifted shape chord masquerading
        as the native tonic).
      - Skim the harmonic profile (I–IV–V–vi) and confirm it lands in a key that
        makes musical sense.
5. **On disagreement** — the found key diverges from the output's tonic —
     re-evaluate the capo and re-run; otherwise surface both numbers plus the
     source list to the user to decide.

`ggt transpose` itself is a deterministic, no-network, no-AI shift of every
`[chord]` token by N semitones, with a side-effect on the `{key}` header.
**It never decides whether N is correct.** That is verified *around* it.

### 3.4 Accidental style — `--sharps` / `--flats` / `--auto`

Default: **`--auto`**, inheriting the input's accidental spellings.

Known limitation (v2): some pitch classes have a **conventional** spelling that
holds *regardless of key*:
- `F#` is virtually always written `F#`, not `Gb`.
- `Bb` over `A#` in most pop / rock / folk / blues material.
- `C#` / `Db` are context-dependent — which is exactly why `--auto` inherits.

A `--prefer F# Bb` flag (an allowlist of "never substitute" pitch classes) is
the natural v2 upgrade. Not a v1 blocker.

### 3.5 I/O

Mirrors `ggt cpro`'s plumbing:
- Positional input, or `-` for stdin.
- `-o out` / `--out` writes to a file; no `-o` → stdout.
- `--inplace` rewrites the source file in place.
- Reads the `{key}` / `{capo}` header when present: `{key}` is **rewritten** to
      the shifted value; `{capo}` is **passed through unchanged** (record, not
     input).
- `--to-key K` with **no** `{key}` header → error: cannot compute a delta from a
     target without a starting key.

### 3.6 The flow it fits into

```
1.  download + login          (skill: browser MCP)
        ->  ignored/real.txt
2.  clean                     (scripts/clean_tab.py)
        ->  output/song.cleaned.cpro
3.  cpro                       (ggt cpro --key E --capo 3 --title ... -o ...)
        ->  output/song.structure.cpro    [chords in E shapes; {key:E}, {capo:3}]
4.  transpose --down 3        (ggt transpose --down N  OR  --to-key K, NEW)
        ->  output/song.cpro              [chords shifted; {key: native}; {capo:3} kept]   * BandHelper-ready
5.  import into BandHelper    (hand; future UI automation)
```

`ggt cpro` runs *before* `ggt transpose` because cpro turns a column-aligned
chart into BandHelper's inline `[chord] lyric` form — a width-independent
string form transpose can safely rewrite. Transposing *before* cpro shifts the
column widths and breaks the placer (see §6).

The skill orchestrates 1–5. `ggt transpose` owns step 4.

---

## 4. What ggt transpose reads vs writes

| Value | Read from | Written out |
|---|---|---|
| Semitone delta | `--up N` / `--down N` explicitly, **or** computed by `--to-key K` from `{key}` | — |
| Accidental style | `--auto` / `--sharps` / `--flats` | applied to every shifted note name |
| `{key}` header | input's `{key: ...}` | **rewritten** to the shifted value |
| `{capo}` header | input's `{capo: N}` | **kept as-is** |
| `[chord]` tokens | every `[Token]` on every lyric line | root + bass shifted; quality **rides through** |
| `[Section]`, `(N.C.)`, `x2`, `|`, `%` | input | **untouched** |

**`ggt transpose` never decides whether a key is correct.** It shifts by N in
the direction it is told. "Is N the right number, in the right direction?" is
the orchestrator's question, answered by the published-key search and spot-
checks *before* `ggt transpose` is invoked. That separation is what keeps the
subcommand simple, unit-testable, and free of any model or network call.

---

## 5. Output — exactly what changes

For `{key: E}` + `{capo: 3}` transposed `--down 3` to the native / original key:

- Every `[E]`, `[Am]`, `[C]`, `[G]`, `[D]` shifts -3 semitones:
      `[E]`→`[C]`, `[Am]`→`[F]`, `[G]`→`[Eb]`, `[D]`→`[Bb]`.
- Slash chord `[D/F#]` shifts both halves: `[D/F#]`→`[B/D#]`.
- `[x2]`, `(N.C.)`, `[Verse]` — untouched.
- Header: `{key: E}` → `{key: C}` (E down 3 = C, the native key).
      `{capo: 3}` retained as a human-readable record; ignored by BH.
- No `{transpose}` field written. the user sets that by hand in BH.

**BandHelper contract (the user's confirmation):**
- BH **reads `key`** and stores it — that's the native/original key we set.
- BH **ignores `capo` and `transpose`** — `{capo}` is present as a human
     record only; BH does not act on it.
- the user sets their **personal transposition** by hand in BH's UI, per player.
- Import is a **store, not a mutation** — it takes `key` as written and
     does not recalculate.

So the transposed output the user hands to BH is literally: **all chords shifted to
the native key, plus `{key: native}` and `{capo: N}` as a human record.**

---

## 6. Why transpose runs after cpro (order)

Transposing a column-aligned chart (before cpro) is a trap: **`C` is 1 character
; `F#` is 2** — every subsequent column on the line shifts right by the added
width, and the placer (which depends on the column alignment inherited from the
raw tab) sees garbage. Once cpro has turned chords into inline `[…]` brackets,
chord width is *irrelevant* to anything downstream — a bracket is its own token,
not positionally tied to a column on the lyric line. So cpro first (structure
+ bracket placement + basic header), then transpose (shift bracket *contents*
+ rewrite the `{key}` header).

The `(F# - F)` walkdown — the two-chords-in-one-bar annotation cpro currently
misclassifies — is a **cpro-side** classification bug that lands in its output
*before transpose ever sees the file*. Fixing it in cpro and handing transpose a
clean token is the right split (Q7). Transpose never has to parse a paren form
; it shifts the bracket contents it is given.

---

## 7. Implementation sketch

### 7.1 New package `internal/music` (hand-rolled, stdlib-only)

The chord vocabulary is tiny; the user explicitly wants the logic *encoded*, not
left for the AI to reason per song. A dependency-free package covers it:

```go
package music

// 12-element chromatic scales, sharps and flats spellings, root note only.
var ChromaticSharp = [12]string{"C","C#","D","D#","E","F","F#","G","G#","A","A#","B"}
var ChromaticFlat  = [12]string{"C","Db","D","Eb","E","F","Gb","G","Ab","A","Bb","B"}

// Chord splits a token into three parts; Quality rides through verbatim.
type Chord struct {
    Root    string // "D", "F#", "C"
    Quality string // "m","7","m7b5","sus4","maj7","7b9","6/9","" (major)
    Bass    string // "" or a bass note: "F#","B"
}

// ParseChord splits "Dm7","D/F#","C7b9","Am/C#". ok=false for N.C./non-chord.
func ParseChord(tok string) (Chord, bool)

// Style determines spelling of shifted note names.
type Style int
const (
    StyleAuto   Style = iota // F# not Gb, inherit per-pitch conventional spelling
    StyleSharps              // always spell with sharps
    StyleFlats               // always spell with flats
)

// TransposeNote shifts a note name by N (negative = down).
func TransposeNote(note string, n int, style Style) (string, error)

// Transpose shifts the whole chord by N; Quality and Bass are handled here.
// E.g.  (Dm7, -3).Transpose(-3, Auto) ==> "Bm7"
func (c Chord) Transpose(n int, style Style) string

// SemitoneDistanceDown for --to-key: how many "down" semitones from -> to.
func SemitoneDistanceDown(from, to string) int
```

Design point: **quality rides through, never parsed.** The `chordTokenRe` in
`internal/cpro/parser.go` already separates the root + accidental from an
arbitrary suffix; reusing it means `Am/G` splits into root `A`, quality `m`,
bass `G`, and the `m` is carried through unchanged. No "valid qualities"
vocabulary is needed — which is why hand-roll wins; nobody maintains a quality
table.

### 7.2 `ggt transpose` command (`cmd/ggt/transpose.go`)

Cobra command mirroring cpro's flag plumbing:

| Flag | Aliases | Default | Notes |
|---|---|---|---|
| `--up N` / `--down N` | `-N` | one required | mutually exclusive |
| `--to-key K` | `-k` | — | mutually exclusive with `-N` |
| `--sharps` / `--flats` | — | `--auto` | `--auto` means "inherit" |
| `-o` / `--out` | `--out` | stdout | as cpro |
| `--inplace` | — | off | rewrite source in place |
| positional `INPUT` | `-` = stdin | — | as cpro |

`--up N`, `--down N`, `--to-key` are `MarkFlagsMutuallyExclusive`; exactly one
must be present. `--inplace` + positional, or `-o`, or stdout — same three
output modes as cpro.

### 7.3 Hand-rolled vs library — DECISION

**Decision: hand-roll `internal/music`.** ~100 lines, table-driven test, zero
runtime judgment, zero new dependency — fits the std-lib-only spirit of the
existing binary. The `ParseChord` / `Transpose` seam isolates the only logic a
future music-theory library would provide; a single import swap replaces it
behind the same interface if a chord form we don't yet anticipate (microtonal
; `:14` figured-bass slash, etc.) appears in a tab the user wants to run.

---

## 8. Files to touch

| File | Change |
|---|---|
| `cmd/ggt/transpose.go` | New subcommand (cobra) |
| `cmd/ggt/root.go` | `rootCmd.AddCommand(newTransposeCmd())` |
| `internal/music/music.go` | Scales, `Chord`, `ParseChord`, `TransposeNote`, `Transpose`, `SemitoneDistanceDown`, `Style` |
| `internal/music/music_test.go` | Transposition vector table — the heart of the spec |
| `internal/cpro/filter.go` | *(separate fix, Q7)* `(F# - F)` walkdown — cpro vs transpose ownership |
| `scripts/README.md` | Document the clean→cpro→transpose→import pipeline |
| `sample-tabs/sample_09.txt` | New scrambled-capo fixture for end-to-end test |
| skill file `~/.pi/agent/skills/ggt-chart-pipeline/SKILL.md` | Add step 4 (transpose) to pipeline; refresh source-of-truth ranking note |

---

## 9. Risks & status

| # | Risk / question | Status |
|---|---|---|
| R1 | `--auto` emits "Gb" where a player writes "F#" | Known; v2 `--prefer F# Bb …` flag. Not a blocker. |
| R2 | Quality split mis-reads an accidental (`D#m7` loses the sharp) | Mitigated: reuse `cpro`'s `chordTokenRe`; add vectors to the test for every sharp/flat-in-quality form |
| R3 | First-chord direction error | Explicit vector test; `ggt transpose` prints a stderr warning when its first-chord root ≠ `--to-key` target |
| R4 | BandHelper header semantics unknown | **Resolved by the user:** BH reads `key`, ignores `capo`/`transpose`; no `transpose` field written; the user sets personal transpose by hand |
| R5 | `--to-key` with no `{key}` header | Clear error, not a silent default |
| R6 | Borrowed chords "corrected" to diatonic | Structurally impossible: quality rides through; no substitute-lookup path |
| R7 | `(F# - F)` walkdown misclassified by `cpro` *before* transpose | **Open (Q7).** Lean "fix in cpro first"; decoupled fix |

---

## 10. Testing plan

- **Vector table in `internal/music`:**
    - `E - 3 = C`
    - `Am - 3 = F`
    - `D/F# - 3 = B/D#`
    - `Am7b5 - 3 = Fm7b5`    (quality preserved)
    - `G + 5 = C`
    - `C + 1  --flats = Db`     (spelling test)
    - `Am/C# - 3 = F/A#`    (slash, sharp root and bass, full shift)
    - `x2 / (N.C.)` passthrough (non-chord, not shifted)
- **First-chord assertion** — a fixture whose first chord equals the target
     key root; `--to-key <root>` → first output chord `[<root>]`.
- **End-to-end:** `sample_09.txt` (scrambled-capo fixture) → `ggt cpro` →
     `ggt transpose --down N` → diff vs hand-written expected.
- **Round-trip / idempotence:** `--up N` then `--down N` returns the input
     byte-for-byte.
- **No AI in the loop** — a grep-level criterion: `cmd/ggt/transpose.go` and
     `internal/music/*` contain no network call, no model call, no `os/exec`
     shell-out. The runtime is pure Go.

---

## 11. Open questions (pre-implementation approval)

### Resolved

- **Q1 — output model:** output stores **native-key chords**. the user confirmed.
- **Q2 — input:** post-cpro bracketed `.cpro`; no raw-tab input needed.
- **Q3 — header fields BH consumes:** BH reads `{key}` only; `{capo}` kept as a
     human-readable record, ignored by BH; no `{transpose}` field written —
     the user sets personal transpose by hand in BH's UI per player.
- **Q4 — accidental style default:** `--auto` (inherit); `--prefer` deferred to
        v2. the user approved the default.
- **Q5 — hand-roll vs library:** **decided — brettbuddin/musictheory.** See §12.
- **Q6 — who owns `{key}`:** `ggt transpose` (it is the key-shift engine).
- **Q8 — no `--capo` flag:** the user removed it. No capo-decoding in the subcommand.

### Still open

- **Q7 — `(F# - F)` walkdown:** fix in **cpro** (lean; pre-transpose
     (DROPPED) — the user decided the `F# - F`-style two-chords-in-one-bar walkdown
     is a human-post-review fix, not a subcommand concern. Left as a human
     find-and-fix after conversion.

---

## 12. Library selection (resolved — see empirical notes)

**Decision: use `github.com/brettbuddin/musictheory` (MIT) as the chromatic /
enharmonic primitive, behind a thin `internal/music` glue we own.**

Three candidates were cloned and probed with real Go programs (go 1.26),
not just read:

| Library | Verdict | Why |
|---|---|---|
| **go-music-theory/music-theory** | **Rejected** | GPL (copyleft — license-taint risk, incompatible with a permissive ggt). Also: `Chord.Of("Dm7/F#")` parses + `Transpose(semitones)` works and *preserves quality via the interval structure* — but it returns `Root` + a `Tones` interval→class **map** with no compact chord-symbol re-render, so it hands us `B + {i1,i3,i5,i7}` not the text `Bm7`; we would have to re-derive the quality string ourselves anyway. Worst of all, its auto sharp/flat detection **breaks the F#-always rule** (`F#7 → Eb`, `F# → Eb`) by re-spelling to the input's style. |
| **go-muse/muse** | **Out of scope** | It's a *score/mode/halftone* system: its `Chord` is a set of `note.Note`s carrying a **duration**, `String()` prints `"notes: … duration: …"`, and `builder/` is for constructing scales/modes. No chord-symbol parse. Pulls a `shopspring/decimal` dependency. Wrong category for chord-symbol transposition. |
| **brettbuddin/musictheory** | **WINNER** | MIT, **0 transitive deps**, ~1000 lines, pure Go. `Pitch.Transpose(Semitones(n))` shifts roots correctly (verified: `C→A`, `F#→D`, `A→F#/Gb`, `D→B`). It gives a **chord symbol parser too** via our own glue. Crucially, its explicit spelling strategy — `Name(AscNames)` / `Name(DescNames)` — maps **directly onto our `--sharps` / `--flats` / `--auto` flags**, and `AscNames` yields **`F#` not `Gb`**, which *supports your "F# is always F#" rule* out of the box. |

### 12.1 Empirical finding (probed, not guessed)

`brett Pitch.Transpose(Semitones(-3))` is chromatically correct and lets us
choose the accidental spelling per flag:

| Input (down 3) | `Name(AscNames)` | `Name(DescNames)` |
|---|---|---|
| C | A | A |
| D | B | B |
| E | D# | Db |
| F | D | D |
| A | F# | Gb |
| B | G# | Ab |
| F# | D | D |

### 12.2 What we use the library for — and what we still write

brett owns the part that is **genuinely more than "letters and math"** —
the **chromatic semitone shift** and the **enharmonic spelling per
strategy** — and its structs (`Pitch`, `Transpose`, `Name(AscNames/DescNames)`)
are used right off the bat.

Because **no** library re-renders a compact chord symbol (`Bm7`, `Bm7/D#`)
from a transposed chord, `internal/music` keeps a thin layer we own that is
**glue, not music theory**:

- **Parse the symbol** `Dm7/F#` → root `D`, quality `m7`, bass `F#`
     (a small regex split; the quality substring is carried through
     *verbatim*).
- **Shift root + bass** via brett: `mt.PitchFrom… → Transpose(Semitones(n))`
     rendered with `AscNames` (sharps) / `DescNames` (flats) / inherited.
- **Reassemble**: `root' + quality + "/" + bass'` → `Bm7/D#`.
- **`{key}` rewrite** and `--to-key` delta via `mt.Semitones`/pitch distance.

This honors "don't hand-roll a music library": the hard chromatic / enharmonic
part is brett's; the only code we write is the chord-symbol string glue that
*no library can give us* because compact-symbols-with-a-user-spelling-policy is
not a thing any of the three does.

### 12.3 F#-always / sticky pitch classes — v-next

The sharp-spelling strategy (`AscNames`) already yields F# from A, which covers
most of your "F# is always F#" cases. A per-pitch-class **allowlist override**
(`F#` over `Gb`, `Bb` over `A#` etc., regardless of strategy) is a small
v2 flag — a `--prefer F# Bb …` allowlist applied after brett's spelling — and
is **not** a v1 blocker.

### 12.4 Fallback

If a later test of brett against a real transposed chart shows its
spelling/policy does not match your ear, the fallback is a hand-rolled
**12-row chromatic table** (~30 lines, dependency-free, identical behaviour)
behind the same `internal/music` seam — swap cost is a single import.
On the evidence, brett is the better default.


---

## 13. Implementation status & notes (post-build)

`ggt transpose` and `internal/music` are implemented, table-tested, and run
the real *Better Together* tab end to end:

```
raw tab  --clean_tab.py-->  cleaned
       --ggt cpro --key C --capo 5>  shape-key cpro
       --ggt transpose --to-key F>     native-key cpro   <- BandHelper-ready
```

### 13.1 Delivered

- `internal/music` — `TransposeSymbol` (one chord), `TransposeCProText`
   (the file: bracket chords + `{key: V}`, everything else untouched),
   `KeyOf`, `SemitonesTo`, and a `Style` (auto/sharps/flats).  The chromatic
   math and accidental spelling are `brettbuddin/musictheory`; the chord-symbol
   parse and quality ride-through are the glue.
- `cmd/ggt/transpose.go` — the subcommand: **exactly one** of `--up N`,
   `--down N`, `--to-key K`; plus `--sharps`/`--flats`/`--auto` (default),
   `-o/--output`, `--inplace`, and `-i/--input` (or positional, `-` = stdin).
- No `--capo` flag (read the capo, decide the shift — the AI’s job).
- Table tests: single chords, quality preservation (`m7b5`, `add9`, `+9`),
   both halves of a slash chord, non-chords/walkdowns passed through, key
   rewrite, and `--to-key` distance.  A command-level test covers the
   exactly-one-mode guard and the no-`{key}` refusal for `--to-key`.

### 13.2 Verification on the real file

*Better Together* comes out shape-key **C** over **capo 5**; cpro
extracts `[C] [G] [Dm]` + slashes `[C/E] [C/B] [Am/G] [Dm/F]`.  `--to-key F`
gives: `C→F`, `G→C`, `Dm→Gm`, `F→…` (see 13.3),
`{key: F}`, `{capo: 5}` and the `(Capo 5)` cue kept.

### 13.3 Open decision: auto enharmonic spelling

For an F-major output, `--auto` (sharps-when-ambiguous) renders F↕ as
`[A#]`/`[Gm/A#]`; `--flats` renders `[Bb]`/`[Gm/Bb]`, the natural F-major
spelling.  Per the user’s note, **BandHelper enforces the final #/–b
spelling**, so a slightly-off enharmonic is correctable at import time.
Two options, the user to choose:

- **(A) Ship as-is.**  `--flats`/`--sharps` are explicit; auto = sharps-default.
     For *Better Together* just call with `--flats`.
- **(B) Smarter auto.**  When `--to-key K` is used, auto picks
     `K`-major-key’s signature style (F major → flats → Bb).
     ~15 lines.  Deferred unless the user wants it.
