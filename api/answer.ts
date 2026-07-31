import { config } from "./config.ts";
import ollama from "./ollama.ts";
import retrieve, { EXAMPLE_QUERY, MatchingChunk } from "./retrieval.ts";

const SYSTEM_PROMPT = `You are a helpful assistant answering questions and searches about the results of crawled web pages. Answer the user's question using ONLY the provided context which matches of the user query against the fetched content.

Rules:
- If the context does not contain the answer, say: "I don't know based on the provided context."
- Do not use outside knowledge.
- Cite sources in this format: [source: {source}#chunk:{chunk_id}]
`;

function formatContext(chunks: MatchingChunk[]): string {
  const parts = ["--- BEGIN CONTEXT ---"];

  for (const c of chunks) {
    parts.push(
      `\nchunk_index="${c.chunk_index}" source="${c.metadata.source}"\n${c.content}`,
    );
  }

  parts.push("--- END CONTEXT ---");

  return parts.join("\n\n");
}

async function* answer(
  q: string,
  chunks: MatchingChunk[],
): AsyncGenerator<{ message: string; created_at: Date }> {
  const userContent = `Question:\n${q}\n\nContext:\n${formatContext(chunks)}`;

  const stream = await ollama.chat({
    model: config.llm.gen_model || "llama3.2:3b",
    messages: [
      { role: "system", content: SYSTEM_PROMPT },
      {
        role: "user",
        content: userContent,
      },
    ],
    options: {
      temperature: 0,
    },
    stream: true,
  });

  for await (const chunk of stream) {
    const {
      message: { content },
      created_at,
    } = chunk;
    yield { message: content, created_at };
  }
}

if (import.meta.main) {
  const chunks = await retrieve(EXAMPLE_QUERY, 16);
  console.log("retrieved chunks", {
    count: chunks.length,
    titles: chunks.map((c) => [c.metadata.title, c.metadata.source]),
  });
  const stream = answer(EXAMPLE_QUERY, chunks);
  process.stdout.write("Answer:\n");
  for await (const chunk of stream) {
    process.stdout.write(chunk.message);
  }
  process.stdout.write("\n");
}
