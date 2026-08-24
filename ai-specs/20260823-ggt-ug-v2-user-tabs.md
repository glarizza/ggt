# ggt ug fetch — user tab support design

## Problem

`ggt ug fetch <url>` only handles UG official tabs via the `pro_meta` API. UG
user-submitted tabs (URLs with `-chords-N`, not `-official-N`) are not returned
by `pro_meta` with the tab URL's ID — they return HTTP 404.

## Key discovery

UG user tabs that have been converted to UG Pro format DO have a version record
in `pro_meta`, but under a *version ID* (not the tab URL ID). The version ID
is embedded in the user-tab HTML page as JSON metadata:
&nbsp;&nbsp;&nbsp;`versions[{"id":1947541,"song_id":15747,"song_name":"...","artist_name":"Queen",...}]`

Calling `pro_meta?id=1947541` returns HTTP 200 with full structured content:
`name`, `artist`, `tempo`, `meta.tuning`, `meta.capo`, `meta.duration`,
`strummingPatterns`, `lyrics`, `tracks` — identical structure to official tabs.

## Design

### New orchestrator: `FetchTabByURL(url) *UGMeta`

Tries two strategies in order:

1. **Direct `pro_meta`** — call `FetchUGMeta(url)` which does
   `pro_meta?id=tabID`. If HTTP 200, return. Official tabs go this route.
2. **Version-ID fallback** (on direct 404):
   a. Fetch HTML page `https://tabs.ultimate-guitar.com/tab/…` with a Go
       `httpclient`.
   b. Extract version ID from HTML via regex on `&quot;versions&quot;:[{&quot;id&quot;:(\d+)`.
   c. Call `FetchUGMeta` with the version ID.
   d. If HTTP 200, return.
   e. If still 404, return a descriptive error.

### HTML version ID extraction

The version ID lives in an HTML-escaped JS variable in the page. For user tabs
the field is:
`&quot;versions&quot;:[{&quot;id&quot;:NNNNNN,...}] `
The regex is:
`&quot;versions&quot;:\[\{&quot;id&quot;:(\d+)`
which gives us the first version ID. Only the first version is used
(other versions in the array are alternate transcriptions).

### Files to add
- `internal/ug/tab.go` — `FetchTabByURL(url) (*UGMeta, error)`.
- `internal/ug/tab_test.go` — tests for version ID extraction from a
  synthetic HTML snippet and for the 2-step orchestration.
- `cmd/ggt/ug.go` — `runUGFetch` calls `FetchTabByURL` instead of
`FetchUGMeta`.
- `ai-specs/20260823-ggt-ug-v2-user-tabs.md` — this design doc.

### Regex for version ID

The HTML has the version record in HTML-escaped JSON:
`&quot;versions&quot;:[{&quot;id&quot;:1947541,...}]`
The Go regex is:
```go
re = regexp.MustCompile(`&quot;versions&quot;:\[\s*\{\s*&quot;id&quot;:\s*(\d+)`)
```
or (less fragile, matches any `versions` occurrence):
```go
re = regexp.MustCompile(`versions.{0,10}"id"[:\s]+(\d{5,})`)
```
The first pattern is more precise.

### Backward compatibility

Existing official tab calls (Hurt, Free Fallin', etc.) are unchanged —
they go via the direct `pro_meta` path. The version-ID fallback only
activates when `pro_meta` returns 404 for the tab URL ID.

### Risks / limitations
- Mraz user tabs (e.g. `i-wont-give-up-chords-4`) return 404 on BOTH
  `pro_meta?id=4` and on the HTML fetch. These tabs may have been
  removed or may not have a Pro version. The error message should clearly
  state "no pro version found for this tab."
- HTML fetching might hit Cloudflare challenge (for some IPs). We're
  already using `curl` in the POC and it worked. Go's `http.Client`
  should also work with a browser-like User-Agent.
- The version ID might not exist for all user tabs. Some user tabs
   might not have a Pro version at all.

## Testing

1. `TestFetchTabByURL_OfficialTab` — mock HTTP server, verify
   `pro_meta?id=1948797` → 200 → returns UGMeta.
2. `TestFetchTabByURL_VersionIDFallback` — mock HTTP server,
   `pro_meta?id=213415` → 404, HTML page returns 200 with
`versions[0].id=1947541`, then `pro_meta?id=1947541` → 200.
3. `TestFetchTabByURL_NoVersionFound` — 404 on both, returns error.
4. `TestExtractVersionID` — pure function, tests regex extraction
   of version ID from HTML snippet.
