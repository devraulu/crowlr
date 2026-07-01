from __future__ import annotations

from langchain_ollama import ChatOllama, OllamaEmbeddings

from crowlr_py.config import Config


def get_embeddings(cfg: Config) -> OllamaEmbeddings:
    return OllamaEmbeddings(base_url=cfg.llm.ollama_url, model=cfg.llm.embed_model)


def get_chat_model(cfg: Config) -> ChatOllama:
    return ChatOllama(base_url=cfg.llm.ollama_url, model=cfg.llm.gen_model)


def embed_query_text(embeddings: OllamaEmbeddings, text: str) -> list[float]:
    return embeddings.embed_query(f"search_query: {text}")


def embed_doc_texts(embeddings: OllamaEmbeddings, texts: list[str]) -> list[list[float]]:
    return embeddings.embed_documents([f"search_document: {t}" for t in texts])
