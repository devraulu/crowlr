import pino from "pino";
import config from "../config.ts";

const transport = pino.transport({
  targets: [
    {
      level: "debug",
      target: "pino/file",
      options: { destination: `${import.meta.dirname}/../server.log` },
    },
    // { target: "pino/file", options: { destination: 1 } },
    { level: "debug", target: "pino-pretty" },
    // {
    //   target: "pino/stdout",
    //   options: {},
    // },
  ],
});

const logger = pino(
  {
    level: config.logLevel,
    timestamp: pino.stdTimeFunctions.isoTime,
    // redact: {
    //   paths: [
    //      "name",
    //      "user.name",
    //      '*.user.name'
    //   ],
    //    censor: `[REDACTED]`,
    //   remove: true,
    // },
    serializers: {
      // user: (u) => ({ id: u.id, email: u.email }),
    },
  },
  transport,
);

export default logger;
