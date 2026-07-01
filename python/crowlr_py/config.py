from __future__ import annotations

import tomllib
from dataclasses import dataclass, field
from pathlib import Path

DEFAULT_CONFIG_PATH = Path(__file__).resolve().parents[2] / "config.toml"


@dataclass
class LLMConfig:
    provider: str = "ollama"
    ollama_url: str = "http://localhost:11434"
    embed_model: str = "nomic-embed-text"
    gen_model: str = "llama3.2:3b"


@dataclass
class LoggingConfig:
    level: str = "info"
    format: str = "text"


@dataclass
class Config:
    dsn: str
    llm: LLMConfig = field(default_factory=LLMConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)


def load(path: str | Path = DEFAULT_CONFIG_PATH) -> Config:
    with open(path, "rb") as f:
        data = tomllib.load(f)

    llm_data = data.get("llm", {})
    logging_data = data.get("logging", {})

    return Config(
        dsn=data["dsn"],
        llm=LLMConfig(
            provider=llm_data.get("provider", LLMConfig.provider),
            ollama_url=llm_data.get("ollama_url", LLMConfig.ollama_url),
            embed_model=llm_data.get("embed_model", LLMConfig.embed_model),
            gen_model=llm_data.get("gen_model", LLMConfig.gen_model),
        ),
        logging=LoggingConfig(
            level=logging_data.get("level", LoggingConfig.level),
            format=logging_data.get("format", LoggingConfig.format),
        ),
    )
