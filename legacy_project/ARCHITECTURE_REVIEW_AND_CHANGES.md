# Architecture Review and Required Changes

**Date**: December 29, 2024  
**Purpose**: Address architectural inconsistencies in microservices integration  
**Status**: 🔴 **CRITICAL CHANGES REQUIRED**

---

## 🚨 **Critical Issues Identified**

### **1. Auth Service ↔ Cache Service Integration (UNNECESSARY)**

**Current Problem**:

- Auth service integrates with cache service for session management
- This violates service boundaries and adds unnecessary complexity
- Auth service should be stateless and not depend on cache service

**Evidence from Documentation**:

```javascript
// From auth-service/CONTEXT.md - UNNECESSARY INTEGRATION
class CacheServiceClient {
  async storeSession(sessionId, sessionData, ttl = 3600) {
    // This should NOT exist in auth service
  }

  async getSession(sessionId) {
    // This should NOT exist in auth service
  }
}
```

**Required Change**: Remove cache service integration from auth service

### **2. User Data Ownership (INCORRECT)**

**Current Problem**:

- Storage service owns user data and provides auth endpoints
- Auth service depends on storage service for user data
- This violates the principle that auth service should own user data

**Evidence from Documentation**:

```go
// From storage-service/internal/api/rest/auth.go - WRONG OWNERSHIP
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // This should be in auth service, not storage service
}

func (h *AuthHandler) GetUserByEmail(w http.ResponseWriter, r *http.Request) {
    // This should be in auth service, not storage service
}
```

**Required Change**: Move user data ownership to auth service

### **3. Profile Service User Access (INCORRECT)**

**Current Problem**:

- Profile service should connect to auth service for user data
- Currently profile service may access user data through storage service
- This violates service boundaries

**Required Change**: Profile service should only access user data through auth service

---

## 🏗️ **Proposed Architecture Changes**

### **New Service Boundaries**

```
                    🔐 JWT Authentication Flow
                            ↓
Client Applications → Profile Service (API Gateway)
                            ↓
        ┌─────────────────────┼─────────────────────┐
        ↓                     ↓                     ↓
   Auth Service          Cache Service         Storage Service
   (Node.js)               (Go)                   (Go)
        ↓                     ↓                     ↓
   JWT Tokens           HTTP Cache API         Profile Data
   User Management      Session Mgmt           (NO USER DATA)
        ↓                     ↓                     ↓
        └─────────────────────┼─────────────────────┘
                            ↓
                    Queue Service (Go)
                            ↓
                      RabbitMQ Broker
                            ↓
                ┌─────────────┼─────────────┐
                ↓             ↓             ↓
         profile.task    email.send   image.process
                ↓             ↓             ↓
        Profile Worker  Email Worker  Image Worker
                ↓             ↓             ↓
            Multi-Worker Service (Go)
```

### **Service Responsibility Matrix**

| Service             | Current Responsibilities                | New Responsibilities                                |
| ------------------- | --------------------------------------- | --------------------------------------------------- |
| **Auth Service**    | JWT tokens, session mgmt via cache      | JWT tokens, user management, user data ownership    |
| **Cache Service**   | Profile caching, session storage        | Profile caching only (no session storage)           |
| **Storage Service** | Profile data, user data, auth endpoints | Profile data only (no user data, no auth endpoints) |
| **Profile Service** | Orchestration, auth validation          | Orchestration, auth validation via auth service     |

---

## 🔧 **Required Implementation Changes**

### **1. Auth Service Changes**

#### **Remove Cache Service Integration**

```javascript
// REMOVE from auth-service/src/service/cacheServiceClient.js
// REMOVE from auth-service/src/service/authenticationService.js

// NEW: Auth service should be stateless
class AuthService {
  constructor() {
    this.storageClient = new StorageServiceClient(); // For user data only
    // NO cache service integration
  }
}
```

#### **Add User Data Ownership**

```javascript
// ADD to auth-service/src/models/User.js
class User {
  constructor(id, email, hashedPassword, role, isActive) {
    this.id = id;
    this.email = email;
    this.hashedPassword = hashedPassword;
    this.role = role;
    this.isActive = isActive;
  }
}

// ADD to auth-service/src/repository/UserRepository.js
class UserRepository {
  async createUser(userData) {
    // Direct database access for user management
  }

  async getUserByEmail(email) {
    // Direct database access for user lookup
  }

  async updateUser(userId, userData) {
    // Direct database access for user updates
  }
}
```

#### **Update Auth Service Endpoints**

```javascript
// ADD to auth-service/src/routes/auth.js
router.post("/api/v1/auth/users", createUser);
router.get("/api/v1/auth/users/:id", getUser);
router.get("/api/v1/auth/users/email/:email", getUserByEmail);
router.put("/api/v1/auth/users/:id", updateUser);
router.delete("/api/v1/auth/users/:id", deleteUser);
```

### **2. Storage Service Changes**

#### **Remove Auth Endpoints**

```go
// REMOVE from storage-service/internal/api/rest/auth.go
// REMOVE all auth-related endpoints:
// - /api/v1/auth/users
// - /api/v1/auth/users/{id}
// - /api/v1/auth/users/email/{email}
// - /api/v1/auth/authenticate
// - /api/v1/auth/audit

// KEEP only profile-related endpoints:
// - /api/v1/profiles
// - /api/v1/profiles/{id}
```

#### **Remove User Data Models**

```go
// REMOVE from storage-service/internal/domain/models/auth.go
// REMOVE AuthUser, AuthAuditLog models

// KEEP only profile-related models:
// - Profile
// - Address
// - Contact
```

#### **Update Database Schema**

```sql
-- REMOVE from storage service database:
-- - users table
-- - auth_audit_logs table
-- - roles table

-- KEEP only:
-- - profiles table
-- - addresses table
-- - contacts table
```

### **3. Profile Service Changes**

#### **Update Auth Service Integration**

```go
// UPDATE in profile-service/internal/infrastructure/auth/
type AuthServiceClient struct {
    baseURL string
    client  *http.Client
}

// ADD new methods for user management
func (c *AuthServiceClient) GetUserByEmail(ctx context.Context, email string) (*User, error) {
    // Call auth service for user data
}

func (c *AuthServiceClient) CreateUser(ctx context.Context, userData *CreateUserRequest) (*User, error) {
    // Call auth service for user creation
}

func (c *AuthServiceClient) UpdateUser(ctx context.Context, userID string, userData *UpdateUserRequest) (*User, error) {
    // Call auth service for user updates
}
```

#### **Remove Direct Storage Service User Access**

```go
// REMOVE from profile-service/internal/infrastructure/storage/
// Any user-related methods that call storage service

// UPDATE profile service to only call auth service for user operations
```

### **4. Cache Service Changes**

#### **Remove Session Management**

```go
// REMOVE from cache-service/internal/domain/services/session_cache_service.go
// REMOVE session-related endpoints:
// - /api/v1/cache/session:{sessionId}
// - /api/v1/cache/blacklist:{tokenId}

// KEEP only profile caching:
// - /api/v1/cache/profile:{profileId}
// - /api/v1/cache/task:{taskId}
```

---

## 📋 **Implementation Checklist**

### **Phase 1: Auth Service Changes**

- [ ] Remove cache service integration from auth service
- [ ] Add user data models to auth service
- [ ] Add user repository to auth service
- [ ] Add user management endpoints to auth service
- [ ] Update auth service deployment configuration
- [ ] Add database migration for user tables in auth service

### **Phase 2: Storage Service Changes**

- [ ] Remove auth endpoints from storage service
- [ ] Remove user data models from storage service
- [ ] Remove auth-related database tables from storage service
- [ ] Update storage service deployment configuration
- [ ] Update storage service documentation

### **Phase 3: Profile Service Changes**

- [ ] Update profile service to call auth service for user operations
- [ ] Remove direct storage service user access from profile service
- [ ] Update profile service deployment configuration
- [ ] Update profile service documentation

### **Phase 4: Cache Service Changes**

- [ ] Remove session management from cache service
- [ ] Update cache service deployment configuration
- [ ] Update cache service documentation

### **Phase 5: Documentation Updates**

- [ ] Update INTEGRATION.md with new architecture
- [ ] Update SERVICES.md with new service responsibilities
- [ ] Update deployment guides
- [ ] Update README files for each service

---

## 🔄 **New Integration Flow**

### **User Registration Flow**

```
Client → Profile Service → Auth Service (create user)
                ↓
        Auth Service → Database (user table)
                ↓
        Auth Service → Profile Service (user created)
                ↓
        Profile Service → Client (success response)
```

### **User Authentication Flow**

```
Client → Profile Service → Auth Service (login)
                ↓
        Auth Service → Database (validate user)
                ↓
        Auth Service → Profile Service (JWT token)
                ↓
        Profile Service → Client (authentication response)
```

### **Profile Operations Flow**

```
Client → Profile Service → Auth Service (validate token)
                ↓
        Auth Service → Profile Service (user context)
                ↓
        Profile Service → Storage Service (profile operations)
                ↓
        Profile Service → Cache Service (caching)
                ↓
        Profile Service → Client (response)
```

---

## ⚠️ **Migration Considerations**

### **Database Migration**

- Auth service needs its own database for user data
- Storage service database needs user tables removed
- Data migration required for existing user data

### **Deployment Changes**

- Auth service needs database deployment
- Storage service database schema changes
- Service discovery updates

### **Testing Requirements**

- Update integration tests for new service boundaries
- Update unit tests for removed functionality
- Update end-to-end tests for new flows

---

## 📊 **Impact Assessment**

### **High Impact Changes**

- Auth service: Major architectural change (add user ownership)
- Storage service: Major reduction in responsibilities
- Profile service: Update integration patterns
- Cache service: Remove session management

### **Medium Impact Changes**

- Database schema changes
- Deployment configuration updates
- Documentation updates

### **Low Impact Changes**

- Worker services: No changes required
- Queue service: No changes required

---

**Status**: 🔴 **CRITICAL CHANGES REQUIRED**  
**Priority**: HIGH  
**Estimated Effort**: 2-3 weeks  
**Risk Level**: MEDIUM (requires careful data migration)
