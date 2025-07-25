import axios from "axios";
import config from "../config/config.js";

class CacheServiceClient {
  constructor(baseURL = config.services.cacheServiceUrl) {
    this.baseURL = baseURL;
    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: 3000,
      headers: {
        "Content-Type": "application/json",
        "User-Agent": "auth-service/1.0.0",
      },
    });

    // Add request interceptor for logging
    this.httpClient.interceptors.request.use(
      (config) => {
        console.log(
          `Cache Service Request: ${config.method?.toUpperCase()} ${config.url}`
        );
        return config;
      },
      (error) => {
        console.error("Cache Service Request Error:", error);
        return Promise.reject(error);
      }
    );

    // Add response interceptor for logging
    this.httpClient.interceptors.response.use(
      (response) => {
        console.log(
          `Cache Service Response: ${response.status} ${response.config.url}`
        );
        return response;
      },
      (error) => {
        console.error(
          "Cache Service Response Error:",
          error.response?.status,
          error.message
        );
        return Promise.reject(error);
      }
    );
  }

  async storeSession(sessionId, sessionData, ttl = 3600) {
    try {
      const response = await this.httpClient.post(
        `/api/v1/cache/session:${sessionId}`,
        {
          value: sessionData,
          ttl: ttl,
        }
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to store session: ${sessionId}`, error.message);
      // Don't throw - cache failures shouldn't break auth flow
      return null;
    }
  }

  async getSession(sessionId) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/cache/session:${sessionId}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to get session: ${sessionId}`, error.message);
      // Return null if session not found or cache unavailable
      return null;
    }
  }

  async invalidateSession(sessionId) {
    try {
      await this.httpClient.delete(`/api/v1/cache/session:${sessionId}`);
      return true;
    } catch (error) {
      console.error(
        `Failed to invalidate session: ${sessionId}`,
        error.message
      );
      // Don't throw - cache failures shouldn't break logout flow
      return false;
    }
  }

  async storeTokenBlacklist(tokenId, expiresAt) {
    try {
      const ttl = Math.max(
        0,
        Math.floor((new Date(expiresAt) - new Date()) / 1000)
      );
      const response = await this.httpClient.post(
        `/api/v1/cache/blacklist:${tokenId}`,
        {
          value: { blacklisted: true, timestamp: new Date().toISOString() },
          ttl: ttl,
        }
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to blacklist token: ${tokenId}`, error.message);
      // Don't throw - cache failures shouldn't break token revocation
      return null;
    }
  }

  async isTokenBlacklisted(tokenId) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/cache/blacklist:${tokenId}`
      );
      return response.data !== null;
    } catch (error) {
      console.error(
        `Failed to check token blacklist: ${tokenId}`,
        error.message
      );
      // If cache is unavailable, assume token is not blacklisted to avoid blocking valid requests
      return false;
    }
  }

  async storeRateLimitCounter(key, count, windowMs) {
    try {
      const ttl = Math.ceil(windowMs / 1000);
      const response = await this.httpClient.post(
        `/api/v1/cache/ratelimit:${key}`,
        {
          value: { count, timestamp: new Date().toISOString() },
          ttl: ttl,
        }
      );
      return response.data;
    } catch (error) {
      console.error(
        `Failed to store rate limit counter: ${key}`,
        error.message
      );
      // Don't throw - cache failures shouldn't break rate limiting (fall back to in-memory)
      return null;
    }
  }

  async getRateLimitCounter(key) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/cache/ratelimit:${key}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to get rate limit counter: ${key}`, error.message);
      // Return null if counter not found or cache unavailable
      return null;
    }
  }

  async incrementRateLimitCounter(key, windowMs) {
    try {
      const ttl = Math.ceil(windowMs / 1000);
      const response = await this.httpClient.post(
        `/api/v1/cache/ratelimit:${key}/increment`,
        {
          ttl: ttl,
        }
      );
      return response.data;
    } catch (error) {
      console.error(
        `Failed to increment rate limit counter: ${key}`,
        error.message
      );
      // Return null if cache unavailable
      return null;
    }
  }

  async storeUserLoginAttempts(userId, attempts, windowMs) {
    try {
      const ttl = Math.ceil(windowMs / 1000);
      const response = await this.httpClient.post(
        `/api/v1/cache/login_attempts:${userId}`,
        {
          value: { attempts, timestamp: new Date().toISOString() },
          ttl: ttl,
        }
      );
      return response.data;
    } catch (error) {
      console.error(
        `Failed to store login attempts for user: ${userId}`,
        error.message
      );
      // Don't throw - cache failures shouldn't break login attempt tracking
      return null;
    }
  }

  async getUserLoginAttempts(userId) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/cache/login_attempts:${userId}`
      );
      return response.data;
    } catch (error) {
      console.error(
        `Failed to get login attempts for user: ${userId}`,
        error.message
      );
      // Return null if not found or cache unavailable
      return null;
    }
  }

  async clearUserLoginAttempts(userId) {
    try {
      await this.httpClient.delete(`/api/v1/cache/login_attempts:${userId}`);
      return true;
    } catch (error) {
      console.error(
        `Failed to clear login attempts for user: ${userId}`,
        error.message
      );
      // Don't throw - cache failures shouldn't break successful login flow
      return false;
    }
  }

  // Generic cache operations
  async set(key, value, ttl = 3600) {
    try {
      const response = await this.httpClient.post(`/api/v1/cache/${key}`, {
        value,
        ttl,
      });
      return response.data;
    } catch (error) {
      console.error(`Failed to set cache key: ${key}`, error.message);
      return null;
    }
  }

  async get(key) {
    try {
      const response = await this.httpClient.get(`/api/v1/cache/${key}`);
      return response.data;
    } catch (error) {
      console.error(`Failed to get cache key: ${key}`, error.message);
      return null;
    }
  }

  async delete(key) {
    try {
      await this.httpClient.delete(`/api/v1/cache/${key}`);
      return true;
    } catch (error) {
      console.error(`Failed to delete cache key: ${key}`, error.message);
      return false;
    }
  }

  async exists(key) {
    try {
      const response = await this.httpClient.get(`/api/v1/cache/${key}/exists`);
      return response.data?.exists || false;
    } catch (error) {
      console.error(
        `Failed to check cache key existence: ${key}`,
        error.message
      );
      return false;
    }
  }

  // Health check methods
  async checkHealth() {
    try {
      const response = await this.httpClient.get("/health");
      return { status: "healthy", data: response.data };
    } catch (error) {
      return { status: "unhealthy", error: error.message };
    }
  }

  async checkReadiness() {
    try {
      const response = await this.httpClient.get("/ready");
      return { status: "ready", data: response.data };
    } catch (error) {
      return { status: "not_ready", error: error.message };
    }
  }

  // Cache statistics
  async getStats() {
    try {
      const response = await this.httpClient.get("/api/v1/cache/stats");
      return response.data;
    } catch (error) {
      console.error("Failed to get cache stats:", error.message);
      return null;
    }
  }
}

export default new CacheServiceClient();
