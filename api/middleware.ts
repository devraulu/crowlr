import z, { ZodObject } from "zod";
import { err } from "./utils/format.ts";
import { type NextFunction, type Request, type Response } from "express";

function validate(schema: { query?: ZodObject; body?: ZodObject }) {
  return async (req: Request, res: Response, next: NextFunction) => {
    if (schema.query) {
      const safeQuery = schema.query.safeParse(req.query);
      if (!safeQuery.success) {
        return res.status(400).json({
          error: "invalid query params: ",
          details: z.treeifyError(safeQuery.error),
        });
      }
      req.query = safeQuery.data as any;
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
      req.body = safeBody.data;
    }

    next();
  };
}
export default validate;
