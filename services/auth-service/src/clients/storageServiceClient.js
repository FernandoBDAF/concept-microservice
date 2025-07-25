import axios from "axios";
import CircuitBreaker from "opossum";

class StorageServiceClient {
  constructor(config) {
    this.baseURL = config.services.storageServiceUrl;
    this.timeout = config.services.timeout;

    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: this.timeout,
      headers: {
        "Content-Type": "application/json",
        "X-Service": "auth-service",
        "X-Service-Version": "1.0.0",
      },
    });

    // Circuit breaker for user operations
    this.userOperationsBreaker = new CircuitBreaker(
      this._executeUserOperation.bind(this),
      {
        timeout: config.circuitBreaker.timeout,
        errorThresholdPercentage:
          config.circuitBreaker.errorThresholdPercentage,
        resetTimeout: config.circuitBreaker.resetTimeout,
        name: "storage-user-operations",
      }
    );

    // Circuit breaker for audit operations (non-blocking)
    this.auditBreaker = new CircuitBreaker(
      this._executeAuditOperation.bind(this),
      {
        timeout: config.circuitBreaker.timeout,
        errorThresholdPercentage:
          config.circuitBreaker.errorThresholdPercentage,
        resetTimeout: config.circuitBreaker.resetTimeout,
        name: "storage-audit-operations",
      }
    );

    this._setupCircuitBreakerEvents();
  }

  // User operations (BLOCKING - critical for auth)
  async getUserByEmail(email) {
    return await this.userOperationsBreaker.fire("getUserByEmail", email);
  }

  async getUserById(id) {
    return await this.userOperationsBreaker.fire("getUserById", id);
  }

  async createUser(userData) {
    return await this.userOperationsBreaker.fire("createUser", userData);
  }

  async updateUser(userId, userData) {
    return await this.userOperationsBreaker.fire(
      "updateUser",
      userId,
      userData
    );
  }

  async recordLoginAttempt(userId, ipAddress, success) {
    return await this.userOperationsBreaker.fire(
      "recordLoginAttempt",
      userId,
      ipAddress,
      success
    );
  }

  // Audit operations (NON-BLOCKING - should not fail auth)
  async logAuditEvent(auditData) {
    return await this.auditBreaker
      .fire("logAuditEvent", auditData)
      .catch((err) => {
        console.error("Audit logging failed:", err.message);
        // Don't throw - audit logging should not block auth operations
      });
  }

  // Private methods for circuit breaker execution
  async _executeUserOperation(operation, ...args) {
    switch (operation) {
      case "getUserByEmail":
        const response = await this.httpClient.get(
          `/api/v1/auth/users/email/${args[0]}`
        );
        return response.data;

      case "getUserById":
        const userResponse = await this.httpClient.get(
          `/api/v1/auth/users/${args[0]}`
        );
        return userResponse.data;

      case "createUser":
        const createResponse = await this.httpClient.post(
          "/api/v1/auth/users",
          args[0]
        );
        return createResponse.data;

      case "updateUser":
        const updateResponse = await this.httpClient.put(
          `/api/v1/auth/users/${args[0]}`,
          args[1]
        );
        return updateResponse.data;

      case "recordLoginAttempt":
        const loginResponse = await this.httpClient.post(
          `/api/v1/auth/users/${args[0]}/login`,
          {
            ip_address: args[1],
            success: args[2],
            timestamp: new Date().toISOString(),
          }
        );
        return loginResponse.data;

      default:
        throw new Error(`Unknown user operation: ${operation}`);
    }
  }

  async _executeAuditOperation(operation, ...args) {
    switch (operation) {
      case "logAuditEvent":
        const response = await this.httpClient.post(
          "/api/v1/auth/audit",
          args[0]
        );
        return response.data;

      default:
        throw new Error(`Unknown audit operation: ${operation}`);
    }
  }

  _setupCircuitBreakerEvents() {
    this.userOperationsBreaker.on("open", () => {
      console.warn(
        "Storage service circuit breaker opened - user operations will fail fast"
      );
    });

    this.userOperationsBreaker.on("close", () => {
      console.info(
        "Storage service circuit breaker closed - user operations restored"
      );
    });

    this.auditBreaker.on("open", () => {
      console.warn(
        "Storage service audit circuit breaker opened - audit logging degraded"
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

export default StorageServiceClient;
