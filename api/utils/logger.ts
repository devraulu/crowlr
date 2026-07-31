import pino, { destination } from "pino";
import { config } from "../config.ts";
const transport = pino.transport({
  targets: [
    {
      target: "pino/file",
      options: { destination: `${import.meta.dirname}/server.log` },
    },
    { target: "pino-pretty" },
  ],
});

const logger = pino(
  {
    level: config.logging.level,
    timestamp: pino.stdTimeFunctions.isoTime,
    redact: {
      paths: [
        // "name",
        // "user.name",
        // '*.user.name'
      ],
      censor: `[REDACTED]`,
      remove: true,
    },
  },
  transport,
);

export default logger;
