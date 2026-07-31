import z, { ZodSchema } from "zod";
import { err } from "./utils/format.ts";
import { NextFunction, Request, Response } from "express";

function validate(schema: { query?: ZodSchema; body?: ZodSchema }) {
  return async (req: Request, res: Response, next: NextFunction) => {
    if (schema.query) {
      const safeQuery = schema.query.safeParse(req.query);
      if (!safeQuery.success) {
        return res.status(400).json({
          error: "invalid query params: ",
          details: z.treeifyError(safeQuery.error),
        });
      }
      req.query = safeQuery.data;
    }

    if (schema.body) {
      const safeBody = schema.body.safeParse(req.body);
      if (!safeBody.success) {
        return res
          .status(400)
          .json(
            err(
              "invalid query params: ",
              z.treeifyError(safeBody.error).errors,
            ),
          );
      }
    }
  };
}
