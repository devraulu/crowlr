import express, {
  type Request,
  type Response,
  type Express,
  type NextFunction,
} from "express";
import cookieParser from "cookie-parser";
import logger from "morgan";
import createError from "http-errors";
import indexRouter from "./routes/index.ts";

const app: Express = express();

app.use(logger("dev"));
app.use(express.json());
app.use(express.urlencoded({ extended: false }));
app.use(cookieParser());

app.use("/", indexRouter);

app.use((req, res, next) => {
  next(createError(404));
});

app.use(
  (
    err: { message: string; status: number },
    req: Request,
    res: Response,
    next: NextFunction,
  ) => {
    res.locals.message = err.message;
    res.locals.error = req.app.get("env") === "development" ? err : {};

    res.status(err.status || 500);
    res.render("error");
  },
);

export default app;
