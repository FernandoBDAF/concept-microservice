import express from "express";
import helmet from "helmet";
import rateLimit from "express-rate-limit";
import config from "./config/config.js";

// Import routes
import healthRoutes from "./routes/healthRoutes.js";
import authV1Routes from "./routes/authV1Routes.js";
import userManagementRoutes from "./routes/userManagementRoutes.js";

// Import services
import metricsService from "./service/metricsService.js";
import migrationService from "./service/migrationService.js";

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
  windowMs: 1 * 60 * 1000, // 1 minute
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
    database: "postgresql",
    endpoints: {
      health: "/health",
      ready: "/ready",
      live: "/live",
      metrics: "/metrics",
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

// V1 API routes
app.use("/v1/auth", authV1Routes);

// User routes (both profile and management)
app.use("/v1/users", userManagementRoutes);

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

// Initialize database and start server
async function startServer() {
  try {
    // Run database migrations
    console.log("🔧 Initializing database...");
    await migrationService.runMigrations();

    // Start the server
    const server = app.listen(config.server.port, () => {
      console.log(`🚀 Auth Service running on port ${config.server.port}`);
      console.log(`📊 Environment: ${config.server.nodeEnv}`);
      console.log(`🏗️  Architecture: Self-contained with PostgreSQL`);
      console.log(
        `🗄️  Database: PostgreSQL with connection pooling + migrations`
      );
      console.log(`🔒 Security: bcrypt + salt, Account locking, Rate limiting`);
      console.log(`📈 Metrics: Prometheus metrics enabled`);
      console.log(`\n🌐 Available endpoints:`);
      console.log(`   Health: http://localhost:${config.server.port}/health`);
      console.log(`   Ready: http://localhost:${config.server.port}/ready`);
      console.log(`   Live: http://localhost:${config.server.port}/live`);
      console.log(`   Metrics: http://localhost:${config.server.port}/metrics`);
      console.log(
        `   Auth API: http://localhost:${config.server.port}/v1/auth/*`
      );
      console.log(`\n📋 Critical endpoints for profile-service:`);
      console.log(`   Login: POST /v1/auth/login`);
      console.log(`   Token Validation: POST /v1/auth/token/validate`);
      console.log(`   User Profile: GET /v1/users/me`);
      console.log(`\n🔧 User Management endpoints (admin only):`);
      console.log(`   Create User: POST /v1/users/users`);
      console.log(`   List Users: GET /v1/users/users`);
      console.log(`   Get User: GET /v1/users/users/:id`);
      console.log(`   Update User: PUT /v1/users/users/:id`);
      console.log(`   Delete User: DELETE /v1/users/users/:id`);
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
  } catch (error) {
    console.error("❌ Failed to start server:", error);
    process.exit(1);
  }
}

// Start the server
startServer();
