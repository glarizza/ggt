# scripts/ — standalone helpers for the ggt chart pipeline

Two Python helpers sit **outside** the `ggt` binary on purpose. They compose
with it through the file/stream boundary and evolve on their own cadence
(both track *tab websites*, not the cpro's internals). `ggt` graduates
anything to a subcommand only when there's a real feature reason, never
speculatively.

```
scripts/clean_tab.py      in.txt   ->  in.cleaned.txt        strip web / tab noise
         |
ggt cpro in.cleaned.txt --key K --capo N --time T --title "..." -o out.chopro
         |
 out.chopro   --imported into BandHelper
```

---

## clean_tab.py

Strip the noise that surrounds a web-downloaded tab so `ggt cpro` gets a
clean chord-over-lyric body. Song metadata is *not* written into the body —
it is passed to `ggt cpro` as flags. `clean` only *removes* things.

```bash
python3 scripts/clean_tab.py in.txt                 # -> stdout
python3 scripts/clean_tab.py in.txt out.clean.txt    # -> file; per-pass report to stderr
```

A new tricky tab that the cleaner mangles is the trigger to add a pass here.
Passes run in fixed order; new passes append at the end so earlier ones stay
stable: envelope -> tab-lanes -> end-legend -> end-markers -> collapse.

## scramble_lyrics.py

Dev-only helper. Freeze a tricky tab into a **copyright-clean** regression
fixture: every lyric *word* is replaced with a same-length fake word
(deterministic, seed 42) while all structure — chord columns, section
headers, tab-lane rows, `X` markers, walkdowns — is preserved. A fixture is
a git-trackable test input that exercises the real chart shape without
committing real lyrics.

Chord-vs-lyric is decided by **mirroring `ggt`'s own cpro parser**
(`internal/cpro/parser.go`): a token is treated as a chord/annotation iff
cpro's regexes accept it (with length/lowercase guards so real words are
never mistaken for chords). If the two ever drift, cpro's tests are the
source of truth and this script's regexes are brought in line.

```bash
python3 scripts/scramble_lyrics.py file.txt        # scramble in place
```

Freezing a fixture: clean the real tab (kept in `ignored/`), then scramble
the *cleaned* output into `sample-tabs/` under a generic number.

---

## House rules (from AGENTS.md)

- **Never commit real lyrics or tabs.** Real charts live in `ignored/`
  (gitignored). Committable fixtures are scrambled and named with generic
   numbers (`sample_08.txt`), never real song or artist names.
- `output/` (gitignored) holds cleaned tabs and generated `.chopro` files
  for BandHelper import — never committed.
