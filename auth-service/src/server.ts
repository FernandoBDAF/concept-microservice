import { createApp } from "./app.js";
import { config } from "./config/index.js";
import { migrationService } from "./infrastructure/database/migrations.js";
import { logger } from "./infrastructure/logging/logger.js";

const app = createApp();

async function startServer() {
  try {
    logger.info("Initializing database...");
    await migrationService.runMigrations();

    const server = app.listen(config.server.port, () => {
      logger.info(
        {
          port: config.server.port,
          env: config.server.nodeEnv,
        },
        "Auth Service running"
      );
    });

    const gracefulShutdown = (signal: string) => {
      logger.info({ signal }, "Shutting down gracefully");
      server.close(() => {
        logger.info("HTTP server closed");
        process.exit(0);
      });

      setTimeout(() => {
        logger.error(
          "Could not close connections in time, forcefully shutting down"
        );
        process.exit(1);
      }, 10000);
    };

    process.on("SIGTERM", () => {
      gracefulShutdown("SIGTERM");
    });
    process.on("SIGINT", () => {
      gracefulShutdown("SIGINT");
    });

    process.on("unhandledRejection", (reason) => {
      logger.error({ reason }, "Unhandled Promise Rejection");
    });

    process.on("uncaughtException", (error) => {
      logger.error({ error }, "Uncaught Exception");
      gracefulShutdown("UNCAUGHT_EXCEPTION");
    });
  } catch (error) {
    logger.error({ error }, "Failed to start server");
    process.exit(1);
  }
}

void startServer();

