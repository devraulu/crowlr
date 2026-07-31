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
import pinoHTTP from "pino-http";
import { errorHandler, logErrors } from "./utils/error.ts";

const app: Express = express();

app.use(pinoHTTP({ logger }));
app.use(express.json());
app.use(express.urlencoded({ extended: false }));
app.use(cookieParser());

app.get("/healthz", (req, res) => {
  res.status(200).send("ok");
});
app.use("/", indexRouter);

app.use((req, res, next) => {
  next(createError(404));
});

// app.use(
//   (
//     err: { message: string; status: number },
//     req: Request,
//     res: Response,
//     next: NextFunction,
//   ) => {
//     res.locals.message = err.message;
//     res.locals.error = req.app.get("env") === "development" ? err : {};
//
//     res.status(err.status || 500);
//     res.render("error");
//   },
// );

app.use(logErrors);
app.use(errorHandler);
export default app;
