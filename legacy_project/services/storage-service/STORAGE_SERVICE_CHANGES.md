# Storage Service Implementation Changes

**Service**: Storage Service  
**Purpose**: Remove user data ownership and auth endpoints  
**Priority**: HIGH  
**Estimated Effort**: 2-3 days (reduced from 3-4 days since no data migration needed)

---

## 🎯 **Objectives**

1. **Remove Auth Endpoints**: Remove all auth-related API endpoints
2. **Remove User Data Models**: Remove user, audit, and role models
3. **Update Database Schema**: Remove user-related tables
4. **Focus on Profile Data**: Keep only profile-related functionality

---

## 🔧 **Implementation Changes**

### **1. Remove Auth Endpoints**

#### **Remove Auth Handler**

```bash
# Remove auth handler file
rm services/storage-service/internal/api/rest/auth.go
```

#### **Update Main Router**

```go
// services/storage-service/internal/api/rest/router.go
// REMOVE auth routes registration

func RegisterRoutes(router *mux.Router) {
    // REMOVE: authHandler := NewAuthHandler(authService)
    // REMOVE: authHandler.RegisterRoutes(router)

    // KEEP: Profile routes only
    profileHandler := NewProfileHandler(profileService)
    profileHandler.RegisterRoutes(router)

    // KEEP: Health and metrics routes
    healthHandler := NewHealthHandler()
    healthHandler.RegisterRoutes(router)
}
```

#### **Remove Auth Service**

```bash
# Remove auth service files
rm services/storage-service/internal/domain/service/auth.go
rm services/storage-service/internal/domain/models/auth.go
rm services/storage-service/internal/infrastructure/repository/auth.go
```

### **2. Remove User Data Models**

#### **Remove Auth Models**

```go
// REMOVE from services/storage-service/internal/domain/models/auth.go
// REMOVE these models:
// - AuthUser
// - AuthAuditLog
// - Role

// KEEP only profile-related models:
// - Profile
// - Address
// - Contact
```

#### **Update Models Package**

```go
// services/storage-service/internal/domain/models/models.go
// REMOVE auth imports and models

package models

// KEEP only profile models
type Profile struct {
    ID        string    `json:"id" db:"id"`
    UserID    string    `json:"user_id" db:"user_id"`
    FirstName string    `json:"first_name" db:"first_name"`
    LastName  string    `json:"last_name" db:"last_name"`
    Email     string    `json:"email" db:"email"`
    Bio       string    `json:"bio" db:"bio"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Address struct {
    ID       string `json:"id" db:"id"`
    ProfileID string `json:"profile_id" db:"profile_id"`
    Street   string `json:"street" db:"street"`
    City     string `json:"city" db:"city"`
    State    string `json:"state" db:"state"`
    ZipCode  string `json:"zip_code" db:"zip_code"`
    Country  string `json:"country" db:"country"`
    Type     string `json:"type" db:"type"`
}

type Contact struct {
    ID       string `json:"id" db:"id"`
    ProfileID string `json:"profile_id" db:"profile_id"`
    Type     string `json:"type" db:"type"`
    Value    string `json:"value" db:"value"`
    Label    string `json:"label" db:"label"`
}
```

### **3. Update Database Schema**

#### **Remove User Tables Migration**

```sql
-- services/storage-service/migrations/002_remove_auth_tables.sql
-- Remove auth-related tables

-- Drop foreign key constraints first
ALTER TABLE IF EXISTS auth_audit_logs DROP CONSTRAINT IF EXISTS auth_audit_logs_user_id_fkey;

-- Drop auth-related tables
DROP TABLE IF EXISTS auth_audit_logs;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;

-- Drop auth-related indexes
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_is_active;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_action;
DROP INDEX IF EXISTS idx_audit_logs_created_at;

-- Verify only profile tables remain
-- Should have: profiles, addresses, contacts
```

#### **Update Database Configuration**

```go
// services/storage-service/internal/infrastructure/database/database.go
// REMOVE auth-related database operations

type Database struct {
    db *sql.DB
}

// REMOVE auth-related methods:
// - CreateUser
// - GetUserByEmail
// - GetUserByID
// - UpdateUser
// - DeleteUser
// - ListUsers
// - CreateAuditLog
// - GetAuditLogs

// KEEP only profile-related methods:
// - CreateProfile
// - GetProfileByID
// - UpdateProfile
// - DeleteProfile
// - ListProfiles
// - CreateAddress
// - GetAddressesByProfileID
// - CreateContact
// - GetContactsByProfileID
```

### **4. Update Service Layer**

#### **Remove Auth Service**

```go
// REMOVE from services/storage-service/internal/domain/service/auth.go
// REMOVE entire auth service

// KEEP only profile service:
// services/storage-service/internal/domain/service/profile.go
type ProfileService struct {
    profileRepo *repository.ProfileRepository
    addressRepo *repository.AddressRepository
    contactRepo *repository.ContactRepository
}

// Profile CRUD operations
func (s *ProfileService) CreateProfile(ctx context.Context, profile *models.Profile) error
func (s *ProfileService) GetProfileByID(ctx context.Context, id string) (*models.Profile, error)
func (s *ProfileService) UpdateProfile(ctx context.Context, id string, profile *models.Profile) error
func (s *ProfileService) DeleteProfile(ctx context.Context, id string) error
func (s *ProfileService) ListProfiles(ctx context.Context, page, pageSize int) ([]*models.Profile, error)
```

### **5. Update Repository Layer**

#### **Remove Auth Repository**

```bash
# Remove auth repository
rm services/storage-service/internal/infrastructure/repository/auth.go
```

#### **Update Profile Repository**

```go
// services/storage-service/internal/infrastructure/repository/profile.go
// KEEP only profile-related operations

type ProfileRepository struct {
    db *sql.DB
}

func (r *ProfileRepository) Create(profile *models.Profile) error
func (r *ProfileRepository) GetByID(id string) (*models.Profile, error)
func (r *ProfileRepository) Update(profile *models.Profile) error
func (r *ProfileRepository) Delete(id string) error
func (r *ProfileRepository) List(page, pageSize int) ([]*models.Profile, error)
```

### **6. Update Configuration**

#### **Update ConfigMap**

```yaml
# k8s/deployment/02-storage-service/configmap.yaml
# REMOVE auth-related configuration

data:
  # 📊 MAIN SERVICE CONFIGURATION
  config.yaml: |
    # 🌐 SERVER CONFIGURATION
    server:
      host: "0.0.0.0"
      port: 8080
      grpc_port: 9090
      read_timeout: 30s
      write_timeout: 30s
      idle_timeout: 120s
      shutdown_timeout: 30s
      max_header_bytes: 1048576

    # 🗄️ DATABASE CONFIGURATION (PostgreSQL)
    database:
      host: "postgres-service"
      port: 5432
      database: "storage"
      user: "storage_user"
      # password comes from secret
      max_connections: 10
      idle_connections: 2
      max_lifetime: 1800s
      connection_timeout: 30s
      query_timeout: 30s
      migration_timeout: 300s
      ssl_mode: "disable"
      log_queries: true

    # 🔍 HEALTH CHECK CONFIGURATION
    health:
      check_interval: 30s
      timeout: 5s
      database_ping_timeout: 10s
      
    # 📊 METRICS CONFIGURATION
    metrics:
      enabled: true
      port: 8081
      path: "/metrics"
      collection_interval: 15s
      include_database_metrics: true
      include_request_metrics: true

    # 📝 LOGGING CONFIGURATION
    logging:
      level: "debug"
      format: "json"
      output: "stdout"
      include_caller: true
      development: true
      
    # 🌐 SERVICE DISCOVERY
    services:
      cache_service:
        url: "http://cache-service:8080"
        timeout: 10s
        max_retries: 3
        
    # 🔧 DEVELOPMENT SETTINGS
    development:
      enable_cors: true
      cors_origins: ["*"]
      enable_debug_endpoints: true
      log_requests: true
      pretty_json: true

  # 🗄️ DATABASE ENVIRONMENT VARIABLES
  STORAGE_SERVER_HTTP_PORT: "8080"
  STORAGE_SERVER_GRPC_PORT: "9090"
  STORAGE_POSTGRES_HOST: "postgres-service"
  STORAGE_POSTGRES_PORT: "5432"
  STORAGE_POSTGRES_DB: "storage"
  STORAGE_POSTGRES_USER: "storage_user"
  # STORAGE_POSTGRES_PASSWORD comes from secret
```

### **7. Update API Documentation**

#### **Update Interface Documentation**

```markdown
# services/storage-service/INTERFACE.md

# REMOVE all auth-related API documentation

# KEEP only profile-related endpoints:

## Profile API

### Create Profile

POST /api/v1/profiles
Content-Type: application/json

Request:
{
"first_name": "string (required)",
"last_name": "string (required)",
"email": "string (required, unique)",
"phone": "string (optional)",
"addresses": [...],
"contacts": [...]
}

### Get Profile by ID

GET /api/v1/profiles/{id}

### Update Profile

PUT /api/v1/profiles/{id}

### Delete Profile

DELETE /api/v1/profiles/{id}

### List Profiles

GET /api/v1/profiles?page=1&page_size=10
```

---

## 📋 **Testing Checklist**

### **Unit Tests**

- [ ] Test profile creation with valid data
- [ ] Test profile creation with invalid data
- [ ] Test profile retrieval by ID
- [ ] Test profile update operations
- [ ] Test profile deletion
- [ ] Test address and contact operations
- [ ] Test database connectivity
- [ ] Test error handling

### **Integration Tests**

- [ ] Test profile CRUD operations
- [ ] Test address and contact management
- [ ] Test database migrations
- [ ] Test API endpoints
- [ ] Test error scenarios

### **End-to-End Tests**

- [ ] Test complete profile lifecycle
- [ ] Test profile service with profile service
- [ ] Test error scenarios
- [ ] Test performance under load

---

## 🚨 **Migration Notes**

### **Data Migration**

1. **Export user data** before removing tables
2. **Verify no dependencies** on user data
3. **Update any references** to user data
4. **Test profile operations** after migration

### **Deployment Order**

1. **Backup existing data** (including user data)
2. **Deploy updated storage service**
3. **Run database migrations**
4. **Verify profile operations work**
5. **Remove auth endpoints from routing**

### **Rollback Plan**

1. **Keep backup** of user data
2. **Monitor error rates** during migration
3. **Have rollback scripts** ready
4. **Test rollback procedure**

---

## 📊 **Impact Assessment**

### **High Impact Changes**

- **Removal of auth endpoints**: Major API change
- **Database schema changes**: Requires migration
- **Service responsibility reduction**: Major architectural change

### **Medium Impact Changes**

- **Configuration updates**: Deployment changes
- **Documentation updates**: API documentation changes
- **Testing updates**: New test requirements

### **Low Impact Changes**

- **Profile functionality**: No changes required
- **Worker services**: No impact
- **Queue service**: No impact

---

## 🔄 **New Service Boundaries**

### **Storage Service Responsibilities**

- ✅ **Profile data management**: CRUD operations for profiles
- ✅ **Address management**: Profile address operations
- ✅ **Contact management**: Profile contact operations
- ✅ **Data persistence**: PostgreSQL for profile data
- ❌ **User management**: Moved to auth service
- ❌ **Authentication**: Moved to auth service
- ❌ **Audit logging**: Moved to auth service

### **API Endpoints (After Changes)**

```
GET    /health                    # Health check
GET    /metrics                   # Prometheus metrics
GET    /api/v1/profiles          # List profiles
POST   /api/v1/profiles          # Create profile
GET    /api/v1/profiles/{id}     # Get profile
PUT    /api/v1/profiles/{id}     # Update profile
DELETE /api/v1/profiles/{id}     # Delete profile
```

---

**Status**: 🔴 **IMPLEMENTATION REQUIRED**  
**Priority**: HIGH  
**Estimated Effort**: 3-4 days  
**Dependencies**: Auth service changes, data migration

---

## 🔍 **Additional Changes Required** (Beyond Initial Analysis)

### **1. Queue Processing Changes**

#### **Remove Auth Message Handler**

- Remove `internal/messaging/auth_handlers.go`
- Remove auth handler registration in `cmd/server/main.go`
- Update `BatchMessageHandler` to remove `batch.auth.process` routing key
- Update message processor tests to remove auth handler tests

#### **Update Batch Processing**

- Remove auth-related batch types from `models.BatchRequest`
- Update batch validation to remove auth type checks
- Remove `processAuthOperation` from `AdvancedBatchOperationsService`
- Update batch operation tests to remove auth scenarios

### **2. Configuration Updates**

#### **Remove Auth-Related Config**

- Remove auth-related environment variables
- Remove auth service configuration sections
- Update deployment manifests to remove auth configs
- Remove auth-related health checks

### **3. Service Layer Changes**

#### **Update Batch Service**

- Remove auth service dependency from `AdvancedBatchOperationsService`
- Update batch service constructor to remove auth service parameter
- Remove auth-related batch processing logic
- Update batch service tests

#### **Update Profile Service**

- Remove any auth service dependencies
- Update profile validation to remove user-related checks
- Remove user ID references from profile operations
- Update profile service tests

### **4. Database Changes**

#### **Schema Updates**

- Drop auth-related tables (no migration needed - development mode)
- Remove foreign key constraints
- Remove auth-related indexes
- Update profile table structure

### **5. Messaging Infrastructure**

#### **Update Message Types**

- Remove auth-related message types
- Update message validation
- Remove auth routing keys
- Update message processing tests

### **6. Testing Infrastructure**

#### **Update Test Suites**

- Remove auth-related test cases
- Update mock data generation
- Remove auth service mocks
- Update integration test scenarios

### **7. Documentation Updates**

#### **API Documentation**

- Remove auth endpoint documentation
- Update batch operation docs to remove auth types
- Update message processing docs
- Update deployment guides

#### **Monitoring Updates**

- Remove auth-related metrics
- Update health check documentation
- Update logging guidelines
- Remove auth-related alerts

---

## 🔄 **Implementation Order**

1. **Phase 1: Code Cleanup (Day 1)**

   - Remove auth handlers and endpoints
   - Remove auth message processing
   - Remove auth service layer
   - Update configuration

2. **Phase 2: Database Updates (Day 1)**

   - Drop auth tables
   - Remove constraints and indexes
   - Update profile table structure
   - Update database tests

3. **Phase 3: Service Updates (Day 2)**

   - Update batch processing
   - Update profile service
   - Update message processing
   - Update configuration handling

4. **Phase 4: Testing & Validation (Day 2-3)**
   - Update test suites
   - Add new test cases
   - Verify message processing
   - Validate batch operations

---

## 🔍 **Impact Analysis**

### **High Impact Areas**

1. **Batch Processing**

   - Requires significant updates to remove auth operations
   - Message handling changes needed
   - Validation logic updates required

2. **Database Schema**

   - Table structure changes
   - Foreign key removal
   - Index updates

3. **Message Processing**
   - Routing key changes
   - Handler removal
   - Validation updates

### **Medium Impact Areas**

1. **Configuration**

   - Environment variable updates
   - Deployment manifest changes
   - Health check modifications

2. **Testing**
   - Test suite updates
   - Mock data changes
   - Integration test modifications

### **Low Impact Areas**

1. **Documentation**

   - API documentation updates
   - Deployment guide changes
   - Monitoring documentation

2. **Metrics**
   - Remove auth metrics
   - Update health checks
   - Modify alerts

---

## 🚨 **Risk Assessment**

### **High Risk**

1. **Service Integration**
   - Other services might expect auth functionality
   - Message processing changes could affect consumers
   - Batch operation changes impact workflows

### **Medium Risk**

1. **Performance**

   - Database schema changes might affect queries
   - Batch processing modifications need testing
   - Message handling changes require validation

2. **Deployment**
   - Configuration changes need coordination
   - Service dependencies need verification
   - Clean environment setup required

### **Low Risk**

1. **Documentation**
   - API documentation updates
   - Configuration guide changes
   - Monitoring updates

---

## ✅ **Success Criteria**

### **Functional Requirements**

- [ ] All auth endpoints removed
- [ ] Database schema updated
- [ ] Message processing working without auth
- [ ] Batch operations functioning correctly
- [ ] Profile operations working independently

### **Technical Requirements**

- [ ] No auth-related code remains
- [ ] All tests passing
- [ ] No deployment issues
- [ ] Performance metrics maintained
- [ ] Clean database schema

### **Operational Requirements**

- [ ] Monitoring working correctly
- [ ] Logging appropriate
- [ ] Alerts properly configured
- [ ] Documentation updated

---

## 🔄 **Development Mode Considerations**

### **Environment Setup**

1. Fresh database setup after changes
2. Clean environment deployment
3. Updated configuration files
4. New test data generation

### **During Implementation**

1. Implement changes directly
2. Run tests frequently
3. Verify in clean environment
4. Update documentation immediately

### **Post-Implementation**

1. Full test suite validation
2. Clean environment verification
3. Updated deployment verification
4. Documentation review

---

**Status**: 🔴 **READY FOR IMPLEMENTATION**  
**Priority**: HIGH  
**Estimated Effort**: 2-3 days  
**Dependencies**: None (self-contained changes)  
**Environment**: Development Mode
