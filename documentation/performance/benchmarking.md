# Performance Benchmarking

> *Migrated from legacy_project/reference-materials/performance/benchmarking.md*

## Overview

This document outlines the benchmarking strategy for the API Service architecture, focusing on measuring and comparing performance metrics across different components and configurations.

## Benchmarking Categories

### Service Performance

- Response time measurements
- Throughput capacity
- Resource utilization
- Error rates
- Latency distribution

### Database Performance

- Query execution time
- Connection pool efficiency
- Index performance
- Cache hit rates
- Write/Read ratios

### Cache Performance

- Hit/miss ratios
- Read/write latency
- Memory utilization
- Eviction rates

## Go Benchmarks

### Repository Benchmarks

```go
func BenchmarkProfileRepository_Get(b *testing.B) {
    db := setupTestDB(b)
    repo := NewProfileRepository(db)
    ctx := context.Background()
    
    // Create test data
    profile := createTestProfile(b, repo)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := repo.Get(ctx, profile.ID)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkProfileRepository_GetByIDs(b *testing.B) {
    db := setupTestDB(b)
    repo := NewProfileRepository(db)
    ctx := context.Background()
    
    // Create test data
    ids := createTestProfiles(b, repo, 100)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := repo.GetByIDs(ctx, ids)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Cache Benchmarks

```go
func BenchmarkCache_GetProfile(b *testing.B) {
    client := setupTestRedis(b)
    cache := NewCache(client, time.Hour)
    ctx := context.Background()
    
    // Pre-populate cache
    profile := &Profile{ID: "test-id", FirstName: "Test"}
    cache.SetProfile(ctx, profile)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := cache.GetProfile(ctx, "test-id")
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkCache_Pipeline(b *testing.B) {
    client := setupTestRedis(b)
    cache := NewCache(client, time.Hour)
    ctx := context.Background()
    
    // Create test IDs
    ids := make([]string, 100)
    for i := range ids {
        ids[i] = fmt.Sprintf("profile-%d", i)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := cache.GetProfiles(ctx, ids)
        if err != nil && !errors.Is(err, redis.Nil) {
            b.Fatal(err)
        }
    }
}
```

### Service Benchmarks

```go
func BenchmarkProfileService_GetProfile_Cached(b *testing.B) {
    service := setupTestService(b)
    ctx := context.Background()
    
    // Create and cache profile
    profile := createAndCacheProfile(b, service)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := service.GetProfile(ctx, profile.ID)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkProfileService_GetProfile_Uncached(b *testing.B) {
    service := setupTestService(b)
    ctx := context.Background()
    
    // Create profile without caching
    profile := createTestProfile(b, service)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Invalidate cache before each request
        service.cache.DeleteProfile(ctx, profile.ID)
        
        _, err := service.GetProfile(ctx, profile.ID)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Benchmarking Tools

- Go testing package (`go test -bench`)
- k6 for load testing
- Prometheus for metrics
- Grafana for visualization

## Benchmarking Process

1. Define baseline metrics
2. Set up monitoring
3. Execute benchmarks
4. Collect data
5. Analyze results
6. Generate reports

## Success Criteria

| Metric | Target |
|--------|--------|
| p95 Response Time (cached) | < 50ms |
| p95 Response Time (uncached) | < 100ms |
| Throughput | > 1000 req/s |
| Error Rate | < 0.1% |
| Cache Hit Rate | > 80% |
| Resource Utilization | < 70% |

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkProfileService_GetProfile_Cached -benchmem ./internal/domain/services/

# Run with profiling
go test -bench=. -cpuprofile=cpu.prof -memprofile=mem.prof ./...

# Analyze profile
go tool pprof cpu.prof
```

## Benchmark Results Template

```
BenchmarkProfileRepository_Get-8          50000         25000 ns/op        1024 B/op       10 allocs/op
BenchmarkProfileRepository_GetByIDs-8     10000        150000 ns/op       10240 B/op       50 allocs/op
BenchmarkCache_GetProfile-8              200000          8000 ns/op         512 B/op        5 allocs/op
BenchmarkCache_Pipeline-8                 50000         30000 ns/op        5120 B/op       25 allocs/op
```

## Cross-References

- [Load Testing Strategy](load-testing-strategy.md)
- [Performance Monitoring](monitoring.md)
- [Performance Optimization](optimization.md)

## Notes

- Regular benchmarking required
- Compare against historical data
- Document all configuration changes
- Consider different load patterns
