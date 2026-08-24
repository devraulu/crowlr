import express, {
  type NextFunction,
  type Request,
  type Response,
} from "express";
import * as z from "zod";
import validate from "../middleware.ts";
import answer from "../answer.ts";
import retrieve from "../retrieval.ts";
import logger from "../utils/logger.ts";
import { ApplicationError } from "../utils/error.ts";

const router = express.Router();

const Message = z.object({
  content: z.string().nonempty(),
  role: z.string().nonempty(),
});
type Message = z.infer<typeof Message>;
const ChatBody = z.object({
  q: z.array(Message).min(1),
});
type ChatBody = z.infer<typeof ChatBody>;

const TOP_K = 16;

router.post(
  "/chat",
  validate({ body: ChatBody }),
  async (
    req: Request<unknown, unknown, ChatBody, unknown>,
    res: Response,
    next: NextFunction,
  ) => {
    const { q } = req.body;
    let heartbeat: NodeJS.Timeout | null = null;

    const lastMessage = q.at(-1);
    const chunks = await retrieve(lastMessage?.content || "", TOP_K);
    logger.info({ count: chunks.length }, "retrieved chunks");
    logger.debug({ chunks }, "retrieved chunks");

    try {
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

      // heartbeat to keep connection alive
      heartbeat = setInterval(() => {
        if (!res.writableEnded) {
          res.write(": heartbeat\n\n");
        }
      }, 15 * 1000);
      req.on("close", () => {
        if (heartbeat) clearInterval(heartbeat);
      });

      const stream = answer(q?.at(-1)?.content || "", chunks);
      let thinkingAcc = "",
        contentAcc = "";
      let inThinking = false;

      for await (const chunk of stream) {
        const {
          message: { content, thinking },
          done,
          done_reason,
          ...usage
        } = chunk;

        if (thinking) {
          if (!inThinking) {
            inThinking = true;
          }
          const data = `data: ${JSON.stringify({ type: "thinking", content: thinking })}\n\n`;
          const canContinue = res.write(data);
          if (!canContinue) {
            await new Promise((resolve) => res.once("drain", resolve));
          }
          thinkingAcc += thinking;
        } else if (content) {
          if (inThinking) {
            inThinking = false;
          }
          const data = `data: ${JSON.stringify({ type: "content", content })}\n\n`;
          const canContinue = res.write(data);
          if (!canContinue) {
            await new Promise((resolve) => res.once("drain", resolve));
          }
          contentAcc += content;
        }

        if (done) {
          res.write(
            `data: ${JSON.stringify({
              type: "done",
              done_reason,
              usage,
            })}\n\n`,
          );
        }
      }

      logger.info(
        {
          thinking: thinkingAcc,
          content: contentAcc,
        },
        "LLM response",
      );
    } catch (e) {
      const err = e as Error;
      logger.error({ err, query: q }, "Error in /chat route");

      if (!res.headersSent) {
        // headers not sent yet: forward to global Express error handler
        return next(err);
      }

      // headers already sent (mid-stream error): write SSE error event
      const isAppError = err instanceof ApplicationError;
      const userMessage = isAppError
        ? err.message
        : "An unexpected error occurred while generating response.";

      res.write(
        `data: ${JSON.stringify({
          type: "error",
          message: userMessage,
        })}\n\n`,
      );
    } finally {
      if (heartbeat) {
        clearInterval(heartbeat);
      }
      if (res.headersSent && !res.writableEnded) {
        res.end();
      }
    }
  },
);

export default router;
