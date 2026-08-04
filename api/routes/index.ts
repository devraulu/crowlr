import express, { type Request } from "express";
import * as z from "zod";
import validate from "../middleware";
import answer from "../answer";
import retrieve from "../retrieval";

const router = express.Router();

const ChatQueryParams = z.object({
  q: z.string().min(1, "Query cannot be empty."),
});
type ChatQueryParams = z.infer<typeof ChatQueryParams>;

const TOP_K = 16;

router.post(
  "/chat",
  validate({ query: ChatQueryParams }),
  async (req: Request<{}, {}, {}, ChatQueryParams>, res, next) => {
    const { q } = req.query;

    const sseHeaders = new Headers({
      "Content-Type": "text/event-stream",
      "Cache-Control": "no-cache, no-transform", // prevents CDNs caching
      Connection: "keep-alive",
      "X-Accel-Buffering": "no", // prevents nginx buffering
    });
    res.setHeaders(sseHeaders);

    // normally buffered until res.end() is called or first chunk sent,
    // but since data is coming later we flush early
    res.flushHeaders();

    const heartbeat = setInterval(() => {
      res.write(": hearbeat\n\n");
    }, 15 * 1000);

    req.on("close", () => clearInterval(heartbeat));

    const chunks = await retrieve(q, TOP_K);
    try {
      const stream = answer(q, chunks);
      let thinkingAcc = "",
        contentAcc = "",
        inThinking = false;

      for await (const chunk of stream) {
        const {
          message: { content, thinking },
          done,
          done_reason,
        } = chunk;

        if (thinking) {
          if (!inThinking) {
            inThinking = true;

            const data = `data: ${JSON.stringify({ type: "thinking", content })}\n\n`;

            // backpressure
            const canContinue = res.write(data);
            if (!canContinue) {
              await new Promise((resolve) => res.once("drain", resolve));
            }
            thinkingAcc += thinking;
          }
        } else if (content) {
          if (inThinking) {
            inThinking = false;
          }
          const data = `data: ${JSON.stringify({ type: "content", content })}\n\n`;

          // backpressure
          const canContinue = res.write(data);
          if (!canContinue) {
            await new Promise((resolve) => res.once("drain", resolve));
          }

          contentAcc += content;
        }

        if (done) {
          // collect stats from the final chunk
          //...
          //{
          //   "model": "gemma4",
          //   "created_at": "2025-10-17T23:14:07.414671Z",
          //   "response": "Hello! How can I help you today?",
          //   "done": true,
          //   "done_reason": "stop",
          //   "total_duration": 174560334,
          //   "load_duration": 101397084,
          //   "prompt_eval_count": 11,
          //   "prompt_eval_duration": 13074791,
          //   "eval_count": 18,
          //   "eval_duration": 52479709
          // }
          res.write(
            `data: ${JSON.stringify({
              type: "done",
              doneReason: done_reason,
              usage: {},
            })}\n\n`,
          );
        }
      }
    } catch (e) {
      // TODO: research error handling approaches here
      // do we lift? do we define a custom error class? what are Ollama errors like?
      // do we handle the possible retrieve error? are we overthinking maybe
      res.write(
        `data: ${JSON.stringify({
          type: "error",
          message: (e as any).message,
        })}\n\n`,
      );
      // throw e;
      next(e);
    } finally {
      res.end();
    }
    next();
  },
);
export default router;
