import express, {
  type Request,
  type Response,
  type Express,
  type NextFunction,
} from "express";
import cookieParser from "cookie-parser";
import createError from "http-errors";
import indexRouter from "./routes/index.ts";
import logger from "./utils/logger.ts";
import { pinoHttp } from "pino-http";
import {
  ApplicationError,
  errorHandler,
  logErrors,
  type CustomError,
} from "./utils/error.ts";
import cors from "cors";

const app: Express = express();

app.use(pinoHttp({ logger }));
app.use(express.json());
app.use(express.urlencoded({ extended: false }));
// app.use(cookieParser());
// app.use(cors());
// app.options("{*splat}", cors());
//
app.get("/healthz", (req, res) => {
  res.status(200).send("ok");
});
// this requires authentication and loads the user for all routes after this call
// app.all('{*splat}', requireAuthentication, loadUser)
app.use("/", indexRouter);

// app.use(logErrors);
function myErrorHandler(
  err: Error,
  req: Request,
  res: Response,
  next: NextFunction,
) {
  if (res.headersSent) {
    return next(err);
  }

  const isAppError = err instanceof ApplicationError;
  // const statusCode = (err as CustomError).statusCode || 500;
  // const status =
  //   (err as CustomError).status ||
  //   (String(statusCode).startsWith("4") ? "fail" : "error");
  logger.error(
    { error: err, isAppError, stack: err.stack },
    "something went wrong",
  );

  // if (process.env.NODE_ENV === "development") {
  //   res.status(statusCode).json({
  //     status,
  //     message: err.message || "Internal Server Error",
  //     error: err,
  //     stack: err.stack,
  //   });
  // } else {
  if (isAppError) {
    res.status(500).json({
      message: err.message,
    });
  } else {
    res.status(500).json({
      status: "error",
      message: "An unexpected internal server error occurred",
    });
  }
}

app.use((req, res, next) => {
  logger.error({}, "404 not found");
  next(createError(404));
});
app.use(myErrorHandler);

if (import.meta.main) {
  app.listen(8081, () => {
    logger.info("server listening on port 8081");
  });
}
export default app;
