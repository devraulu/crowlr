import { type IConnected } from "pg-promise";
import ollama from "./ollama.ts";
import { db, pgp } from "./db.ts";
import type { FixLater } from "./types/index.ts";
import logger from "./utils/logger.ts";
import { meta } from "zod";

const CHUNK_SIZE = 500;
const OVERLAP = 20;

interface Document {
  content: string;
  source: string;
  referrer: string;
  fetched_at: string;
  page_id: string;
  metadata: {
    Title: string;
    Author: string;
    URL: string;
    Hostname: string;
    Description: string;
    Sitename: string;
    Date: string;
    Categories: string[];
    Tags: string[];
    ID: string;
    Fingerprint: string;
    License: string;
    Language: string;
    Image: string;
    PageType: string;
  };
}

async function ingestDocuments(docs: Document[]) {
  logger.info({ length: docs.length }, "ingesting documents");
  let conn: IConnected<unknown, FixLater> | null = null;
  try {
    conn = await db.connect();

    const BATCH_SIZE = 5;
    for (let i = 0; i < docs.length; i += BATCH_SIZE) {
      try {
        const batch = await Promise.all(
          docs.slice(i, i + BATCH_SIZE).map(async ({ content, ...page }) => {
            const chunked = chunkText(content, CHUNK_SIZE, OVERLAP);
            logger.info({ length: chunked.length }, "chunked text");
            const embeddings = await embedTexts(chunked.map((c) => c.content));
            logger.info({ length: embeddings.length }, "embeddings");
            return {
              ...page,
              embeddings,
              chunks: chunked,
            };
          }),
        );
        await conn?.task(async (t) => {
          const ins = pgp.helpers.insert(
            batch.flatMap(({ metadata, ...page }) =>
              page.embeddings.map((embedding, j) => ({
                page_id: page.page_id,
                chunk_index: page.chunks?.[j].index,
                embedding,
                content: page.chunks?.[j].content,
                metadata: {
                  referrer: page.referrer,
                  source: page.source,
                  title: metadata.Title,
                  author: metadata.Author,
                  url: metadata.URL,
                  description: metadata.Description,
                  date: metadata.Date,
                  categories: metadata.Categories,
                  language: metadata.Language,
                  chunker: `token_${CHUNK_SIZE}_${OVERLAP}`,
                  embed_model: process.env.EMBED_MODEL,
                },
              })),
            ),
            ["page_id", "chunk_index", "embedding", "content", "metadata"],
            "chunks",
          );

          await t.none(ins + " ON CONFLICT (page_id, chunk_index) DO NOTHING");
          // for (const [i, emb] of embeddings.entries()) {
          //   const chunk = chunked[i];
          //   const metadata = {
          //     ...page,
          //     ...page.metadata,
          //     metadata: undefined,
          //     chunker: `token_${CHUNK_SIZE}_${OVERLAP}`,
          //     embed_model: process.env.EMBED_MODEL,
          //   };
          //
          //   await t.none(
          //     `INSERT INTO chunks (page_id, chunk_index, content, embedding, metadata) VALUES ($1, $2, $3, $4::vector, $5::jsonb)
          //   ON CONFLICT (page_id, chunk_index) DO NOTHING`,
          //     [page.page_id, i, chunk, emb, metadata],
          //   );
          // }
        });
      } catch (err) {
        // TODO: handle error
        logger.error({ err }, "something went wrong embedding page.");
      } finally {
        logger.info(
          {
            // page_id: page.page_id,
            // source: page.source,
          },
          "done embedding page",
        );
      }
    }
    // for (const { content, ...page } of docs) {
    // }
  } catch (e) {
    // TODO: handle error
    logger.error({ error: e }, "error ingesting documents");
  } finally {
    logger.info("done ingesting documents");
    if (conn) conn.done();
  }
}

function chunkText(
  content: string,
  max: number,
  overlap: number,
): { index: number; content: string }[] {
  const words = content.trim().split(/\s+/);
  if (words.length == 0) {
    return [];
  }
  let step = max - overlap;
  if (step <= 0) {
    step = max;
  }

  const chunks = [];
  let index = 0;

  for (let start = 0; start < words.length; start += step) {
    const end = Math.min(start + max, words.length);
    chunks.push({ index, content: words.slice(start, end).join(" ") });
    index++;
    if (end == words.length) {
      break;
    }
  }
  return chunks;
}

async function embedTexts(texts: string[]): Promise<number[][]> {
  const response = await ollama.embed({
    model: process.env.EMBED_MODEL || "llama3.2:3b",
    input: texts,
    dimensions: parseInt(process.env.EMBED_DIMENSIONS || "0") || undefined,
  });
  return response.embeddings;
}

if (import.meta.main) {
  let conn: IConnected<unknown, FixLater> | null = null;
  try {
    conn = await db.connect();
    const query = `SELECT id as page_id, content, url as source, referrer, fetched_at, metadata FROM pages p WHERE p.id NOT IN
    (SELECT DISTINCT CAST(c.metadata::json ->> 'page_id' AS INTEGER) FROM chunks c)`;
    const result = await conn.many<Document>(query);
    await ingestDocuments(result);
  } catch (err) {
    logger.error({ err }, "ingestion went wrong");
  } finally {
    logger.info("done ingesting documents");
    if (conn) conn.done();

    pgp.end();
    logger.info("closed pg pool");
  }
}
