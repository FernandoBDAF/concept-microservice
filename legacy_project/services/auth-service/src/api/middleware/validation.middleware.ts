import type { NextFunction, Request, Response } from "express";
import type { AnyZodObject, ZodError } from "zod";
import { ValidationError } from "../../utils/errors.js";

export const validate = (schema: AnyZodObject) => {
  return async (
    req: Request,
    _res: Response,
    next: NextFunction
  ): Promise<void> => {
    try {
      const result = await schema.parseAsync({
        body: req.body,
        query: req.query,
        params: req.params,
        headers: req.headers,
      });

      if (result.body) req.body = result.body;
      if (result.query) req.query = result.query;
      if (result.params) req.params = result.params;

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

