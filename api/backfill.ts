import pool from "./db.ts";
import vectorStore from "./agent/vector-store.ts";
import type { Document } from "@langchain/core/documents";

import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

const RETRIEVAL_K = 16;

const text = `SELECT id as page_id, html, url as source, title, referrer, last_modified, fetched_at FROM pages p WHERE p.id NOT IN
  (SELECT DISTINCT CAST(t.metadata::json ->> 'page_id' AS INTEGER) FROM testlangchain t)`;
type Row = {
  html: string;
  source: string;
  title: string;
  referrer: string;
  last_modified: string;
  fetched_at: string;
  page_id: string;
};
const pages = await pool.query<Row>(text);
console.log({ pages: pages.rowCount });

const htmlSplitter = RecursiveCharacterTextSplitter.fromLanguage("html", {
  chunkSize: 1000,
  chunkOverlap: 0,
});

const unsplitDocuments: Document[] = pages.rows.map(({ html, ...r }) => ({
  pageContent: html,
  metadata: { ...r },
}));
console.log({ unsplitDocsCount: unsplitDocuments.length });

const chunks: Document[] = await htmlSplitter.splitDocuments(unsplitDocuments);

console.log({ chunksLength: chunks.length });

const limit = 1000 * 5;
let i = 0;

const ids = Array.from({ length: chunks.length }, () => crypto.randomUUID());

while (i < chunks.length) {
  console.log("saving documents to vector db", { last: i, limit });

  const last = Math.min(i + limit, chunks.length);
  await vectorStore.addDocuments(chunks.slice(i, last), {
    ids: ids.slice(i, last),
  });
  i += limit;
}

console.log("added documents to vector db successfully");
const similaritySearchResults = await vectorStore.similaritySearchWithScore(
  "wolf",
  RETRIEVAL_K,
  {},
);

for (const [doc, score] of similaritySearchResults.sort(
  ([, a], [, b]) => a - b,
)) {
  console.log(
    `* [SIM=${score.toFixed(4)}] ${doc.pageContent.length} [${JSON.stringify(doc.metadata)}]`,
  );
}
