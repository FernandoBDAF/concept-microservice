# Performance Documentation

Performance optimization, testing, and monitoring guides.

## Contents

- [Performance Guide](performance-guide.md) - Overall performance guide
- [Load Testing](load-testing-strategy.md) - Load testing strategy and tools
- [Benchmarking](benchmarking.md) - Benchmarking methodology
- [Optimization](optimization.md) - Performance optimization techniques
- [Monitoring](monitoring.md) - Performance monitoring setup

## Performance Targets

| Metric | Target |
|--------|--------|
| API Response (cached) | p99 < 50ms |
| API Response (uncached) | p99 < 100ms |
| Health Check | < 1ms |
| Availability | 99.9% |

## Tools

- **k6** - Load testing
- **Prometheus** - Metrics collection
- **Grafana** - Performance dashboards

---

*Migrated from legacy_project/reference-materials/performance/*
