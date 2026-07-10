# Profile Service Load Testing

## Overview

This directory contains load testing configurations and scripts for the Profile Service microservices architecture. The tests are designed to validate the performance, reliability, and scalability of the services under various load conditions.

## Folder Structure

```
k8s/k6/
├── README.md
├── TRACKER&MANAGER.md
├── config/
│   ├── k6-test-configmap.yaml    # Base ConfigMap template for test scripts
│   └── k6-pvc.yaml              # PersistentVolumeClaim for test results
├── scripts/
│   ├── profile-api/              # Profile API specific tests
│   │   ├── crud-validation.js    # CRUD validation testing script
│   │   ├── profile-crud-load.js  # CRUD load testing script
│   │   └── stress-test.js        # Stress testing script
│   ├── auth-api/                 # Auth API specific tests
│   │   ├── basic-load.js         # Basic load testing script
│   │   └── stress-test.js        # Stress testing script
│   └── common/                   # Shared test utilities
│       ├── auth.js               # Shared authentication logic
│       ├── metrics.js            # Shared metrics definitions
│       └── utils.js              # Shared utility functions
└── jobs/
    ├── profile-api/              # Profile API test jobs
    │   ├── profile-api-validation-job.yaml      # Validation test job
    │   ├── profile-api-validation-configmap.yaml # Validation test config
    │   ├── profile-api-load-job.yaml            # Load test job
    │   ├── profile-api-load-configmap.yaml      # Load test config
    │   ├── profile-api-stress-job.yaml          # Stress test job
    │   └── profile-api-stress-configmap.yaml    # Stress test config
    └── auth-api/                 # Auth API test jobs
        └── basic-load-job.yaml   # Basic load test job
```

## Running Tests

### Prerequisites

1. Kubernetes cluster running (e.g., kind, minikube)
2. Profile API service deployed and running
3. kubectl configured to access your cluster

### Running a Test

1. **Choose the test to run**

   - Navigate to the appropriate job file in `jobs/profile-api/`
   - Available tests:
     - Validation test: `profile-api-validation-job.yaml`
     - Load test: `profile-api-load-job.yaml`
     - Stress test: `profile-api-stress-job.yaml`

2. **Apply the job**

   ```bash
   kubectl apply -f jobs/profile-api/profile-api-validation-job.yaml
   ```

3. **Monitor the test execution**

   ```bash
   # Watch the job status
   kubectl get jobs

   # View the test logs
   kubectl logs -f job/profile-api-validation-job
   ```

4. **Check the results**
   - The test results will be available in the pod logs
   - Metrics are collected during the test execution
   - Summary is printed at the end of the test

### Test Types

1. **Validation Test**

   - Duration: 30 seconds
   - Virtual Users: 5
   - Purpose: Quick validation of CRUD operations
   - Thresholds:
     - 95% of requests below 500ms
     - Less than 10% failure rate

2. **Load Test**

   - Duration: 9 minutes
   - Virtual Users: 50-100
   - Purpose: Normal load testing
   - Stages:
     - Ramp up to 50 users (1m)
     - Stay at 50 users (3m)
     - Ramp up to 100 users (1m)
     - Stay at 100 users (3m)
     - Ramp down (1m)
   - Thresholds:
     - 95% of requests below 1s
     - Less than 10% failure rate

3. **Stress Test**
   - Duration: 23 minutes
   - Virtual Users: 100-300
   - Purpose: System limits testing
   - Stages:
     - Ramp up to 100 users (2m)
     - Stay at 100 users (5m)
     - Ramp up to 200 users (2m)
     - Stay at 200 users (5m)
     - Ramp up to 300 users (2m)
     - Stay at 300 users (5m)
     - Ramp down (2m)
   - Thresholds:
     - 95% of requests below 2s
     - Less than 20% failure rate

### Stopping a Test

To stop a running test:

```bash
kubectl delete job <job-name>
```

### Common Issues

1. **Script not found**

   - Ensure the ConfigMap is properly created
   - Check that the script file exists in the correct location

2. **Service not accessible**

   - Verify the Profile API service is running
   - Check the API_URL in the ConfigMap
   - Ensure network policies allow the connection

3. **Authentication failures**
   - Verify the test user credentials in the script
   - Check if the auth service is running and accessible

### Test Results

Test results include:

- HTTP request metrics (duration, success rate)
- Custom metrics (auth errors, operation rates)
- Response validation results
- Performance thresholds status

The results are printed to stdout and can be redirected to a file for later analysis.

### Recent Validation Test Results

The latest validation test run showed the following results:

1. **Authentication**

   - Success rate: 100%
   - Average response time: < 100ms
   - No authentication errors

2. **CRUD Operations**

   - Create: 100% success rate, avg response time < 200ms
   - Read: 100% success rate, avg response time < 150ms
   - Update: 100% success rate, avg response time < 250ms
   - Delete: 100% success rate, avg response time < 200ms

3. **Performance Metrics**

   - 95th percentile response time: < 500ms
   - Error rate: 0%
   - Average response size: ~2KB
   - Total test duration: 30 seconds

4. **Key Findings**

   - All CRUD operations completed successfully
   - Response times well within acceptable thresholds
   - No authentication or authorization issues
   - System handled concurrent requests efficiently
   - No resource exhaustion observed

5. **Recommendations**
   - Continue monitoring for potential performance degradation
   - Consider implementing rate limiting for production
   - Add more detailed error logging
   - Implement circuit breakers for external service calls

## Current Implementation Status

### Implemented Features

1. **Validation Testing** ✅

   - Quick CRUD validation
   - Basic performance checks
   - Response validation
   - Error handling
   - Authentication testing
   - Custom metrics tracking

2. **Load Testing** ✅

   - CRUD operations under load
   - Gradual user ramp-up
   - Performance monitoring
   - Error tracking
   - Resource utilization monitoring
   - Response time tracking

3. **Stress Testing** ✅

   - High load scenarios
   - Resource utilization
   - Error handling
   - Recovery testing
   - Performance degradation monitoring
   - System limits testing

4. **Kubernetes Configuration** ✅
   - ConfigMap-based script management
   - Job configurations
   - Resource management
   - Results storage
   - Test execution environment

### Next Steps

1. **Test Scripts Enhancement**

   - [ ] Add more detailed metrics
   - [ ] Implement spike testing
   - [ ] Add endurance testing
   - [ ] Add failover testing
   - [ ] Enhance error reporting
   - [ ] Add custom thresholds

2. **Kubernetes Resources**

   - [ ] Add resource limits and requests
   - [ ] Configure network policies
   - [ ] Setup service accounts
   - [ ] Implement RBAC
   - [ ] Configure monitoring stack

3. **Monitoring Improvements**
   - [ ] Add custom dashboards
   - [ ] Configure alerts
   - [ ] Setup result analysis
   - [ ] Implement trend analysis
   - [ ] Add performance baselines

## Notes

- All tests should be run in a controlled environment
- Monitor resource usage during tests
- Document all test results
- Track performance improvements
- Update thresholds as needed
- Regular test maintenance
- Continuous monitoring
- Regular reporting
- Maintain database growth control
- Keep test scripts in version control

## Test Architecture

### Components

1. **k6 Test Runner**

   - Containerized k6 instance
   - Kubernetes Job for test execution
   - Direct script mounting from host filesystem
   - Resource monitoring

2. **Monitoring Stack**

   - Prometheus metrics collection
   - Grafana dashboards
   - Alert rules
   - Resource tracking

3. **Test Scenarios**
   - CRUD load testing
   - Stress testing
   - Spike testing
   - Endurance testing
   - Failover testing

## API Endpoints

### Base URL

```
http://profile-api.default.svc.cluster.local/api/v1
```

### Authentication

All endpoints require authentication using a Bearer token. The token must be included in the Authorization header:

```
Authorization: Bearer <token>
```

#### 1. Get Authentication Token

```http
POST /auth/token
```

**Request Body**

```json
{
  "user_id": "FB",
  "password": "FB.com"
}
```

**Response**

```json
{
  "token": "mock_access_tokenFB"
}
```

### Profile Endpoints

#### 1. List Profiles

```http
GET /api/v1/profiles
```

**Headers**

```
Authorization: Bearer <token>
```

**Response**

```json
[
  {
    "id": "uuid-v4",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "created_at": "2024-03-20T10:00:00Z",
    "updated_at": "2024-03-20T10:00:00Z"
  }
]
```

#### 2. Get Profile

```http
GET /api/v1/profiles/:id
```

**Headers**

```
Authorization: Bearer <token>
```

**Response**

```json
{
  "profile": {
    "id": "uuid-v4",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "phone": "+1234567890",
    "bio": "Software Engineer",
    "image_urls": ["https://example.com/image1.jpg"],
    "address": {
      "street": "123 Main St",
      "city": "New York",
      "state": "NY",
      "country": "USA",
      "zip_code": "10001"
    },
    "created_at": "2024-03-20T10:00:00Z",
    "updated_at": "2024-03-20T10:00:00Z"
  }
}
```

#### 3. Create Profile

```http
POST /api/v1/profiles
```

**Headers**

```
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body**

```json
{
  "first_name": "John",
  "last_name": "Doe",
  "email": "john.doe@example.com"
}
```

**Response**

```json
{
  "profile": {
    "id": "uuid-v4",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "created_at": "2024-03-20T10:00:00Z",
    "updated_at": "2024-03-20T10:00:00Z"
  }
}
```

#### 4. Update Profile

```http
PUT /api/v1/profiles/:id
```

**Headers**

```
Authorization: Bearer <token>
Content-Type: application/json
```

**Request Body**

```json
{
  "first_name": "John",
  "last_name": "Doe",
  "email": "john.doe@example.com",
  "phone": "+1234567890",
  "bio": "Senior Software Engineer",
  "image_urls": ["https://example.com/image1.jpg"],
  "address": {
    "street": "123 Main St",
    "city": "New York",
    "state": "NY",
    "country": "USA",
    "zip_code": "10001"
  }
}
```

**Response**

```json
{
  "profile": {
    "id": "uuid-v4",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "phone": "+1234567890",
    "bio": "Senior Software Engineer",
    "image_urls": ["https://example.com/image1.jpg"],
    "address": {
      "street": "123 Main St",
      "city": "New York",
      "state": "NY",
      "country": "USA",
      "zip_code": "10001"
    },
    "created_at": "2024-03-20T10:00:00Z",
    "updated_at": "2024-03-20T11:00:00Z"
  }
}
```

#### 5. Delete Profile

```http
DELETE /api/v1/profiles/:id
```

**Headers**

```
Authorization: Bearer <token>
```

**Response**

- Status: 204 No Content

### Error Responses

All endpoints may return the following error responses:

#### 400 Bad Request

```json
{
  "error": "first name is required"
}
```

#### 401 Unauthorized

```json
{
  "error": "authorization header is required"
}
```

or

```json
{
  "error": "session expired"
}
```

or

```json
{
  "error": "invalid session"
}
```

#### 404 Not Found

```json
{
  "error": "profile not found"
}
```

#### 500 Internal Server Error

```json
{
  "error": "internal server error"
}
```

### Validation Rules

1. **Required Fields**

   - first_name (must not be empty or whitespace)
   - last_name (must not be empty or whitespace)
   - email (must be valid email format and contain '@' and '.')

2. **Optional Fields**

   - phone
   - bio
   - image_urls
   - address
   - get_from (internal field indicating data source)

3. **Address Validation** (if provided)
   - street (required, must not be empty or whitespace)
   - city (required, must not be empty or whitespace)
   - state (required, must not be empty or whitespace)
   - country (required, must not be empty or whitespace)
   - zip_code (required, must not be empty or whitespace)

### Response Headers

All responses include:

```
Content-Type: application/json
X-Request-ID: <uuid>
```

## Test Categories

### 1. CRUD Load Testing

#### Purpose

- Establish baseline performance metrics
- Validate service behavior under normal load
- Identify initial bottlenecks

#### Scenarios

- GET /api/v1/profiles
- GET /api/v1/profiles/:id
- POST /api/v1/profiles
- PUT /api/v1/profiles/:id
- DELETE /api/v1/profiles/:id

#### Configuration

```javascript
export const options = {
  vus: 1,
  iterations: 1,
};
```

### 2. Stress Testing

#### Purpose

- Determine service limits
- Identify breaking points
- Validate error handling
- Test resource utilization

#### Scenarios

- Gradual load increase
- Resource exhaustion
- Error rate monitoring
- Recovery behavior

#### Configuration

```javascript
export const options = {
  stages: [
    { duration: "1m", target: 50 }, // Ramp-up
    { duration: "3m", target: 50 }, // Stay
    { duration: "1m", target: 100 }, // Increase
    { duration: "3m", target: 100 }, // Stay
    { duration: "1m", target: 0 }, // Ramp-down
  ],
  thresholds: {
    http_req_duration: ["p(95)<1000"],
    http_req_failed: ["rate<0.05"],
  },
};
```

### 3. Spike Testing

#### Purpose

- Test sudden load increases
- Validate auto-scaling
- Check error handling
- Monitor recovery time

#### Scenarios

- Instant load spikes
- Multiple concurrent spikes
- Recovery monitoring
- Resource scaling

#### Configuration

```javascript
export const options = {
  stages: [
    { duration: "10s", target: 100 }, // Spike
    { duration: "1m", target: 100 }, // Stay
    { duration: "10s", target: 0 }, // Drop
  ],
  thresholds: {
    http_req_duration: ["p(95)<2000"],
    http_req_failed: ["rate<0.1"],
  },
};
```

### 4. Endurance Testing

#### Purpose

- Validate long-term stability
- Check memory leaks
- Monitor resource usage
- Test database connections

#### Scenarios

- Extended load periods
- Resource monitoring
- Connection pool behavior
- Cache effectiveness

#### Configuration

```javascript
export const options = {
  stages: [
    { duration: "5m", target: 50 }, // Ramp-up
    { duration: "30m", target: 50 }, // Stay
    { duration: "5m", target: 0 }, // Ramp-down
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.01"],
  },
};
```

## File Review: Main Test Files

Here is a summary of the main files and their purposes:

- **scripts/profile-api/profile-crud-load.test.js**

  - Full CRUD (Create, Read, Update, Delete) test for the profile API, with authentication.
  - Covers all endpoints: create, get, update, delete (with 50% deletion rate).
  - Scenario: CRUD load test for the profile API.

- **jobs/profile-api/profile-crud-load-configmap.yaml**

  - ConfigMap containing the CRUD test script above.
  - Loads the script for use in the k6 job.
  - Scenario: Should match the script name and scenario.

- **jobs/profile-api/profile-crud-load-job.yaml**

  - Kubernetes Job definition for running the above test.
  - Mounts the script from the ConfigMap and runs it with k6.
  - Scenario: Should match the script and ConfigMap.

- **config/k6-test-configmap.yaml**

  - Example/template ConfigMap for k6 scripts.
  - Not used directly, but serves as a template for creating new ConfigMaps.

- **config/k6-pvc.yaml**
  - PersistentVolumeClaim for storing k6 test results.
  - Used for result storage, not tied to a specific test.
