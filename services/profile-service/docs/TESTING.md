# Profile Service Testing Guide

## Overview

This document provides comprehensive guidance for running and understanding the Profile Service test suite. The testing infrastructure validates the complete integration with the upgraded queue-service and multi-worker architecture implemented in Phase 4.

## Test Structure

```
test/
├── integration/          # End-to-end integration tests
│   ├── queue_service_test.go      # Queue service integration
│   └── multi_worker_test.go       # Multi-worker flow validation
└── performance/          # Performance and load tests
    └── load_test.go              # Throughput and latency tests
```

## Quick Start

### Run All Tests

```bash
# Full test suite (includes load tests)
go test ./test/... -v

# Short mode (skips heavy load tests)
go test ./test/... -v -short

# Run specific test suite
go test ./test/integration/... -v
go test ./test/performance/... -v -short
```

## Test Suites

### 1. Integration Tests (`test/integration/`)

#### Queue Service Integration (`queue_service_test.go`)

**Purpose**: Validates end-to-end task processing through the queue-service

**Tests Included**:

- `TestProfileTaskEndToEnd` - Profile update task flow
- `TestEmailTaskEndToEnd` - Email notification task flow
- `TestImageTaskEndToEnd` - Image processing task flow
- `TestTaskStatusTracking` - Task status updates
- `TestErrorHandling` - Error scenarios and recovery

**Expected Results**:

```
✅ TestProfileTaskEndToEnd - Profile tasks processed correctly
✅ TestEmailTaskEndToEnd - Email tasks routed to mock worker
✅ TestImageTaskEndToEnd - Image tasks routed to mock worker
✅ TestTaskStatusTracking - Status updates work properly
✅ TestErrorHandling - Graceful error handling
```

**Run Command**:

```bash
go test ./test/integration/queue_service_test.go -v
```

#### Multi-Worker Integration (`multi_worker_test.go`)

**Purpose**: Validates concurrent processing across multiple worker types

**Tests Included**:

- `TestMultiWorkerTaskProcessing` - Concurrent task processing
- `TestWorkerLoadBalancing` - Load distribution across workers
- `TestWorkerFailureHandling` - Worker failure scenarios
- `TestTaskPriorityHandling` - Priority-based task processing

**Expected Results**:

```
✅ All workers process tasks concurrently
✅ Load is distributed evenly across workers
✅ Failed workers don't block other tasks
✅ High-priority tasks processed first
```

**Run Command**:

```bash
go test ./test/integration/multi_worker_test.go -v
```

### 2. Performance Tests (`test/performance/`)

#### Load Testing (`load_test.go`)

**Purpose**: Validates system performance under load conditions

**Tests Included**:

- `TestAPIResponseTimeTarget` - API response time validation
- `TestErrorRateUnderLoad` - Error rate under heavy load
- `TestQueueServiceCommunicationPerformance` - Queue communication performance
- `TestConcurrentTaskSubmission` - Concurrent task handling

**Expected Results**:

```
✅ API Response Time: < 50ms average (typically 0.2-10ms)
✅ Error Rate: < 1% under load (typically < 0.1%)
✅ Queue Communication: < 100ms for fast/medium queues
✅ Throughput: > 1000 requests/second
✅ Concurrent Tasks: 100+ simultaneous tasks processed
```

**Run Commands**:

```bash
# Short performance tests (recommended)
go test ./test/performance/... -v -short

# Full load tests (heavy, use sparingly)
go test ./test/performance/... -v
```

## Performance Targets

| Metric              | Target         | Typical Results |
| ------------------- | -------------- | --------------- |
| API Response Time   | < 50ms         | 0.2-10ms        |
| Error Rate          | < 1%           | < 0.1%          |
| Throughput          | > 1000 req/sec | 1200+ req/sec   |
| Queue Communication | < 100ms        | 10-60ms         |
| Concurrent Tasks    | 100+           | 200+            |

## Test Environment Setup

### Prerequisites

- Go 1.21+
- Running queue-service instance
- Mock workers configured
- Test database available

### Environment Variables

```bash
# Queue service connection
QUEUE_SERVICE_URL=http://localhost:8080

# Database connection (test)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=profile_service_test
DB_USER=test_user
DB_PASSWORD=test_pass

# Worker configurations
PROFILE_WORKER_ENABLED=true
EMAIL_WORKER_ENABLED=true
IMAGE_WORKER_ENABLED=true
```

## Troubleshooting

### Common Issues

#### 1. "Profile task payload must be an object" Error

**Cause**: Payload type mismatch in specialized handlers
**Status**: ✅ **FIXED** - Handlers now convert structs to maps for validation

#### 2. "Response data should be EmailTaskResponse type" Error

**Cause**: JSON unmarshaling converts response data to maps
**Status**: ✅ **FIXED** - Tests now handle map-to-struct conversion

#### 3. Queue Service Connection Failures

**Cause**: Queue service not running or incorrect URL
**Solution**:

```bash
# Check queue service status
curl http://localhost:8080/health

# Start queue service if needed
cd ../queue-service && go run main.go
```

#### 4. Database Connection Issues

**Cause**: Test database not available
**Solution**:

```bash
# Create test database
createdb profile_service_test

# Run migrations
go run cmd/migrate/main.go
```

#### 5. Worker Mock Failures

**Cause**: Worker mock services not responding
**Solution**: Restart test suite, mocks are auto-configured

### Debug Commands

```bash
# Run with verbose output
go test ./test/... -v -run TestSpecificTest

# Run with race detection
go test ./test/... -race

# Run with coverage
go test ./test/... -cover -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out
```

## Test Maintenance

### Adding New Tests

1. Place integration tests in `test/integration/`
2. Place performance tests in `test/performance/`
3. Follow existing naming conventions
4. Include proper cleanup in test teardown
5. Update this documentation

### Performance Baselines

- Run performance tests weekly to track regression
- Update targets if system requirements change
- Monitor for performance degradation over time

### CI/CD Integration

```bash
# Recommended CI pipeline commands
go test ./test/integration/... -v -short
go test ./test/performance/... -v -short -timeout=10m
```

## Test Coverage

Current test coverage includes:
✅ API endpoint validation  
✅ Queue service integration
✅ Multi-worker processing
✅ Error handling scenarios
✅ Performance benchmarks
✅ Concurrent processing
✅ Task status tracking
✅ Response validation

## Success Criteria

Phase 4 testing is considered successful when:

- ✅ All integration tests pass consistently
- ✅ Performance targets are met
- ✅ Error rates stay below 1%
- ✅ No memory leaks or race conditions
- ✅ Clean shutdown and startup
- ✅ Proper error handling and recovery
