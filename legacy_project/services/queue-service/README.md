# Queue Service

A production-ready microservice that provides reliable message queuing capabilities using RabbitMQ as the message broker. This service acts as the central message publisher in a microservices architecture, routing messages to different worker types for asynchronous processing.

## Status

**Current Status**: 🎉 **PRODUCTION READY** - Complete multi-worker architecture with RabbitMQ best practices implementation.

## Features

- Multi-worker message routing (profile, email, image)
- Publisher confirms for reliable delivery
- RabbitMQ best practices implementation
- Comprehensive testing and validation
- Production-ready monitoring and health checks

## Testing and Validation

The service includes comprehensive testing infrastructure to validate all functionality:

### **Test Suite Overview**

1. **Validation Script** (`validate_service.sh`) - Quick bash-based validation
2. **Integration Tests** (`integration_test.go`) - Comprehensive Go test suite
3. **Multi-Worker Tests** (`multi_worker_test.go`) - Multi-worker isolation testing

### **Running Tests**

```bash
# Run all tests
go test -v .

# Run validation script (requires service running)
./validate_service.sh

# Run specific test categories
go test -v . -run TestWorkerServiceCompatibility
go test -v . -run TestMultiWorker
```

### **Test Coverage**

- **Worker-Service Compatibility**: Message format compatibility validation
- **Routing Key Distribution**: All three routing keys (`profile.task`, `email.send`, `image.process`)
- **Backward Compatibility**: Legacy API support without routing keys
- **Message Format Compliance**: `metadata` field, `json.RawMessage` payload validation
- **Multi-Worker Isolation**: No cross-worker message contamination
- **Publisher Confirms**: Reliable delivery validation
- **High-Volume Testing**: 30+ message distribution testing

### **Expected Test Results**

All tests should pass with:
- ✅ **99%+ Publisher Confirm Success Rate**
- ✅ **Zero Cross-Worker Message Contamination**
- ✅ **Complete Routing Key Validation**
- ✅ **Worker-Service Message Format Compatibility**
- ✅ **Backward Compatibility Maintained**

## Migration and Upgrade

The service has been fully upgraded from the original implementation. Key migration benefits achieved:

### **Migration Completed**
- ✅ **Worker-Service Compatibility**: Messages can be consumed by worker-service
- ✅ **Multi-Worker Support**: Support for email and image workers
- ✅ **RabbitMQ Best Practices**: Aligned with industry standards
- ✅ **Publisher Confirms**: Reliable message delivery
- ✅ **Enhanced Monitoring**: Per-worker-type metrics

### **Breaking Changes Implemented**
- **Message Format**: Changed from `Headers` to `Metadata`, `interface{}` to `json.RawMessage`
- **Exchange Strategy**: Single exchange with routing keys instead of per-queue exchanges
- **API Enhancement**: Added routing key support (backward compatible)

### **Rollback Procedures**
If issues occur, emergency rollback procedures are available:
```bash
# Emergency rollback (if needed)
kubectl scale deployment queue-service --replicas=0
kubectl apply -f ./queue-service-backup-$(date +%Y%m%d)/
kubectl rollout restart deployment/queue-service
```

## Documentation

- `INTERFACE.md` - Service interfaces and API endpoints
- `CONTEXT.md` - Technical implementation details
- `TRACKER.md` - Implementation progress (COMPLETE)
- `IMPLEMENTATION_HISTORY.md` - Complete development history

## Quick Start

```bash
# Run the service
go run cmd/main.go

# Test message publishing
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "routing_key": "profile.task",
    "payload": {"user_id": "123", "action": "update"},
    "metadata": {"source": "test"}
  }'

# Check health
curl http://localhost:8080/health

# Validate all functionality
./validate_service.sh
```

For detailed technical information, see the documentation files listed above.
