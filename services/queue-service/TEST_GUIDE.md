# Queue Service Test Guide

This guide explains how to run the comprehensive test suite for the upgraded queue-service and what to expect from each test.

## Prerequisites

1. **Dependencies**: Ensure you have the required tools installed:

   ```bash
   # Required for validation script
   curl --version
   jq --version

   # Required for Go tests
   go version
   ```

2. **Service**: Start the queue service before running tests:
   ```bash
   go run cmd/main.go
   ```

## Test Overview

The queue service has three types of tests:

1. **Validation Script** (`validate_service.sh`) - Quick bash-based validation
2. **Integration Tests** (`integration_test.go`) - Comprehensive Go test suite
3. **Multi-Worker Tests** (`multi_worker_test.go`) - Multi-worker isolation testing

---

## 1. Quick Validation Script

### How to Run

```bash
./validate_service.sh
```

### What It Does

- ✅ **Health Check**: Verifies service is running at `http://localhost:8080`
- ✅ **Routing Keys**: Tests `/api/v1/queue/routing-keys` endpoint
- ✅ **Message Publishing**: Tests all three worker types (`profile.task`, `email.send`, `image.process`)
- ✅ **Backward Compatibility**: Tests legacy API without routing keys
- ✅ **Validation**: Tests invalid routing key rejection
- ✅ **Message Format**: Tests worker-service compatible format

### Expected Output

```
🚀 Queue Service Validation
==========================

✅ Queue service is running at http://localhost:8080
✅ Routing keys endpoint working correctly
Available routing keys: profile.task, email.send, image.process

✅ Profile Task: Message published (ID: uuid-here)
✅ Profile Task: Message status retrieved (published)
✅ Email Task: Message published (ID: uuid-here)
✅ Email Task: Message status retrieved (published)
✅ Image Processing Task: Message published (ID: uuid-here)
✅ Image Processing Task: Message status retrieved (published)

✅ Backward compatibility: Routing key correctly inferred as profile.task
✅ Routing key validation: Invalid routing key correctly rejected
✅ Message format: Compatible with worker-service expectations

✅ All validation tests passed! 🎉

Queue service is ready for production use with:
• Worker-service compatible message format
• Multi-worker routing key support
• Backward compatibility maintained
• RabbitMQ best practices implemented
• Publisher confirms enabled
```

---

## 2. Integration Tests

### How to Run

```bash
# Run all integration tests
go test -v .

# Run specific test
go test -v . -run TestWorkerServiceCompatibility

# Run with race detection
go test -v -race .
```

### What It Does

#### **TestWorkerServiceCompatibility**

- Tests message publishing for all worker types
- Verifies worker-service compatible message format
- Tests `profile.task`, `email.send`, `image.process` routing
- Validates message status API

#### **TestBackwardCompatibility**

- Tests API without routing keys specified
- Verifies automatic routing key inference
- Ensures legacy clients continue working

#### **TestRoutingKeyValidation**

- Tests valid routing key acceptance
- Tests invalid routing key rejection
- Validates API error handling

#### **TestSupportedRoutingKeys**

- Tests `/api/v1/queue/routing-keys` endpoint
- Verifies all routing keys are present
- Validates configuration details

#### **TestMessageFormatCompatibility**

- Tests message serialization/deserialization
- Verifies field names match worker-service expectations
- Tests `metadata` vs `headers`, `json.RawMessage` payload

### Expected Output

```
🚀 Running integration tests against queue service at http://localhost:8080
=== RUN   TestWorkerServiceCompatibility
=== RUN   TestWorkerServiceCompatibility/Profile_Task
✅ Profile Task: Message published successfully with routing key profile.task
=== RUN   TestWorkerServiceCompatibility/Email_Task
✅ Email Task: Message published successfully with routing key email.send
=== RUN   TestWorkerServiceCompatibility/Image_Processing_Task
✅ Image Processing Task: Message published successfully with routing key image.process
--- PASS: TestWorkerServiceCompatibility (0.30s)

=== RUN   TestBackwardCompatibility
✅ Profile Update (Legacy): Routing key correctly inferred as profile.task
✅ Email Send (Legacy): Routing key correctly inferred as email.send
✅ Image Process (Legacy): Routing key correctly inferred as image.process
--- PASS: TestBackwardCompatibility (0.20s)

=== RUN   TestRoutingKeyValidation
✅ Valid Profile Routing Key: Valid routing key accepted
✅ Valid Email Routing Key: Valid routing key accepted
✅ Valid Image Routing Key: Valid routing key accepted
✅ Invalid Routing Key: Validation correctly rejected invalid routing key
--- PASS: TestRoutingKeyValidation (0.15s)

=== RUN   TestSupportedRoutingKeys
✅ Routing keys endpoint working correctly
--- PASS: TestSupportedRoutingKeys (0.05s)

=== RUN   TestMessageFormatCompatibility
✅ Message format compatible with worker-service expectations
--- PASS: TestMessageFormatCompatibility (0.02s)

✅ All integration tests passed!
PASS
```

---

## 3. Multi-Worker Tests

### How to Run

```bash
# Run multi-worker specific tests
go test -v . -run TestMultiWorker

# Run specific multi-worker test
go test -v . -run TestMultiWorkerRoutingIsolation
```

### What It Does

#### **TestMultiWorkerRoutingIsolation**

- Tests message isolation between worker types
- Verifies no cross-worker message contamination
- Tests `profile.task` → `tasks-exchange` → `profile-processing`
- Tests `email.send` → `email-tasks` → `email-processing`
- Tests `image.process` → `image-tasks` → `image-processing`

#### **TestDynamicExchangeAndQueueCreation**

- Tests dynamic RabbitMQ topology creation
- Verifies exchanges and queues created on first message
- Tests all worker types create proper infrastructure

#### **TestWorkerSpecificConfiguration**

- Verifies worker-specific configuration application
- Tests different prefetch counts per worker
- Validates exchange and queue naming patterns

#### **TestHighVolumeMessageDistribution**

- Tests 30+ messages across all worker types
- Verifies equal distribution (10 messages per worker)
- Tests high-throughput scenarios

#### **TestDeadLetterQueueConfiguration**

- Verifies dead letter queue setup
- Tests DLQ naming patterns
- Validates worker-specific DLQ configuration

### Expected Output

```
=== RUN   TestMultiWorkerRoutingIsolation
=== RUN   TestMultiWorkerRoutingIsolation/Profile_Worker_Isolation
✅ Profile Worker Isolation: Message routed to tasks-exchange -> profile-processing (ID: uuid)
=== RUN   TestMultiWorkerRoutingIsolation/Email_Worker_Isolation
✅ Email Worker Isolation: Message routed to email-tasks -> email-processing (ID: uuid)
=== RUN   TestMultiWorkerRoutingIsolation/Image_Worker_Isolation
✅ Image Worker Isolation: Message routed to image-tasks -> image-processing (ID: uuid)
=== RUN   TestMultiWorkerRoutingIsolation/CrossContaminationCheck
✅ Cross-contamination check: All messages properly isolated
--- PASS: TestMultiWorkerRoutingIsolation (0.25s)

=== RUN   TestDynamicExchangeAndQueueCreation
✅ Profile Exchange Creation: Dynamic topology created successfully
✅ Email Exchange Creation: Dynamic topology created successfully
✅ Image Exchange Creation: Dynamic topology created successfully
--- PASS: TestDynamicExchangeAndQueueCreation (0.15s)

=== RUN   TestWorkerSpecificConfiguration
✅ Worker-specific configurations verified:
   - Profile: tasks-exchange -> profile-processing (prefetch: 1)
   - Email: email-tasks -> email-processing (prefetch: 5)
   - Image: image-tasks -> image-processing (prefetch: 1)
--- PASS: TestWorkerSpecificConfiguration (0.08s)

=== RUN   TestHighVolumeMessageDistribution
✅ High-volume distribution test: 30 messages distributed across 3 worker types
--- PASS: TestHighVolumeMessageDistribution (0.50s)

=== RUN   TestDeadLetterQueueConfiguration
✅ Dead letter queue configuration verified for all worker types
--- PASS: TestDeadLetterQueueConfiguration (0.05s)
```

---

## Test Success Criteria

### ✅ All Tests Should Pass With:

1. **Worker-Service Compatibility**: All message formats accepted by worker-service
2. **Multi-Worker Support**: All 3 routing keys working (`profile.task`, `email.send`, `image.process`)
3. **Backward Compatibility**: Legacy API calls without routing keys work
4. **Validation**: Invalid routing keys properly rejected
5. **Message Format**: `metadata` field (not `headers`), `json.RawMessage` payload
6. **Routing Isolation**: No cross-worker message contamination
7. **Dynamic Topology**: Exchanges and queues created dynamically
8. **Configuration**: Worker-specific prefetch, TTL, and retry settings
9. **High Volume**: Service handles 30+ messages correctly
10. **Dead Letter Queues**: Proper DLQ configuration for all workers

---

## Troubleshooting

### Service Not Running

```
⚠️  Queue service not running at http://localhost:8080
```

**Solution**: Start the service with `go run cmd/main.go`

### Missing Dependencies

```
curl: command not found
jq: command not found
```

**Solution**: Install required tools:

```bash
# macOS
brew install curl jq

# Ubuntu/Debian
sudo apt-get install curl jq
```

### Test Failures

1. **Check Service Logs**: Look for RabbitMQ connection issues
2. **Check RabbitMQ**: Ensure RabbitMQ is running
3. **Check Configuration**: Verify environment variables
4. **Check Ports**: Ensure port 8080 is not blocked

---

## Configuration Testing

You can test different worker configurations using environment variables:

```bash
# Test with custom profile worker settings
export RABBITMQ_PROFILE_PREFETCH=2
export RABBITMQ_PROFILE_TTL=48h
export RABBITMQ_PROFILE_MAX_RETRIES=5

# Test with custom email worker settings
export RABBITMQ_EMAIL_PREFETCH=10
export RABBITMQ_EMAIL_TTL=30m
export RABBITMQ_EMAIL_MAX_RETRIES=3

# Restart service and run tests
go run cmd/main.go &
./validate_service.sh
```

---

## Summary

The test suite provides comprehensive validation of:

- ✅ **Critical Integration Fixes**: Message format compatibility
- ✅ **RabbitMQ Best Practices**: Publisher confirms, single exchange pattern
- ✅ **Multi-Worker Architecture**: Routing isolation and configuration
- ✅ **API Enhancements**: Routing key support with validation
- ✅ **Backward Compatibility**: Legacy client support
- ✅ **Production Readiness**: High-volume and error handling

**Expected Total Test Time**: ~2-3 minutes for full suite

**Production Readiness**: All tests passing indicates the service is ready for production deployment with full multi-worker architecture support.
