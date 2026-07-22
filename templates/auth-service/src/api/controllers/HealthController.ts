import type { Request, Response } from "express";
import { db } from "../../infrastructure/database/connection.js";
import { config } from "../../config/index.js";
import { register } from "../../infrastructure/metrics/metrics.js";

// eslint-disable-next-line @typescript-eslint/no-extraneous-class
export class HealthController {
  static async health(_req: Request, res: Response): Promise<void> {
    const dependencies: Record<string, "healthy" | "unhealthy"> = {};
    let status: "healthy" | "degraded" | "unhealthy" = "healthy";

    try {
      const dbHealthy = await db.healthCheck();
      dependencies.database = dbHealthy ? "healthy" : "unhealthy";
      if (!dbHealthy) status = "degraded";
    } catch {
      dependencies.database = "unhealthy";
      status = "degraded";
    }

    res.status(status === "healthy" ? 200 : 503).json({
      status,
      timestamp: new Date().toISOString(),
      service: "auth-service",
      version: process.env.npm_package_version ?? "2.0.0",
      environment: config.server.nodeEnv,
      dependencies,
      uptime: process.uptime(),
    });
  }

  static async ready(_req: Request, res: Response): Promise<void> {
    const isHealthy = await db.healthCheck();
    if (isHealthy) {
      res.status(200).json({
        status: "ready",
        timestamp: new Date().toISOString(),
        message: "Auth service is ready to accept requests",
      });
      return;
    }

    res.status(503).json({
      status: "not ready",
      timestamp: new Date().toISOString(),
      message: "Database is not available",
    });
  }

  static live(_req: Request, res: Response): void {
    res.status(200).json({
      status: "alive",
      timestamp: new Date().toISOString(),
      uptime: process.uptime(),
      memory: process.memoryUsage(),
    });
  }

  static async metrics(_req: Request, res: Response): Promise<void> {
    if (!config.metrics.enabled) {
      res.status(404).json({
        status: "error",
        message: "Metrics are disabled",
        code: "METRICS_DISABLED",
      });
      return;
    }

    res.setHeader("Content-Type", register.contentType);
    res.send(await register.metrics());
  }
}

