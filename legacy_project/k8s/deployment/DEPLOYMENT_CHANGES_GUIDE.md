# Deployment Changes Guide

**Purpose**: Update Kubernetes deployment manifests to be compliant with architectural changes  
**Priority**: HIGH  
**Estimated Effort**: 2-3 days  
**Scope**: All service deployments in `k8s/deployment/`

---

## 🎯 **Objectives**

1. **Update Auth Service Deployment**: Add database, remove cache dependencies
2. **Update Storage Service Deployment**: Remove auth endpoints, focus on profile data
3. **Update Profile Service Deployment**: Add auth service integration, remove storage user access
4. **Update Cache Service Deployment**: Remove session management
5. **Ensure Service Dependencies**: Update service discovery and networking

---

## 🔧 **Implementation Changes**

### **1. Auth Service Deployment Changes**

#### **Add Database StatefulSet**

```yaml
# k8s/deployment/03-auth-service/auth-postgres-statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: auth-postgres
  labels:
    app: auth-postgres
    component: database
    service: auth
spec:
  serviceName: auth-postgres-service
  replicas: 1
  selector:
    matchLabels:
      app: auth-postgres
  template:
    metadata:
      labels:
        app: auth-postgres
        component: database
        service: auth
    spec:
      securityContext:
        runAsUser: 999
        runAsGroup: 999
        fsGroup: 999
      containers:
        - name: postgres
          image: postgres:15-alpine
          ports:
            - containerPort: 5432
              name: postgres
          env:
            - name: POSTGRES_DB
              value: "auth_db"
            - name: POSTGRES_USER
              value: "auth_user"
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: auth-postgres-secret
                  key: password
            - name: PGDATA
              value: "/var/lib/postgresql/data/pgdata"
          volumeMounts:
            - name: auth-postgres-data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            exec:
              command: ["pg_isready", "-U", "auth_user", "-d", "auth_db"]
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "auth_user", "-d", "auth_db"]
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: auth-postgres-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
---
apiVersion: v1
kind: Service
metadata:
  name: auth-postgres-service
  labels:
    app: auth-postgres
    component: database
    service: auth
spec:
  selector:
    app: auth-postgres
  ports:
    - protocol: TCP
      port: 5432
      targetPort: 5432
      name: postgres
  type: ClusterIP
```

#### **Add Database Secret**

```yaml
# k8s/deployment/03-auth-service/auth-postgres-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: auth-postgres-secret
  labels:
    service: auth
    component: database
type: Opaque
data:
  password: YXV0aF9wYXNzd29yZA== # auth_password (base64 encoded)
```

#### **Update Auth Service ConfigMap**

```yaml
# k8s/deployment/03-auth-service/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: auth-service-config
  labels:
    app: auth-service
    service: auth
data:
  # 🌐 SERVICE INTEGRATION CONFIGURATION
  # REMOVE: CACHE_SERVICE_URL: "http://cache-service:8080"

  # ADD: Database configuration
  DATABASE_HOST: "auth-postgres-service"
  DATABASE_PORT: "5432"
  DATABASE_NAME: "auth_db"
  DATABASE_USER: "auth_user"
  # DATABASE_PASSWORD comes from secret

  # KEEP: Storage service for migration period
  STORAGE_SERVICE_URL: "http://storage-service:8080"

  # 🔄 CIRCUIT BREAKER CONFIGURATION
  CIRCUIT_BREAKER_TIMEOUT: "3000"
  CIRCUIT_BREAKER_ERROR_THRESHOLD: "50"
  CIRCUIT_BREAKER_RESET_TIMEOUT: "30000"

  # 🔒 SECURITY CONFIGURATION
  RATE_LIMIT_WINDOW_MS: "900000"
  RATE_LIMIT_MAX_REQUESTS: "5"
  ACCOUNT_LOCKOUT_ATTEMPTS: "5"
  ACCOUNT_LOCKOUT_DURATION_MS: "1800000"

  # 🏥 HEALTH CHECK CONFIGURATION
  HEALTH_CHECK_INTERVAL: "30s"
  HEALTH_CHECK_TIMEOUT: "5s"

  # 📊 LOGGING CONFIGURATION
  LOG_LEVEL: "debug"
  LOG_FORMAT: "json"
  NODE_ENV: "kind-production"

  # 🖥️ SERVER CONFIGURATION
  PORT: "8080"
  HOST: "0.0.0.0"
  METRICS_PORT: "8081"

  # 🔑 JWT CONFIGURATION
  JWT_ALGORITHM: "RS256"
  JWT_ISSUER: "auth-service"
  JWT_AUDIENCE: "microservices-ecosystem"
  ACCESS_TOKEN_EXPIRY: "1h"
  REFRESH_TOKEN_EXPIRY: "7d"

  # 📈 METRICS CONFIGURATION
  METRICS_ENABLED: "true"
  METRICS_PREFIX: "auth_service_"
```

#### **Update Auth Service Deployment**

```yaml
# k8s/deployment/03-auth-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  labels:
    app: auth-service
    service: auth
spec:
  replicas: 2
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
        service: auth
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: auth-service
          image: auth-service:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081
              name: metrics
          env:
            # Database configuration
            - name: DATABASE_HOST
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: DATABASE_HOST
            - name: DATABASE_PORT
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: DATABASE_PORT
            - name: DATABASE_NAME
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: DATABASE_NAME
            - name: DATABASE_USER
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: DATABASE_USER
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: auth-postgres-secret
                  key: password
            # Service URLs
            - name: STORAGE_SERVICE_URL
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: STORAGE_SERVICE_URL
            # JWT Configuration
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: auth-service-secret
                  key: jwt_secret
            # Other configurations
            - name: NODE_ENV
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: NODE_ENV
            - name: LOG_LEVEL
              valueFrom:
                configMapKeyRef:
                  name: auth-service-config
                  key: LOG_LEVEL
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

### **2. Storage Service Deployment Changes**

#### **Update Storage Service ConfigMap**

```yaml
# k8s/deployment/02-storage-service/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: storage-service-config
  labels:
    app: storage-service
    service: storage
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

#### **Update Storage Service Deployment**

```yaml
# k8s/deployment/02-storage-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: storage-service
  labels:
    app: storage-service
    service: storage
spec:
  replicas: 2
  selector:
    matchLabels:
      app: storage-service
  template:
    metadata:
      labels:
        app: storage-service
        service: storage
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: storage-service
          image: storage-service:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 9090
              name: grpc
            - containerPort: 8081
              name: metrics
          env:
            # Database configuration
            - name: STORAGE_POSTGRES_HOST
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_POSTGRES_HOST
            - name: STORAGE_POSTGRES_PORT
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_POSTGRES_PORT
            - name: STORAGE_POSTGRES_DB
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_POSTGRES_DB
            - name: STORAGE_POSTGRES_USER
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_POSTGRES_USER
            - name: STORAGE_POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: storage-postgres-secret
                  key: password
            # Server configuration
            - name: STORAGE_SERVER_HTTP_PORT
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_SERVER_HTTP_PORT
            - name: STORAGE_SERVER_GRPC_PORT
              valueFrom:
                configMapKeyRef:
                  name: storage-service-config
                  key: STORAGE_SERVER_GRPC_PORT
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

### **3. Profile Service Deployment Changes**

#### **Update Profile Service ConfigMap**

```yaml
# k8s/deployment/05-profile-service/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: profile-service-config
  labels:
    app: profile-service
    service: profile
data:
  # 🌐 SERVICE INTEGRATION CONFIGURATION
  AUTH_SERVICE_URL: "http://auth-service:8080"
  CACHE_SERVICE_URL: "http://cache-service:8080"
  STORAGE_SERVICE_URL: "http://storage-service:8080"
  QUEUE_SERVICE_URL: "http://queue-service:8080"

  # ADD: Auth service specific configuration
  AUTH_SERVICE_TIMEOUT: "5000"
  AUTH_SERVICE_RETRIES: "3"
  AUTH_SERVICE_CIRCUIT_BREAKER_TIMEOUT: "3000"
  AUTH_SERVICE_CIRCUIT_BREAKER_ERROR_THRESHOLD: "50"
  AUTH_SERVICE_CIRCUIT_BREAKER_RESET_TIMEOUT: "30000"

  # KEEP: Existing configuration
  SERVICE_TIMEOUT: "5000"
  SERVICE_RETRIES: "3"
  CIRCUIT_BREAKER_TIMEOUT: "3000"
  CIRCUIT_BREAKER_ERROR_THRESHOLD: "50"
  CIRCUIT_BREAKER_RESET_TIMEOUT: "30000"

  # 🎯 MULTI-WORKER ROUTING CONFIGURATION
  ROUTING_KEY_PROFILE_UPDATE: "profile.task"
  ROUTING_KEY_EMAIL_NOTIFICATION: "email.send"
  ROUTING_KEY_IMAGE_PROCESSING: "image.process"
  ROUTING_KEY_DEFAULT_FALLBACK: "profile.task"

  # Task type configuration for validation
  SUPPORTED_TASK_TYPES: "profile_update,email_notification,image_processing"
  TASK_VALIDATION_ENABLED: "true"
  TASK_PAYLOAD_VALIDATION_ENABLED: "true"

  # 📊 PERFORMANCE CONFIGURATION
  API_RESPONSE_TIME_THRESHOLD: "100ms"
  QUEUE_COMMUNICATION_THRESHOLD: "200ms"
  ERROR_RATE_THRESHOLD: "0.05"
  THROUGHPUT_TARGET: "100"

  # 🔄 QUEUE SERVICE INTEGRATION
  QUEUE_SERVICE_HEALTH_CHECK_INTERVAL: "30s"
  QUEUE_SERVICE_CONNECTION_POOL_SIZE: "5"
  QUEUE_SERVICE_KEEP_ALIVE_ENABLED: "true"
  QUEUE_SERVICE_MAX_RETRIES: "3"
  QUEUE_SERVICE_RETRY_DELAY: "1s"

  # 📨 MESSAGE FORMAT CONFIGURATION
  MESSAGE_FORMAT_VERSION: "v2.0"
  MESSAGE_PAYLOAD_TYPE: "json.RawMessage"
  MESSAGE_TIMESTAMP_FORMAT: "RFC3339"
  MESSAGE_METADATA_ENABLED: "true"

  # 🔧 MULTI-WORKER TASK PROCESSING
  TASK_PROCESSING_ENABLED: "true"
  TASK_ROUTING_ENABLED: "true"
  TASK_FALLBACK_ENABLED: "true"

  # Profile Worker Configuration
  PROFILE_WORKER_ENABLED: "true"
  PROFILE_WORKER_QUEUE: "profile-processing"
  PROFILE_WORKER_ROUTING_KEY: "profile.task"
  PROFILE_WORKER_TIMEOUT: "2m"
  PROFILE_WORKER_RETRY_ATTEMPTS: "3"

  # Email Worker Configuration
  EMAIL_WORKER_ENABLED: "true"
  EMAIL_WORKER_QUEUE: "email-processing"
  EMAIL_WORKER_ROUTING_KEY: "email.send"
  EMAIL_WORKER_TIMEOUT: "1m"
  EMAIL_WORKER_RETRY_ATTEMPTS: "3"
  EMAIL_TEMPLATES_SUPPORTED: "welcome,profile_updated,password_reset,notification"

  # Image Worker Configuration
  IMAGE_WORKER_ENABLED: "true"
  IMAGE_WORKER_QUEUE: "image-processing"
  IMAGE_WORKER_ROUTING_KEY: "image.process"
  IMAGE_WORKER_TIMEOUT: "3m"
  IMAGE_WORKER_RETRY_ATTEMPTS: "2"
  IMAGE_FORMATS_SUPPORTED: "jpeg,jpg,png,webp,gif"
  IMAGE_MAX_SIZE: "5MB"
  IMAGE_OPERATIONS_SUPPORTED: "resize,compress,convert,crop"
```

#### **Update Profile Service Deployment**

```yaml
# k8s/deployment/05-profile-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
  labels:
    app: profile-service
    service: profile
spec:
  replicas: 2
  selector:
    matchLabels:
      app: profile-service
  template:
    metadata:
      labels:
        app: profile-service
        service: profile
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: profile-service
          image: profile-service:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081
              name: metrics
          env:
            # Auth service configuration
            - name: AUTH_SERVICE_URL
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_URL
            - name: AUTH_SERVICE_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_TIMEOUT
            - name: AUTH_SERVICE_RETRIES
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_RETRIES
            - name: AUTH_SERVICE_CIRCUIT_BREAKER_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_CIRCUIT_BREAKER_TIMEOUT
            - name: AUTH_SERVICE_CIRCUIT_BREAKER_ERROR_THRESHOLD
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_CIRCUIT_BREAKER_ERROR_THRESHOLD
            - name: AUTH_SERVICE_CIRCUIT_BREAKER_RESET_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: AUTH_SERVICE_CIRCUIT_BREAKER_RESET_TIMEOUT
            # Other service URLs
            - name: CACHE_SERVICE_URL
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: CACHE_SERVICE_URL
            - name: STORAGE_SERVICE_URL
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: STORAGE_SERVICE_URL
            - name: QUEUE_SERVICE_URL
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: QUEUE_SERVICE_URL
            # Service configuration
            - name: SERVICE_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: SERVICE_TIMEOUT
            - name: SERVICE_RETRIES
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: SERVICE_RETRIES
            # Worker configuration
            - name: PROFILE_WORKER_ENABLED
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: PROFILE_WORKER_ENABLED
            - name: EMAIL_WORKER_ENABLED
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: EMAIL_WORKER_ENABLED
            - name: IMAGE_WORKER_ENABLED
              valueFrom:
                configMapKeyRef:
                  name: profile-service-config
                  key: IMAGE_WORKER_ENABLED
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

### **4. Cache Service Deployment Changes**

#### **Update Cache Service ConfigMap**

```yaml
# k8s/deployment/01-cache-service/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cache-service-config
  labels:
    app: cache-service
    service: cache
data:
  # 🌐 SERVICE CONFIGURATION
  CACHE_SERVICE_HOST: "0.0.0.0"
  CACHE_SERVICE_PORT: "8080"
  CACHE_SERVICE_METRICS_PORT: "8081"

  # 🗄️ REDIS CONFIGURATION
  REDIS_HOST: "redis-service"
  REDIS_PORT: "6379"
  REDIS_DB: "0"
  REDIS_PASSWORD: "" # No password for development
  REDIS_MAX_RETRIES: "3"
  REDIS_DIAL_TIMEOUT: "5s"
  REDIS_READ_TIMEOUT: "3s"
  REDIS_WRITE_TIMEOUT: "3s"
  REDIS_POOL_SIZE: "10"
  REDIS_MIN_IDLE_CONNS: "2"

  # 📊 CACHE CONFIGURATION
  CACHE_DEFAULT_TTL: "3600" # 1 hour
  CACHE_MAX_TTL: "86400" # 24 hours
  CACHE_CLEANUP_INTERVAL: "300" # 5 minutes

  # 🔍 HEALTH CHECK CONFIGURATION
  HEALTH_CHECK_INTERVAL: "30s"
  HEALTH_CHECK_TIMEOUT: "5s"
  REDIS_PING_TIMEOUT: "10s"

  # 📊 METRICS CONFIGURATION
  METRICS_ENABLED: "true"
  METRICS_PREFIX: "cache_service_"
  METRICS_COLLECTION_INTERVAL: "15s"

  # 📝 LOGGING CONFIGURATION
  LOG_LEVEL: "debug"
  LOG_FORMAT: "json"
  LOG_OUTPUT: "stdout"
  LOG_INCLUDE_CALLER: "true"
  LOG_DEVELOPMENT: "true"

  # 🔧 DEVELOPMENT SETTINGS
  DEVELOPMENT_ENABLE_CORS: "true"
  DEVELOPMENT_CORS_ORIGINS: "*"
  DEVELOPMENT_ENABLE_DEBUG_ENDPOINTS: "true"
  DEVELOPMENT_LOG_REQUESTS: "true"
  DEVELOPMENT_PRETTY_JSON: "true"

  # REMOVE: Session management configuration
  # SESSION_CACHE_ENABLED: "false"
  # SESSION_DEFAULT_TTL: "3600"
  # SESSION_CLEANUP_INTERVAL: "300"
```

#### **Update Cache Service Deployment**

```yaml
# k8s/deployment/01-cache-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cache-service
  labels:
    app: cache-service
    service: cache
spec:
  replicas: 2
  selector:
    matchLabels:
      app: cache-service
  template:
    metadata:
      labels:
        app: cache-service
        service: cache
    spec:
      securityContext:
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: cache-service
          image: cache-service:latest
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081
              name: metrics
          env:
            # Redis configuration
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_HOST
            - name: REDIS_PORT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_PORT
            - name: REDIS_DB
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_DB
            - name: REDIS_PASSWORD
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_PASSWORD
            - name: REDIS_MAX_RETRIES
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_MAX_RETRIES
            - name: REDIS_DIAL_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_DIAL_TIMEOUT
            - name: REDIS_READ_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_READ_TIMEOUT
            - name: REDIS_WRITE_TIMEOUT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_WRITE_TIMEOUT
            - name: REDIS_POOL_SIZE
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_POOL_SIZE
            - name: REDIS_MIN_IDLE_CONNS
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: REDIS_MIN_IDLE_CONNS
            # Cache configuration
            - name: CACHE_DEFAULT_TTL
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_DEFAULT_TTL
            - name: CACHE_MAX_TTL
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_MAX_TTL
            - name: CACHE_CLEANUP_INTERVAL
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_CLEANUP_INTERVAL
            # Service configuration
            - name: CACHE_SERVICE_HOST
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_SERVICE_HOST
            - name: CACHE_SERVICE_PORT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_SERVICE_PORT
            - name: CACHE_SERVICE_METRICS_PORT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: CACHE_SERVICE_METRICS_PORT
            # Logging configuration
            - name: LOG_LEVEL
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: LOG_LEVEL
            - name: LOG_FORMAT
              valueFrom:
                configMapKeyRef:
                  name: cache-service-config
                  key: LOG_FORMAT
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

### **5. Update Network Policies**

#### **Auth Service Network Policy**

```yaml
# k8s/deployment/03-auth-service/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: auth-service-network-policy
  labels:
    app: auth-service
    service: auth
spec:
  podSelector:
    matchLabels:
      app: auth-service
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: profile-service
      ports:
        - protocol: TCP
          port: 8080
    - from:
        - podSelector:
            matchLabels:
              app: storage-service
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: auth-postgres
      ports:
        - protocol: TCP
          port: 5432
    - to:
        - podSelector:
            matchLabels:
              app: storage-service
      ports:
        - protocol: TCP
          port: 8080
```

#### **Storage Service Network Policy**

```yaml
# k8s/deployment/02-storage-service/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: storage-service-network-policy
  labels:
    app: storage-service
    service: storage
spec:
  podSelector:
    matchLabels:
      app: storage-service
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: profile-service
      ports:
        - protocol: TCP
          port: 8080
        - protocol: TCP
          port: 9090
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
    - to:
        - podSelector:
            matchLabels:
              app: cache-service
      ports:
        - protocol: TCP
          port: 8080
```

#### **Profile Service Network Policy**

```yaml
# k8s/deployment/05-profile-service/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: profile-service-network-policy
  labels:
    app: profile-service
    service: profile
spec:
  podSelector:
    matchLabels:
      app: profile-service
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: auth-service
      ports:
        - protocol: TCP
          port: 8080
    - to:
        - podSelector:
            matchLabels:
              app: storage-service
      ports:
        - protocol: TCP
          port: 8080
    - to:
        - podSelector:
            matchLabels:
              app: cache-service
      ports:
        - protocol: TCP
          port: 8080
    - to:
        - podSelector:
            matchLabels:
              app: queue-service
      ports:
        - protocol: TCP
          port: 8080
```

### **6. Update Service Dependencies**

#### **Update Kustomization Files**

```yaml
# k8s/deployment/03-auth-service/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - auth-postgres-statefulset.yaml
  - auth-postgres-secret.yaml
  - network-policy.yaml

commonLabels:
  app.kubernetes.io/name: auth-service
  app.kubernetes.io/part-of: microservices-ecosystem
```

```yaml
# k8s/deployment/02-storage-service/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - network-policy.yaml

commonLabels:
  app.kubernetes.io/name: storage-service
  app.kubernetes.io/part-of: microservices-ecosystem
```

```yaml
# k8s/deployment/05-profile-service/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - network-policy.yaml

commonLabels:
  app.kubernetes.io/name: profile-service
  app.kubernetes.io/part-of: microservices-ecosystem
```

---

## 📋 **Testing Checklist**

### **Deployment Tests**

- [ ] Test auth service database connectivity
- [ ] Test auth service user management endpoints
- [ ] Test storage service profile endpoints (no auth endpoints)
- [ ] Test profile service auth service integration
- [ ] Test cache service (no session management)
- [ ] Test network policies and service communication
- [ ] Test service health checks and readiness probes
- [ ] Test resource limits and requests

### **Integration Tests**

- [ ] Test user registration flow (profile → auth → database)
- [ ] Test user authentication flow (profile → auth → database)
- [ ] Test profile operations with user context
- [ ] Test error scenarios and circuit breakers
- [ ] Test service discovery and load balancing

### **End-to-End Tests**

- [ ] Test complete user lifecycle
- [ ] Test profile creation with user context
- [ ] Test authentication and authorization flows
- [ ] Test error handling and recovery
- [ ] Test performance under load

---

## 🚨 **Migration Notes**

### **Deployment Order**

1. **Deploy auth service** with new database and user management
2. **Update profile service** with auth service integration
3. **Update storage service** to remove auth endpoints
4. **Update cache service** to remove session management
5. **Update network policies** for new service boundaries
6. **Test all integrations** and verify service communication

### **Data Migration**

1. **Export user data** from storage service database
2. **Import user data** to auth service database
3. **Verify data integrity** after migration
4. **Update service configurations** to use new endpoints

### **Rollback Plan**

1. **Keep backup** of all configurations
2. **Monitor error rates** during deployment
3. **Have rollback scripts** ready for each service
4. **Test rollback procedures** before deployment

---

## 📊 **Impact Assessment**

### **High Impact Changes**

- **Auth service**: New database deployment and user management
- **Storage service**: Removal of auth endpoints and user data
- **Profile service**: New auth service integration
- **Network policies**: Updated service communication patterns

### **Medium Impact Changes**

- **Configuration updates**: New environment variables and settings
- **Service dependencies**: Updated service discovery and integration
- **Resource allocation**: Updated CPU and memory requirements

### **Low Impact Changes**

- **Cache service**: Minor configuration updates
- **Worker services**: No changes required
- **Queue service**: No changes required

---

**Status**: 🔴 **IMPLEMENTATION REQUIRED**  
**Priority**: HIGH  
**Estimated Effort**: 2-3 days  
**Dependencies**: Service code changes, database migrations
