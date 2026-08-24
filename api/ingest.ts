import { type IConnected } from "pg-promise";
import ollama from "./ollama.ts";
import { db, pgp } from "./db.ts";
import type { FixLater } from "./types/index.ts";
import logger from "./utils/logger.ts";

const CHUNK_SIZE = 500;
const OVERLAP = 20;

interface Document {
  content: string;
  source: string;
  title: string;
  referrer: string;
  last_modified: string;
  fetched_at: string;
  page_id: string;
}

async function ingestDocuments(docs: Document[]) {
  logger.info({ length: docs.length }, "ingesting documents");
  let conn: IConnected<unknown, FixLater> | null = null;
  try {
    conn = await db.connect();

    for (const { content, ...page } of docs) {
      try {
        const chunkObjs = chunkText(content, CHUNK_SIZE, OVERLAP);
        logger.info({ length: chunkObjs.length }, "chunked text");
        const chunkTexts = chunkObjs.map((c) => `search_document:` + c.content);
        const embeddings = await embedTexts(chunkTexts);
        logger.info({ length: embeddings.length }, "embeddings");

        await conn.task(async (t) => {
          for (const [i, emb] of embeddings.entries()) {
            const chunk = chunkTexts[i];
            const metadata = {
              ...page,
              chunker: `token_${CHUNK_SIZE}_${OVERLAP}`,
              embed_model: process.env.EMBED_MODEL,
            };

            await t.one(
              `INSERT INTO chunks (page_id, chunk_index, content, embedding, metadata) VALUES ($1, $2, $3, $4::vector, $5::jsonb)
            ON CONFLICT (page_id, chunk_index) DO NOTHING RETURNING id`,
              [page.page_id, i, chunk, emb, metadata],
            );
          }
        });
      } catch (e) {
        // TODO: handle error

        logger.error(e, "something went wrong embedding page.");
      } finally {
        logger.info(
          {
            page_id: page.page_id,
            source: page.source,
          },
          "done embedding page",
        );
      }
    }
  } catch (e) {
    logger.error(e, "error ingesting documents");
    throw e;
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
    dimensions: parseInt(process.env.EMBED_DIMENSIONS || "0"),
  });
  return response.embeddings;
}

if (import.meta.main) {
  let conn: IConnected<{}, any> | null = null;
  try {
    logger.info("starting ingest");
    conn = await db.connect();
    logger.info("connected to db");
    const query = `SELECT id as page_id, content, url as source, title, referrer, fetched_at FROM pages p WHERE p.id NOT IN
    (SELECT DISTINCT CAST(c.metadata::json ->> 'page_id' AS INTEGER) FROM chunks c)`;
    const result = await conn?.many<Document>(query);
    logger.info({ count: result.length }, "got results from db");
    await ingestDocuments(result || []);
  } catch (e) {
    logger.error(e, "error ingesting documents");
  } finally {
    logger.info("done ingesting documents");
    if (conn) conn.done();
    pgp.end();
    logger.info("closed pg pool");
  }
}
