import axios from "axios";
import CircuitBreaker from "opossum";

class CacheServiceClient {
  constructor(config) {
    this.baseURL = config.services.cacheServiceUrl;
    this.timeout = config.services.timeout;

    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: this.timeout,
      headers: {
        "Content-Type": "application/json",
        "X-Service": "auth-service",
      },
    });

    this.cacheBreaker = new CircuitBreaker(
      this._executeCacheOperation.bind(this),
      {
        timeout: config.circuitBreaker.timeout,
        errorThresholdPercentage:
          config.circuitBreaker.errorThresholdPercentage,
        resetTimeout: config.circuitBreaker.resetTimeout,
        name: "cache-operations",
      }
    );

    this._setupCircuitBreakerEvents();
  }

  // Session operations (NON-BLOCKING - auth should work without cache)
  async storeSession(sessionId, sessionData, ttl = 3600) {
    return await this.cacheBreaker
      .fire("storeSession", sessionId, sessionData, ttl)
      .catch((err) => {
        console.error("Session storage failed:", err.message);
        // Don't throw - session storage failure should not block auth
      });
  }

  async getSession(sessionId) {
    return await this.cacheBreaker.fire("getSession", sessionId);
  }

  async invalidateSession(sessionId) {
    return await this.cacheBreaker
      .fire("invalidateSession", sessionId)
      .catch((err) => {
        console.error("Session invalidation failed:", err.message);
      });
  }

  // Token blacklist operations (NON-BLOCKING - fail open)
  async blacklistToken(tokenId, ttl) {
    return await this.cacheBreaker
      .fire("blacklistToken", tokenId, ttl)
      .catch((err) => {
        console.error("Token blacklisting failed:", err.message);
      });
  }

  async isTokenBlacklisted(tokenId) {
    return await this.cacheBreaker
      .fire("isTokenBlacklisted", tokenId)
      .catch((err) => {
        console.error("Token blacklist check failed:", err.message);
        return false; // Fail open for token validation
      });
  }

  // Private method for circuit breaker
  async _executeCacheOperation(operation, ...args) {
    switch (operation) {
      case "storeSession":
        const response = await this.httpClient.post(
          `/api/v1/cache/session:${args[0]}`,
          {
            value: args[1],
            ttl: args[2],
          }
        );
        return response.data;

      case "getSession":
        const getResponse = await this.httpClient.get(
          `/api/v1/cache/session:${args[0]}`
        );
        return getResponse.data;

      case "invalidateSession":
        await this.httpClient.delete(`/api/v1/cache/session:${args[0]}`);
        return true;

      case "blacklistToken":
        const blacklistResponse = await this.httpClient.post(
          `/api/v1/cache/blacklist:${args[0]}`,
          {
            value: "blacklisted",
            ttl: args[1],
          }
        );
        return blacklistResponse.data;

      case "isTokenBlacklisted":
        try {
          await this.httpClient.get(`/api/v1/cache/blacklist:${args[0]}`);
          return true; // Token exists in blacklist
        } catch (error) {
          if (error.response && error.response.status === 404) {
            return false; // Token not in blacklist
          }
          throw error; // Other errors should be handled by circuit breaker
        }

      default:
        throw new Error(`Unknown cache operation: ${operation}`);
    }
  }

  _setupCircuitBreakerEvents() {
    this.cacheBreaker.on("open", () => {
      console.warn(
        "Cache service circuit breaker opened - cache operations degraded"
      );
    });

    this.cacheBreaker.on("close", () => {
      console.info(
        "Cache service circuit breaker closed - cache operations restored"
      );
    });
  }

  // Health check
  async healthCheck() {
    try {
      const response = await this.httpClient.get("/health", { timeout: 2000 });
      return response.status === 200;
    } catch (error) {
      return false;
    }
  }
}

export default CacheServiceClient;
