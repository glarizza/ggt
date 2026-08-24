# ggt ug — design doc and POC results

Status: implemented (2026-08-23). Design doc + POC + Go implementation committed together.

## Problem

After 5 UG pipeline runs we had a working *workflow* but zero *tooling*.
Every stage of the pipeline was typed inline in a chat heredoc each time.
This caused 3 concrete problems:
- The Python `re.sub(..., re.DOTALL)` positional-arg bug (DOTALL as the
   `count` argument, not a flag — Python 3.11 deprecation).
- No post-condition check — a missed strip was invisible.
- No type safety between stages.

Gary's instruction: "if it's the same 3 steps every time, in the same order,
that set is one workflow, not 3 scripts. Don't let the implementation steps
dictate the subcommand API; let the user's intent drive the subcommands."

## POC result (the lynchpin question)

**`pro_meta` API is public — no auth needed.**

```
$ curl 'https://api-web.ultimate-guitar.com/v1/tab/pro/meta?id=1948797'
# HTTP 200, full JSON, no cookies, no Cloudflare, no login
```

The whole "auth lynchpin" turned out to be a non-problem. The
`browser_evaluate_script` we've been using was going through the *tab page*
(Cloudflare-gated). The *API* (`api-web.ultimate-guitar.com`) is public.
`ggt ug fetch` is a 20-line `net/http.Get()`.

### Other POC findings

- **`name` and `artist` ARE in the `pro_meta` API response.** 4 out of 5
   songs returned them. UG sometimes adds trailing spaces (e.g.
   "Free Fallin' " with a space) — we trim via `strings.TrimSpace()`.
- **URL ID extraction:** UG URLs are
   `.../tab/<artist-slug>/<song-slug>-<id>`. The trailing number after the
   last `-` is the tab id. `ExtractTabID()` handles this.
- **`strummingPatterns[].bpm` is closer to the real tempo than the
   top-level `tempo` field.** For all 5 trail songs, top-level tempo was
   120 (UG default); strumming BPMs were 135, 85, 90, 96, 91 respectively.
- **Mraz "I Won't Give Up" (user tab):** `pro_meta` returns HTTP 404.
   Likely not indexed in the official-tabs API. User-tab path is out of
   scope for now.

## Architecture

### Internal packages (`internal/ug/`)

| File             | Purpose                                             |
| ---------------- | --------------------------------------------------- |
| `meta.go`        | `UGMeta` struct, `FetchUGMeta(url)`, `ExtractTabID` |
| `clean.go`       | `Clean(text)` — 6-pass UG-lyrics strip + post-condition check |
| `chords.go`      | `AnalyzeChords(cleanText)` → `ChordFrequency`, `TonalCandidate` |
| `clean_test.go`  | 6 tests: `TestClean_Hurt`, `TestClean_FreeFallin`, `TestClean_PostCondition`, `TestExtractTabID`, `TestExtractTabID_Errors`, `TestAnalyzeChords_Hurt` |
| `testdata/`      | `hurt-lyrics.txt`, `hurt-clean-expected.txt`, `freefallin-lyrics.txt`, `freefallin-clean-expected.txt` — real UG tab data used as fixtures |

### User-facing API (`cmd/ggt/ug.go`)

```
ggt ug fetch <tab-url> [flags]    # fetch from UG API, then clean
ggt ug       <meta.json>  [flags] # clean a saved pro_meta JSON
```

Both modes share the same flag set via `ugFlagSet`:
```
--facts (-f)    Print fact sheet to stderr
--chords (-c)   Print chord frequency to stderr
--title         Title for conversion (default: from UG)
--artist        Artist for conversion (default: from UG)
--key           Key for cpro mode; triggers full ChordPro conversion
--capo N        Capo fret number
--remove-capo   De-cap the body (requires --capo N)
--tempo N       Tempo in BPM (default: from UG)
--time SIG      Time signature
--duration D    Duration mm:ss (default: from UG)
-o, --output F  Write to file instead of stdout
```

### What `ggt ug` does NOT do

- `browser_navigate_page` — Cloudflare handling still needed for the *page*.
  `pro_meta` API bypasses Cloudflare.
- `parallel-search_web_search` cross-check — agent-side judgment, not tooling.
- `ggt cpro` call — the *final* conversion still needs user-chosen
    key/tempo/`--time` via `ggt cpro` flags. `ggt ug` optionally runs
    `cpro` internally when `--key` is set, but `ggt cpro` remains the
    universal ChordPro tool for all paths.

## Key design decisions

1. **Clean approach: keep `[tab]` content, strip the markers.**
   Don't drop `[tab]...[/tab]` blocks entirely — that was a Python bug
   (using `re.DOTALL` as a positional argument to `re.sub`). The correct
   approach strips only the `[tab]` and `[/tab]` markers, keeping the
   chord+lyric content between them. This is what produces the
   "chord row on line N, lyric row on line N+1" layout that `ggt cpro`
   can handle.

2. **Post-condition check in the Go code, not a separate step.**
   `Clean()` checks its own output for fragments before returning.
   If a strip pattern missed, `Clean()` returns an error, not a clean-
   looking output that silently misses something. This is the "hard
   error, not a warning" pattern.

3. **`name`/`artist` auto-fills from UG meta data.**
   If the user doesn't pass `--title`/`--artist`, `ggt ug` fills them
   from `meta.Name` / `meta.Artist` (with `strings.TrimSpace()` to
   handle UG's occasional trailing spaces).

4. **`--facts` and `--chords` go to stderr; the clean text goes to
   stdout.** This lets you do `ggt ug fetch <url> > clean.txt 2>
   facts.txt` and pipeline naturally.

5. **`--key` triggers the chopro path.** Pass `--key A --tempo 91 --time
   4/4` to `ggt ug fetch` and it does the fetch + clean + chopro in
   one call. Pass no `--key` and it just prints clean text. This
   mirrors the "clean then cpro separately" pipeline but in one
   invocation if you have all the values.

6. **Test data are real UG tab excerpts.** `internal/ug/testdata/`
   has real UG `lyrics` and known-good clean outputs for 2 songs
   (Hurt, Free Fallin'). The copyright concern: these are real
   copyrighted lyrics. This is the test-fixture issue — the
   `sample-tabs/` and `testdata/` convention uses *scrambled* lyrics,
   not real ones. The UG test fixtures need to be either scrambled
   or we need a different approach (e.g. just the first 10 lines as
   a minimal fixture, or a "no-copy" policy for `internal/ug/`).

## Test results

```
$ go test ./internal/ug/ -v
=== RUN   TestClean_Hurt
--- PASS: TestClean_Hurt (0.00s)
=== RUN   TestClean_FreeFallin
--- PASS: TestClean_FreeFallin (0.00s)
=== RUN   TestClean_PostCondition
--- PASS: TestClean_PostCondition (0.00s)
=== RUN   TestExtractTabID
--- PASS: TestExtractTabID (0.00s)
=== RUN   TestExtractTabID_Errors
--- PASS: TestExtractTabID_Errors (0.00s)
=== RUN   TestAnalyzeChords_Hurt
--- PASS: TestAnalyzeChords_Hurt (0.00s)
PASS
ok  	ggt/internal/ug	0.307s
```

## `ggt ug` output verified against known-good

```
$ ggt ug fetch <hurt-url> --facts --chords -o out.txt
$ diff out.txt ignored/hurt-clean.txt
(no differences)
$ ggt ug fetch <hurt-url> --key A --tempo 91 --time 4/4 --duration 3:41 -o out.chopro
$ diff out.chopro ignored/ready_for_import/hurt.chopro
(no differences)
```

## Open work / next steps

1. **Copyright in `testdata/`:** the test fixtures use real UG tab
   lyrics. Need to either (a) scramble them, (b) replace with short
   synthetic fixtures, or (c) document a test-data convention.
2. **Time signature cross-check:** `ggt ug` doesn't determine the
   time signature — still needs `--time` from the agent's judgment
   after cross-checking. The `--time` flag is there for `ggt cpro`
   to emit `{time: 4/4}` but no one cross-checks it yet.
3. **Mraz / user tabs:** `pro_meta` returns 404 for user tabs. Need
   a separate API path for user-submitted tabs.
4. **`ggt cpro` direct mode via `ggt ug`:** the `--key` path produces
   a chopro output but doesn't print the chopro header to stdout
   — only to the `-o` file. Need to also print to stdout when no
   `-o` flag. (The current code prints the clean text, not chopro,
   when `--key` is set and no `-o`.)
5. **Personal-transpose reminder:** print at the end of `ggt ug` when
   `--capo N` is set, the "remember to set -N in BandHelper" line.
   Currently this is an agent-side instruction.
6. **`chords` flag on top-level `ggt ug <meta.json>`:** the
   `--chords` flag is on the top-level command, but testing shows
   it works for the fetch subcommand. Verify top-level mode too.

## Files touched this commit

| File                              | Change                          |
| --------------------------------- | ------------------------------- |
| `internal/ug/meta.go`             | New — UGMeta+FetchUGMeta+ExtractTabID  |
| `internal/ug/clean.go`            | New — 6-pass UG-lyrics clean + post-condition |
| `internal/ug/chords.go`           | New — AnalyzeChords + TonalCandidate  |
| `internal/ug/clean_test.go`       | New — 6 tests, 2 real tab fixtures  |
| `internal/ug/testdata/*.txt`      | New — 4 fixture files (2 songs)  |
| `cmd/ggt/ug.go`                   | New — 10 KB Cobra subcommand      |
| `cmd/ggt/root.go`                 | Register `newUGCmd`           |
| `ai-specs/20260823-ggt-trail-scripts.md` | Superseded by this doc |
