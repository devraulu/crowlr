import { NextFunction, Request, Response } from "express";
import logger from "./logger";

export class ApplicationError extends Error {
  public readonly statusCode: number;
  public readonly status: string;
  public readonly isOperational: boolean;

  constructor(message: string, statusCode: number) {
    super(message);
    this.statusCode = statusCode;
    this.status = String(statusCode).startsWith("4") ? "fail" : "error";
    this.isOperational = true;

    Error.captureStackTrace(this, this.constructor);

    Object.setPrototypeOf(this, ApplicationError.prototype);
  }
}

interface CustomError extends Error {
  statusCode?: number;
  status?: string;
  isOperational?: boolean;
}

export const errorHandler = (
  err: CustomError,
  req: Request,
  res: Response,
  next: NextFunction,
) => {
  // if streaming let Express handle the error
  if (res.headersSent) {
    return next(err);
  }

  err.statusCode = err.statusCode || 500;
  err.status = err.status || "error";

  if (process.env.NODE_ENV == "development") {
    res.status(err.statusCode || 500).json({
      error: err,
      stack: err.stack,
      status: err.status,
      message: err.message || "internal server error",
    });
  } else if (process.env.NODE_ENV == "production") {
    if (err.isOperational) {
      res.status(err.statusCode).json({
        status: err.status,
        message: err.message,
      });
    } else {
      res.status(500).json({
        status: "error",
        message: err.message || "internal server error",
      });
    }
  }
};

export const logErrors = (
  err: Error,
  req: Request,
  res: Response,
  next: NextFunction,
) => {
  logger.error(
    {
      message: err.message || 500,
      method: req.method,
      url: req.originalUrl,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    },
    "error",
  );

  next(err);
};
