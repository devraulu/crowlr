const config = {
  embedModel: process.env.EMBED_MODEL || "nomic-embed-text",
  embedDimensions: Number(process.env.EMBED_DIMENSIONS) || undefined,
  logLevel: process.env.LOG_LEVEL || "info",
  ollamaHost: process.env.OLLAMA_HOST,
  dsn:
    process.env.DSN ||
    "postgres://crowlr:password@db:5432/crowlr?sslmode=disable",
};

export default config;
