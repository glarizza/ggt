# 20260824-ggt-ug-content-truncation.md

Design/bug doc: `ggt ug fetch` truncates UG user-tab content at the first
*escaped quote inside the tab text*, so a whole chord+lyric body is dropped and
`sour37`-style amalgam tabs (and any tab whose prose contains a quoted phrase)
come back as a one-paragraph intro note.

Priority: fix `ggt ug fetch` so a `-chords-` URL that HAS a chord body returns the
full body. This is independent of the per-song conversion work (deferred until the
fix lands).

---

## PROBLEM

URL: `https://tabs.ultimate-guitar.com/tab/barenaked-ladies/some-fantastic-chords-1219927`

What a browser shows at that URL: a full chord chart by UG user `sour37`
(21 votes, "amalgamation of various tabs", Standard tuning, Capo 3) — ~5500 chars
of `[ch]...[/ch]`/`[tab]...[/tab]` chord+lyric markup:

```
This is how I play it, solo acoustic, bit of an amalgamation of various tabs and
chord patterns out there...recommend watching the "bathroom session" version on
the youtubes for a sense of rythm...

Capo 3

[Intro]
[ch]G[/ch] [ch]Am7[/ch] [ch]Bm[/ch] [ch]Am7[/ch] x2
[Chorus]
[tab][ch]G[/ch] [ch]D/F#[/ch] [ch]Cadd9[/ch] [ch]A7[/ch]
There's a lot I will never do
...
```

What `ggt ug fetch` returns (cleaned stdout) — exit 0, no error:

```
This is how I play it, solo acoustic, bit of an amalgamation of various tabs and
chord patterns out there...recommend watching the \
```

135 characters. The intro note only. The `[Intro]`/`[Verse]`/`[Chorus]`/`[Bridge]`/
`[Outro]` body, every chord, and every lyric line are silently gone.
`ggt cpro` then has nothing to convert. Because `ggt ug fetch` exits 0, this
failure is invisible — it looks like "a tab that just has a note", not "the body
was truncated".

This is the same class of bug as `20260824-ggt-ug-user-tab-fix.md` (a tab the URL
points at not being the tab you get), but a distinct root cause: here we DO fetch the
right tab (`content`, `revision_id`, `username=sour37`, 2→all ours), but the content
string is cut short.

---

## ROOT CAUSE

`internal/ug/tabdata.go`, `extractContent` → the HTML-escape fallback regex:

```go
reContentHTML = regexp.MustCompile(
    `&quot;content&quot;:&quot;(.+?)&quot;`)
```

### How UG embeds the tab

UG embeds the tab data in the page as **HTML-escaped JSON**. In the page bytes the
`content` value looks like:

```
&quot;content&quot;:&quot;This is how I play it, ... recommend watching
the \&quot;bathroom session\&quot; version on \r\nthe youtubes...
Capo 3\r\n\r\n[Intro]\r\n[ch]G[/ch] [ch]Am7[/ch] ...
```

Key detail: **inner quotes inside the tab prose are escaped as `\&quot;`**
(a JSON-escaped `\"` that is itself HTML-entity-encoded). CRLF stays literal
(`\r\n` as 4 chars: `\`,`r`,`\`,`n`).

### Why it truncates

`(.+?)` is **non-greedy**, so it stops at the *first* `&quot;` after the opening
delimiter. The first `&quot;` is the one inside `\&quot;bathroom session\&quot;`.
The captured value is therefore:

```
This is how I play it, ... recommend watching the \
```

`htmlUnescape` then turns the trailing `\&quot;`→ (the lone `\` is a JSON escape
that `unescapeJSONString` leaves as `\`), giving the 135-char intro note. Every
character after the first inner escaped quote — the entire chord body — is discarded.

### Why the plain-JSON path didn't save us

`extractContent` tries the plain-JSON regex first:

```go
reContentPlain = regexp.MustCompile(`"content"\s*:\s*"((?:\\.|[^"\\])*)"`)
```

That one is escape-correct (`(?:\\.|[^"\\])*` skips backslash-escaped pairs). BUT
it requires literal ASCII `"` delimiters. This page's content is HTML-escaped
(`&quot;`, not `"`), so `reContentPlain` does **not** match, and `extractContent`
falls through to the broken non-greedy `reContentHTML`. So we hit exactly the
un-escaped-quote-unsafe path.

### Why the existing test passed

`internal/ug/tab_test.go:45` ships a single HTML-escaped content fixture:

```
&quot;content&quot;:&quot;Capo on 7th Fret\\r\\n\\r\\n[Intro]\\r\\n[ch]Em[/ch] [ch]Cadd9[/ch]\\r\\n&quot;
```

That value contains **no inner escaped quote**, so `(.)+?` runs clean to the real
closing `&quot;`. The test asserts `found==true` only — it never checks that the
*full* value (with an inner escaped quote) survives. A tab with a quoted phrase
in prose is untested, which is why the truncation shipped.

---

## REPRODUCTION

```
ggt ug fetch 'https://tabs.ultimate-guitar.com/tab/barenaked-ladies/some-fantastic-chords-1219927'
```

Exit 0; prints the 135-char intro note. `grep -c '\[ch\]' ` on the output = 0.
A correct extraction yields ~5500 chars with dozens of `[ch]...[/ch]`.

Unit repro (no network): feed `extractContent` a string containing an inner
`\\&quot;`-escaped quote and assert the returned value contains the text after that
quote (e.g. the `[Intro]` block). Current code returns only the pre-quote prefix.

---

## THE FIX

Make the HTML-escaped content extraction **escape-aware** instead of regex
non-greedy. Two equivalent options:

### Option A (preferred): escape-aware scanner

Stop using a regex for the HTML path. Locate `&quot;content&quot;:&quot;` with
`strings.Index`, then scan the raw bytes to the *closing* delimiter, treating
`&quot;` that is **preceded by an un-escaped backslash** as an inner quote (keep
scanning) and a bare `&quot;` as the close.

Sketch:

```go
func extractContentHTML(raw string) (string, bool) {
    open := `&quot;content&quot;:&quot;`
    i := strings.Index(raw, open)
    if i < 0 {
        return "", false
    }
    i += len(open)
    start := i
    for i < len(raw) {
        if raw[i] == '\\' {                 // JSON escape: skip the escaped token
            if i+1 < len(raw) && raw[i+1] == '&' {
                // skip the following &entity; (e.g. &quot; &amp; &lt;)
                if j := strings.IndexByte(raw[i+1:], ';'); j >= 0 {
                    i += j + 1
                } else {
                    i = len(raw)
                }
            } else {
                i += 2                       // escaped single char (\r, \n, \t, \<char>)
            }
            continue
        }
        if raw[i:i+6] == `&quot;` {         // un-escaped closing delimiter
            return raw[start:i], true
        }
        i++
    }
    return raw[start:], true                // unterminated: best-effort, not 0
}
```

This is the correct model: inner quotes are always `\&quot;` (backslash-prefixed),
the true close is a bare `&quot;`. No lookahead needed (Go `regexp` has none), no
key-order assumptions. `extractContent` becomes: plain-JSON first (unchanged),
else `extractContentHTML`.

### Option B (minimal): greedy + structural tail anchor

`content` is the first key in UG's embedded object, so its value's closing
`&quot;` is always immediately followed by `,` then the next `&quot;` key. A
greedy match that requires a trailing `,` (or `}`) survives inner `\&quot;`:

```go
reContentHTML = regexp.MustCompile(
    `&quot;content&quot;:&quot;(.*)&quot;\s*[,&}]`)                 // group1 = full value
```

`.*` runs to the last `&quot;` that still leaves a `,`/`}` after it — i.e. the real
close — so inner `\&quot;...` are swallowed. Downside: depends on `content` being
first (holds in observed fixtures, but is a structural assumption) and on a char
following the close. **Option A is strictly more robust** and is recommended.

---

## CLEAN STEP COMPATIBILITY

Once the full content comes through, the rest of the pipeline already handles this
tab (per `20260824-ggt-ug-user-tab-fix.md`):

- CRLF → LF: `normalizeUserTabContent` runs `unescapeJSONString` (collapses
   literal `\r\n`→`\n`) before `Clean()`. ✓
- Leading `Capo 3` instruction: `extractCapoFromInstruction` /
   `stripCapoInstruction` catch `^Capo\s+on\s+?(\d+)...` — note this tab's
   instruction is just `Capo 3` (no "Fret"/ordinal), which the existing regex
   `^capo\s+(?:on\s+)?(\d+)\s*(?:rd|th)?\s*(?:fret)?\b` **does** match (`capo 3`). ✓
   Worth a fixture to lock that.
- `[ch]...[/ch]`, `[tab]...[/tab]`, `[Intro]`/`[Verse]`/`[Chorus]`/`[Bridge]`/
   `[Outro]` section labels, `x2` repeats: already handled by `clean.go` / `cpro`. ✓

---

## IMPACT

| File | Change |
|---|---|
| `internal/ug/tabdata.go` | Replace the non-greedy `reContentHTML`/regex fallback in `extractContent` with an escape-aware scanner (Option A) — or the greedy structural regex (Option B). |
| `internal/ug/tab_test.go` | Add a fixture asserting a value **with an inner `\&quot;`-escaped quote** is extracted *in full* (text after the quote present), and that a bare `&quot;`-close terminates. Lock the `Capo 3` (no "Fret") instruction strip. |
| `internal/ug` | No signature changes; `extractContent`/`reContentHTML` stay. Purely internal behaviour. |

**Do we break existing tests?** No. The plain-JSON path is untouched. The HTML path
strictly *gains* the inner-escaped-quote case; the existing no-inner-quote fixture
still extracts correctly under either option. `make test` should stay green plus
the new fixture.

**Risk:** UG HTML entity-set. Option A handles any entity after a backslash by
skipping to `;`. If UG ever emits an un-backslashed bare `&quot;` mid-content, both
options stop there — acceptable and matches valid JSON (a bare `"` in a JSON string
is a syntax error).

---

## TESTING PLAN

1. `make test` — all existing `internal/ug` tests still pass.
2. `go vet ./...` clean.
3. New unit test: `extractContent` on a value containing `\&quot;bathroom session\&quot;`
   returns text that includes the material *after* `"bathroom session"` (e.g. the
   `[Intro]` block). Currently fails; passes after fix.
4. New unit test: `stripCapoInstruction` on `Capo 3\n...` yields `"3"` and strips the
   line (locks the no-"Fret" form found in this tab).
5. Live test:
```
ggt ug fetch 'https://tabs.ultimate-guitar.com/tab/barenaked-ladies/some-fantastic-chords-1219927' -f -c
grep -c '\[ch\]' ignored/.
```
   Expected: full body (~5500 chars), `grep -c '\[ch\]'` ≫ 0, fact sheet reports
   `Source: user-tab-html`, `Capo (UG data): 3`, `Votes: 21`, `sour37`.
6. `make build` → re-run the `some-fantastic` conversion (the deferred song task)
   end-to-end and confirm 0 UG fragments in the output.

---

## ACCEPTANCE CRITERIA

- `ggt ug fetch` on a `-chords-` tab whose prose contains a quoted phrase returns the
  **entire** chord+lyric body, not the pre-quote prefix.
- `grep -c '\[ch\]'` on that output is > 0.
- Existing PRO-tab and user-tab extractions unaffected.
- New regression test locks the inner-escaped-quote case.

---

## VERSION

This is a bug fix to an existing capability → **bump patch**:
`make version-patch` (e.g. `0.5.0` → `0.5.1`). Not a new feature, not breaking.

---

## NOTE FOR THE DEFERRED SONG TASK (Some Fantastic)

Facts gathered while diagnosing — resume after the fix lands so we run it through the
*fixed* `ggt ug fetch` (do not hand-clean a truncated body):

- **Capo (UG-authoritative):** 3. No cross-check (capo is gospel per skill).
- **Key (inferred + cross-checked → Bb):** raw shapes are G-major diatonic
   (`G / Am7 / Bm / Bm7 / Em / D/F# / Cadd9 / A7 / Em7`); G + capo 3 = **Bb**.
   Cross-checks agree: Singing Carrots "Original Key: Bb Major"; ChordU lists
   `Bb, F, Eb, Gm, C` and `Bb, C, Eb, F, Gm` (Bb-major diatonic). → `{key: Bb}`.
   Personal-transpose reminder after import: **−3**.
- **Tempo:** NOT in user-tab HTML. Sources found so far: none specific yet —
   SongBPM `@barenaked-ladies` list page did not surface a "Some Fantastic" row in
   the excerpts; needs a direct SongBPM/Tunebat lookup (+ a 2nd source) before import.
- **Duration:** NOT in user-tab HTML. Spotify lists the studio track at **4:16**
   (`open.spotify.com/track/5owFDMw1NaFwsfW4uGewK3`); the `donignacio` review calls
   it a ~3-min (shortest on the album) but Spotify's 4:16 is the stronger figure.
   Cross-check a 2nd duration source before import.
- **Time sig:** assume 4/4, flag as unverified.
- **Title/Artist:** `Some Fantastic` / `Barenaked Ladies` (from URL slug; also the
   UG `tab` object / schema.org `CreativeWork`).
- Output target after fix: `ignored/ready_for_import/some-fantastic.chopro`.
   Only land once tempo (2+ sources) and duration (2+ sources or flagged "assumed")
   clear the skill's gate.
