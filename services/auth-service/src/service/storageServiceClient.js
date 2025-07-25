import axios from "axios";
import config from "../config/config.js";

class StorageServiceClient {
  constructor(baseURL = config.services.storageServiceUrl) {
    this.baseURL = baseURL;
    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: 5000,
      headers: {
        "Content-Type": "application/json",
        "User-Agent": "auth-service/1.0.0",
      },
    });

    // Add request interceptor for logging
    this.httpClient.interceptors.request.use(
      (config) => {
        console.log(
          `Storage Service Request: ${config.method?.toUpperCase()} ${
            config.url
          }`
        );
        return config;
      },
      (error) => {
        console.error("Storage Service Request Error:", error);
        return Promise.reject(error);
      }
    );

    // Add response interceptor for logging
    this.httpClient.interceptors.response.use(
      (response) => {
        console.log(
          `Storage Service Response: ${response.status} ${response.config.url}`
        );
        return response;
      },
      (error) => {
        console.error(
          "Storage Service Response Error:",
          error.response?.status,
          error.message
        );
        return Promise.reject(error);
      }
    );
  }

  async getUserByEmail(email) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/auth/users/email/${encodeURIComponent(email)}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to get user by email: ${email}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async getUserById(userId) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/auth/users/${userId}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to get user by ID: ${userId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async createUser(userData) {
    try {
      const response = await this.httpClient.post(
        "/api/v1/auth/users",
        userData
      );
      return response.data;
    } catch (error) {
      console.error("Failed to create user:", error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async updateUser(userId, userData) {
    try {
      const response = await this.httpClient.put(
        `/api/v1/auth/users/${userId}`,
        userData
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to update user: ${userId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async deleteUser(userId) {
    try {
      const response = await this.httpClient.delete(
        `/api/v1/auth/users/${userId}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to delete user: ${userId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async recordLoginAttempt(userId, loginData) {
    try {
      const response = await this.httpClient.post(
        `/api/v1/auth/users/${userId}/login`,
        loginData
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to record login attempt: ${userId}`, error.message);
      // Don't throw - login attempt recording is non-critical
      return null;
    }
  }

  async lockAccount(userId, lockData) {
    try {
      const response = await this.httpClient.post(
        `/api/v1/auth/users/${userId}/lock`,
        lockData
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to lock account: ${userId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async logAuditEvent(auditData) {
    try {
      // Async audit logging - non-blocking
      const response = await this.httpClient.post(
        "/api/v1/auth/audit",
        auditData
      );
      return response.data;
    } catch (error) {
      console.error("Audit logging failed:", error.message);
      // Don't throw - audit logging failures shouldn't break auth flow
      return null;
    }
  }

  async getAuditLogs(filters = {}) {
    try {
      const response = await this.httpClient.get("/api/v1/auth/audit", {
        params: filters,
      });
      return response.data;
    } catch (error) {
      console.error("Failed to get audit logs:", error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  // Role management methods
  async getRoles() {
    try {
      const response = await this.httpClient.get("/api/v1/auth/roles");
      return response.data;
    } catch (error) {
      console.error("Failed to get roles:", error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async createRole(roleData) {
    try {
      const response = await this.httpClient.post(
        "/api/v1/auth/roles",
        roleData
      );
      return response.data;
    } catch (error) {
      console.error("Failed to create role:", error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async getRole(roleId) {
    try {
      const response = await this.httpClient.get(
        `/api/v1/auth/roles/${roleId}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to get role: ${roleId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async updateRole(roleId, roleData) {
    try {
      const response = await this.httpClient.put(
        `/api/v1/auth/roles/${roleId}`,
        roleData
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to update role: ${roleId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  async deleteRole(roleId) {
    try {
      const response = await this.httpClient.delete(
        `/api/v1/auth/roles/${roleId}`
      );
      return response.data;
    } catch (error) {
      console.error(`Failed to delete role: ${roleId}`, error.message);
      throw new Error(`Storage service unavailable: ${error.message}`);
    }
  }

  // Health check method
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
}

export default new StorageServiceClient();
