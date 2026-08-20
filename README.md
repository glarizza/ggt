# ggt — Gary's Guitar Tool

A CLI tool for working with guitar chord charts and BandHelper
`.chopro` files.

## Subcommands

### `ggt chopro`

Convert a tab chart (chord lines above lyric lines) to inline
ChordPro format with a BandHelper `{key: value}` header.

```bash
ggt chopro song.txt

# pipe output to a file
ggt chopro song.txt > song.chopro

# with song metadata
ggt chopro song.txt --title "Song Title" --key G --capo 2

# write to file directly (quiet when -o is set)
ggt chopro song.txt -o song.chopro
```

#### Flags

| Flag | Description |
|------|-------------|
| `-o, --output` | Write to file instead of stdout |
| `--title` | Song title for `{title:}` header field |
| `--artist` | Artist name for `{artist:}` header field |
| `--key` | Song key for `{key:}` header field |
| `--capo` | Capo fret number: adds `{capo: N}` header AND `(Capo N)` body line |
| `--tempo` | Tempo in BPM for `{tempo:}` header field |
| `--time` | Time signature for `{time:}` header field |
| `--duration` | Duration for `{duration:}` header field |

## How it works

```
G            D
hello        world
```

becomes

```
[G]hello         [D]world
```

### Rules

- **Section headers** like `[Verse 1]`, `[Chorus]` are dropped but a
  single blank line is inserted at each section boundary, giving
  visual separation between song sections.
- **Chord lines** are lines where every whitespace-separated token is
  either a chord (`G`, `Am7`, `Cadd9`, `D/F#`, `(D/B)`, `G*`) or an
  allowed annotation (`x2`, `,`, `N.C.`).
- **Merged lines:** a chord line immediately followed by a lyric line
  has each chord placed in front of the word that starts under the
  chord's first character. No mid-word splitting.
- **Stacking:** two or more chords on the same word stack — first before
  the word, remaining after in left-to-right order.
- **Standalone chord lines** (not preceded by a lyric line) get every
  chord bracketed in place with original spacing preserved.
- **Parenthesized chords** like `(Gm)` mean "this chord is still
  ringing from the previous line" — the outer parens are stripped.
- **`X` at end of a section** is a "song ends here" marker and is
  dropped from output.
- **`(Instrumental)`** lines are dropped.
- **`N.C.`** is rendered as literal `(N.C.)`, never bracketed.

### Output format

```
{title: Song Title}
{key: G}
{capo: 2}
(Capo 2)
[G]hello         [D]world
```

- Header is BandHelper's `{key: value}` format, one per line.
- Header is only emitted when at least one metadata flag is set.
- `(Capo N)` is always the first body line when `--capo` is set.

## Building

```bash
make build                # -> bin/ggt
make test                 # go test ./...
ggt chopro chart.txt      # build + run
```

## Copyright

See `AGENTS.md` for the copyright policy. Briefly: song lyrics and tab
charts are copyrighted. Scrambled test data lives in `sample-tabs/`
(filenames do not identify real songs). Real copyrighted content can
live in `ignored/` but must never be committed.
