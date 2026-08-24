import { type IConnected } from "pg-promise";
import ollama from "./ollama.ts";
import { db, pgp } from "./db.ts";
import logger from "./utils/logger.ts";
import type { FixLater } from "./types/index.ts";
import config from "./config.ts";

async function embedQuery(q: string): Promise<number[]> {
  const response = await ollama.embed({
    model: config.embedModel,
    input: q,
    dimensions: config.embedDimensions,
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
  k = 0,
): Promise<MatchingChunk[]> {
  let conn: IConnected<object, FixLater> | null = null;

  try {
    let embedding;
    try {
      embedding = await embedQuery(q);
    } catch (err) {
      logger.error({ err }, "embedding query failed");
      throw err;
    }
    conn = await db.connect();

    const results = await conn.many<MatchingChunk>(
      `SELECT id, page_id, chunk_index, content, embedding, metadata, created_at, embedding <=> $1::vector as distance FROM chunks LIMIT $2`,
      [embedding, k],
    );

    return results;
  } catch (e) {
    logger.error({ error: e, stack: e.stack }, "error in retrieve");
    // throw new ApplicationError("Failed to retrieve search context");
  } finally {
    await conn?.done();
  }

  return [];
}

export const EXAMPLE_QUESTION = `What are some interesting facts about wolves?`;
if (import.meta.main) {
  const res = await retrieve(EXAMPLE_QUESTION, 16);
  console.log("results", res);
  pgp.end();
  console.log("closed pg pool");
}
