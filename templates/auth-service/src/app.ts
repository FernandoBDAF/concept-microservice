import express from "express";
import helmet from "helmet";
import { config } from "./config/index.js";
import { registerRoutes } from "./api/routes/index.js";
import jwksRoutes from "./api/routes/jwks.routes.js";
import { errorHandler, notFoundHandler } from "./api/middleware/error.middleware.js";
import { requestIdMiddleware } from "./api/middleware/requestId.middleware.js";
import { metricsMiddleware } from "./api/middleware/metrics.middleware.js";

export const createApp = () => {
  const app = express();

  // Honour X-Forwarded-* from one proxy hop (the ingress). Edge rate limiting
  // and TLS termination live at that ingress (see README "Rate limiting").
  app.set("trust proxy", 1);

  const helmetOptions = config.server.isDevelopment
    ? { contentSecurityPolicy: false }
    : {};
  app.use(helmet(helmetOptions));

  // JWKS is public infrastructure (ADR-009.1): mounted before any auth so
  // verification keys are always reachable.
  app.use(jwksRoutes);

  app.use(requestIdMiddleware);
  app.use(metricsMiddleware);

  app.use(express.json({ limit: "1mb" }));
  app.use(express.urlencoded({ extended: true }));

  app.get("/", (_req, res) => {
    res.status(200).json({
      service: "auth-service",
      version: process.env.npm_package_version ?? "1.0.0",
      status: "running",
      timestamp: new Date().toISOString(),
      environment: config.server.nodeEnv,
      architecture: "microservices",
      database: "postgresql",
      endpoints: {
        health: "/health",
        ready: "/ready",
        live: "/live",
        auth: {
          v1: "/v1/auth/*",
        },
        users: {
          profile: "/v1/users/me",
          management: "/v1/users/*",
        },
      },
    });
  });

  registerRoutes(app);

  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
};

