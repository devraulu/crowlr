import toml from "toml";
import fs from "fs";

export type Config = {
  dsn: string;
  llm: {
    provider: string;
    ollama_url: string;
    embed_model: string;
    embed_dimensions: number;
    gen_model: string;
  };
  logging: {
    level: string;
  };
};

export const config: Config = toml.parse(
  fs.readFileSync("../config.toml", "utf-8"),
);

export default config;
