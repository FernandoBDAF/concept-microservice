import StorageServiceClient from "../clients/storageServiceClient.js";
import CacheServiceClient from "../clients/cacheServiceClient.js";
import config from "../config/config.js";

class HealthService {
  constructor() {
    this.storageClient = new StorageServiceClient(config);
    this.cacheClient = new CacheServiceClient(config);
  }

  async checkHealth() {
    const health = {
      status: "healthy",
      timestamp: new Date().toISOString(),
      service: "auth-service",
      version: process.env.npm_package_version || "1.0.0",
      environment: config.server.nodeEnv,
      dependencies: {},
      uptime: process.uptime(),
    };

    // Check storage-service
    try {
      const storageHealthy = await this.storageClient.healthCheck();
      health.dependencies.storage = storageHealthy ? "healthy" : "unhealthy";
      if (!storageHealthy) health.status = "degraded";
    } catch (error) {
      health.dependencies.storage = "unhealthy";
      health.status = "degraded";
    }

    // Check cache-service
    try {
      const cacheHealthy = await this.cacheClient.healthCheck();
      health.dependencies.cache = cacheHealthy ? "healthy" : "unhealthy";
      if (!cacheHealthy) health.status = "degraded";
    } catch (error) {
      health.dependencies.cache = "unhealthy";
      health.status = "degraded";
    }

    return health;
  }

  async checkReadiness() {
    try {
      // For auth-service, ready when storage-service is available
      // Cache-service is optional for readiness
      const storageHealthy = await this.storageClient.healthCheck();

      if (storageHealthy) {
        return {
          status: "ready",
          timestamp: new Date().toISOString(),
          message: "Auth service is ready to accept requests",
        };
      } else {
        return {
          status: "not ready",
          timestamp: new Date().toISOString(),
          message: "Storage service is not available",
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
