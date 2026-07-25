import toml from "toml";
import fs from "fs";

export type Config = {
  dsn: string;
  llm: {
    provider: string;
    ollama_url: string;
    embed_model: string;
    gen_model: string;
  };
};
export const config: Config = toml.parse(
  fs.readFileSync("../config.toml", "utf-8"),
);

console.log({ config });
