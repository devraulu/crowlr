import pool from "./db.ts";
import vectorStore from "./vector-store.ts";
import type { Document } from "@langchain/core/documents";

import { RecursiveCharacterTextSplitter } from "@langchain/textsplitters";

const RETRIEVAL_K = 16;

const text = `SELECT id as page_id, html, url as source, title, referrer, last_modified, fetched_at FROM pages p WHERE p.id NOT IN
  (SELECT DISTINCT CAST(t.meta::json ->> 'page_id' AS INTEGER) FROM testlangchain t)`;
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

const htmlSplitter = RecursiveCharacterTextSplitter.fromLanguage("html", {
  chunkSize: 200,
  chunkOverlap: 0,
});

const unsplitDocuments: Document[] = pages.rows.map(({ html, ...r }) => ({
  pageContent: html,
  metadata: { ...r },
}));
console.log({ unsplitDocsCount: unsplitDocuments.length });

const chunks: Document[] = await htmlSplitter.splitDocuments(unsplitDocuments);

console.log({ chunksLength: chunks.length });

// const { documents, ids } = chunks
//   .map(({ text, ...r }) => ({
//     doc: {} as Document,
//     id: crypto.randomUUID(),
//   }))
//   .reduce<{
//     documents: Document[];
//     ids: ReturnType<typeof crypto.randomUUID>[];
//   }>(
//     (acc, { doc, id }) => ({
//       documents: [...acc.documents, doc],
//       ids: [...acc.ids, id],
//     }),
//     { documents: [], ids: [] },
//   );

// console.log({ documents, ids });
await vectorStore.addDocuments(chunks);

const similaritySearchResults = await vectorStore.similaritySearchWithScore(
  "wolf",
  RETRIEVAL_K,
  {},
);

for (const [doc, score] of similaritySearchResults) {
  console.log(
    `* [SIM=${score.toFixed(4)}] ${doc.pageContent} [${JSON.stringify(doc.metadata)}]`,
  );
}
