import express, { type Express } from "express";
import cookieParser from "cookie-parser";
import createError from "http-errors";
import indexRouter from "./routes/index.ts";
import logger from "./utils/logger.ts";
import { pinoHttp } from "pino-http";
import { errorHandler, logErrors } from "./utils/error.ts";
import cors from "cors";

const app: Express = express();

app.use(pinoHttp({ logger }));
app.use(express.json());
app.use(express.urlencoded({ extended: false }));
app.use(cookieParser());
app.use(cors());

app.get("/healthz", (req, res) => {
  res.status(200).send("ok");
});
app.use("/", indexRouter);

app.use((req, res, next) => {
  next(createError(404));
});

app.use(logErrors);
app.use(errorHandler);
export default app;
