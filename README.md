# ggt — Gary's Guitar Tool

`ggt` is a small, single-purpose Golang CLI tool for guitar chord charts, built for the
workflow of a live-band setlist: fetch a chart from where it lives, convert it
into a **BandHelper-ready `.chopro` file in the native (sounding) key**, and
transpose it when a song has to move between keys.

The tool exists to do *that* conversion — and to do it well. It is not a
general tab framework and it is not a substitute for BandHelper; it is the
bridge between the charts you collect online and a `.chopro` file that BandHelper can open.

---

## Subcommands

| Command | What it does |
| --- | --- |
| `ggt ug fetch <url>` | Pull a tab / chord chart off the **Ultimate Guitar** site and clean its web markup into a plain-text chart — *step one*: get the source data that needs shaping. |
| `ggt cpro [INPUT]` | Convert a chart into a `.chopro` file: inline chord placement with a `{key}` / `{capo}` / `{tempo}` header. For capo charts it can fold in the transpose math so the output is in the **native key** (see `--capo` / `--remove-capo`). |
| `ggt transpose [INPUT]` | Shift an *existing* `.chopro` file to another key — `--up N`, `--down N`, or `--to-key K` — rewriting the `{key: V}` header to the new tonic. |
| `ggt version` | Report the binary's version, commit, build date, Go toolchain, platform, and its `release` vs `local`/`source` role, then exit. |

Run `ggt <subcommand> --help` for the full flag list of any command.

> **Two notes on naming.** The subcommand that writes the file is `cpro`
> (short for Chord**pro**); the file it produces — and the one BandHelper opens —
> uses the `.chopro` extension. The order above is the order you actually *use*
> them: fetch, then convert (and potentially transpose).

---

## `ug` — pull tab data from UG

`ug` is the first step of the workflow: it grabs a tab off the Ultimate Guitar
website and strips the web markup. Its job is simply to get the
*source data* out of the page so it can be formatted into a `.chopro` file.

### `ggt ug fetch <url>` — fetch + clean, then pipe to `cpro`

This is the front door. It fetches a tab, cleans the markup, and writes the
clean text to a file that the `cpro` subcommand then consumes:

```bash
# 1. Fetch + clean a tab from UG.
ggt ug fetch "https://www.ultimate-guitar.com/tab/artist/song-12345" -o clean.txt

# 2. Convert the clean text to .chopro.
ggt cpro clean.txt --key G --tempo 90 -o song.chopro
```

`ggt ug fetch` does **fetch + clean only**; it does **not** convert. The key and
tempo are your call, and you set them on the `cpro` step, not here.

### Fetching several tabs in a loop

`ug` does not take a list of tab IDs; batching is the shell's job. Loop over
one `fetch` and one `cpro` per tab:

```bash
for TAB_ID in 12345 12346 12347; do
  URL="https://www.ultimate-guitar.com/tab/artist/song-$TAB_ID"
  ggt ug fetch "$URL" -o "clean_$TAB_ID.txt"
  ggt cpro "clean_$TAB_ID.txt" --key G --tempo 90 -o "song_$TAB_ID.chopro"
done
```

Each tab is its own `fetch` then its own `cpro` — one job per command, chained by
the names of the files between them.

### `ggt ug [meta.json]` — clean a saved tab JSON offline

To clean a tab you've already pulled (offline, from a saved tab JSON), run
`ggt ug` on that file:

```bash
# Clean only:
ggt ug saved_tab.json -o clean.txt

# Clean + convert to .chopro in one step (pass --key to trigger the cpro step):
ggt ug saved_tab.json --key G --tempo 90 -o song.chopro
```

---

## `cpro` — convert sourced data to the `.chopro` format, in the native key

The `cpro` subcommand reads a plain-text guitar tab chart and re-emits it as a
ChordPro-format `.chopro` file with **inline chord placement**: every chord is
placed in square brackets immediately before the word it aligns with.

```bash
ggt cpro chart.txt -o song.chopro --key G --capo 2 --tempo 90 --time "4/4"
```

**Input** (a plain tab; the lyric tokens below are illustrative, not a real song):

```text
F              G
la la la        la la
C            D
la la           la
```

**Output** (`ggt cpro chart.txt --key G --capo 2 --title "Song" --tempo 90 --time "4/4"`):

```text
{title: Song}
{key: G}
{capo: 2}
{tempo: 90}
{time: 4/4}
[F]la la la        [G]la la
[C]la la            [D]la
```

`--key` becomes `{key: }`. `--capo`, `--tempo`, `--time`, `--duration`,
`--artist`, and `--title` become the matching
`{capo: }`, `{tempo: }`, `{time: }`, `{duration: }`, `{artist: }`, `{title: }`
header lines — the exact thing BandHelper wants.

The core placement rule, drawn out below, is that **the chord goes in front of
the word it aligns with, not the word before it**.

### `--capo` and `--remove-capo` (capo charts, in the native key)

There are times where a tab/chart requires the use of a capo. `BandHelper` wants
`.chopro` files to be in their native/*sounding* key. Doing that playing-key →
native-key math by hand is exactly what nobody wants to do, so `cpro` reaches
into the same transpose math that `ggt transpose` uses and can do it for you.
That's the reason both of these flags exist:

* `--capo N` — by default, just records `{capo: N}` and leaves chord shapes in
  their as-fretted *playing* key.
* `--remove-capo` — "Removes" the capo by shifting every chord shape up by `N`
  frets so they match the `--key` you gave and drops the `{capo: N}` indicator for
  the .chopro file, producing the native-key chart for import. Effectively, this
  option flag "does the playing-key → native-key conversion math" for you.

`--remove-capo` **requires `--capo N`** (it needs a capo fret to de-cap *by*);
it errors out otherwise. A real example — capo 4 into native `G`:

```bash
# --remove-capo de-caps the playing-key shapes into native key G and drops {capo:}.
ggt cpro chart.txt --key G --capo 4 --remove-capo --tempo 100 --time 4/4 -o song.chopro
```

---

## `transpose` — move an existing `.chopro` between keys

`transpose` takes an existing `.chopro` (the output of the `cpro` subcommand) and
shifts every bracketed chord by a number of semitones, then rewrites the `{key: V}`
header to the new tonic. It is the `cpro` subcommand's native-key math exposed on its own,
for the case where you already have a `.chopro` file and just want it in another key
without re-running the conversion from a chart.

Walk-downs (`F# - F`), annotations (`x2`, `N.C.`), **section headers**
(`[Bridge]`, `[Chorus]`, …), `{capo: N}`, and every other header field are
left exactly as they are — only the chord *roots* move. `transpose` detects
section headers and never rewrites them, so the PR #1
section-tags-transposed-into-chords bug
(`20260916-ggt-section-tag-transpose-corruption.md`) cannot recur.

Exactly one of `--up N`, `--down N`, or `--to-key K` is required:

```bash
# Land on a target key, writing to a new file:
ggt transpose in.chopro --to-key C -o out.chopro

# Shift up / down by an exact number of semitones:
ggt transpose in.chopro --up 2
ggt transpose in.chopro --down 3

# Read the .chopro from stdin, write to a new file:
cat in.chopro | ggt transpose --to-key F -o out.chopro

# Or rewrite a file directly, in place:
ggt transpose in.chopro --to-key F --inplace

# Prefer flat spellings:
ggt transpose in.chopro --to-key Bb --flats
```

---

## `version` — build provenance

`ggt version` prints the binary's full build identity and exits — the
one-liner that answers "is this an installed release or a freshly built dev
binary, and from where?":

```text
$ ggt version
ggt 0.5.2
  commit:      <short SHA at build time>
  built:       <RFC3339 timestamp>
  go:         go1.26.x
  platform:    <GOOS>/<GOARCH>
  build:      local build
```

A release build (tagged, stamped, cut by hand via `make release`) reports
`release`; a local `go build` or `make build` reports `local build` or `source`.
The version string and commit come from the `VERSION` file and `git rev-parse
HEAD` at build time, baked in with `ldflags`. See [Development](#development).

---

## The `cpro` rule

Guitar tab charts put chord symbols above lyrics to show which chord to play
*when you sing a given word*. `cpro` follows that reading strictly: **the chord
goes in front of the word it aligns with.**

For a typical line:

```text
F            G
la la la    la la
```

F belongs over `la la la` and G falls on the second `la la`, so the two chords
are placed as `[F]la la la` and `[G]la la`. The chord is **not** shifted to the
previous word.

That rule is the difference between a conversion that *preserves playability*
and one that merely looks plausible. BandHelper renders the `[F]` exactly where
the player expects it — directly in front of the word that chords with it — and
that's the whole point.

---

## Why ggt exists, subcommand by subcommand

Every subcommand in `ggt` exists for a concrete reason in this workflow, and
they compose left-to-right:

* **`ug` — get the source data.** Many guitar players get their tabs from the
  Ultimate Guitar site. `ug` is the step that pulls the chart off the
  page as a plain-text file that the rest of the pipeline can work on. It's
  the *first* step simply because a tab is something you have to fetch before
  you can shape it.
* **`cpro` — convert to native-key `.chopro`.** This is the heart of the tool.
  BandHelper imports files in ChordPro format, and it requires the file to be in the
  *native* key — the sounding key, not the key your fretting hand is
  in when a capo is on.  Anyone who plays capo charts would otherwise have to
  convert the as-fretted playing key into the native key by hand — exactly the
  mental math nobody wants to do. So the `cpro` subcommand reaches into the same
  transpose math that `ggt transpose` runs and does it for you, via
  `--remove-capo`. The `--capo` / `--remove-capo` flags are optional because not
  every chart has a capo — but for a setlist built on capo chords they're how
  every one of those charts reaches the native key.
* **`transpose` — repurpose the native-key converter.** Once `cpro` can shift
  a chart into its native key, that same shift is useful on its own — for when
  you already have a `.chopro` and just want the next song's key out of it
  quickly, without re-running the full `ug` + `cpro` cycle. `ggt transpose`
  is that math, exposed for standalone use.
* **`version` — build provenance.** So you can tell a release-built binary
  apart from a just-now dev binary, on the machine in front of you.

### Some principles the whole tool follows

* **Each subcommand does one job, and they pipe into each other.** `ug` pulls
  the chart, `cpro` shapes it into a `.chopro`, `transpose` moves a
  `.chopro` between keys. They compose through the shell with intermediate
  files — `ggt ug fetch … | ggt cpro …`. There's deliberately *no* single
  "do everything" command in `ggt`; that's more surface for bugs, not less,
  and the whole point here is *fewer* moving parts per command.
* **Chord theory, not heuristics.** The native-key math `transpose` and
  `cpro --remove-capo` do is real music-theory math (native vs playing key,
  `capo N → key K`) — local, deterministic, and unit-tested.
* **BandHelper alignment.** `cpro`'s one job is to produce the inline-bracked
  `{key: }` shape BandHelper consumes — no hand-tuning of the output afterward.
* **Best-effort, conservative parsing.** When a chart has chord tokens `ggt`
  doesn't fully understand (walk-downs like `(F# - F)`, slide / grab
  notations, stray `|…|` separators UG sometimes leaves in), it leaves the
  unrecognized part on the line rather than silently rewriting it. A clean,
  partial result is the success case, not a failure — you read it, you decide.
* **One canonical native-key form.** A song has one correct native-key
  `.chopro`, and `ggt transpose` gives you every playing position on demand
  from that one file. No need for a dozen copies.

---

## Install

### From an official release (recommended)

Releases are published to the [GitHub Releases page](https://github.com/glarizza/ggt/releases).
Pick the asset that matches your OS and CPU. Four are produced — **x86-64 and
ARM64, for Linux and macOS**:

| Asset | Install on |
| --- | --- |
| `ggt_<version>_darwin_amd64.tar.gz` | macOS, Intel |
| `ggt_<version>_darwin_arm64.tar.gz` | macOS, Apple Silicon |
| `ggt_<version>_linux_amd64.tar.gz` | Linux, x86-64 (most servers/PCs) |
| `ggt_<version>_linux_arm64.tar.gz` | Linux, ARM64 (Raspberry Pi, Graviton, M-arm dev kit) |

Each tarball contains the `ggt` binary plus `LICENSE` and `README.md`; a
sibling `ggt_<version>_checksums.txt` lists the SHA-256 of every asset. A
typical macOS-Apple-Silicon install:

```bash
# 1. Download the matching tarball and its checksums from the release page,
#    into their own folder, then:
cd ~/Downloads
# 2. Verify the checksum before running anything.
shasum -a 256 -c ggt_<version>_checksums.txt           # macOS
# sha256sum -c ggt_<version>_checksums.txt             # Linux
# 3. Extract and install the binary onto your PATH.
tar xzf ggt_<version>_darwin_arm64.tar.gz
install -m 755 ggt /usr/local/bin/                     # or ~/bin/
# 4. Sanity check.
ggt version
```

On Linux the install step is usually `sudo install -m 755 ggt /usr/bin` (or
drop it into a directory that's on your `PATH`).

> **Which tarball?** Run `uname -m`: `x86_64` → `amd64`, `arm64`/`aarch64`
> → `arm64`. Combine with `darwin` on macOS or `linux` on Linux (the OS part
> of the filename).

### From source

The project builds with Go 1.26, depends on `github.com/spf13/cobra` and the
Go standard library, and nothing else:

```bash
git clone https://github.com/glarizza/ggt
cd ggt
make build           # stamps a dev build into bin/ggt
./bin/ggt version
# or install a stamped release build to ~/go/bin:
go install -tags release ./cmd/ggt
```

---

## Development

```bash
# Stamped dev build (includes VERSION, commit, and build date baked in):
make build

# Build + run the full test suite:
make test

# Run `ggt cpro` on a chart through the Makefile:
make run FILE=path/to/tab.txt
make run FILE=path/to/tab.txt OUT=path/to/song.chopro

# Bump VERSION:
make version-patch     # 0.5.1 -> 0.5.2
make version-minor     # 0.5.1 -> 0.6.0
make version-major     # 0.5.1 -> 1.0.0

# Inspect the build stamp:
./bin/ggt version
```

`make build-stamp` compiles with `-tags release` so the binary is a proper
release; `make build` (the default) leaves the tag off, marking it as a
development build. Both write to `bin/ggt` and stamp `Version`, `GitCommit`,
`BuildDate`, and `Build=release|local` into `ggt/internal/version`.

### Cutting a release

Releases are **cut by hand with `make release`** (tag + push + Goreleaser),
not by any CI job:

```bash
make version-patch              # bump VERSION (e.g. 0.5.1 -> 0.5.2)
git commit -am "release: 0.5.2"     # commit the bump + the work it carries
make release-dryrun             # eyeball the 4-platform artifacts in dist/ (no tag/push)
make release                    # tag v$(VERSION), push, cut the GitHub Release
```

`make release` refuses to clobber a `v$(VERSION)` tag that already exists
locally or on the remote, and it publishes via
[GoReleaser](https://goreleaser.com) (the `.goreleaser.yml` matrix) to the
[GitHub Releases page](https://github.com/glarizza/ggt/releases), producing
the four `ggt_<version>_<os>_<arch>.tar.gz` assets documented in
[Install](#install).

---

## A single static dependency

The project uses only the Go standard library plus
`github.com/spf13/cobra`, a CLI framework required because the tool has
multiple subcommands. It has no `go-git` and no other runtime dependency; the
git plumbing that stamps the version into the binary happens at *build* time in
the `Makefile`, not at runtime via Go.

That keeps `ggt` a single static binary: it runs anywhere a Go binary runs,
with no network access of its own beyond the UG fetch in `ug`, and no
build-time secrets of any kind.

---

## License

Released under the MIT License — see [`LICENSE`](./LICENSE)
(`Copyright (c) 2026 Gary Larizza`).
