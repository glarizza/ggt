#!/usr/bin/env python3
"""
clean_tab.py -- Strip noise from a web-downloaded guitar tab so it produces
a clean chord-over-lyric body suitable for `ggt chopro`.

WHY THIS EXISTS
----------------
When a tab is downloaded from a web page you get far more than the tab:
site chrome, a metadata envelope (title, URL, version/difficulty,
`Key:` / `Capo:` / `Tuning:` prose lines, free-form notes), occasional
6-string *tablature* lanes (`e|--12--|`, `B|---10/12----|`), and a
trailing slide/harmonic legend after an asterisk banner. `ggt chopro`
consumes a clean chord-over-lyric body; if any of this noise survives it
is rendered as lyric garbage in BandHelper, and TAB lanes in particular
cannot be displayed at all (BandHelper has no tab renderer).

Song metadata (title, artist, key, capo, tempo, time, duration) is NOT
written into the body -- it is passed to `ggt chopro` as command-line
flags. `clean` only *removes* things so they don't corrupt the body.

This is a standalone script on purpose (see scripts/README.md). It has no
coupling to the ggt binary and evolves against tab *websites*; it composes
with ggt through the file/pipe boundary:

    python3 scripts/clean_tab.py in.txt | ggt chopro --key F# --capo 5 -o out

A new tricky tab that the cleaner mangles is the trigger to (a) add/extend
a pass here, and (b) freeze that tab into a copyright-clean fixture with
`scripts/scramble_lyrics.py` for regression.

USAGE
-----
    python3 scripts/clean_tab.py INPUT [OUTPUT]

INPUT   raw text file (an extracted tab from a web page)
OUTPUT  cleaned body written here (default: stdout)

A per-pass diagnostic report is written to stderr so the operator can
review exactly what was stripped. New passes are appended at the END of
the PASS list, keeping earlier passes stable.
"""

import re
import sys
from pathlib import Path


# --- line-pattern recognisers ------------------------------------------------

# A section header is a line that is only a bracketed tag: [Verse], [Chorus],
# [Intro], [Interlude], [Instrumental], [Bridge], [Outro], [Verse 1], etc.
SECTION_RE = re.compile(r"^\s*\[[^\]]+\]\s*$")

# A 6-string TAB-lane line: optional indent, a single string label (a note
# letter), a pipe, then a lane body made only of dashes / digits / slashes /
# backslashes / spaces / pipes. Catches `e|--12--|`, `B|---10/12----|`,
# while never matching a chord+lyric line (which has letters/spaces after the
# label, or no pipe at all).
TAB_LANE_RE = re.compile(r"^\s*[A-Za-z]\|[-\d/\s\\|]+$")

# A banner of asterisks that opens a notation/slide legend, e.g.
#    ****************************
ASTERISK_BANNER_RE = re.compile(r"^\s*\*{3,}\s*$")

# A standalone end-of-song marker `X` (classic tab-chart "song ends here").
END_MARKER_RE = re.compile(r"^\s*X\s*$")

# Recognised metadata labels that may appear even mid-body.
META_LABELS = ("key", "capo", "tuning", "tempo", "time signature",
               "difficulty", "version", "bpm", "duration")
META_LINE_RE = re.compile(
    r"^\s*(key|capo|tuning|tempo|time signature|time|difficulty|version|"
    r"bpm|duration)\s*:",
    re.IGNORECASE,
)
URL_RE = re.compile(r"^\s*https?://\S+\s*$")

# Free-form instruction lines (not lyrics) that commonly sit around UG tabs.
PROSE_NOTE_RE = re.compile(
    r"^\s*(intro is|chords? are|chords are by|slid\w+ (up|down)|"
    r"harmon\w+|tap\w*|hammer-on|pull-off|natural harmonic|"
    r"bpm\s*[:=]|tempo\s*[:=]|regardless of capo)",
    re.IGNORECASE,
)


def _report(passname, detail):
    sys.stderr.write("[{}] {}\n".format(passname, detail))


def is_metadata_line(line):
    return bool(META_LINE_RE.match(line) or URL_RE.match(line)
                or PROSE_NOTE_RE.match(line))


# --- passes ------------------------------------------------------------------

def pass_envelope(lines):
    """Drop the leading metadata block.

    Everything above the first [Section] header is header envelope (title,
    url, version, key/capo/tuning, prose notes, blanks) and is discarded.
    If NO section header exists anywhere, fall back to dropping only
    recognisable metadata/prose/URL lines, so we never nuke a
    section-less chart.
    """
    first_section = next((i for i, l in enumerate(lines)
                          if SECTION_RE.match(l)), None)
    if first_section is not None:
        n = first_section
        if n:
            _report("envelope", "dropped {} leading line(s) before first "
                    "section header".format(n))
        return lines[first_section:]

    kept = [l for l in lines if not is_metadata_line(l)]
    n = len(lines) - len(kept)
    if n:
        _report("envelope", "no [Section] header; fell back to metadata-line "
                "strip (removed {} line(s))".format(n))
    return kept


def pass_tab_lanes(lines):
    """Drop 6-string TAB-lane rows (fret drawings)."""
    kept = [l for l in lines if not TAB_LANE_RE.match(l)]
    n = len(lines) - len(kept)
    if n:
        _report("tab-lanes", "dropped {} TAB-lane line(s) (string/fret "
                "notation; cannot render in BandHelper)".format(n))
    return kept


def pass_end_legend(lines):
    """Truncate at the first asterisk banner, dropping it and everything after.

    The legend (slide/harmonic/tap key) sits at the tail of UG tabs after a
    `*****` banner. The banner marks the end of song content.
    """
    cut = next((i for i, l in enumerate(lines)
                if ASTERISK_BANNER_RE.match(l)), None)
    if cut is not None:
        n = len(lines) - cut
        _report("end-legend", "truncated at asterisk banner (line {}); "
                "dropped {} trailing line(s)".format(cut + 1, n))
        return lines[:cut]
    return lines


def pass_end_markers(lines):
    """Drop standalone end-of-song 'X' markers and any asterisk-only lines."""
    kept = [l for l in lines
            if not END_MARKER_RE.match(l) and not ASTERISK_BANNER_RE.match(l)]
    n = len(lines) - len(kept)
    if n:
        _report("end-markers", "dropped {} end-marker/asterisk line(s)".format(n))
    return kept


def pass_collapse(lines):
    """Collapse 3+ consecutive blank lines to one; strip a trailing run."""
    out = []
    run = 0
    for l in lines:
        if l.strip() == "":
            run += 1
            if run <= 2:
                out.append(l)
        else:
            run = 0
            out.append(l)
    while out and out[-1].strip() == "":
        out.pop()
    return out


# --- driver ------------------------------------------------------------------

PASSES = [
    ("envelope",    pass_envelope),
    ("tab-lanes",   pass_tab_lanes),
    ("end-legend",  pass_end_legend),
    ("end-markers", pass_end_markers),
    ("collapse",    pass_collapse),
]


def clean(text):
    lines = text.split("\n")
    for _, fn in PASSES:
        lines = fn(lines)
    return "\n".join(lines)


def main(argv):
    if len(argv) < 2:
        sys.stderr.write("usage: clean_tab.py INPUT [OUTPUT]\n")
        return 2
    src = Path(argv[1])
    if not src.exists():
        sys.stderr.write("input not found: {}\n".format(src))
        return 1
    text = src.read_text()
    out = clean(text)

    if len(argv) >= 3:
        dest = Path(argv[2])
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(out)
        sys.stderr.write("\n[output] wrote {} bytes -> {}\n"
                         .format(len(out.encode()), dest))
    else:
        sys.stdout.write(out)
        if out and not out.endswith("\n"):
            sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
