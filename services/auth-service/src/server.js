import express from "express";
import helmet from "helmet";
import rateLimit from "express-rate-limit";
import config from "./config/config.js";

// Import routes
import healthRoutes from "./routes/healthRoutes.js";
import authV1Routes from "./routes/authV1Routes.js";
import userV1Routes from "./routes/userV1Routes.js";

// Import services
import metricsService from "./service/metricsService.js";

const app = express();

// Trust proxy for proper IP detection
app.set("trust proxy", 1);

// Security middleware
app.use(
  helmet({
    contentSecurityPolicy:
      config.server.nodeEnv === "development" ? false : undefined,
  })
);

// Global rate limiting
const globalRateLimit = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100, // 100 requests per window
  standardHeaders: true,
  legacyHeaders: false,
});

app.use(globalRateLimit);

// Body parsing middleware
app.use(express.json({ limit: "1mb" }));
app.use(express.urlencoded({ extended: true }));

// Metrics endpoint
app.get("/metrics", async (req, res) => {
  res.set("Content-Type", "text/plain");
  res.end(await metricsService.getMetrics());
});

// Health and monitoring endpoints (no auth required)
app.use(healthRoutes);

// Root endpoint
app.get("/", (req, res) => {
  res.status(200).json({
    service: "auth-service",
    version: "1.0.0",
    status: "running",
    timestamp: new Date().toISOString(),
    environment: config.server.nodeEnv,
    architecture: "microservices",
    integration: {
      storage: config.services.storageServiceUrl,
      cache: config.services.cacheServiceUrl,
    },
    endpoints: {
      health: "/health",
      ready: "/ready",
      live: "/live",
      metrics: "/metrics",
      auth: {
        v1: "/v1/auth/*",
        users: "/v1/users/*",
      },
    },
  });
});

// V1 API routes (profile-service compatible)
app.use("/v1/auth", authV1Routes);
app.use("/v1/users", userV1Routes);

// Global error handler
app.use((error, req, res, next) => {
  console.error("Unhandled error:", error);

  // Don't expose internal errors in production
  const message =
    config.server.nodeEnv === "development"
      ? error.message
      : "An internal server error occurred";

  res.status(500).json({
    status: "error",
    message,
    data: null,
  });
});

// Start the server
const server = app.listen(config.server.port, () => {
  console.log(`🚀 Auth Service running on port ${config.server.port}`);
  console.log(`📊 Environment: ${config.server.nodeEnv}`);
  console.log(`🏗️  Architecture: Microservices Integration`);
  console.log(`🔗 Storage Service: ${config.services.storageServiceUrl}`);
  console.log(`🗄️  Cache Service: ${config.services.cacheServiceUrl}`);
  console.log(`🔒 Security: Rate limiting, Circuit breakers, Audit logging`);
  console.log(`📈 Metrics: Prometheus metrics enabled`);
  console.log(`\n🌐 Available endpoints:`);
  console.log(`   Health: http://localhost:${config.server.port}/health`);
  console.log(`   Ready: http://localhost:${config.server.port}/ready`);
  console.log(`   Live: http://localhost:${config.server.port}/live`);
  console.log(`   Metrics: http://localhost:${config.server.port}/metrics`);
  console.log(`   Auth API: http://localhost:${config.server.port}/v1/auth/*`);
  console.log(`\n📋 Critical endpoints for profile-service:`);
  console.log(`   Login: POST /v1/auth/login`);
  console.log(`   Token Validation: POST /v1/auth/token/validate`);
  console.log(`   User Profile: GET /v1/users/me`);
  console.log(`\nPress CTRL+C to stop server`);
});

// Graceful shutdown
const gracefulShutdown = (signal) => {
  console.log(`\n${signal} received. Shutting down gracefully...`);

  server.close(() => {
    console.log("HTTP server closed.");
    process.exit(0);
  });

  // Force close after 10 seconds
  setTimeout(() => {
    console.error(
      "Could not close connections in time, forcefully shutting down"
    );
    process.exit(1);
  }, 10000);
};

process.on("SIGTERM", () => gracefulShutdown("SIGTERM"));
process.on("SIGINT", () => gracefulShutdown("SIGINT"));

process.on("unhandledRejection", (reason, promise) => {
  console.error("Unhandled Promise Rejection:", reason);
});

process.on("uncaughtException", (error) => {
  console.error("Uncaught Exception:", error);
  gracefulShutdown("UNCAUGHT_EXCEPTION");
});
