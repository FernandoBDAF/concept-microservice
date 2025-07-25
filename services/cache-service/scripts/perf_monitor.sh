#!/bin/bash

# Cache Service Performance Monitoring Script
# Task 2.5: Performance optimization and connection pool tuning

echo "🚀 Cache Service Performance Monitor"
echo "===================================="

# Check if cache service is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "⚠️  Cache service is not running on localhost:8080"
    echo "   Start the service with: go run cmd/server/main.go"
    exit 1
fi

echo "✅ Cache service is running"

# Monitor health and metrics
echo ""
echo "📊 Service Health Status:"
curl -s http://localhost:8080/health | jq '.' 2>/dev/null || curl -s http://localhost:8080/health

echo ""
echo "📈 Service Metrics (sample):"
curl -s http://localhost:8081/metrics 2>/dev/null | grep -E "(cache_hits_total|cache_misses_total|cache_operation_duration|redis_connections)" | head -10

echo ""
echo "💡 Performance Optimization Tips:"
echo "================================="
echo "✅ Connection Pool Tuning:"
echo "   - Monitor redis_connections_active metric"
echo "   - Optimal pool size: 50-100 for most workloads"
echo "   - Adjust CACHE_REDIS_POOL_SIZE based on load"

echo ""
echo "✅ Cache Performance:"
echo "   - Target hit ratio > 90%: cache_hits_total / (cache_hits_total + cache_misses_total)"
echo "   - Monitor cache_operation_duration_seconds histogram"
echo "   - Use batch operations for multiple keys"

echo ""
echo "✅ Circuit Breaker Health:"
echo "   - Monitor circuit_breaker_state (0=closed, 1=half-open, 2=open)"
echo "   - Check circuit_breaker_trips_total for failure patterns"
echo "   - Tune CACHE_CIRCUIT_BREAKER_READY_TO_TRIP if needed"

echo ""
echo "✅ Recommended Production Settings:"
echo "   export CACHE_REDIS_POOL_SIZE=100"
echo "   export CACHE_REDIS_MIN_IDLE_CONNS=25"
echo "   export CACHE_CIRCUIT_BREAKER_READY_TO_TRIP=5"
echo "   export CACHE_CACHE_BATCH_SIZE=100"

echo ""
echo "🎯 To run comprehensive performance tests:"
echo "   go test -bench=. ./test/... -benchmem"
echo "   go run scripts/load_test.go (if available)" 