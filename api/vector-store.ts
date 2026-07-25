import { OllamaEmbeddings } from "@langchain/ollama";
import { config } from "./config.ts";
import { PGVectorStore, type PGVectorStoreArgs } from "@langchain/pgvector";
import pool from "./db.ts";

const embeddings = new OllamaEmbeddings({
  model: config.llm.embed_model,
});

const storeConfig: PGVectorStoreArgs & {
  dimensions?: number;
} = {
  pool,
  tableName: "testlangchain",
  columns: {
    idColumnName: "id",
    vectorColumnName: "vector",
    contentColumnName: "content",
    metadataColumnName: "metadata",
  },
  distanceStrategy: "cosine",
  dimensions: 768,
};

const vectorStore = await PGVectorStore.initialize(embeddings, storeConfig);

export default vectorStore;
