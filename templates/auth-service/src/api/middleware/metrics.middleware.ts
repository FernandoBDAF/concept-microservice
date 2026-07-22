import type { NextFunction, Request, Response } from "express";
import { config } from "../../config/index.js";
import {
  httpRequestDuration,
  httpRequestsTotal,
} from "../../infrastructure/metrics/metrics.js";

export const metricsMiddleware = (
  req: Request,
  res: Response,
  next: NextFunction
): void => {
  if (!config.metrics.enabled) {
    next();
    return;
  }

  const start = process.hrtime.bigint();

  res.on("finish", () => {
    const durationSeconds =
      Number(process.hrtime.bigint() - start) / 1_000_000_000;
    const labels = {
      method: req.method,
      route: req.path,
      status_code: String(res.statusCode),
    };
    httpRequestDuration.observe(labels, durationSeconds);
    httpRequestsTotal.inc(labels);
  });

  next();
};
