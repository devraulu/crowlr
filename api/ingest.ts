import { config } from "./config.ts";
import { type IConnected } from "pg-promise";
import ollama from "./ollama.ts";
import { db, pgp } from "./db.ts";

const CHUNK_SIZE = 500;
const OVERLAP = 20;

type Document = {
  content: string;
  source: string;
  title: string;
  referrer: string;
  last_modified: string;
  fetched_at: string;
  page_id: string;
};

async function ingestDocuments(docs: Document[]) {
  console.log("ingesting documents", { length: docs.length });
  let conn: IConnected<{}, any> | null = null;
  try {
    conn = await db.connect();

    for (const { content, ...page } of docs) {
      try {
        const chunkObjs = chunkText(content, CHUNK_SIZE, OVERLAP);
        console.log("chunked text", { length: chunkObjs.length });
        const chunkTexts = chunkObjs.map((c) => `search_document:` + c.content);
        const embeddings = await embedTexts(chunkTexts);
        console.log("embeddings", { length: embeddings.length });

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
        console.error("something went wrong embedding page.", {
          page_id: page.page_id,
          error: e,
        });
      } finally {
        console.log("done embedding page", {
          page_id: page.page_id,
          source: page.source,
        });
      }
    }
  } catch (e) {
    // TODO: handle error
    console.log("error ingesting documents", e);
  } finally {
    console.log("done ingesting documents");
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
    conn = await db.connect();
    const query = `SELECT id as page_id, html as content, url as source, title, referrer, last_modified, fetched_at FROM pages p WHERE p.id NOT IN
    (SELECT DISTINCT CAST(c.metadata::json ->> 'page_id' AS INTEGER) FROM chunks c)`;
    const result = await conn.many<Document>(query);
    await ingestDocuments(result);
  } catch (e) {
    // TODO: handle error
  } finally {
    console.log("done ingesting documents");
    if (conn) conn.done();
    pgp.end();
    console.log("closed pg pool");
  }
}
