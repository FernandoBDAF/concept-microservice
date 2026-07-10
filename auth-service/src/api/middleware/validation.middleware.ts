import type { NextFunction, Request, Response } from "express";
import type { ZodError, ZodTypeAny } from "zod";
import { ValidationError } from "../../utils/errors.js";

interface ParsedRequest {
  body?: unknown;
  query?: unknown;
  params?: unknown;
}

export const validate = (schema: ZodTypeAny) => {
  return async (
    req: Request,
    _res: Response,
    next: NextFunction
  ): Promise<void> => {
    try {
      const raw = (await schema.parseAsync({
        body: req.body as unknown,
        query: req.query as unknown,
        params: req.params as unknown,
        headers: req.headers as unknown,
      })) as unknown;
      const parsed = raw as ParsedRequest;

      if (parsed.body !== undefined) {
        req.body = parsed.body;
      }
      if (parsed.query !== undefined && parsed.query !== null) {
        req.query = parsed.query as Request["query"];
      }
      if (parsed.params !== undefined && parsed.params !== null) {
        req.params = parsed.params as Request["params"];
      }

      next();
    } catch (error) {
      const zodError = error as ZodError;
      const details = zodError.errors.reduce<Record<string, string>>(
        (acc, err) => {
          const path = err.path.join(".");
          acc[path] = err.message;
          return acc;
        },
        {}
      );

      next(new ValidationError("Validation failed", details));
    }
  };
};

