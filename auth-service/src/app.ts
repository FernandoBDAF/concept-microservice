import express from "express";
import helmet from "helmet";
import { config } from "./config/index.js";
import { registerRoutes } from "./api/routes/index.js";
import jwksRoutes from "./api/routes/jwks.routes.js";
import { errorHandler, notFoundHandler } from "./api/middleware/error.middleware.js";
import { globalRateLimit } from "./api/middleware/rateLimit.middleware.js";
import { requestIdMiddleware } from "./api/middleware/requestId.middleware.js";
import { metricsMiddleware } from "./api/middleware/metrics.middleware.js";
import swaggerUi from "swagger-ui-express";
import { generateOpenApiDocument } from "./api/docs/openapi.js";
import "./api/docs/index.js";

export const createApp = () => {
  const app = express();

  app.set("trust proxy", 1);

  const helmetOptions = config.server.isDevelopment
    ? { contentSecurityPolicy: false }
    : {};
  app.use(helmet(helmetOptions));

  // JWKS is public infrastructure (ADR-009.1): mounted before the global rate
  // limiter and any auth so verification keys are always reachable.
  app.use(jwksRoutes);

  app.use(globalRateLimit);
  app.use(requestIdMiddleware);
  app.use(metricsMiddleware);

  app.use(express.json({ limit: "1mb" }));
  app.use(express.urlencoded({ extended: true }));

  if (config.server.isDevelopment) {
    const openApiDoc = generateOpenApiDocument();
    app.use("/api-docs", swaggerUi.serve, swaggerUi.setup(openApiDoc));
    app.get("/api-docs.json", (_req, res) => res.json(openApiDoc));
  }

  app.get("/", (_req, res) => {
    res.status(200).json({
      service: "auth-service",
      version: process.env.npm_package_version ?? "2.0.0",
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

