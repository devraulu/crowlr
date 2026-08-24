import { type NextFunction, type Request, type Response } from "express";
import logger from "./logger.ts";

export class ApplicationError extends Error {
  constructor(message: string) {
    super(message);
    // this.statusCode = statusCode;
    // this.status = String(statusCode).startsWith("4") ? "fail" : "error";

    Error.captureStackTrace(this, this.constructor);
    Object.setPrototypeOf(this, ApplicationError.prototype);
  }
}

export class BadRequestError extends ApplicationError {
  constructor(message = "Bad Request") {
    super(message);
  }
}

export class NotFoundError extends ApplicationError {
  constructor(message = "Resource Not Found") {
    super(message);
  }
}

export class DatabaseError extends ApplicationError {
  constructor(message = "Database operation failed") {
    super(message);
  }
}

export class AIServiceError extends ApplicationError {
  constructor(message = "AI service failed to process request") {
    super(message);
  }
}

export interface CustomError extends Error {}

export const logErrors = (
  err: CustomError,
  req: Request,
  res: Response,
  next: NextFunction,
) => {
  logger.error(
    {
      message: err.message,
      // statusCode: err.statusCode || 500,
      method: req.method,
      url: req.originalUrl,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    },
    "Unhandled Application Error",
  );

  next(err);
};

export const errorHandler = (
  err: Error | CustomError,
  req: Request,
  res: Response,
  next: NextFunction,
) => {
  logger.error(
    {
      message: err.message || "Internal Server Error",
      error: err,
      stack: err.stack,
    },
    "An error occurred",
  );
  if (res.headersSent) {
    return next(err);
  }

  const isAppError = err instanceof ApplicationError;

  // const statusCode = isAppError
  //   // ? err.statusCode
  //   // : (err as CustomError).statusCode || 500;
  // const status = isAppError
  //   ? err.status
  //   : (err as CustomError).status ||
  //     (String(statusCode).startsWith("4") ? "fail" : "error");
  // logger.error({ error: err }, "something went wrong");

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
      status,
      message: err.message,
    });
  } else {
    res.status(500).json({
      status: "error",
      message: "An unexpected internal server error occurred",
    });
  }
};
