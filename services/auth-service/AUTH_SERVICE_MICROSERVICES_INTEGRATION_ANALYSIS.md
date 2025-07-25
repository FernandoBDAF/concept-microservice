# Auth Service Microservices Integration Analysis (REVISED)

## Executive Summary

**Document Purpose**: CRITICAL architectural correction for auth-service microservices integration  
**Current Problem**: Implementation violates microservices principles with direct database access  
**Required Action**: Complete architectural realignment with storage-service integration  
**Priority**: BLOCKING - Foundation service must be corrected before ecosystem integration

## 🚨 **CRITICAL ARCHITECTURAL VIOLATIONS IDENTIFIED**

### **Problem 1: Direct Database Access (ANTI-PATTERN)**

**❌ Current Violation**: Auth-service directly connects to PostgreSQL via Prisma

```javascript
// ❌ WRONG: Direct database access violates microservices architecture
import { PrismaClient } from "@prisma/client";
import prisma from "../prismaClient.js";

class UserService {
  async findById(id) {
    return await prisma.user.findUnique({
      // DIRECT DB ACCESS
      where: { id },
    });
  }
}
```

**✅ Required Pattern**: Auth-service MUST use storage-service for all data operations

```javascript
// ✅ CORRECT: HTTP client to storage-service
class StorageServiceClient {
  async getUserById(id) {
    const response = await this.httpClient.get(`/api/v1/auth/users/${id}`);
    return response.data;
  }
}
```

### **Problem 2: Unnecessary Cloud Dependencies (BLOAT)**

**❌ Current Violation**: AWS-specific code in microservices environment

```javascript
// ❌ WRONG: AWS X-Ray in Kubernetes microservices
import AWSXRay from "aws-xray-sdk";
import { capturePrisma } from "@cosva-lab/aws-xray-sdk-prisma";

// ❌ WRONG: AWS S3 dependencies for image upload
import { S3Client } from "@aws-sdk/client-s3";
```

**✅ Required Pattern**: Kubernetes-native observability and storage

```javascript
// ✅ CORRECT: Prometheus metrics for Kubernetes
import promClient from "prom-client";

// ✅ CORRECT: Local storage or dedicated storage service
// No AWS dependencies in microservices
```

### **Problem 3: Legacy Application Code (TECHNICAL DEBT)**

**❌ Current Violation**: Tweet functionality in authentication service

```javascript
// ❌ WRONG: Tweet functionality in auth-service
class TweetService {
  async createTweet(author, text, imgUrl) {
    return await prisma.tweet.create({
      // WRONG DOMAIN
      data: { author, text, imgUrl },
    });
  }
}
```

**✅ Required Pattern**: Single responsibility - authentication only

```javascript
// ✅ CORRECT: Authentication-only functionality
class AuthenticationService {
  async authenticateUser(email, password) {
    // Only authentication logic
  }
}
```

## 🎯 **REQUIRED ARCHITECTURAL CORRECTIONS**

### **Correction 1: Storage-Service Integration (MANDATORY)**

#### **Replace Direct Database Access**

**REMOVE**: All Prisma-based data operations
**REPLACE WITH**: HTTP clients to storage-service

```javascript
// ✅ CORRECT: Storage-service client implementation
class StorageServiceClient {
  constructor() {
    this.baseURL =
      process.env.STORAGE_SERVICE_URL || "http://storage-service:8080";
    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: 5000,
      headers: {
        "Content-Type": "application/json",
        "X-Service": "auth-service",
      },
    });
  }

  // User operations via storage-service
  async getUserByEmail(email) {
    const response = await this.httpClient.get(
      `/api/v1/auth/users/email/${email}`
    );
    return response.data;
  }

  async createUser(userData) {
    const response = await this.httpClient.post("/api/v1/auth/users", userData);
    return response.data;
  }

  async updateUser(userId, userData) {
    const response = await this.httpClient.put(
      `/api/v1/auth/users/${userId}`,
      userData
    );
    return response.data;
  }

  // Audit logging via storage-service
  async logAuditEvent(auditData) {
    return this.httpClient
      .post("/api/v1/auth/audit", auditData)
      .catch((err) => console.error("Audit logging failed:", err.message));
  }

  // Role management via storage-service
  async getUserRoles(userId) {
    const response = await this.httpClient.get(
      `/api/v1/auth/users/${userId}/roles`
    );
    return response.data;
  }
}
```

#### **Circuit Breaker Implementation**

```javascript
// ✅ CORRECT: Circuit breaker for storage-service integration
import CircuitBreaker from "opossum";

class AuthenticationService {
  constructor() {
    this.storageClient = new StorageServiceClient();

    // Circuit breaker for critical auth operations
    this.getUserBreaker = new CircuitBreaker(
      this.storageClient.getUserByEmail.bind(this.storageClient),
      {
        timeout: 3000,
        errorThresholdPercentage: 50,
        resetTimeout: 30000,
        name: "storage-getUserByEmail",
      }
    );
  }

  async authenticateUser(email, password, req) {
    try {
      // Get user via storage-service with circuit breaker
      const user = await this.getUserBreaker.fire(email);

      // Validate password locally (Argon2)
      const isValid = await this.validatePassword(
        password,
        user.hashed_password,
        user.salt
      );

      if (!isValid) {
        // Audit failed attempt via storage-service
        await this.storageClient.logAuditEvent({
          user_id: user.id,
          action: "LOGIN_FAILED",
          ip_address: req.ip,
          success: false,
        });
        throw new Error("Invalid credentials");
      }

      // Generate JWT token (local operation)
      const token = await this.generateJWT(user);

      // Store session via cache-service
      await this.cacheClient.storeSession(token.jti, {
        userId: user.id,
        email: user.email,
        role: user.role,
      });

      return token;
    } catch (error) {
      console.error("Authentication failed:", error.message);
      throw error;
    }
  }
}
```

### **Correction 2: Cache-Service Integration (MANDATORY)**

#### **Session Management via Cache-Service**

```javascript
// ✅ CORRECT: Cache-service client for session management
class CacheServiceClient {
  constructor() {
    this.baseURL = process.env.CACHE_SERVICE_URL || "http://cache-service:8080";
    this.httpClient = axios.create({
      baseURL: this.baseURL,
      timeout: 3000,
    });
  }

  async storeSession(sessionId, sessionData, ttl = 3600) {
    return await this.httpClient.post(`/api/v1/cache/session:${sessionId}`, {
      value: sessionData,
      ttl: ttl,
    });
  }

  async getSession(sessionId) {
    const response = await this.httpClient.get(
      `/api/v1/cache/session:${sessionId}`
    );
    return response.data;
  }

  async invalidateSession(sessionId) {
    await this.httpClient.delete(`/api/v1/cache/session:${sessionId}`);
  }

  async blacklistToken(tokenId, ttl) {
    return await this.httpClient.post(`/api/v1/cache/blacklist:${tokenId}`, {
      value: "blacklisted",
      ttl: ttl,
    });
  }
}
```

### **Correction 3: Remove Cloud Dependencies (MANDATORY)**

#### **Replace AWS X-Ray with Prometheus**

```javascript
// ❌ REMOVE: AWS X-Ray dependencies
// import AWSXRay from "aws-xray-sdk";
// import { capturePrisma } from "@cosva-lab/aws-xray-sdk-prisma";

// ✅ ADD: Prometheus metrics for Kubernetes
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
      labelNames: ["method"],
      registers: [this.register],
    });
  }

  recordAuthAttempt(status, method = "password") {
    this.authAttempts.inc({ status, method });
  }

  recordAuthDuration(duration, method = "password") {
    this.authDuration.observe({ method }, duration);
  }

  getMetrics() {
    return this.register.metrics();
  }
}
```

## 🗑️ **CODE CLEANUP REQUIREMENTS**

### **Files to COMPLETELY REMOVE**

```bash
# 1. Database schema and migrations (use storage-service instead)
rm -rf prisma/
rm src/prismaClient.js

# 2. AWS-specific services
rm src/service/secretsService.js
rm src/middleware/uploadImageToS3Middleware.js

# 3. Legacy tweet functionality
rm src/service/tweetService.js
rm src/routes/tweetRoutes.js

# 4. Direct database services
rm src/service/userService.js      # Replace with storage-service client
rm src/service/sessionService.js   # Replace with cache-service client
rm src/service/auditService.js     # Replace with storage-service client
```

### **Dependencies to REMOVE from package.json**

```json
{
  "dependencies": {
    // ❌ REMOVE: AWS dependencies
    // "@aws-sdk/client-s3": "^3.325.0",
    // "@aws-sdk/client-secrets-manager": "^3.338.0",
    // "@cosva-lab/aws-xray-sdk-prisma": "^0.0.7",
    // "aws-xray-sdk": "^3.5.0",

    // ❌ REMOVE: Database dependencies (use storage-service)
    // "@prisma/client": "^4.13.0",
    // "prisma": "^4.13.0",

    // ❌ REMOVE: File upload dependencies
    // "multer": "^1.4.5-lts.1",
    // "multer-s3": "^3.0.1",

    // ❌ REMOVE: Duplicate password hashing
    // "bcrypt": "^5.0.1",  // Keep only argon2

    // ✅ KEEP: Essential dependencies
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

### **Configuration to REMOVE/UPDATE**

```javascript
// ❌ REMOVE: AWS configuration
// this.aws = {
//   region: process.env.AWS_REGION || "us-east-1",
//   xrayEnabled: process.env.AWS_XRAY_ENABLED === "true",
// };

// ❌ REMOVE: Database configuration
// this.database = {
//   url: process.env.DATABASE_URL,
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

## 🏗️ **REQUIRED SERVICE ARCHITECTURE**

### **Correct Auth-Service Architecture**

```
┌─────────────────────────────────────────────────────────────────┐
│                        Auth Service                             │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐   │
│  │  HTTP API       │ │ Authentication  │ │ Integration     │   │
│  │                 │ │                 │ │                 │   │
│  │ • POST /login   │ │ • JWT Generate  │ │ • Storage Client│   │
│  │ • POST /validate│ │ • Password Hash │ │ • Cache Client  │   │
│  │ • POST /refresh │ │ • Token Verify  │ │ • Circuit Break │   │
│  │ • GET /health   │ │ • Rate Limiting │ │ • Health Checks │   │
│  └─────────────────┘ └─────────────────┘ └─────────────────┘   │
└─────────────────────────┬─────────────────┬─────────────────────┘
                          │                 │
          ┌───────────────┘                 └─────────────────┐
          │ HTTP Requests                   HTTP Requests     │
          ▼                                                   ▼
┌─────────────────────────────────┐ ┌─────────────────────────────────┐
│       Storage Service           │ │        Cache Service            │
│                                 │ │                                 │
│ • POST /api/v1/auth/users       │ │ • POST /api/v1/cache/session:*  │
│ • GET /api/v1/auth/users/email  │ │ • GET /api/v1/cache/session:*   │
│ • POST /api/v1/auth/audit       │ │ • DELETE /api/v1/cache/session:*│
│ • GET /api/v1/auth/roles        │ │ • POST /api/v1/cache/blacklist:*│
└─────────────────────────────────┘ └─────────────────────────────────┘
```

### **Required Service Integration Clients**

#### **1. Storage Service Client (MANDATORY)**

```javascript
// src/clients/storageServiceClient.js
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

    // Circuit breakers for different operations
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
  }

  // User operations
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

  // Audit operations (non-blocking)
  async logAuditEvent(auditData) {
    return await this.auditBreaker
      .fire("logAuditEvent", auditData)
      .catch((err) => {
        console.error("Audit logging failed:", err.message);
        // Don't throw - audit logging should not block auth operations
      });
  }

  // Private methods for circuit breaker
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

#### **2. Cache Service Client (MANDATORY)**

```javascript
// src/clients/cacheServiceClient.js
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
  }

  // Session operations
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

  // Token blacklist operations
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

## 🚀 **DEPLOYMENT REQUIREMENTS**

### **Environment Variables (MANDATORY)**

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

### **Health Check Requirements**

```javascript
// src/routes/healthRoutes.js
import express from "express";

const router = express.Router();

// Health check with dependency validation
router.get("/health", async (req, res) => {
  const health = {
    status: "healthy",
    timestamp: new Date().toISOString(),
    service: "auth-service",
    version: "1.0.0",
    dependencies: {},
  };

  // Check storage-service
  try {
    const storageHealthy = await req.app.locals.storageClient.healthCheck();
    health.dependencies.storage = storageHealthy ? "healthy" : "unhealthy";
    if (!storageHealthy) health.status = "degraded";
  } catch (error) {
    health.dependencies.storage = "unhealthy";
    health.status = "degraded";
  }

  // Check cache-service
  try {
    const cacheHealthy = await req.app.locals.cacheClient.healthCheck();
    health.dependencies.cache = cacheHealthy ? "healthy" : "unhealthy";
    if (!cacheHealthy) health.status = "degraded";
  } catch (error) {
    health.dependencies.cache = "unhealthy";
    health.status = "degraded";
  }

  const statusCode = health.status === "healthy" ? 200 : 503;
  res.status(statusCode).json(health);
});

// Readiness check (same as health for auth-service)
router.get("/ready", async (req, res) => {
  // For auth-service, ready when dependencies are available
  const health = await req.app.locals.healthCheck();
  const statusCode = health.status === "healthy" ? 200 : 503;
  res.status(statusCode).json(health);
});

// Liveness check (basic service functionality)
router.get("/live", (req, res) => {
  res.status(200).json({
    status: "alive",
    timestamp: new Date().toISOString(),
    uptime: process.uptime(),
  });
});

export default router;
```

## 📋 **IMPLEMENTATION CHECKLIST**

### **Phase 1: Code Cleanup (Week 1)**

- [ ] **Remove Prisma dependencies and database code**

  - [ ] Delete `prisma/` directory
  - [ ] Remove `src/prismaClient.js`
  - [ ] Remove database-related scripts from `package.json`
  - [ ] Remove `@prisma/client` and `prisma` from dependencies

- [ ] **Remove AWS dependencies**

  - [ ] Remove AWS SDK packages from `package.json`
  - [ ] Remove `aws-xray-sdk` and related packages
  - [ ] Delete `src/service/secretsService.js`
  - [ ] Delete `src/middleware/uploadImageToS3Middleware.js`
  - [ ] Remove AWS configuration from `config.js`

- [ ] **Remove legacy application code**
  - [ ] Delete `src/service/tweetService.js`
  - [ ] Delete tweet-related routes
  - [ ] Remove Tweet model references
  - [ ] Clean up unused imports

### **Phase 2: Service Integration (Week 2)**

- [ ] **Implement Storage Service Client**

  - [ ] Create `src/clients/storageServiceClient.js`
  - [ ] Implement circuit breaker patterns
  - [ ] Add comprehensive error handling
  - [ ] Add health check integration

- [ ] **Implement Cache Service Client**

  - [ ] Create `src/clients/cacheServiceClient.js`
  - [ ] Implement session management
  - [ ] Add token blacklist functionality
  - [ ] Add circuit breaker protection

- [ ] **Update Authentication Service**
  - [ ] Replace direct database calls with service clients
  - [ ] Implement proper error handling
  - [ ] Add comprehensive logging
  - [ ] Update JWT token management

### **Phase 3: Configuration and Deployment (Week 3)**

- [ ] **Update Configuration**

  - [ ] Remove database configuration
  - [ ] Add service integration configuration
  - [ ] Add circuit breaker configuration
  - [ ] Update environment variable handling

- [ ] **Update Health Checks**

  - [ ] Implement dependency health checks
  - [ ] Add service availability monitoring
  - [ ] Update readiness and liveness probes
  - [ ] Add metrics collection

- [ ] **Create Deployment Manifests**
  - [ ] Follow `MICROSERVICES_DEPLOYMENT_STANDARD.md`
  - [ ] Create Kubernetes manifests
  - [ ] Add service dependencies
  - [ ] Configure proper resource limits

### **Phase 4: Testing and Validation (Week 4)**

- [ ] **Integration Testing**

  - [ ] Test storage-service integration
  - [ ] Test cache-service integration
  - [ ] Test circuit breaker functionality
  - [ ] Test health check endpoints

- [ ] **Performance Testing**

  - [ ] Validate authentication latency < 200ms
  - [ ] Test circuit breaker behavior
  - [ ] Validate service resilience
  - [ ] Test concurrent authentication

- [ ] **Security Testing**
  - [ ] Test JWT token generation and validation
  - [ ] Test rate limiting functionality
  - [ ] Test account lockout behavior
  - [ ] Validate audit logging

## 🎯 **SUCCESS CRITERIA**

### **Functional Requirements**

- [ ] **No Direct Database Access**: All data operations via storage-service
- [ ] **No AWS Dependencies**: Pure Kubernetes microservices architecture
- [ ] **No Legacy Code**: Only authentication functionality remains
- [ ] **Service Integration**: Proper HTTP clients with circuit breakers
- [ ] **Health Checks**: Comprehensive dependency monitoring

### **Performance Requirements**

- [ ] **Authentication**: < 200ms (95th percentile)
- [ ] **Token Validation**: < 50ms (95th percentile)
- [ ] **Circuit Breaker**: < 3s timeout, 50% error threshold
- [ ] **Service Resilience**: Graceful degradation when dependencies fail

### **Architecture Requirements**

- [ ] **Microservices Compliance**: No direct database access
- [ ] **Service Discovery**: HTTP-based service communication
- [ ] **Observability**: Prometheus metrics, structured logging
- [ ] **Deployment**: Kubernetes-native with proper resource management

## 🚨 **CRITICAL NEXT STEPS**

### **Immediate Actions Required**

1. **STOP current implementation** - It violates microservices architecture
2. **Remove all database code** - Use storage-service instead
3. **Remove all AWS code** - Use Kubernetes-native patterns
4. **Remove all legacy code** - Focus only on authentication

### **Implementation Priority**

1. **Week 1**: Code cleanup and dependency removal
2. **Week 2**: Service client implementation
3. **Week 3**: Configuration and deployment
4. **Week 4**: Testing and validation

### **Architecture Validation**

Before proceeding, ensure:

- ✅ Storage-service has auth data models implemented
- ✅ Cache-service HTTP API is functional
- ✅ Kubernetes cluster is ready for deployment
- ✅ All team members understand microservices architecture

---

**Document Status**: CRITICAL ARCHITECTURAL CORRECTION REQUIRED  
**Implementation Priority**: BLOCKING - Must be completed before ecosystem integration  
**Estimated Timeline**: 4 weeks for complete architectural realignment  
**Dependencies**: Storage-service auth extension, cache-service HTTP API

**Next Action**: Begin immediate code cleanup and dependency removal as outlined in Phase 1 checklist.
