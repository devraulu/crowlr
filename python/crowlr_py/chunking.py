from __future__ import annotations

from dataclasses import dataclass


@dataclass
class Chunk:
    index: int
    content: str


def chunk_text(text: str, max_words: int, overlap_words: int) -> list[Chunk]:
    words = text.split()
    if not words:
        return []

    step = max_words - overlap_words
    if step <= 0:
        step = max_words

    chunks: list[Chunk] = []
    idx = 0
    start = 0
    while start < len(words):
        end = min(start + max_words, len(words))
        chunks.append(Chunk(idx, " ".join(words[start:end])))
        idx += 1
        if end == len(words):
            break
        start += step

    return chunks
