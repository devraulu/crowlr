import { IConnected } from "pg-promise";
import { config } from "./config";
import ollama from "./ollama";
import { db, pgp } from "./db";

async function embedQuery(q: string): Promise<number[]> {
  const response = await ollama.embed({
    model: config.llm.embed_model,
    input: q,
    dimensions: config.llm.embed_dimensions,
  });
  return response.embeddings?.[0];
}

export type MatchingChunk = {
  id: number;
  page_id: number;
  chunk_index: number;
  content: string;
  embedding: number[][];
  metadata: {
    source: string;
    title: string;
    referrer: string;
    last_modified: string;
    fetched_at: string;
    page_id: string;
    chunker: string;
    embed_model: string;
  };
  created_at: Date;
  distance: number;
};

export default async function retrieve(
  q: string,
  k: number = 0,
): Promise<MatchingChunk[]> {
  const embedding = await embedQuery(q);
  let conn: IConnected<{}, any> | null = null;
  conn = await db.connect();

  try {
    const results = await conn.many<MatchingChunk>(
      `SELECT id, page_id, chunk_index, content, embedding, metadata, created_at, embedding <=> $1::vector as distance FROM chunks LIMIT $2`,
      [embedding, k],
    );

    return results;
  } catch (e) {
  } finally {
    await conn?.done();
  }
  return [];
}

export const EXAMPLE_QUERY = `What are some interesting facts about wolves?`;
if (import.meta.main) {
  const res = await retrieve(EXAMPLE_QUERY, 16);
  console.log("results", res);
  pgp.end();
  console.log("closed pg pool");
}
