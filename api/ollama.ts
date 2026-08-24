import { Ollama } from "ollama";
import config from "./config.ts";

const ollama = new Ollama({
  host: config.ollamaHost,
});
export default ollama;
