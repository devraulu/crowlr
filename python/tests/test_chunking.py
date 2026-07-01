from crowlr_py.chunking import Chunk, chunk_text


def test_empty_text_returns_no_chunks():
    assert chunk_text("", 400, 50) == []
    assert chunk_text("   ", 400, 50) == []


def test_short_text_returns_single_chunk():
    assert chunk_text("one two three", 400, 50) == [Chunk(0, "one two three")]


def test_overlap_slides_window():
    text = "one two three four five six"
    assert chunk_text(text, 4, 2) == [
        Chunk(0, "one two three four"),
        Chunk(1, "three four five six"),
    ]


def test_overlap_greater_than_max_words_falls_back_to_max_words_step():
    text = " ".join(str(i) for i in range(10))
    chunks = chunk_text(text, 3, 5)
    assert [c.content for c in chunks] == ["0 1 2", "3 4 5", "6 7 8", "9"]


def test_exact_multiple_of_step_does_not_duplicate_final_chunk():
    text = "one two three four"
    chunks = chunk_text(text, 2, 0)
    assert [c.content for c in chunks] == ["one two", "three four"]
