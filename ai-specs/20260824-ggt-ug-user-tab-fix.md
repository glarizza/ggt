# 20260824-ggt-ug-user-tab-fix.md

Design doc: fix `ggt ug fetch` to fetch the tab the URL actually shows, not an
arbitrary version of the same song.

---

## PROBLEM

`ggt u` silently substitutes a DIFFERENT UG tab for user-submitted (`-chords-`) URLs.

URL: `https://tabs.ultimate-guitar.com/tab/sarah-mclachlan/building-a-mystery-chords-696128`

- What a browser shows at that URL: Em-shape tab, capo **7**, user `fashy`sguitar11``,
  311 votes.
- What `ggt u` fetches: F#m-shape tab, capo **5**, user `vilkot1`, the UG Pro version.

The user tab (`696128`) and the UG Pro tab (`3871616`) are DIFFERENT tabs by DIFFERENT
authors. `ggt u`, when asked for tab `696128`, returns tab `3871616`.

This violates a core principle of the tool: "if this URL is requested, this tab
is returned."

---

## ROOT CAUSE

Two bugs compound:

### Bug 1: `pro_meta` API is PRO-only

`pro_meta` (at `api-web.ultimate-guitar.com/v1/tab/pro/meta?id=X`) returns HTTP 404 for
any user tab whose ID is not a UG Pro tab. So `pro_meta?id=696128` → 404.

`ggt u`'s `fetchUserTabFromHTML` fallback then extracts a version ID from the HTML and
re-tries `pro_meta`. This is the wrong strategy for user tabs — user tabs DON'T have a
`pro_meta` endpoint.

### Bug 2: `extractVersionID` picks the wrong ID

`extractVersionID` matches the first entry in UG's `versions[]` array embedded in the
HTML. For `building-a-mystery-chords-696128`, that array is:

    versions[0].id = 3871616   ← UG Pro tab, F#m shapes, capo 5
    versions[1].id = 93340     ← another user tab
    versions[2].id = 1209729   ← another user tab
    versions[3].id = 1878625   ← another user tab

`696128` is `versions[4]` or worse, not in the `versions` array at all. It's in a
separate `"tab":{"id":696128,...}` object in the HTML. So `versions[0].id = 3871616` is
the UG Pro tab, not the one the URL points to.

`ggt u` then calls `pro_meta?id=3871616` and gets the UG Pro (F#m, capo 5) — wrong.

---

## DATA MODEL UNDERSTANDING (what UG actually provides)

### Two tab types

| Type | URL pattern | `pro_meta` API? | HTML `content` field? |
|---|---|---|---|
| **UG Pro / official** | `-official-XXXXXXX` | YES (HTTP 200) | Yes (in HTML page) |
| **User-submitted** | `-chords-XXXXXX` | NO (HTTP 404) | YES (in HTML page) |

UG's `pro_meta` endpoint serves UG Pro tabs only. User tabs are served entirely via HTTP
HTML. Both are available in the HTML with `content` (full chord+lyric text) and `meta`
(capo, tuning).

### HTML embedded data (works for ALL tabs)

UG embeds the full tab data in the page HTML as HTML-escaped JSON. These fields are
always present:

| Field | Path in HTML | Content |
|---|---|---|
| Chord+lyric text | `content` | Full tab with `[ch]...[/ch]`, `[tab]...[/tab]`, `[syllable]...` |
| Capo | `meta.capo` | Integer (0 = no capo) |
| Tuning | `meta.tuning.value` | String: `"E A D G B E"` or `"Gb B E A B E"` |
| Title | `tab.song_name` / `tab.localised_song_name` | Song title |
| Artist | `tab.artist_name` | Artist name |
| Author | `tab.username` | UG user who submitted the tab |
| Votes | `tab.votes` | Tab popularity |
| Difficulty | `meta.difficulty` | `"easy"` / `"intermediate"` / `"advanced"` |

### Fields NOT in HTML (must come from `pro_meta` OR external cross-check)

- `tempo` — UG top-level `tempo` IS in `pro_meta` (often 120 as a default, unreliable).
  Not in HTML.
- `strumBPM` — in `pro_meta` only, from `strummingPatterns[].bpm`. Often closer to real.
- `duration` (in ms) — in `pro_meta` only. Not in HTML.
- `tracks` — in `pro_meta` only.

---

## THE FIX

### Strategy change

`ggt u`'s `FetchTabByURL` flow should be:

```
1. Try pro_meta with the URL's tab ID.
   - If HTTP 200: use pro_meta data. (PRO tab path, current behaviour.)
   - If HTTP 404: DON'T fall back to versions[0].id.
     Instead, extract content+meta from the HTML we already fetched.
     (NEW: user-tab HTML-direct path.)
2. For user tabs, output a fact sheet that says:
   "Fetched user tab by <username> via HTML (no pro_meta available)."
   Fields tempo/duration/strumBPM: "Not available — cross-check externally."
3. Clean() the extracted content.
4. Output clean text + fact sheet, same as PRO tab path.
```

### Key change in `tab.go`

Current (wrong):
```
FetchTabByURL(tabURL):
    meta, err := FetchUGMeta(tabURL)       # ← pro_meta by URL's ID
    if err == ErrNotFound:
        return fetchUserTabFromHTML(tabURL)  # ← WRONG: re-fetches pro_meta,
                                             #   grabs versions[0].id
```

New (correct):
```
FetchTabByURL(tabURL):
    meta, err := FetchUGMeta(tabURL)       # ← pro_meta by URL's ID
     
    if err == ErrNotFound:
        # User tab: extract from HTML directly,
        # NO second API call, NO versions[0].id fallback
        return fetchContentFromHTML(tabURL)  # ← NEW
     
    return meta, nil
```

### New function `fetchContentFromHTML`

Signature:
```
func fetchContentFromHTML(tabURL string) (*UGMetaFromHTML, error)
```

Input: the UG web-page URL (same as `tabURL`).
Output: a typed struct with the fields UG's HTML actually provides. Not `*UGMeta`
(bug: `UGMeta` is a `pro_meta` schema and has fields that don't apply here).

`UGMetaFromHTML`:
```
type UGMetaFromHTML struct {
    TabID      int     // the tab's own ID from the URL
    SongName   string  // from tab.song_name
    ArtistName string  // from tab.artist_name
    Content    string  // the full content field (UG markup, needs Clean())
    Capo       int     // from meta.capo
    Tuning     string  // from meta.tuning.value
    Difficulty string  // from meta.difficulty
    Username   string  // from tab.username (tab author)
    Votes      int     // from tab.votes
    HTMLSource bool    // always true — marks this as a user-tab path
}
```

### Fact sheet output (user-tab path)

Same layout as PRO-tab fact sheet but flags missing fields:

```
Song:                Building a Mystery
Artist:              Sarah McLachlan
Source:              UG user tab by "fashy`sguitar11" (id=696128, 311 votes)
Tuning:              E A D G B E
Capo (HTML):         7
Difficulty:          intermediate
Tempo:               NOT AVAILABLE in user-tab HTML — cross-check externally
StrumBPM:            NOT AVAILABLE
Duration:            NOT AVAILABLE
```

The "Capo (HTML)" label (not "Capo (UG data)") signals the source is HTML-embedded, not
`pro_meta`.

---

## CLEAN STEP COMPATIBILITY

The clean step (`internal/ug/clean.go`) already handles both `[ch app="..."]...[/ch]`
(pro tabs, `reChOpen` with `[^]]*`) and `[ch]...[/ch]` (user tabs, plain `[ch]`).
This was confirmed in the earlier user-tab support work.

But: the HTML `content` field has `\r\n` (Windows CRLF) in it and a `Capo on 7th Fret`
instruction string at the top. Both need stripping:

1. **CRLF → LF**: `clean_text = clean_text.Replace("\r\n", "\n").Replace("\r", "\n")`.
   Should be in the HTML-extraction path, before Clean().
2. **Capo instruction string**: UG user tabs often start the content with
   `Capo on 7th Fret` or similar. The clean step should strip it.
   Regex: `(?m)^Capo\s+on\s+(\d+)(?:rd|th)?\s+Fret\s*$` (case-insensitive).
   But: we ALSO need to capture the fret number from this string as a cross-check
   against `meta.capo` — they should agree.
3. **`(Instrumental)` and X-only lines**: already handled by `reXOnlyLine` and
   `reInstrumentalLine`.

---

## VERSION-IDENTIFICATION BUG (secondary, also fix)

The `extractVersionID` regex should be fixed to prefer the "main tab id" from the URL
over `versions[0].id`. Two approaches:

### Approach A: URL-matching (preferred)

Parse the URL to get the tab ID (e.g., `696128`). Then verify that ID appears in the
embedded HTML (in the `"tab":{"id":696128}` field). If it does, that IS the version to
fetch. No need to iterate `versions[]` at all.

This is actually the right design: the URL already tells us which tab the user wants.
The `content` field in the HTML is that tab's content. No need to re-derive a version ID
at all.

### Approach B: `versions[]` with `tab.url` matching

Find the `versions[]` entry where `versions[i].tab_url` matches the requested URL. Use
that entry's `capo` and other fields. But for the HTML path this is unnecessary if we
extract `content` directly.

**Recommendation: Approach A.** Use the URL's own tab ID. Verify it's in the HTML's
`"tab":{"id":XXX}` field. Extract `content`/`capo`/`tuning` from HTML directly.
No `versions[]` array needed for the HTML path.

---

## NEW TEST FIXTURES

Copyright-clean fixtures for HTML user-tab scraping:

| File | Content |
|---|---|
| `internal/ug/testdata/ug003-user-tab.html` | HTML snippet with `content`, `meta.capo=7`, `tab.id=333`, `versions[]`, no `pro_meta` |
| `internal/ug/testdata/ug004-user-tab.html` | HTML with `capo` instruction line at top, CRLF endings |
| `internal/ug/testdata/ug005-user-tab.html` | `content` with `(Instrumental)` and `X` section markers |

### Test assertions

- `fetchContentFromHTML(ug_url)` returns correct `SongName`, `ArtistName`, `Capo=7`,
  `Tuning="E A D G B E"`, `Content` length > 500.
- `Clean()` on `Content` from `ug004` returns 0 fragments (CRLF + capo instruction
  stripped).
- `Clean()` on `Content` from `ug005` returns 0 fragments and no `(Instrumental)` /
  X lines.
- Old PRO-tab tests (`ug001`, `ug002`) still pass unchanged.

---

## IMPACT ANALYSIS

| File | Change |
|---|---|
| `internal/ug/tab.go` | Replace `fetchUserTabFromHTML` with `fetchContentFromHTML`; stop calling `extractVersionID` for user tabs; remove `versions[0].id` fallback |
| `internal/ug/tab_test.go` | Update to test `fetchContentFromHTML` with `ug003-005` fixtures |
| `internal/ug/meta.go` | Add `UGMetaFromHTML` struct (or reuse `UGMeta` with new fields) |
| `cmd/ggt/ug.go` | `printFactSheetFromMeta`: handle `UGMetaFromHTML` type; add "Source: user tab by X" line |
| `internal/ug/testdata/ug003-005-*.html` | New copyright-clean fixtures |
| `ai-specs/` | THIS design doc |

**Does this break any existing tests?** The PRO-tab path (HTTP 200 from `pro_meta`) is
unchanged. The only behavioural change is the HTTP 404 path. If no test exercises the
HTTP 404 path with live URLs, no existing tests break.

**Risk: UG HTML structure changes.** User-tab HTML is a React-rendered page. This is
more fragile than `pro_meta` JSON. Mitigation: the fact sheet explicitly says
"Source: HTML extraction" so a user knows if a future UG layout change breaks it.

---

## NOT IN SCOPE (but noted)

- Tempo / duration for user tabs. UG HTML doesn't provide them. These must be
  cross-checked externally (SongBPM, Tunebat, YouTube time). The fact sheet should
  flag them as "NOT AVAILABLE".
- `--version N` override flag. Not needed for this fix — the URL IS the version
  identifier. But `--version N` might still be useful for the PRO tab path (to
  try a different `versions[i]` when the default is wrong).
- Enharmonic spelling fix for `ggt cpro` output. Separate issue, separate design doc.

---

## TESTING PLAN

1. `make test` — all existing tests pass.
2. `go vet ./...` — clean.
3. Live test:
```
ggt u fetch 'https://tabs.ultimate-guitar.com/tab/sarah-mclachlan/building-a-mystery-chords-696128' -f -c
```
   Expected fact sheet output:
```
Song: Building a Mystery
Artist: Sarah McLachlan
Source: UG user tab by "fashy`sguitar11" (id=696128, 311 votes)
Tuning: E A D G B E
Capo (HTML): 7
Difficulty: intermediate
Tempo: NOT AVAILABLE in user-tab HTML — cross-check externally
StrumBPM: NOT AVAILABLE
Duration: NOT AVAILABLE
```

4. Live test: run `ggt u fetch | ggt cpro` for building-a-mystery with `--capo 7 --remove-capo --key Bm`:
   body chords should be `D, B/C, G, Em, Am, ...` — all diatonic to Bm.

5. Regenerate `building-a-mystery.chopro` with `--capo 7 --remove-capo --key Bm`.
   Verify: Bm diatonic, 0 UG residue, no `Capo on 7th Fret` text in output.

6. `make build` and confirm binary reports `ggt 0.5.0` (bump minor for new HTML-extraction capability).

---

## VERSIONING

This is a new capability (HTML-extraction path for user tabs) → **bump minor**:
`make version-minor` → `0.4.0` → `0.5.0`.

---

## ACCEPTANCE CRITERIA

- `ggt u fetch '-chords-696128'` returns the tab that `696128` points to — NOT `3871616`.
- Fact sheet reports "Source: user tab by X (id=696128)".
- `content` from HTML passes through `Clean()` with 0 fragments.
- `building-a-mystery.chopro` regenerated with `--capo 7 --remove-capo --key Bm` is
  diatonic to Bm.
- PRO-tab URL (e.g. `-official-3871616`) still works via `pro_meta` fast path —
  NO regression.
- All 14 existing `internal/ug` tests pass.
- New tests for `ug003-005` HTML fixtures pass.
- `go vet ./...` clean.
- `make build` produces `ggt 0.5.0` binary.
