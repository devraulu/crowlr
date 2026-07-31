import { tool } from "langchain";
import vectorStore from "./vector-store.ts";
import backend from "./backend.ts";
import z from "zod";

const RETRIEVED_K = 16;
export const searchCrawledSet = tool(
  async ({ query }) => {
    const similaritySearchResults = await vectorStore.similaritySearch(
      query,
      RETRIEVED_K,
    );
    console.log({ similaritySearchResults });

    const batchId = crypto.randomUUID().slice(0, 8);
    const uploads: Array<[string, Uint8Array]> = [];
    const savedPaths: string[] = [];
    const encoder = new TextEncoder();

    similaritySearchResults.forEach((doc, i) => {
      const path = `/retrieved/${batchId}/chunk_${i + 1}.md`;
      const content = `## Source: ${doc.metadata.source ?? "unknown"}\n\n# Title: ${doc.metadata.title}\n\n${doc.pageContent}`;

      uploads.push([path, encoder.encode(content)]);
      savedPaths.push(path);
    });

    backend.uploadFiles(uploads);
    return `Saved ${savedPaths.length} matching document chunks:\n${savedPaths.join("\n")}`;
  },
  {
    name: "search_crawled_set",
    description:
      "Search crawled set and save matching chunks to the agent filesystem.",
    schema: z.object({
      query: z.string().describe("Natural language search query"),
    }),
  },
);
