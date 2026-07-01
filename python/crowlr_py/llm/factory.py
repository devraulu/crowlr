from __future__ import annotations

from langchain_core.embeddings import Embeddings
from langchain_core.language_models.chat_models import BaseChatModel

from crowlr_py.config import Config
from crowlr_py.llm import ollama as ollama_provider


def get_embeddings(cfg: Config) -> Embeddings:
    if cfg.llm.provider == "ollama":
        return ollama_provider.get_embeddings(cfg)
    raise ValueError(f"unsupported llm provider: {cfg.llm.provider}")


def get_chat_model(cfg: Config) -> BaseChatModel:
    if cfg.llm.provider == "ollama":
        return ollama_provider.get_chat_model(cfg)
    raise ValueError(f"unsupported llm provider: {cfg.llm.provider}")


def embed_query_text(cfg: Config, embeddings: Embeddings, text: str) -> list[float]:
    if cfg.llm.provider == "ollama":
        return ollama_provider.embed_query_text(embeddings, text)
    return embeddings.embed_query(text)


def embed_doc_texts(cfg: Config, embeddings: Embeddings, texts: list[str]) -> list[list[float]]:
    if cfg.llm.provider == "ollama":
        return ollama_provider.embed_doc_texts(embeddings, texts)
    return embeddings.embed_documents(texts)
