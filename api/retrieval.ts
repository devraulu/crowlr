import { type IConnected } from "pg-promise";
import ollama from "./ollama.ts";
import { db, pgp } from "./db.ts";

async function embedQuery(q: string): Promise<number[]> {
  const response = await ollama.embed({
    model: process.env.EMBED_MODEL || "llama3.2:3b",
    input: q,
    dimensions: parseInt(process.env.EMBED_DIMENSIONS || "0"),
  });
  return response.embeddings?.[0];
}

export interface MatchingChunk {
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
}

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
    throw e;
  } finally {
    await conn?.done();
  }
}

export const EXAMPLE_QUESTION = `What are some interesting facts about wolves?`;
if (import.meta.main) {
  const res = await retrieve(EXAMPLE_QUESTION, 16);
  console.log("results", res);
  pgp.end();
  console.log("closed pg pool");
}
