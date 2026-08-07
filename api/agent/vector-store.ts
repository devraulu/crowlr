import { OllamaEmbeddings } from "@langchain/ollama";
 
import { PGVectorStore, type PGVectorStoreArgs } from "@langchain/pgvector";
import pool from "../db.ts";

const embeddings = new OllamaEmbeddings({
  model: process.env.EMBED_MODEL || "llama3.2:3b",
});
console.log("Initialized Ollama embeddings", embeddings.model);

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
console.log("Initialized vector store", vectorStore.collectionName);

export default vectorStore;
