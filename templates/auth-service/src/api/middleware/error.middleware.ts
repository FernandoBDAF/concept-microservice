import type {
  ErrorRequestHandler,
  NextFunction,
  Request,
  Response,
} from "express";
import type { ApiError } from "../../types/index.js";
import { config } from "../../config/index.js";
import { AppError } from "../../utils/errors.js";
import { logger } from "../../infrastructure/logging/logger.js";

export const errorHandler: ErrorRequestHandler = (
  error: Error,
  req: Request,
  res: Response,
  _next: NextFunction
): void => {
  const requestId = req.id;

  logger.error(
    {
      err: error,
      requestId,
      path: req.path,
      method: req.method,
    },
    "Request error"
  );

  if (error instanceof AppError) {
    const response: ApiError = {
      status: "error",
      message: error.message,
      code: error.code,
    };

    if (error.details) {
      response.details = error.details;
    }

    if (config.server.isDevelopment) {
      if (error.stack) {
        response.stack = error.stack;
      }
    }

    res.status(error.statusCode).json(response);
    return;
  }

  const response: ApiError = {
    status: "error",
    message: config.server.isProduction
      ? "An internal server error occurred"
      : error.message,
    code: "INTERNAL_ERROR",
  };

  if (config.server.isDevelopment) {
    if (error.stack) {
      response.stack = error.stack;
    }
  }

  res.status(500).json(response);
};

export const notFoundHandler = (req: Request, res: Response): void => {
  res.status(404).json({
    status: "error",
    message: `Route ${req.method} ${req.path} not found`,
    code: "NOT_FOUND",
  });
};

