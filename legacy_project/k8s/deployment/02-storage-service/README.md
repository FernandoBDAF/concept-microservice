# Storage Service – Kubernetes Deployment

**Service**: PostgreSQL-backed profile data storage service  
**Port**: NodePort 30082 (HTTP), 30092 (gRPC)  
**Dependencies**: PostgreSQL StatefulSet  
**Technology**: Go/Node.js with PostgreSQL 15.x backend  
**Focus**: Profile data only (user management moved to Auth Service)

---

## 🧱 Components

| Resource        | Description                                   |
| --------------- | --------------------------------------------- |
| **Deployment**  | Storage service app running on port 8080      |
| **Service**     | NodePort 30082 (HTTP) and 30092 (gRPC)        |
| **ConfigMap**   | Application config and database settings      |
| **Secret**      | PostgreSQL credentials and API keys           |
| **StatefulSet** | PostgreSQL 15.x with persistent storage (2Gi) |

## 🔁 Dependencies

- **PostgreSQL StatefulSet**: Primary database for profile data storage
- **Storage Class**: `standard` for PostgreSQL persistent volumes
- **No upstream services**: Foundation layer service
- **No auth functionality**: User management handled by Auth Service

## 🚀 Deployment

### Quick Deploy

```bash
# Deploy all components
kubectl apply -f .

# Wait for readiness
kubectl wait --for=condition=Available deployment/storage-service --timeout=300s
kubectl wait --for=condition=Ready pod/postgres-0 --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Deploy PostgreSQL backend first
kubectl apply -f postgres-statefulset.yaml
kubectl wait --for=condition=Ready pod/postgres-0 --timeout=300s

# 2. Verify database initialization
kubectl logs postgres-0 | grep "database system is ready"

# 3. Deploy application components
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# 4. Verify deployment
kubectl get pods -l app=storage-service
kubectl get pods -l app=postgres
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30082/health

# Database connectivity check
kubectl exec postgres-0 -- pg_isready -U profile_user

# Test database connection from service
kubectl exec deployment/storage-service -- nc -zv postgres-service 5432
```

### API Testing

```bash
# Create a profile
curl -X POST http://localhost:30082/api/v1/profiles \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "bio": "Test user"
  }'

# List profiles
curl -X GET http://localhost:30082/api/v1/profiles

# Get specific profile
curl -X GET http://localhost:30082/api/v1/profiles/{profile-id}
```

### Database Direct Access

```bash
# Connect to database
kubectl exec -it postgres-0 -- psql -U profile_user -d profile_db

# Check tables
\dt

# Check profile data
SELECT * FROM profiles LIMIT 5;
```

## 📊 Monitoring

### Metrics Endpoints

- **Application Metrics**: `http://localhost:30082/metrics`
- **Health Status**: `http://localhost:30082/health`
- **Database Status**: Included in health endpoint

### Key Metrics

- Database connection pool status
- Query response times
- Profile CRUD operation counts
- PostgreSQL connection status

## 🔧 Configuration

### Environment Variables

| Variable      | Description         | Default            |
| ------------- | ------------------- | ------------------ |
| `DB_HOST`     | PostgreSQL hostname | `postgres-service` |
| `DB_PORT`     | PostgreSQL port     | `5432`             |
| `DB_NAME`     | Database name       | `profile_db`       |
| `DB_USER`     | Database user       | `profile_user`     |
| `DB_PASSWORD` | Database password   | From secret        |
| `SERVER_PORT` | HTTP server port    | `8080`             |
| `GRPC_PORT`   | gRPC server port    | `50052`            |

### Database Schema

- **profiles**: User profile data
- **addresses**: User address information
- **contacts**: Contact information
- **users**: Authentication user data

### Resource Limits

- **Storage Service**: 300m CPU request, 1000m limit; 512Mi memory request, 1Gi limit
- **PostgreSQL**: 300m CPU request, 500m limit; 512Mi memory request, 1Gi limit

## 🚨 Troubleshooting

### Common Issues

**String Formatting Errors**

```bash
# Check for formatting issues in logs
kubectl logs deployment/storage-service | grep -E "(error|panic|fatal)"

# Common issue: Port formatting
# Fixed: Use string port instead of integer in configuration
```

**Database Connection Issues**

```bash
# Check PostgreSQL status
kubectl exec postgres-0 -- pg_isready -U profile_user

# Test database connectivity
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "SELECT version();"

# Check database initialization
kubectl logs postgres-0 | grep "database system is ready"
```

**gRPC Port Issues**

```bash
# Check gRPC port configuration
kubectl get svc storage-service -o yaml | grep -A5 -B5 grpc

# Verify targetPort matches container port
kubectl describe deployment storage-service | grep -A10 -B10 50052
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled where possible
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### Database Security

- **Encrypted passwords**: Using PostgreSQL password hashing
- **Limited permissions**: Database user has only necessary privileges
- **Network isolation**: PostgreSQL only accessible from storage service

### Network Policies

- Ingress: Allows traffic from auth service, profile service, and monitoring
- Egress: Allows DNS and PostgreSQL communication only

## 📋 Service Information

| Aspect        | Details                                    |
| ------------- | ------------------------------------------ |
| **Namespace** | `default`                                  |
| **Labels**    | `app=storage-service`, `component=storage` |
| **Replicas**  | 1 (Kind optimized)                         |
| **Strategy**  | RollingUpdate                              |
| **Protocols** | HTTP (8080), gRPC (50052)                  |

## 🗄️ Database Information

| Aspect             | Details                   |
| ------------------ | ------------------------- |
| **Version**        | PostgreSQL 15.x           |
| **Storage**        | 2Gi persistent volume     |
| **Backup**         | Manual via pg_dump        |
| **Initialization** | Automated schema creation |
| **Monitoring**     | Built-in health checks    |

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#storage-service-guide)
