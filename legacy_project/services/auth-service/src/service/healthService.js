import db from "./databaseService.js";

class HealthService {
  constructor() {
    this.db = db;
  }

  async checkHealth() {
    const health = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      service: "auth-service",
      version: process.env.npm_package_version || "1.0.0",
      environment: process.env.NODE_ENV || "development",
      dependencies: {},
      uptime: process.uptime(),
    };

    // Check database
    try {
      const dbHealthy = await this.db.healthCheck();
      health.dependencies.database = dbHealthy ? "healthy" : "unhealthy";
      if (!dbHealthy) health.status = "degraded";
    } catch (error) {
      health.dependencies.database = "unhealthy";
      health.status = "degraded";
      console.error("Database health check failed:", error);
    }

    return health;
  }

  async checkReadiness() {
    try {
      // For auth-service, ready when database is available
      const dbHealthy = await this.db.healthCheck();

      if (dbHealthy) {
        return {
          status: "ready",
          timestamp: new Date().toISOString(),
          message: "Auth service is ready to accept requests",
        };
      } else {
        return {
          status: "not ready",
          timestamp: new Date().toISOString(),
          message: "Database is not available",
        };
      }
    } catch (error) {
      return {
        status: "not ready",
        timestamp: new Date().toISOString(),
        error: error.message,
      };
    }
  }

  checkLiveness() {
    return {
      status: "alive",
      timestamp: new Date().toISOString(),
      uptime: process.uptime(),
      memory: process.memoryUsage(),
    };
  }
}

export default new HealthService();
