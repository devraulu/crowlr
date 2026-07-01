from crowlr_py import config


def test_load_reads_dsn_and_defaults(tmp_path):
    toml_path = tmp_path / "config.toml"
    toml_path.write_text(
        """
dsn = "postgres://user:pass@localhost/db"

[llm]
gen_model = "custom-model"
"""
    )

    cfg = config.load(toml_path)

    assert cfg.dsn == "postgres://user:pass@localhost/db"
    assert cfg.llm.provider == "ollama"
    assert cfg.llm.ollama_url == "http://localhost:11434"
    assert cfg.llm.embed_model == "nomic-embed-text"
    assert cfg.llm.gen_model == "custom-model"
    assert cfg.logging.level == "info"


def test_load_reads_llm_provider_override(tmp_path):
    toml_path = tmp_path / "config.toml"
    toml_path.write_text(
        """
dsn = "postgres://user:pass@localhost/db"

[llm]
provider = "openai"
"""
    )

    cfg = config.load(toml_path)

    assert cfg.llm.provider == "openai"
