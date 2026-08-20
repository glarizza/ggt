#!/usr/bin/env python3
"""
scramble_lyrics.py — Replace lyric words in a guitar tab chart with
same-length fake words, preserving chord alignment.

Usage:
    python3 scramble_lyrics.py <input.txt>
    python3 scramble_lyrics.py sample-tabs/*.txt

All chord lines, section headers ([Chorus], [Verse 1]), blank lines,
X markers, and (Instrumental) lines are preserved. Only actual lyric
text is replaced. The replacement is deterministic (seeded with 42).
"""

import re
import random
import sys
from pathlib import Path

random.seed(42)

# Common English words by character length for the scrambler
COMMON = {
      1: ["a", "I", "x"],
      2: ["we", "he", "it", "is", "am", "do", "no", "to", "of", "on", "if", "up", "my", "go", "in"],
      3: ["the", "and", "you", "but", "was", "not", "are", "for", "him", "she", "all", "can", "had", "her"],
      4: ["that", "with", "this", "have", "from", "want", "just", "been", "like", "here", "what", "when", "they", "your", "over"],
      5: ["there", "could", "would", "about", "never", "their", "every", "some", "only", "into", "will", "been", "were", "make"],
      6: ["would", "should", "people", "another", "around", "become", "through", "before", "always", "little", "little", "little"],
      7: ["thought", "without", "started", "nothing", "looking", "another", "happened", "different", "something"],
      8: ["beautiful", "something", "everyone", "thousand", "different", "happened", "everything", "different"],
      9: ["something", "different", "everytime", "something", "everything", "happening"],
     10: ["everything", "everything", "something", "everything", "different"],
}


def fake_word(length: int) -> str:
    """Return a pseudo-random word of exactly `length` characters."""
    if 0 < length <= 10:
        return random.choice(COMMON[length])
    # longer words: generate a consonant-vowel pseudopattern
    result = []
    pattern = "cvvcvvcvvcvv"
    i = 0
    while len("".join(result)) < length:
        kind = pattern[i % len(pattern)]
        if kind == "c":
            result.append(random.choice("bcdfghjklmnpqrst"))
        else:
            result.append(random.choice("aeiou"))
        i += 1
    return "".join(result[:length])


chord_char_re = re.compile(r"^[A-G][#b]?[A-Za-z0-9#b+\/*\-]*$")


def is_chord_line(line: str) -> bool:
    """True for chord-only lines, section headers, blank/structural lines."""
    s = line.strip()
    if not s or s == "X" or s == "(Instrumental)":
        return True
    if re.match(r"^\[.*\]$", s):
        return True
    # Any token containing lowercase letters = lyric text
    for tok in s.split():
        if any(c.islower() for c in tok):
            return False
    return True


def scramble_lyric_line(line: str) -> str:
    """Replace each word in a lyric line with a same-length fake word."""
    result = []
    for token in re.split(r"(\s+)", line):
        if not token:
            continue
        if token.isspace():
            result.append(token)
            continue
        result.append(fake_word(len(token)))
    return "".join(result)


def scramble_path(path: Path) -> None:
    with open(path) as f:
        lines = f.readlines()
    new_lines = [
        line if is_chord_line(line) else scramble_lyric_line(line)
        for line in lines
    ]
    with open(path, "w") as f:
        f.writelines(new_lines)
    print(f"scrambled: {path}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 scramble_lyrics.py <file> [file2 ...]", file=sys.stderr)
        sys.exit(1)
    for path in sys.argv[1:]:
        p = Path(path)
        if not p.exists():
            print(f"file not found: {path}", file=sys.stderr)
            sys.exit(1)
        scramble_path(p)
    print("Done.")
