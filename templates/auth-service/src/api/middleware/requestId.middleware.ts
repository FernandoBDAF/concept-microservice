import type { NextFunction, Request, Response } from "express";
import crypto from "node:crypto";

export const requestIdMiddleware = (
  req: Request,
  res: Response,
  next: NextFunction
): void => {
  const requestId = req.headers["x-request-id"]?.toString() ?? crypto.randomUUID();
  req.id = requestId;
  res.setHeader("x-request-id", requestId);
  next();
};

