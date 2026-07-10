# Auth Service – Kubernetes Deployment

**Service**: JWT-based authentication and authorization service with dedicated database  
**Port**: NodePort 30083 (HTTP)  
**Dependencies**: Own PostgreSQL database (auth-postgres)  
**Technology**: Node.js with JWT tokens, bcrypt, and PostgreSQL

---

## 🧱 Components

| Resource             | Description                                       |
| -------------------- | ------------------------------------------------- |
| **StatefulSet**      | PostgreSQL database for user data (auth-postgres) |
| **Database Service** | ClusterIP service for PostgreSQL access           |
| **Database Secret**  | PostgreSQL credentials                            |
| **Deployment**       | Auth service app running on port 8080             |
| **Service**          | NodePort 30083 for external access                |
| **ConfigMap**        | Database connection and JWT configuration         |
| **Secret**           | JWT secrets, API keys, and service tokens         |

## 🔁 Dependencies

- **PostgreSQL Database**: Dedicated auth-postgres StatefulSet for user data
- **Network connectivity**: Must reach `auth-postgres-service:5432`
- **Storage Service**: Optional integration for profile data during migration

## 🚀 Deployment

### Quick Deploy

```bash
# Deploy all components (including database)
kubectl apply -f auth-postgres-secret.yaml
kubectl apply -f auth-postgres-statefulset.yaml
kubectl wait --for=condition=Ready pod/auth-postgres-0 --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Ensure Storage Service is running
kubectl get pods -l app=storage-service
curl http://localhost:30082/health

# 2. Deploy auth service components
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# 3. Verify deployment
kubectl get pods -l app=auth-service
kubectl logs deployment/auth-service | grep -i storage
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30083/health

# Check storage service integration
kubectl exec deployment/auth-service -- curl -s http://storage-service:8080/health
```

### Authentication Testing

```bash
# User registration
curl -X POST http://localhost:30083/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "securepassword123",
    "first_name": "Test",
    "last_name": "User"
  }'

# User login
curl -X POST http://localhost:30083/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "securepassword123"
  }'

# Token validation
TOKEN="<jwt-token-from-login>"
curl -X GET http://localhost:30083/api/v1/auth/token/validate \
  -H "Authorization: Bearer $TOKEN"
```

## 📊 Monitoring

### Metrics Endpoints

- **Health Status**: `http://localhost:30083/health`
- **Metrics**: `http://localhost:30083/metrics` (if enabled)
- **Dependencies**: Storage service connectivity in health endpoint

### Key Metrics

- JWT token generation rate
- Authentication success/failure rates
- Storage service integration status
- Response times for auth operations

## 🔧 Configuration

### Environment Variables

| Variable                  | Description              | Default                       |
| ------------------------- | ------------------------ | ----------------------------- |
| `JWT_SECRET`              | JWT signing secret       | From secret                   |
| `JWT_REFRESH_SECRET`      | Refresh token secret     | From secret                   |
| `JWT_EXPIRES_IN`          | Token expiration time    | `24h`                         |
| `STORAGE_SERVICE_URL`     | Storage service endpoint | `http://storage-service:8080` |
| `STORAGE_SERVICE_API_KEY` | Storage service API key  | From secret                   |
| `SERVER_PORT`             | HTTP server port         | `8080`                        |
| `BCRYPT_ROUNDS`           | Password hashing rounds  | `12`                          |

### JWT Configuration

- **Algorithm**: RS256 (RSA with SHA-256)
- **Token Expiration**: 24 hours (configurable)
- **Refresh Token**: 7 days (configurable)
- **Issuer**: `microservices-auth`

### Resource Limits

- **CPU**: 150m request, 400m limit
- **Memory**: 128Mi request, 256Mi limit

## 🚨 Troubleshooting

### Common Issues

**JWT Token Issues**

```bash
# Check JWT secret configuration
kubectl get secret auth-service-secrets -o yaml

# Verify JWT secret is properly base64 encoded
kubectl get secret auth-service-secrets -o jsonpath='{.data.JWT_SECRET}' | base64 -d

# Check token validation logs
kubectl logs deployment/auth-service | grep -i jwt
```

**Storage Service Integration Issues**

```bash
# Check storage service connectivity
kubectl exec deployment/auth-service -- nc -zv storage-service 8080

# Verify API key configuration
kubectl logs deployment/auth-service | grep -i "storage.*connection"

# Test storage service health
kubectl exec deployment/auth-service -- curl -s http://storage-service:8080/health
```

**Rate Limiting Issues**

```bash
# Check for rate limiting in logs
kubectl logs deployment/auth-service | grep -i "rate limit"

# Rate limiting may cause test failures - this is by design
# Use delays between requests for testing
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### Authentication Security

- **Password Hashing**: bcrypt with 12 rounds
- **JWT Signing**: RS256 algorithm
- **Token Validation**: Comprehensive token validation
- **Rate Limiting**: Protection against brute force attacks
- **API Key Protection**: Secure storage service communication

### Network Policies

- Ingress: Allows traffic from ingress controller, profile service, and monitoring
- Egress: Allows DNS and storage service communication only

## 🔐 JWT Token Structure

### Access Token Claims

```json
{
  "sub": "user-id",
  "email": "user@example.com",
  "iat": 1640995200,
  "exp": 1641081600,
  "iss": "microservices-auth",
  "aud": "microservices"
}
```

### Token Usage

- **Authorization Header**: `Authorization: Bearer <token>`
- **Token Validation**: All services can validate tokens
- **Refresh Flow**: Use refresh token to get new access token

## 📋 Service Information

| Aspect        | Details                              |
| ------------- | ------------------------------------ |
| **Namespace** | `default`                            |
| **Labels**    | `app=auth-service`, `component=auth` |
| **Replicas**  | 1 (Kind optimized)                   |
| **Strategy**  | RollingUpdate                        |
| **Protocol**  | HTTP only                            |

## 🔗 API Endpoints

| Endpoint                      | Method | Description       |
| ----------------------------- | ------ | ----------------- |
| `/health`                     | GET    | Health check      |
| `/api/v1/auth/register`       | POST   | User registration |
| `/api/v1/auth/login`          | POST   | User login        |
| `/api/v1/auth/token/validate` | GET    | Token validation  |
| `/api/v1/auth/token/refresh`  | POST   | Token refresh     |
| `/api/v1/auth/logout`         | POST   | User logout       |

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#auth-service-guide)
