# Load Testing Strategy

> *Migrated from legacy_project/reference-materials/performance/load-testing-strategy.md*

## Overview

This document outlines our load testing strategy using k6, a modern load testing tool that integrates well with our architecture.

## Why k6?

k6 was chosen for our load testing needs because:

1. **Integration with Existing Stack**

   - Native integration with Grafana
   - Compatible with our Kubernetes environment
   - Works with our existing monitoring setup

2. **Protocol Support**

   - Native support for REST APIs
   - WebSocket support
   - Custom protocol support via JavaScript

3. **Developer Experience**

   - JavaScript-based test scripts
   - Modern, developer-friendly approach
   - Good documentation and community support

4. **Performance and Scalability**
   - Efficient resource usage
   - Support for distributed testing
   - Real-time metrics collection

## Test Scenarios

### 1. Basic Load Test

```javascript
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "30s", target: 20 }, // Ramp up to 20 users
    { duration: "1m", target: 20 }, // Stay at 20 users
    { duration: "30s", target: 0 }, // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"], // 95% of requests should be below 500ms
    http_req_failed: ["rate<0.01"], // Less than 1% of requests should fail
  },
};

export default function () {
  const BASE_URL = __ENV.BASE_URL || "http://api-service:8080";

  // Test List Profiles
  const listResponse = http.get(`${BASE_URL}/api/v1/profiles`);
  check(listResponse, {
    "list profiles status is 200": (r) => r.status === 200,
    "list profiles has data": (r) => r.json().profiles !== undefined,
  });

  // Test Create Profile
  const createPayload = JSON.stringify({
    first_name: "Test",
    last_name: "User",
    email: `test${__VU}@example.com`,
  });

  const createResponse = http.post(
    `${BASE_URL}/api/v1/profiles`,
    createPayload,
    {
      headers: { "Content-Type": "application/json" },
    }
  );

  check(createResponse, {
    "create profile status is 201": (r) => r.status === 201,
    "create profile has id": (r) => r.json().id !== undefined,
  });

  sleep(1);
}
```

### 2. Stress Test

```javascript
export const options = {
  stages: [
    { duration: "2m", target: 100 }, // Ramp up to 100 users
    { duration: "5m", target: 100 }, // Stay at 100 users
    { duration: "2m", target: 200 }, // Ramp up to 200 users
    { duration: "5m", target: 200 }, // Stay at 200 users
    { duration: "2m", target: 0 }, // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ["p(95)<1000"], // 95% of requests should be below 1s
    http_req_failed: ["rate<0.05"], // Less than 5% of requests should fail
  },
};
```

### 3. Spike Test

```javascript
export const options = {
  stages: [
    { duration: "10s", target: 50 }, // Ramp up to 50 users
    { duration: "1m", target: 50 }, // Stay at 50 users
    { duration: "10s", target: 200 }, // Spike to 200 users
    { duration: "3m", target: 200 }, // Stay at 200 users
    { duration: "10s", target: 50 }, // Ramp down to 50 users
    { duration: "3m", target: 50 }, // Stay at 50 users
    { duration: "10s", target: 0 }, // Ramp down to 0 users
  ],
};
```

### 4. Cache Performance Test

```javascript
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  stages: [
    { duration: "1m", target: 50 },
    { duration: "2m", target: 50 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<100"], // Cached requests should be fast
    "http_req_duration{cache:hit}": ["p(95)<50"],
    "http_req_duration{cache:miss}": ["p(95)<200"],
  },
};

export default function () {
  const BASE_URL = __ENV.BASE_URL || "http://api-service:8080";
  const profileId = "test-profile-id";

  // First request - likely cache miss
  const firstResponse = http.get(`${BASE_URL}/api/v1/profiles/${profileId}`, {
    tags: { cache: "miss" },
  });

  check(firstResponse, {
    "first request status is 200": (r) => r.status === 200,
  });

  sleep(0.1);

  // Second request - should be cache hit
  const secondResponse = http.get(`${BASE_URL}/api/v1/profiles/${profileId}`, {
    tags: { cache: "hit" },
  });

  check(secondResponse, {
    "second request status is 200": (r) => r.status === 200,
    "cache hit is faster": (r) => r.timings.duration < 50,
  });

  sleep(1);
}
```

## Metrics to Monitor

1. **Response Time**

   - p95, p99 latencies
   - Average response time
   - Time to first byte

2. **Error Rates**

   - HTTP error rates
   - Failed requests
   - Timeout rates

3. **Resource Usage**

   - CPU utilization
   - Memory usage
   - Network I/O

4. **Business Metrics**
   - Requests per second
   - Concurrent users
   - Success rate

## Running Tests

### Local Development

```bash
# Run a specific test
k6 run tests/basic-load.js

# Run with environment variables
k6 run -e BASE_URL=http://localhost:8080 tests/basic-load.js

# Output to JSON for analysis
k6 run --out json=results.json tests/basic-load.js
```

### Kubernetes

```bash
# Deploy k6 job
kubectl apply -f k8s/k6-job.yaml

# Monitor test progress
kubectl logs -f job/k6-load-test
```

## Test Results Analysis

1. **Grafana Dashboards**

   - Real-time metrics
   - Historical data
   - Custom visualizations

2. **Alerting**

   - Response time thresholds
   - Error rate thresholds
   - Resource usage alerts

3. **Reporting**
   - Test summary
   - Performance trends
   - Bottleneck analysis

## Best Practices

1. **Test Design**

   - Start with basic scenarios
   - Gradually increase complexity
   - Include error scenarios
   - Test all critical paths

2. **Environment**

   - Use production-like data
   - Monitor all components
   - Isolate test environment
   - Clean up test data

3. **Execution**

   - Run tests during off-peak hours
   - Monitor system resources
   - Have rollback plan
   - Document test results

4. **Analysis**
   - Compare with baselines
   - Identify bottlenecks
   - Track improvements
   - Share findings

## Cross-References

- [Benchmarking](benchmarking.md)
- [Monitoring](monitoring.md)
- [Optimization](optimization.md)
- [Prometheus Guide](../development/tools/prometheus.md)
