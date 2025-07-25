# Auth Service Microservices Integration Implementation Request (REVISED)

## 🚨 **CRITICAL ARCHITECTURAL CORRECTION REQUIRED**

**Current Problem**: Implementation violates microservices principles with direct database access and cloud dependencies  
**Required Action**: Complete architectural realignment with storage-service and cache-service integration  
**Priority**: BLOCKING - Foundation service must be corrected before ecosystem integration

## Task Context

**Primary Objective**: Transform Node.js auth-service from monolithic database-dependent application to proper microservices orchestration layer that integrates with storage-service and cache-service.

**Current Violations**:

- ❌ Direct Prisma/PostgreSQL database access
- ❌ AWS-specific dependencies (X-Ray, S3, Secrets Manager)
- ❌ Legacy tweet functionality
- ❌ Monolithic architecture patterns

**Required Architecture**: Thin orchestration layer with HTTP service integration

```
Auth Service (Node.js) - Orchestration Layer
├── HTTP API Layer (Express.js)
├── Service Integration Clients (HTTP)
├── Authentication Logic (JWT, Argon2)
└── Circuit Breakers & Health Checks

Dependencies:
├── Storage Service (User data, audit logs)
├── Cache Service (Sessions, token blacklist)
└── NO direct database access
```

## Documentation References

### 1. AUTH_SERVICE_MICROSERVICES_INTEGRATION_ANALYSIS.md (CRITICAL)

- **Section**: Complete architectural violation analysis and correction requirements
- **Purpose**: Identifies all code that must be removed and architectural patterns that must be implemented
- **Impact**: Defines exact implementation requirements for microservices compliance
- **Critical Elements**:
  - Files and dependencies to completely remove
  - Required service integration clients
  - Circuit breaker and health check patterns
  - Deployment and configuration requirements

### 2. integrations_storage_cache+all-services.md

- **Section**: Auth-service integration with storage and cache services
- **Purpose**: Defines how auth-service integrates within the microservices ecosystem
- **Impact**: Provides context for service integration patterns and data flow
- **Key Requirements**:
  - Storage-service auth data models and endpoints
  - Cache-service session management patterns
  - Service discovery and communication protocols

### 3. MICROSERVICES_DEPLOYMENT_STANDARD.md

- **Section**: Kubernetes deployment patterns and standards
- **Purpose**: Defines deployment structure and operational requirements
- **Impact**: Ensures consistent deployment patterns across services
- **Requirements**:
  - Dual deployment approach (manual and kustomize)
  - Health checks and monitoring integration
  - Service dependencies and network policies

## 🗑️ **PHASE 1: CODE CLEANUP (MANDATORY - Week 1)**

### **1.1 Files to COMPLETELY REMOVE**

```bash
# Database-related files (REMOVE ALL)
rm -rf prisma/
rm src/prismaClient.js

# AWS-specific files (REMOVE ALL)
rm src/service/secretsService.js
rm src/middleware/uploadImageToS3Middleware.js

# Legacy application files (REMOVE ALL)
rm src/service/tweetService.js
rm src/routes/tweetRoutes.js
rm src/routes/imageRoutes.js

# Direct database services (REPLACE WITH SERVICE CLIENTS)
rm src/service/userService.js      # Replace with StorageServiceClient
rm src/service/sessionService.js   # Replace with CacheServiceClient
rm src/service/auditService.js     # Replace with StorageServiceClient
```

### **1.2 Dependencies to REMOVE from package.json**

```json
{
  "dependencies": {
    // ❌ REMOVE: AWS dependencies
    // "@aws-sdk/client-s3": "^3.325.0",
    // "@aws-sdk/client-secrets-manager": "^3.338.0",
    // "@cosva-lab/aws-xray-sdk-prisma": "^0.0.7",
    // "aws-xray-sdk": "^3.5.0",

    // ❌ REMOVE: Database dependencies
    // "@prisma/client": "^4.13.0",
    // "prisma": "^4.13.0",

    // ❌ REMOVE: File upload dependencies
    // "multer": "^1.4.5-lts.1",
    // "multer-s3": "^3.0.1",

    // ❌ REMOVE: Duplicate password hashing
    // "bcrypt": "^5.0.1",  // Keep only argon2

    // ✅ KEEP: Essential microservices dependencies
    "argon2": "^0.31.2", // Password hashing
    "axios": "^1.6.2", // HTTP client for service integration
    "express": "^4.17.1", // Web framework
    "express-rate-limit": "^7.1.5", // Rate limiting
    "helmet": "^7.1.0", // Security headers
    "jsonwebtoken": "^8.5.1", // JWT tokens
    "opossum": "^8.0.0", // Circuit breaker
    "prom-client": "^15.1.0", // Prometheus metrics
    "validator": "^13.11.0" // Input validation
  }
}
```

### **1.3 Configuration Cleanup**

```javascript
// ❌ REMOVE from config.js
// this.database = {
//   url: process.env.DATABASE_URL,
// };

// this.aws = {
//   region: process.env.AWS_REGION || "us-east-1",
//   xrayEnabled: process.env.AWS_XRAY_ENABLED === "true",
// };

// ✅ ADD: Service integration configuration
this.services = {
  storageServiceUrl:
    process.env.STORAGE_SERVICE_URL || "http://storage-service:8080",
  cacheServiceUrl: process.env.CACHE_SERVICE_URL || "http://cache-service:8080",
  timeout: parseInt(process.env.SERVICE_TIMEOUT) || 5000,
  retries: parseInt(process.env.SERVICE_RETRIES) || 3,
};

this.circuitBreaker = {
  timeout: parseInt(process.env.CIRCUIT_BREAKER_TIMEOUT) || 3000,
  errorThresholdPercentage:
    parseInt(process.env.CIRCUIT_BREAKER_ERROR_THRESHOLD) || 50,
  resetTimeout: parseInt(process.env.CIRCUIT_BREAKER_RESET_TIMEOUT) || 30000,
};
```

## 🏗️ **PHASE 2: SERVICE INTEGRATION CLIENTS (MANDATORY - Week 2)**

### **2.1 Storage Service Client Implementation**

**Create**: `src/clients/storageServiceClient.js`

```javascript
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
```

### **2.2 Cache Service Client Implementation**

**Create**: `src/clients/cacheServiceClient.js`

```javascript
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
```

## 🔐 **PHASE 3: AUTHENTICATION SERVICE REDESIGN (MANDATORY - Week 2)**

### **3.1 Authentication Service with Service Integration**

**Replace**: `src/service/authenticationService.js`

```javascript
import StorageServiceClient from "../clients/storageServiceClient.js";
import CacheServiceClient from "../clients/cacheServiceClient.js";
import passwordService from "./passwordService.js";
import tokenService from "./tokenService.js";
import config from "../config/config.js";

class AuthenticationService {
  constructor() {
    this.storageClient = new StorageServiceClient(config);
    this.cacheClient = new CacheServiceClient(config);
    this.passwordService = passwordService;
    this.tokenService = tokenService;
  }

  async authenticateUser(email, password, req) {
    const startTime = Date.now();

    try {
      console.log(`Authentication attempt for user: ${email}`);

      // 1. Get user data via storage-service
      const user = await this.storageClient.getUserByEmail(email);

      if (!user) {
        await this._recordFailedAttempt(null, email, req, "USER_NOT_FOUND");
        throw new Error("Invalid credentials");
      }

      // 2. Check if account is locked
      if (user.locked_until && new Date(user.locked_until) > new Date()) {
        await this._recordFailedAttempt(user.id, email, req, "ACCOUNT_LOCKED");
        throw new Error("Account is temporarily locked");
      }

      // 3. Check if account is active
      if (!user.is_active) {
        await this._recordFailedAttempt(
          user.id,
          email,
          req,
          "ACCOUNT_INACTIVE"
        );
        throw new Error("Account is inactive");
      }

      // 4. Validate password locally (Argon2)
      const isValid = await this.passwordService.validatePassword(
        password,
        user.hashed_password,
        user.salt
      );

      if (!isValid) {
        await this._recordFailedAttempt(
          user.id,
          email,
          req,
          "INVALID_PASSWORD"
        );
        throw new Error("Invalid credentials");
      }

      // 5. Generate JWT tokens
      const tokens = await this.tokenService.generateTokens(user);

      // 6. Store session in cache (non-blocking)
      const sessionData = {
        userId: user.id,
        email: user.email,
        role: user.role,
        firstName: user.first_name,
        lastName: user.last_name,
        loginTime: new Date().toISOString(),
      };

      this.cacheClient.storeSession(tokens.jti, sessionData, 3600);

      // 7. Record successful login via storage-service (non-blocking)
      this.storageClient.recordLoginAttempt(user.id, req.ip, true);
      this.storageClient.logAuditEvent({
        user_id: user.id,
        action: "LOGIN_SUCCESS",
        ip_address: req.ip,
        user_agent: req.get("User-Agent"),
        success: true,
        details: JSON.stringify({
          loginTime: new Date().toISOString(),
          tokenId: tokens.jti,
        }),
      });

      // Record metrics
      const duration = Date.now() - startTime;
      console.log(`Authentication successful for ${email} in ${duration}ms`);

      return {
        status: "success",
        message: "Authentication successful",
        data: {
          access_token: tokens.accessToken,
          refresh_token: tokens.refreshToken,
          token_type: "bearer",
          expires_in: 3600,
          user: {
            id: user.id,
            email: user.email,
            firstName: user.first_name,
            lastName: user.last_name,
            role: user.role,
          },
        },
      };
    } catch (error) {
      const duration = Date.now() - startTime;
      console.error(
        `Authentication failed for ${email} in ${duration}ms:`,
        error.message
      );
      throw error;
    }
  }

  async validateToken(token) {
    try {
      // 1. Verify JWT token locally
      const decoded = await this.tokenService.verifyToken(token);

      // 2. Check if token is blacklisted (non-blocking)
      const isBlacklisted = await this.cacheClient.isTokenBlacklisted(
        decoded.jti
      );
      if (isBlacklisted) {
        throw new Error("Token has been revoked");
      }

      // 3. Get session data from cache (optional)
      const sessionData = await this.cacheClient.getSession(decoded.jti);

      return {
        valid: true,
        user: {
          id: decoded.userId,
          email: decoded.email,
          role: decoded.role,
          firstName: decoded.firstName,
          lastName: decoded.lastName,
        },
        session: sessionData,
      };
    } catch (error) {
      console.error("Token validation failed:", error.message);
      return {
        valid: false,
        error: error.message,
      };
    }
  }

  async refreshToken(refreshToken) {
    try {
      // 1. Verify refresh token
      const decoded = await this.tokenService.verifyRefreshToken(refreshToken);

      // 2. Get user data via storage-service
      const user = await this.storageClient.getUserById(decoded.userId);

      if (!user || !user.is_active) {
        throw new Error("User not found or inactive");
      }

      // 3. Generate new tokens
      const tokens = await this.tokenService.generateTokens(user);

      // 4. Blacklist old token
      await this.cacheClient.blacklistToken(decoded.jti, 3600);

      // 5. Store new session
      const sessionData = {
        userId: user.id,
        email: user.email,
        role: user.role,
        refreshTime: new Date().toISOString(),
      };

      this.cacheClient.storeSession(tokens.jti, sessionData, 3600);

      return {
        status: "success",
        message: "Token refreshed successfully",
        data: {
          access_token: tokens.accessToken,
          refresh_token: tokens.refreshToken,
          token_type: "bearer",
          expires_in: 3600,
        },
      };
    } catch (error) {
      console.error("Token refresh failed:", error.message);
      throw error;
    }
  }

  async logout(token) {
    try {
      const decoded = await this.tokenService.verifyToken(token);

      // Blacklist token
      await this.cacheClient.blacklistToken(decoded.jti, 3600);

      // Invalidate session
      await this.cacheClient.invalidateSession(decoded.jti);

      // Log audit event
      this.storageClient.logAuditEvent({
        user_id: decoded.userId,
        action: "LOGOUT",
        ip_address: "unknown", // Will need to be passed from request
        success: true,
        details: JSON.stringify({
          logoutTime: new Date().toISOString(),
          tokenId: decoded.jti,
        }),
      });

      return {
        status: "success",
        message: "Logged out successfully",
      };
    } catch (error) {
      console.error("Logout failed:", error.message);
      throw error;
    }
  }

  // Private method for recording failed attempts
  async _recordFailedAttempt(userId, email, req, reason) {
    const auditData = {
      user_id: userId,
      action: "LOGIN_FAILED",
      ip_address: req.ip,
      user_agent: req.get("User-Agent"),
      success: false,
      details: JSON.stringify({
        email: email,
        reason: reason,
        timestamp: new Date().toISOString(),
      }),
    };

    // Record via storage-service (non-blocking)
    this.storageClient.logAuditEvent(auditData);

    if (userId) {
      this.storageClient.recordLoginAttempt(userId, req.ip, false);
    }
  }
}

export default new AuthenticationService();
```

## 🌐 **PHASE 4: API ROUTES WITH AUTH-SERVICE-OLD COMPATIBILITY (MANDATORY - Week 3)**

### **4.1 V1 Authentication Routes**

**Update**: `src/routes/authV1Routes.js`

```javascript
import express from "express";
import authenticationService from "../service/authenticationService.js";
import rateLimit from "express-rate-limit";

const router = express.Router();

// Rate limiting for auth endpoints
const authRateLimit = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 5, // 5 attempts per window
  message: {
    status: "error",
    message: "Too many authentication attempts, please try again later",
  },
  standardHeaders: true,
  legacyHeaders: false,
});

// POST /v1/auth/login - Compatible with auth-service-old
router.post("/login", authRateLimit, async (req, res) => {
  try {
    const { user_id, password } = req.body; // Note: user_id is email for compatibility

    if (!user_id || !password) {
      return res.status(400).json({
        status: "error",
        message: "Email and password are required",
      });
    }

    const result = await authenticationService.authenticateUser(
      user_id,
      password,
      req
    );
    res.json(result);
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: error.message,
    });
  }
});

// POST /v1/auth/token/validate - Compatible with auth-service-old
router.post("/token/validate", async (req, res) => {
  try {
    const token = req.headers.authorization?.split(" ")[1] || req.body.token;

    if (!token) {
      return res.status(400).json({
        status: "error",
        message: "Token is required",
      });
    }

    const validation = await authenticationService.validateToken(token);

    if (validation.valid) {
      res.json({
        status: "success",
        message: "Token is valid",
        data: {
          valid: true,
          user: validation.user,
        },
      });
    } else {
      res.status(401).json({
        status: "error",
        message: "Invalid token",
        data: {
          valid: false,
        },
      });
    }
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: "Invalid token",
      data: {
        valid: false,
      },
    });
  }
});

// POST /v1/auth/token/refresh - Token refresh endpoint
router.post("/token/refresh", async (req, res) => {
  try {
    const { refresh_token } = req.body;

    if (!refresh_token) {
      return res.status(400).json({
        status: "error",
        message: "Refresh token is required",
      });
    }

    const result = await authenticationService.refreshToken(refresh_token);
    res.json(result);
  } catch (error) {
    res.status(401).json({
      status: "error",
      message: error.message,
    });
  }
});

// POST /v1/auth/logout - Logout endpoint
router.post("/logout", async (req, res) => {
  try {
    const token = req.headers.authorization?.split(" ")[1];

    if (!token) {
      return res.status(400).json({
        status: "error",
        message: "Token is required",
      });
    }

    const result = await authenticationService.logout(token);
    res.json(result);
  } catch (error) {
    res.status(400).json({
      status: "error",
      message: error.message,
    });
  }
});

export default router;
```

### **4.2 User Management Routes**

**Create**: `src/routes/userV1Routes.js`

```javascript
import express from "express";
import authenticationService from "../service/authenticationService.js";
import { requiresAuth } from "../middleware/authMiddleware.js";

const router = express.Router();

// GET /v1/users/me - Get current user profile (auth-service-old compatible)
router.get("/me", requiresAuth(), async (req, res) => {
  try {
    const user = req.user; // Set by auth middleware

    res.json({
      status: "success",
      message: "User profile retrieved",
      data: {
        user: {
          id: user.id,
          email: user.email,
          firstName: user.firstName,
          lastName: user.lastName,
          role: user.role,
        },
      },
    });
  } catch (error) {
    res.status(500).json({
      status: "error",
      message: "Failed to retrieve user profile",
    });
  }
});

// GET /v1/users/{id} - Get user by ID (requires admin role)
router.get("/:id", requiresAuth(["admin"]), async (req, res) => {
  try {
    const { id } = req.params;

    // This would require extending storage-service client
    // For now, return current user if requesting own profile
    if (id === req.user.id) {
      return res.json({
        status: "success",
        message: "User profile retrieved",
        data: {
          user: req.user,
        },
      });
    }

    res.status(403).json({
      status: "error",
      message: "Access denied",
    });
  } catch (error) {
    res.status(500).json({
      status: "error",
      message: "Failed to retrieve user profile",
    });
  }
});

export default router;
```

## 🏥 **PHASE 5: HEALTH CHECKS AND MONITORING (MANDATORY - Week 3)**

### **5.1 Health Check Service**

**Create**: `src/service/healthService.js`

```javascript
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
```

### **5.2 Health Check Routes**

**Create**: `src/routes/healthRoutes.js`

```javascript
import express from "express";
import healthService from "../service/healthService.js";

const router = express.Router();

// GET /health - Comprehensive health check with dependencies
router.get("/health", async (req, res) => {
  try {
    const health = await healthService.checkHealth();
    const statusCode = health.status === "healthy" ? 200 : 503;
    res.status(statusCode).json(health);
  } catch (error) {
    res.status(503).json({
      status: "unhealthy",
      timestamp: new Date().toISOString(),
      error: error.message,
    });
  }
});

// GET /ready - Readiness probe for Kubernetes
router.get("/ready", async (req, res) => {
  try {
    const readiness = await healthService.checkReadiness();
    const statusCode = readiness.status === "ready" ? 200 : 503;
    res.status(statusCode).json(readiness);
  } catch (error) {
    res.status(503).json({
      status: "not ready",
      timestamp: new Date().toISOString(),
      error: error.message,
    });
  }
});

// GET /live - Liveness probe for Kubernetes
router.get("/live", (req, res) => {
  const liveness = healthService.checkLiveness();
  res.status(200).json(liveness);
});

export default router;
```

### **5.3 Prometheus Metrics**

**Create**: `src/service/metricsService.js`

```javascript
import promClient from "prom-client";

class MetricsService {
  constructor() {
    // Create metrics registry
    this.register = new promClient.Registry();

    // Add default metrics
    promClient.collectDefaultMetrics({ register: this.register });

    // Custom auth metrics
    this.authAttempts = new promClient.Counter({
      name: "auth_attempts_total",
      help: "Total number of authentication attempts",
      labelNames: ["status", "method"],
      registers: [this.register],
    });

    this.authDuration = new promClient.Histogram({
      name: "auth_duration_seconds",
      help: "Authentication request duration",
      labelNames: ["method", "status"],
      buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5],
      registers: [this.register],
    });

    this.serviceIntegrationDuration = new promClient.Histogram({
      name: "auth_service_integration_duration_seconds",
      help: "Duration of service integration calls",
      labelNames: ["service", "operation", "status"],
      buckets: [0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5],
      registers: [this.register],
    });

    this.activeTokens = new promClient.Gauge({
      name: "auth_active_tokens_total",
      help: "Number of active JWT tokens",
      registers: [this.register],
    });

    this.circuitBreakerState = new promClient.Gauge({
      name: "auth_circuit_breaker_state",
      help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
      labelNames: ["service", "operation"],
      registers: [this.register],
    });
  }

  recordAuthAttempt(status, method = "password") {
    this.authAttempts.inc({ status, method });
  }

  recordAuthDuration(duration, method = "password", status = "success") {
    this.authDuration.observe({ method, status }, duration / 1000);
  }

  recordServiceIntegration(service, operation, duration, status = "success") {
    this.serviceIntegrationDuration.observe(
      { service, operation, status },
      duration / 1000
    );
  }

  updateCircuitBreakerState(service, operation, state) {
    // 0=closed, 1=open, 2=half-open
    const stateValue = state === "closed" ? 0 : state === "open" ? 1 : 2;
    this.circuitBreakerState.set({ service, operation }, stateValue);
  }

  getMetrics() {
    return this.register.metrics();
  }
}

export default new MetricsService();
```

## 🚀 **PHASE 6: DEPLOYMENT CONFIGURATION (MANDATORY - Week 4)**

### **6.1 Environment Variables**

```yaml
# ✅ REQUIRED: Service integration
- name: STORAGE_SERVICE_URL
  value: "http://storage-service:8080"
- name: CACHE_SERVICE_URL
  value: "http://cache-service:8080"
- name: SERVICE_TIMEOUT
  value: "5000"
- name: SERVICE_RETRIES
  value: "3"

# ✅ REQUIRED: Circuit breaker configuration
- name: CIRCUIT_BREAKER_TIMEOUT
  value: "3000"
- name: CIRCUIT_BREAKER_ERROR_THRESHOLD
  value: "50"
- name: CIRCUIT_BREAKER_RESET_TIMEOUT
  value: "30000"

# ✅ REQUIRED: JWT configuration
- name: JWT_PRIVATE_KEY_SECRET
  valueFrom:
    secretKeyRef:
      name: auth-service-secrets
      key: jwt-private-key
- name: JWT_PUBLIC_KEY_SECRET
  valueFrom:
    secretKeyRef:
      name: auth-service-secrets
      key: jwt-public-key

# ✅ REQUIRED: Security configuration
- name: RATE_LIMIT_WINDOW_MS
  value: "900000" # 15 minutes
- name: RATE_LIMIT_MAX_REQUESTS
  value: "5"
- name: ACCOUNT_LOCKOUT_ATTEMPTS
  value: "5"
- name: ACCOUNT_LOCKOUT_DURATION_MS
  value: "1800000" # 30 minutes

# ❌ REMOVE: Database configuration
# - name: DATABASE_URL
#   value: "postgresql://..."

# ❌ REMOVE: AWS configuration
# - name: AWS_REGION
#   value: "us-east-1"
# - name: AWS_XRAY_ENABLED
#   value: "false"
```

### **6.2 Updated Server Configuration**

**Update**: `src/server.js`

```javascript
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
  res.set("Content-Type", promClient.register.contentType);
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
```

## 📋 **IMPLEMENTATION CHECKLIST**

### **Week 1: Code Cleanup ✅**

- [ ] Remove all Prisma dependencies and database files
- [ ] Remove all AWS dependencies and cloud-specific code
- [ ] Remove all legacy tweet functionality
- [ ] Update package.json dependencies
- [ ] Update configuration for service integration

### **Week 2: Service Integration ✅**

- [ ] Implement StorageServiceClient with circuit breakers
- [ ] Implement CacheServiceClient with circuit breakers
- [ ] Redesign AuthenticationService for service integration
- [ ] Update all authentication flows to use service clients
- [ ] Add comprehensive error handling and logging

### **Week 3: API and Health Checks ✅**

- [ ] Update V1 auth routes for auth-service-old compatibility
- [ ] Implement user management routes
- [ ] Create comprehensive health check service
- [ ] Add Prometheus metrics collection
- [ ] Test all API endpoints with service integration

### **Week 4: Deployment and Testing ✅**

- [ ] Create Kubernetes deployment manifests
- [ ] Configure environment variables and secrets
- [ ] Deploy to kind cluster for testing
- [ ] Validate integration with storage-service and cache-service
- [ ] Performance testing and optimization

## 🎯 **SUCCESS CRITERIA**

### **Functional Requirements**

- [ ] **No Direct Database Access**: All data operations via storage-service HTTP API
- [ ] **No AWS Dependencies**: Pure Kubernetes microservices architecture
- [ ] **No Legacy Code**: Only authentication functionality remains
- [ ] **Service Integration**: Proper HTTP clients with circuit breakers
- [ ] **API Compatibility**: 100% compatible with auth-service-old endpoints

### **Performance Requirements**

- [ ] **Authentication**: < 200ms (95th percentile) including service calls
- [ ] **Token Validation**: < 50ms (95th percentile) with cache integration
- [ ] **Circuit Breaker**: < 3s timeout, 50% error threshold
- [ ] **Service Resilience**: Graceful degradation when dependencies fail

### **Architecture Requirements**

- [ ] **Microservices Compliance**: HTTP-based service communication only
- [ ] **Service Discovery**: Proper service URL configuration
- [ ] **Observability**: Prometheus metrics and structured logging
- [ ] **Deployment**: Kubernetes-native with proper resource management

## 🚨 **CRITICAL IMPLEMENTATION NOTES**

1. **NO DATABASE ACCESS**: Auth-service must NEVER connect directly to any database
2. **SERVICE INTEGRATION ONLY**: All data operations via HTTP calls to other services
3. **CIRCUIT BREAKER MANDATORY**: All service calls must use circuit breaker patterns
4. **FAIL GRACEFULLY**: Cache failures should not block authentication
5. **AUDIT EVERYTHING**: All authentication events must be logged via storage-service

---

**Implementation Status**: READY FOR MICROSERVICES INTEGRATION  
**Architecture**: Thin orchestration layer with HTTP service integration  
**Timeline**: 4 weeks for complete architectural realignment  
**Dependencies**: Storage-service auth extension, cache-service HTTP API

**Next Action**: Begin Phase 1 code cleanup immediately - remove all database and AWS dependencies
