import pino from "pino";
 

const transport = pino.transport({
  targets: [
    {
      target: "pino/file",
      options: { destination: `${import.meta.dirname}/../server.log` },
    },
    { target: "pino-pretty" },
  ],
});

const logger = pino(
  {
    level: process.env.LOG_LEVEL || "info",
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
