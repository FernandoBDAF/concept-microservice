# Queue Service Migration Guide

## Overview

This document provides a comprehensive guide for migrating the queue-service from the current implementation to the new RabbitMQ best practices aligned architecture with multi-worker support.

**⚠️ CRITICAL**: This migration involves **breaking changes** to message format and routing strategy. Careful planning and execution are required to prevent message loss and service disruption.

## Migration Summary

### **What's Changing**

#### **1. Message Format (BREAKING CHANGE)**

```go
// BEFORE (Current - Broken)
type Message struct {
    Type    MessageType       `json:"type"`        // Enum
    Headers map[string]string `json:"headers"`     // Called "headers"
    Payload interface{}       `json:"payload"`     // interface{}
}

// AFTER (Target - Compatible)
type Message struct {
    Type     string            `json:"type"`       // String
    Metadata map[string]string `json:"metadata"`   // Called "metadata"
    Payload  json.RawMessage   `json:"payload"`    // json.RawMessage
}
```

#### **2. Exchange Strategy (BREAKING CHANGE)**

```go
// BEFORE (Current - Wrong)
exchangeName := queueName + ".exchange"  // Creates multiple exchanges
err = channel.Publish(exchangeName, queueName, ...)

// AFTER (Target - Best Practice)
err = channel.Publish("tasks-exchange", "profile.task", ...)  // Single exchange + routing keys
```

#### **3. API Enhancement (BACKWARD COMPATIBLE)**

```json
// NEW: Routing key support
{
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": {...},
  "metadata": {...}
}
```

### **Migration Benefits**

- ✅ **Worker-Service Compatibility**: Messages can be consumed by worker-service
- ✅ **Multi-Worker Support**: Support for email and image workers
- ✅ **RabbitMQ Best Practices**: Aligned with industry standards
- ✅ **Publisher Confirms**: Reliable message delivery
- ✅ **Enhanced Monitoring**: Per-worker-type metrics
- ✅ **Simplified Architecture**: Cleaner, more maintainable code

## Pre-Migration Checklist

### **1. Environment Assessment**

#### **Current State Verification**

```bash
# Check current queue-service version
kubectl get deployment queue-service -o jsonpath='{.spec.template.spec.containers[0].image}'

# Verify RabbitMQ state
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_exchanges
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_bindings

# Check current message flow
kubectl logs deployment/queue-service | grep "published\|consumed"
kubectl logs deployment/worker-service | grep "received\|processed"
```

#### **Backup Current Configuration**

```bash
# Backup RabbitMQ definitions
kubectl exec -it rabbitmq-0 -- rabbitmqctl export_definitions /tmp/backup.json
kubectl cp rabbitmq-0:/tmp/backup.json ./rabbitmq-backup-$(date +%Y%m%d).json

# Backup current manifests
cp -r k8s/profile-service/base/queue-service/ ./queue-service-backup-$(date +%Y%m%d)/
cp -r k8s/profile-service/base/worker-service/ ./worker-service-backup-$(date +%Y%m%d)/
```

### **2. Dependency Verification**

#### **Worker-Service Compatibility**

```bash
# Verify worker-service uses common queue package
grep -r "github.com/fernandobarroso/common/queue" services/worker-service/

# Check worker-service message format expectations
grep -A 10 -B 5 "Metadata\|json.RawMessage" services/worker-service/
```

#### **Profile-Service Integration**

```bash
# Check profile-service queue client implementation
grep -A 20 "PublishMessage" services/profile-service/internal/pkg/messaging/queue_client.go
```

### **3. Testing Environment Setup**

#### **Create Migration Test Environment**

```bash
# Create test namespace
kubectl create namespace queue-migration-test

# Deploy test RabbitMQ
kubectl apply -f k8s/debug/queue-service-upgrade/rabbitmq.yaml -n queue-migration-test

# Deploy current queue-service for comparison
kubectl apply -f k8s/profile-service/base/queue-service/ -n queue-migration-test
```

## Migration Phases

### **Phase 1: Message Format Alignment** [CRITICAL - 4 hours]

#### **1.1 Update Message Model**

**File**: `services/queue-service/internal/domain/model/message.go`

```go
// Replace MessageType enum with string
type Message struct {
    ID        string            `json:"id" validate:"required"`
    Type      string            `json:"type" validate:"required"`  // CHANGED: string instead of MessageType
    Timestamp time.Time         `json:"timestamp" validate:"required"`
    CorrelationID string        `json:"correlation_id"`
    Payload   json.RawMessage   `json:"payload" validate:"required"`  // CHANGED: json.RawMessage
    Metadata  map[string]string `json:"metadata"`                     // CHANGED: renamed from Headers
    Priority  int32             `json:"priority" validate:"min=0,max=9"`
}

// Remove MessageType constants - use strings directly
```

#### **1.2 Update HTTP Handler for Backward Compatibility**

**File**: `services/queue-service/internal/adapters/http/handler.go`

```go
// Add backward compatibility layer
type PublishRequest struct {
    Type       string            `json:"type" binding:"required"`
    RoutingKey string            `json:"routing_key"`              // NEW: optional for backward compatibility
    Payload    json.RawMessage   `json:"payload" binding:"required"`
    Metadata   map[string]string `json:"metadata"`                 // NEW
    Headers    map[string]string `json:"headers"`                  // DEPRECATED: for backward compatibility
    Priority   int32             `json:"priority"`
}

func (h *Handler) PublishMessage(c *gin.Context) {
    var req PublishRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "invalid_request", "details": err.Error()})
        return
    }

    // Backward compatibility: headers → metadata
    if req.Metadata == nil && req.Headers != nil {
        req.Metadata = req.Headers
    }

    // Default routing key for backward compatibility
    if req.RoutingKey == "" {
        req.RoutingKey = "profile.task"  // Default to profile worker
    }

    // ... rest of handler logic
}
```

#### **1.3 Testing Message Format Changes**

```bash
# Test new message format
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "routing_key": "profile.task",
    "payload": {"user_id": "test123", "action": "update"},
    "metadata": {"source": "migration_test"}
  }'

# Test backward compatibility
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "payload": {"user_id": "test123", "action": "update"},
    "headers": {"source": "legacy_test"}
  }'
```

### **Phase 2: Exchange Strategy Overhaul** [CRITICAL - 6 hours]

#### **2.1 Update RabbitMQ Client**

**File**: `services/queue-service/internal/adapters/rabbitmq/rabbitmq.go`

```go
// Replace complex per-queue exchange setup
func (r *RabbitMQ) setupExchangesAndQueues() error {
    // Define exchange and queue configurations
    configs := []struct {
        Exchange   string
        Queue      string
        RoutingKey string
        TTL        time.Duration
    }{
        {"tasks-exchange", "profile-processing", "profile.task", 24 * time.Hour},
        {"email-tasks", "email-processing", "email.send", 1 * time.Hour},
        {"image-tasks", "image-processing", "image.process", 6 * time.Hour},
    }

    for _, config := range configs {
        // Declare exchange
        err := r.channel.ExchangeDeclare(
            config.Exchange, "direct", true, false, false, false, nil,
        )
        if err != nil {
            return fmt.Errorf("failed to declare exchange %s: %w", config.Exchange, err)
        }

        // Declare queue
        _, err = r.channel.QueueDeclare(
            config.Queue, true, false, false, false,
            amqp.Table{
                "x-message-ttl":             int32(config.TTL.Milliseconds()),
                "x-dead-letter-exchange":    config.Queue + ".dlx",
                "x-dead-letter-routing-key": config.Queue,
            },
        )
        if err != nil {
            return fmt.Errorf("failed to declare queue %s: %w", config.Queue, err)
        }

        // Bind queue to exchange
        err = r.channel.QueueBind(
            config.Queue, config.RoutingKey, config.Exchange, false, nil,
        )
        if err != nil {
            return fmt.Errorf("failed to bind queue %s: %w", config.Queue, err)
        }
    }

    return nil
}

// Simplified publish method
func (r *RabbitMQ) Publish(routingKey string, msg *model.Message) error {
    // Get exchange for routing key
    exchange, err := r.getExchangeForRoutingKey(routingKey)
    if err != nil {
        return err
    }

    // Ensure exchange and queue setup
    if err := r.setupExchangesAndQueues(); err != nil {
        return err
    }

    // Marshal message
    body, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    // Publish to exchange with routing key
    return r.channel.Publish(
        exchange,
        routingKey,
        false, // mandatory
        false, // immediate
        amqp.Publishing{
            ContentType:  "application/json",
            Body:         body,
            DeliveryMode: amqp.Persistent,
            MessageId:    msg.ID,
            Timestamp:    msg.Timestamp,
        },
    )
}

func (r *RabbitMQ) getExchangeForRoutingKey(routingKey string) (string, error) {
    switch routingKey {
    case "profile.task":
        return "tasks-exchange", nil
    case "email.send":
        return "email-tasks", nil
    case "image.process":
        return "image-tasks", nil
    default:
        return "", fmt.Errorf("invalid routing key: %s", routingKey)
    }
}
```

#### **2.2 Testing Exchange Strategy**

```bash
# Deploy updated queue-service
kubectl apply -k k8s/profile-service/base/queue-service/

# Verify exchange setup
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_exchanges
# Should show: tasks-exchange, email-tasks, image-tasks

# Verify queue bindings
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_bindings
# Should show proper routing key bindings

# Test message routing
curl -X POST http://queue-service:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "routing_key": "profile.task",
    "payload": {"user_id": "test123"},
    "metadata": {"test": "routing"}
  }'

# Verify message in correct queue
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages
```

### **Phase 3: Publisher Confirms Implementation** [HIGH PRIORITY - 3 hours]

#### **3.1 Add Publisher Confirms**

**File**: `services/queue-service/internal/adapters/rabbitmq/rabbitmq.go`

```go
type RabbitMQ struct {
    conn     *amqp.Connection
    channel  *amqp.Channel
    confirms <-chan amqp.Confirmation  // NEW
    config   *Config
}

func (r *RabbitMQ) connect() error {
    // ... existing connection logic ...

    // Enable publisher confirms
    if err := r.channel.Confirm(false); err != nil {
        return fmt.Errorf("failed to enable publisher confirms: %w", err)
    }

    r.confirms = r.channel.NotifyPublish(make(chan amqp.Confirmation, 1))

    return nil
}

func (r *RabbitMQ) PublishWithConfirm(routingKey string, msg *model.Message) error {
    // Publish message
    if err := r.Publish(routingKey, msg); err != nil {
        return err
    }

    // Wait for publisher confirm
    select {
    case confirm := <-r.confirms:
        if confirm.Ack {
            return nil
        } else {
            return fmt.Errorf("message nacked by broker")
        }
    case <-time.After(5 * time.Second):
        return fmt.Errorf("publisher confirm timeout")
    }
}
```

#### **3.2 Update Service Layer**

**File**: `services/queue-service/internal/domain/service/queue.go`

```go
func (s *QueueService) PublishMessage(msg *model.Message, routingKey string) error {
    start := time.Now()

    // Store message status
    s.mu.Lock()
    s.messageStore[msg.ID] = &model.MessageStatus{
        ID:         msg.ID,
        Status:     "accepted",
        RoutingKey: routingKey,
        Timestamp:  time.Now().UTC().Format(time.RFC3339),
    }
    s.mu.Unlock()

    // Publish with confirm
    if err := s.rabbitmq.PublishWithConfirm(routingKey, msg); err != nil {
        s.updateMessageStatus(msg.ID, "failed")
        s.metrics.ErrorsTotal.Inc()
        return fmt.Errorf("failed to publish message: %w", err)
    }

    // Update status on successful confirm
    s.updateMessageStatus(msg.ID, "confirmed")
    s.metrics.MessagesTotal.Inc()
    s.metrics.ProcessingDuration.Observe(time.Since(start).Seconds())

    return nil
}
```

### **Phase 4: Integration Testing** [MEDIUM PRIORITY - 4 hours]

#### **4.1 End-to-End Integration Test**

```bash
# Create comprehensive test environment
kubectl apply -f k8s/debug/queue-service-upgrade/

# Test message flow: profile-service → queue-service → worker-service
# 1. Publish message via profile-service
curl -X POST http://profile-service:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test123",
    "action": "update",
    "data": {"name": "Test User"}
  }'

# 2. Verify message in queue-service
kubectl logs deployment/queue-service | grep "published.*profile.task"

# 3. Verify message consumption by worker-service
kubectl logs deployment/worker-service | grep "received.*profile.task"

# 4. Check RabbitMQ state
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages consumers
```

#### **4.2 Load Testing**

```bash
# Simple load test script
cat << 'EOF' > load_test.sh
#!/bin/bash
for i in {1..100}; do
  curl -X POST http://queue-service:8080/api/v1/queue/messages \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"profile_update\",
      \"routing_key\": \"profile.task\",
      \"payload\": {\"user_id\": \"user$i\", \"action\": \"update\"},
      \"metadata\": {\"test\": \"load_test_$i\"}
    }" &
done
wait
EOF

chmod +x load_test.sh
./load_test.sh

# Monitor results
kubectl logs deployment/queue-service | grep -c "accepted"
kubectl logs deployment/worker-service | grep -c "processed"
```

### **Phase 5: Production Deployment** [MEDIUM PRIORITY - 2 hours]

#### **5.1 Blue-Green Deployment Strategy**

```bash
# 1. Deploy new version alongside current
kubectl apply -f k8s/profile-service/base/queue-service/ --dry-run=client -o yaml > queue-service-new.yaml
sed 's/queue-service/queue-service-new/g' queue-service-new.yaml | kubectl apply -f -

# 2. Test new version
kubectl port-forward service/queue-service-new 8081:8080 &
curl -X POST http://localhost:8081/api/v1/queue/messages -d '...'

# 3. Switch traffic gradually
kubectl patch service queue-service -p '{"spec":{"selector":{"version":"new"}}}'

# 4. Monitor for issues
kubectl logs deployment/queue-service-new -f

# 5. Clean up old version (after validation)
kubectl delete deployment queue-service-old
```

#### **5.2 Rollback Procedure**

```bash
# Emergency rollback if issues occur
kubectl patch service queue-service -p '{"spec":{"selector":{"version":"old"}}}'
kubectl scale deployment queue-service-new --replicas=0
kubectl scale deployment queue-service-old --replicas=3
```

## Post-Migration Validation

### **1. Functional Validation**

#### **Message Flow Verification**

```bash
# Test all routing keys
routing_keys=("profile.task" "email.send" "image.process")
for key in "${routing_keys[@]}"; do
  echo "Testing routing key: $key"
  curl -X POST http://queue-service:8080/api/v1/queue/messages \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"test_message\",
      \"routing_key\": \"$key\",
      \"payload\": {\"test\": \"data\"},
      \"metadata\": {\"validation\": \"post_migration\"}
    }"
done

# Verify messages route to correct queues
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages
```

#### **Publisher Confirms Validation**

```bash
# Check publisher confirm metrics
curl http://queue-service:8080/metrics | grep "queue_messages_confirmed"
curl http://queue-service:8080/metrics | grep "queue_publisher_confirm"
```

#### **Worker Integration Validation**

```bash
# Verify worker-service can process new message format
kubectl logs deployment/worker-service | grep "processed successfully"
kubectl logs deployment/worker-service | grep -v "unmarshal error\|parse error"
```

### **2. Performance Validation**

#### **Throughput Testing**

```bash
# Measure message throughput
start_time=$(date +%s)
for i in {1..1000}; do
  curl -s -X POST http://queue-service:8080/api/v1/queue/messages \
    -H "Content-Type: application/json" \
    -d "{\"type\":\"perf_test\",\"routing_key\":\"profile.task\",\"payload\":{\"id\":$i}}" > /dev/null &
done
wait
end_time=$(date +%s)
echo "Published 1000 messages in $((end_time - start_time)) seconds"
```

#### **Latency Testing**

```bash
# Measure end-to-end latency
curl -w "Total time: %{time_total}s\n" \
  -X POST http://queue-service:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{"type":"latency_test","routing_key":"profile.task","payload":{"timestamp":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}}'
```

### **3. Monitoring Validation**

#### **Metrics Verification**

```bash
# Check all new metrics are being collected
curl http://queue-service:8080/metrics | grep -E "(routing_key|publisher_confirm|exchange_health)"

# Verify Prometheus is scraping metrics
kubectl logs deployment/prometheus | grep queue-service
```

#### **Health Check Validation**

```bash
# Verify enhanced health checks
curl http://queue-service:8080/health | jq '.checks'

# Should show:
# - rabbitmq_connection: "healthy"
# - rabbitmq_channel: "healthy"
# - exchange_status: "healthy"
# - publisher_confirms: "enabled"
```

## Troubleshooting

### **Common Migration Issues**

#### **1. Message Format Compatibility Issues**

**Symptom**: Worker-service cannot parse messages

```bash
kubectl logs deployment/worker-service | grep "unmarshal\|parse"
```

**Solution**: Check message format alignment

```bash
# Compare message formats
kubectl exec -it rabbitmq-0 -- rabbitmqctl get_queue profile-processing 1
# Verify message structure matches worker expectations
```

#### **2. Routing Key Problems**

**Symptom**: Messages not reaching target queues

```bash
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages
# Shows messages stuck or going to wrong queues
```

**Solution**: Verify exchange bindings

```bash
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_bindings
# Check routing key → queue mappings
```

#### **3. Publisher Confirm Failures**

**Symptom**: High publisher confirm timeout errors

```bash
kubectl logs deployment/queue-service | grep "publisher confirm timeout"
```

**Solution**: Check RabbitMQ performance and connection health

```bash
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_connections
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_channels
```

### **Emergency Procedures**

#### **1. Immediate Rollback**

```bash
# Emergency rollback script
cat << 'EOF' > emergency_rollback.sh
#!/bin/bash
echo "Starting emergency rollback..."

# Stop new queue-service
kubectl scale deployment queue-service --replicas=0

# Restore backup configuration
kubectl apply -f ./queue-service-backup-$(date +%Y%m%d)/

# Restart services
kubectl rollout restart deployment/queue-service
kubectl rollout restart deployment/worker-service

# Wait for services to be ready
kubectl wait --for=condition=available deployment/queue-service --timeout=300s
kubectl wait --for=condition=available deployment/worker-service --timeout=300s

echo "Emergency rollback completed"
EOF

chmod +x emergency_rollback.sh
```

#### **2. Message Recovery**

```bash
# If messages are lost, check dead letter queues
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages | grep dlq

# Recover messages from DLQ if needed
kubectl exec -it rabbitmq-0 -- rabbitmqctl get_queue profile-processing.dlq 10
```

## Success Criteria

### **Migration Complete When:**

- [ ] **Message Compatibility**: Worker-service successfully processes messages from queue-service
- [ ] **Routing Key Support**: All three routing keys (profile.task, email.send, image.process) work correctly
- [ ] **Publisher Confirms**: 99%+ publisher confirm success rate
- [ ] **Performance**: Equal or better throughput compared to pre-migration
- [ ] **Zero Message Loss**: No messages lost during migration process
- [ ] **Monitoring**: All new metrics collecting data correctly
- [ ] **Health Checks**: Enhanced health checks reporting correctly
- [ ] **Documentation**: All documentation updated to reflect new architecture

### **Rollback Criteria:**

Rollback immediately if:

- Message loss rate > 0.1%
- Publisher confirm failure rate > 5%
- Worker-service error rate > 10%
- API response time > 2x baseline
- Any critical service becomes unavailable

## Post-Migration Tasks

### **1. Documentation Updates**

- [ ] Update API documentation with new routing key examples
- [ ] Update integration guides for client services
- [ ] Create troubleshooting runbook for new architecture
- [ ] Update monitoring and alerting documentation

### **2. Monitoring Enhancements**

- [ ] Set up alerts for new metrics
- [ ] Create Grafana dashboards for routing key distribution
- [ ] Configure publisher confirm failure alerts
- [ ] Set up capacity planning metrics

### **3. Future Improvements**

- [ ] Plan email worker implementation
- [ ] Plan image worker implementation
- [ ] Consider advanced routing patterns
- [ ] Evaluate performance optimization opportunities

---

**Migration Status**: 🔄 **Ready for Execution** - All phases planned and tested. Execute according to timeline in `TRACKER.md`.
