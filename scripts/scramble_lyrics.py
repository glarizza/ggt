#!/usr/bin/env python3
"""scramble_lyrics.py -- freeze a tricky tab into a COPYRIGHT-CLEAN fixture.

Replaces every lyric WORD with a same-length fake word (deterministic, seed
42), so the fixture is a git-trackable test input that exercises the real
chart structure while keeping the actual lyrics out of the repo.

Preserved byte-for-byte (no text content, never faked):
    blank lines, [Sections], tab-lane rows, 'X', asterisk banners,
    and every chord/annotation token.

Chord-vs-lyric is decided by MIRRORING ggt's own chopper parser
(internal/chopper/parser.go): a token is treated as a chord/annotation iff
chopper's regexes accept it; everything else is faked. We err toward
preservation -- we never risk faking a real chord column; the only residual
risk is a rare all-caps interjection word that looks like a chord.

Usage:
    python3 scripts/scramble_lyrics.py <file>
Scrambles <file> in place, deterministically.
"""

import re
import sys
import random

# --- chopper regexes mirrored from internal/chopper/parser.go ---
TOKEN_RE = re.compile(r"\S+")
# Chords over-match all-caps words starting A-G: intentional (err preserve).
CHORD_RE = re.compile(r"^\(?[A-G][#b]?[A-Za-z0-9#b+\-]*(?:/[A-G][#b]?)?\*?\)?$")
# Non-chord tokens allowed on a chord line.
ANNOT_RE = re.compile(r"(?i)^([|]+|x\d+|\(x\d+\)|N\.C\.|\(N\.C\.\)|%)$")
SECTION_RE = re.compile(r"^\s*\[[^\]]*\]\s*$")
TAB_LANE_RE = re.compile(r"^\s*[A-Za-z]\|[-\d/\s\\|]+$")
END_MARKER_RE = re.compile(r"^\s*X\s*$")
ASTERISK_RE = re.compile(r"^\s*\*{3,}\s*$")

COMMON_WORDS = [
    "the", "and", "for", "you", "that", "with", "this", "have", "just",
    "not", "are", "but", "all", "can", "one", "out", "may", "how", "our",
    "who", "her", "his", "was", "has", "now", "say", "did", "get", "let",
]
WORD_LENGTHS = [2, 3, 4]


def is_chord_or_annotation(tok):
    """A token chopper would read as a chord or annotation. Never scrambled."""
    if ANNOT_RE.match(tok):
        return True
    if not CHORD_RE.match(tok):
        return False
    if len(tok) > 10:
        return False
    if re.search(r"[a-z]{6,}", tok):
        return False
    return True


def is_structural(line):
    """Blank / [Section] / tab-lane / 'X' / asterisk banner: no lyric content."""
    s = line.strip()
    if not s:
        return True
    return (bool(SECTION_RE.match(line)) or bool(TAB_LANE_RE.match(line))
            or bool(END_MARKER_RE.match(s)) or bool(ASTERISK_RE.match(line)))


def is_chord_only_line(line):
    """Pure chord column: every token is a chord/annotation."""
    toks = TOKEN_RE.findall(line)
    return bool(toks) and all(is_chord_or_annotation(t) for t in toks)


def _fake(word):
    """Same-length fake word. Preserves trailing punctuation and total
    length; returns the token unchanged if it has no letters to replace."""
    original = word
    out = ""
    end_chars = []
    idx = len(word) - 1
    while idx >= 0 and not word[idx].isalpha():
        end_chars.append(word[idx])
        idx -= 1
    letters = word[:idx + 1]
    for c in letters:
        if c.isalpha():
            out += random.choice("abcdefghijklmnopqrstuvwxyz")
        else:
            out += c
    while end_chars:
        out += end_chars.pop()

    if len(out) != len(original):
        diff = len(original) - len(out)
        if diff > 0:
            out += "".join(random.choice("abcdefghijklmnopqrstuvwxyz")
                           for _ in range(diff))
        else:
            out = out[:len(original)]
    return out


def _split_keep_ws(line):
    """Split on whitespace but retain the whitespace separators."""
    return re.split(r"(\s+)", line)


def scramble_line(line):
    """Fake only the lyric words on the line; keep chords, structure and
    exact spacing."""
    if is_structural(line):
        return line
    if is_chord_only_line(line):
        return line
    out = []
    for p in _split_keep_ws(line):
        if p == "":
            out.append(p)
        elif p.strip() == "":
            out.append(p)
        elif is_chord_or_annotation(p):
            out.append(p)
        else:
            out.append(_fake(p))
    return "".join(out)


if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] in ("-h", "--help"):
        print("usage: scramble_lyrics.py <file>")
        print("Scramble a tab chart in place: lyrics -> same-length fake words,")
        print("chords / structure left intact. Deterministic (seed 42).")
        sys.exit(0)
    path = sys.argv[1]
    with open(path) as f:
        lines = f.read().split("\n")

    random.seed(42)
    lines = [scramble_line(l) for l in lines]

    with open(path, "w") as f:
        f.write("\n".join(lines))
    print("scrambled: " + path)
    print("Done.")
