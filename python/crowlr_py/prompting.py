from __future__ import annotations

from crowlr_py.retrieval import RetrievedChunk


def build_prompt(query: str, chunks: list[RetrievedChunk]) -> str:
    parts = [
        "You're a search bot providing users summary of information stored in a database from what a crawler fetched and stored. You're getting the retrieved embeddings that matched the user query."
        "Users queries can be vague and only specify words or they could be detailed and ask uestions."
        "Answer the question using only the context below."
        "Return markdown that can be rendered by the server."
        "If the context doesn't contain the answer, say you don't know.\n\n"
    ]
    for i, c in enumerate(chunks, start=1):
        parts.append(f"[{i}] Source: {c.page_url}\n{c.content}\n\n")
    parts.append(f"Question: {query}\n\nAnswer (cite sources using markdown footnotes)")

    return "".join(parts)
