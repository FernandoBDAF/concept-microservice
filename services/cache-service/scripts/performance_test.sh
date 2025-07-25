#!/bin/bash

# Cache Service Performance Testing Script
# Task 4.4: Performance testing and optimization

set -e

# Configuration
BASE_URL="${CACHE_SERVICE_URL:-http://localhost:8080}"
CONCURRENT_WORKERS="${CONCURRENT_WORKERS:-50}"
TOTAL_REQUESTS="${TOTAL_REQUESTS:-10000}"
TEST_DURATION="${TEST_DURATION:-60}"
WARMUP_DURATION="${WARMUP_DURATION:-10}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_header() {
    echo -e "\n${BLUE}🔥$1${NC}"
    echo "$(printf '=%.0s' {1..50})"
}

# Check dependencies
check_dependencies() {
    log_header "Checking Dependencies"
    
    command -v curl >/dev/null 2>&1 || { log_error "curl is required but not installed. Aborting."; exit 1; }
    command -v jq >/dev/null 2>&1 || { log_error "jq is required but not installed. Aborting."; exit 1; }
    command -v wrk >/dev/null 2>&1 || { log_warning "wrk not found, using curl-based testing instead"; }
    
    log_success "Dependencies check completed"
}

# Health check
health_check() {
    log_header "Phase 1: Health Check"
    
    log_info "Checking cache service health at $BASE_URL/health"
    
    if curl -s -f "$BASE_URL/health" >/dev/null; then
        log_success "Cache service is healthy"
    else
        log_error "Cache service health check failed"
        return 1
    fi
    
    # Check metrics endpoint
    log_info "Checking metrics endpoint at $BASE_URL/metrics"
    if curl -s -f "$BASE_URL/metrics" >/dev/null; then
        log_success "Metrics endpoint is accessible"
    else
        log_warning "Metrics endpoint not accessible"
    fi
}

# Warmup phase
warmup_phase() {
    log_header "Phase 2: Warmup ($WARMUP_DURATION seconds)"
    
    log_info "Pre-populating cache with test data..."
    
    # Create warmup data
    for i in $(seq 1 1000); do
        key="warmup-key-$i"
        value="warmup-value-$(openssl rand -hex 32)"
        
        curl -s -X POST "$BASE_URL/api/v1/cache" \
            -H "Content-Type: application/json" \
            -d "{\"key\":\"$key\",\"value\":\"$value\",\"ttl\":\"1h\"}" >/dev/null
        
        if [ $((i % 100)) -eq 0 ]; then
            log_info "Warmup progress: $i/1000 keys"
        fi
    done
    
    log_success "Warmup completed - 1000 keys pre-populated"
    sleep 2
}

# Basic performance test using curl
curl_performance_test() {
    log_header "Phase 3: Load Test (curl-based)"
    
    local test_results_file="/tmp/cache_perf_results.txt"
    local errors_file="/tmp/cache_perf_errors.txt"
    local start_time
    local end_time
    local duration
    
    echo "timestamp,operation,latency_ms,status" > "$test_results_file"
    
    log_info "Starting load test with $CONCURRENT_WORKERS workers for $TEST_DURATION seconds"
    
    start_time=$(date +%s)
    
    # Run concurrent workers
    for worker in $(seq 1 "$CONCURRENT_WORKERS"); do
        {
            local worker_requests=$((TOTAL_REQUESTS / CONCURRENT_WORKERS))
            
            for i in $(seq 1 "$worker_requests"); do
                local operation
                local key="test-key-$((RANDOM % 1000))"
                local start_ms
                local end_ms
                local latency_ms
                local status
                
                # 80% GET, 20% SET operations
                if [ $((RANDOM % 10)) -lt 8 ]; then
                    operation="GET"
                    start_ms=$(date +%s%3N)
                    status=$(curl -s -w "%{http_code}" -o /dev/null "$BASE_URL/api/v1/cache/$key")
                    end_ms=$(date +%s%3N)
                else
                    operation="SET"
                    local value="test-value-$(openssl rand -hex 16)"
                    start_ms=$(date +%s%3N)
                    status=$(curl -s -w "%{http_code}" -o /dev/null \
                        -X POST "$BASE_URL/api/v1/cache" \
                        -H "Content-Type: application/json" \
                        -d "{\"key\":\"$key\",\"value\":\"$value\",\"ttl\":\"1h\"}")
                    end_ms=$(date +%s%3N)
                fi
                
                latency_ms=$((end_ms - start_ms))
                echo "$(date +%s%3N),$operation,$latency_ms,$status" >> "$test_results_file"
                
                # Track errors
                if [[ ! "$status" =~ ^(200|201|404)$ ]]; then
                    echo "Worker $worker: $operation failed with status $status" >> "$errors_file"
                fi
            done
        } &
    done
    
    # Wait for all workers to complete
    wait
    
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    log_success "Load test completed in ${duration}s"
    
    # Analyze results
    analyze_results "$test_results_file" "$errors_file" "$duration"
}

# Advanced performance test using wrk (if available)
wrk_performance_test() {
    log_header "Phase 3: Load Test (wrk-based)"
    
    if ! command -v wrk >/dev/null 2>&1; then
        curl_performance_test
        return
    fi
    
    log_info "Running wrk performance test..."
    
    # Create Lua script for wrk
    cat > /tmp/cache_test.lua << 'EOF'
-- Cache service test script for wrk
local counter = 1
local host = "localhost:8080"

request = function()
    local key = "test-key-" .. math.random(1, 1000)
    
    -- 80% GET, 20% SET operations
    if math.random() < 0.8 then
        -- GET operation
        return wrk.format("GET", "/api/v1/cache/" .. key)
    else
        -- SET operation
        local value = "test-value-" .. counter
        counter = counter + 1
        local body = '{"key":"' .. key .. '","value":"' .. value .. '","ttl":"1h"}'
        local headers = {}
        headers["Content-Type"] = "application/json"
        return wrk.format("POST", "/api/v1/cache", headers, body)
    end
end

response = function(status, headers, body)
    if status ~= 200 and status ~= 201 and status ~= 404 then
        print("Error: " .. status)
    end
end
EOF

    # Run wrk test
    log_info "Testing GET operations..."
    wrk -t12 -c50 -d30s -s /tmp/cache_test.lua "$BASE_URL" > /tmp/wrk_results.txt
    
    # Display results
    cat /tmp/wrk_results.txt
    
    # Parse wrk results for validation
    local rps
    rps=$(grep "Requests/sec:" /tmp/wrk_results.txt | awk '{print $2}' | cut -d'.' -f1)
    
    if [ -n "$rps" ] && [ "$rps" -gt 10000 ]; then
        log_success "Throughput target achieved: ${rps} RPS (target: >10,000)"
    else
        log_warning "Throughput below target: ${rps} RPS (target: >10,000)"
    fi
    
    rm -f /tmp/cache_test.lua /tmp/wrk_results.txt
}

# Analyze test results
analyze_results() {
    local results_file="$1"
    local errors_file="$2"
    local duration="$3"
    
    log_header "Phase 4: Results Analysis"
    
    if [ ! -f "$results_file" ]; then
        log_error "Results file not found"
        return 1
    fi
    
    local total_requests
    local successful_requests
    local failed_requests
    local get_requests
    local set_requests
    local avg_latency
    local p95_latency
    local p99_latency
    local rps
    
    # Calculate basic metrics
    total_requests=$(tail -n +2 "$results_file" | wc -l)
    successful_requests=$(tail -n +2 "$results_file" | awk -F',' '$4 ~ /^(200|201|404)$/ {count++} END {print count+0}')
    failed_requests=$((total_requests - successful_requests))
    get_requests=$(tail -n +2 "$results_file" | grep ",GET," | wc -l)
    set_requests=$(tail -n +2 "$results_file" | grep ",SET," | wc -l)
    
    # Calculate latency statistics
    avg_latency=$(tail -n +2 "$results_file" | awk -F',' '$4 ~ /^(200|201|404)$/ {sum+=$3; count++} END {print (count>0) ? sum/count : 0}')
    p95_latency=$(tail -n +2 "$results_file" | awk -F',' '$4 ~ /^(200|201|404)$/ {print $3}' | sort -n | awk '{a[NR]=$1} END {print (NR>0) ? a[int(NR*0.95)] : 0}')
    p99_latency=$(tail -n +2 "$results_file" | awk -F',' '$4 ~ /^(200|201|404)$/ {print $3}' | sort -n | awk '{a[NR]=$1} END {print (NR>0) ? a[int(NR*0.99)] : 0}')
    
    # Calculate RPS
    rps=$(echo "$total_requests / $duration" | bc -l | xargs printf "%.2f")
    
    # Display results
    echo
    log_info "📊 Performance Test Results"
    echo "==============================="
    echo "Total Requests: $total_requests"
    echo "Successful: $successful_requests"
    echo "Failed: $failed_requests"
    echo "Success Rate: $(echo "scale=2; $successful_requests * 100 / $total_requests" | bc)%"
    echo ""
    echo "Operation Breakdown:"
    echo "  GET: $get_requests"
    echo "  SET: $set_requests"
    echo ""
    echo "Latency Statistics (ms):"
    echo "  Average: $(printf "%.2f" "$avg_latency")"
    echo "  95th percentile: $p95_latency"
    echo "  99th percentile: $p99_latency"
    echo ""
    echo "Throughput: ${rps} requests/second"
    echo "Test Duration: ${duration}s"
    
    # Validate against targets
    validate_performance "$avg_latency" "$p99_latency" "$rps" "$failed_requests" "$total_requests"
    
    # Show errors if any
    if [ -f "$errors_file" ] && [ -s "$errors_file" ]; then
        echo ""
        log_warning "Errors encountered:"
        head -10 "$errors_file"
        if [ "$(wc -l < "$errors_file")" -gt 10 ]; then
            echo "... and $(($(wc -l < "$errors_file") - 10)) more errors"
        fi
    fi
    
    # Cleanup
    rm -f "$results_file" "$errors_file"
}

# Validate performance against targets
validate_performance() {
    local avg_latency="$1"
    local p99_latency="$2"
    local rps="$3"
    local failed_requests="$4"
    local total_requests="$5"
    
    log_header "Performance Target Validation"
    
    # GET latency target: < 1ms average
    if [ "$(echo "$avg_latency < 1000" | bc)" -eq 1 ]; then
        log_success "Average latency within target: ${avg_latency}ms < 1000ms"
    else
        log_warning "Average latency exceeds target: ${avg_latency}ms > 1000ms"
    fi
    
    # P99 latency target: < 5ms
    if [ "$(echo "$p99_latency < 5000" | bc)" -eq 1 ]; then
        log_success "P99 latency within target: ${p99_latency}ms < 5000ms"
    else
        log_warning "P99 latency exceeds target: ${p99_latency}ms > 5000ms"
    fi
    
    # Throughput target: > 10,000 RPS
    if [ "$(echo "$rps > 10000" | bc)" -eq 1 ]; then
        log_success "Throughput exceeds target: ${rps} RPS > 10,000"
    else
        log_warning "Throughput below target: ${rps} RPS < 10,000"
    fi
    
    # Error rate target: < 1%
    local error_rate
    error_rate=$(echo "scale=2; $failed_requests * 100 / $total_requests" | bc)
    if [ "$(echo "$error_rate < 1" | bc)" -eq 1 ]; then
        log_success "Error rate within target: ${error_rate}% < 1%"
    else
        log_warning "Error rate exceeds target: ${error_rate}% > 1%"
    fi
}

# Resource utilization check
check_resource_utilization() {
    log_header "Resource Utilization Check"
    
    log_info "Checking cache service resource usage..."
    
    # Get metrics from cache service
    if curl -s "$BASE_URL/metrics" > /tmp/metrics.txt; then
        # Extract key metrics
        local memory_usage
        local cpu_usage
        local redis_connections
        local cache_hit_ratio
        
        memory_usage=$(grep "process_resident_memory_bytes" /tmp/metrics.txt | head -1 | awk '{print $2}')
        redis_connections=$(grep "redis_connected_clients" /tmp/metrics.txt | head -1 | awk '{print $2}')
        
        if [ -n "$memory_usage" ]; then
            memory_mb=$(echo "$memory_usage / 1024 / 1024" | bc)
            log_info "Memory usage: ${memory_mb} MB"
        fi
        
        if [ -n "$redis_connections" ]; then
            log_info "Redis connections: $redis_connections"
        fi
        
        # Check cache hit ratio
        local hits
        local misses
        hits=$(grep "cache_hits_total" /tmp/metrics.txt | head -1 | awk '{print $2}' || echo "0")
        misses=$(grep "cache_misses_total" /tmp/metrics.txt | head -1 | awk '{print $2}' || echo "0")
        
        if [ "$hits" != "0" ] || [ "$misses" != "0" ]; then
            cache_hit_ratio=$(echo "scale=2; $hits * 100 / ($hits + $misses)" | bc)
            log_info "Cache hit ratio: ${cache_hit_ratio}%"
            
            if [ "$(echo "$cache_hit_ratio > 80" | bc)" -eq 1 ]; then
                log_success "Cache hit ratio above 80% target"
            else
                log_warning "Cache hit ratio below 80% target"
            fi
        fi
        
        rm -f /tmp/metrics.txt
    else
        log_warning "Could not retrieve metrics from cache service"
    fi
}

# Performance optimization recommendations
optimization_recommendations() {
    log_header "Performance Optimization Recommendations"
    
    echo "🔧 Performance Tuning Tips:"
    echo ""
    echo "1. Connection Pool Optimization:"
    echo "   - Monitor Redis connection pool utilization"
    echo "   - Adjust pool size based on concurrent load"
    echo "   - Set appropriate connection timeouts"
    echo ""
    echo "2. Cache Configuration:"
    echo "   - Optimize TTL values for different data types"
    echo "   - Implement cache warming strategies"
    echo "   - Monitor cache hit ratios and adjust policies"
    echo ""
    echo "3. Redis Optimization:"
    echo "   - Configure Redis memory policies (allkeys-lru)"
    echo "   - Enable compression for large values"
    echo "   - Monitor Redis slow log"
    echo ""
    echo "4. Application Optimization:"
    echo "   - Use batch operations for multiple keys"
    echo "   - Implement circuit breakers for resilience"
    echo "   - Monitor and optimize serialization overhead"
    echo ""
    echo "5. Infrastructure Optimization:"
    echo "   - Use SSD storage for Redis persistence"
    echo "   - Ensure adequate network bandwidth"
    echo "   - Consider Redis clustering for horizontal scaling"
    echo ""
    echo "6. Monitoring and Alerting:"
    echo "   - Set up alerts for latency, error rate, and throughput"
    echo "   - Monitor resource utilization trends"
    echo "   - Implement health checks and readiness probes"
}

# Main execution
main() {
    log_header "🚀 Cache Service Performance Testing Suite"
    
    echo "Configuration:"
    echo "  Base URL: $BASE_URL"
    echo "  Concurrent Workers: $CONCURRENT_WORKERS"
    echo "  Total Requests: $TOTAL_REQUESTS"
    echo "  Test Duration: ${TEST_DURATION}s"
    echo "  Warmup Duration: ${WARMUP_DURATION}s"
    echo ""
    
    # Run test phases
    check_dependencies
    health_check
    warmup_phase
    
    # Choose performance test method
    if command -v wrk >/dev/null 2>&1; then
        wrk_performance_test
    else
        curl_performance_test
    fi
    
    check_resource_utilization
    optimization_recommendations
    
    log_success "Performance testing completed successfully!"
    echo ""
    echo "📈 Next Steps:"
    echo "1. Analyze results and identify bottlenecks"
    echo "2. Implement recommended optimizations"
    echo "3. Re-run tests to validate improvements"
    echo "4. Set up continuous performance monitoring"
}

# Run main function
main "$@" 