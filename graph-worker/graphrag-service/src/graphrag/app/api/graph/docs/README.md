# Graph Data API

RESTful API for querying and exploring GraphRAG knowledge graph data.

## Quick Start

```bash
# Start the server (from GraphRAG directory)
cd /path/to/GraphRAG
python -m app.graph_api.server --port 8081

# Test with curl
curl http://localhost:8081/api/health
```

## Overview

The Graph Data API provides endpoints for:

- **Entity Operations** - Search and retrieve entity details
- **Community Operations** - Explore hierarchical communities
- **Relationship Operations** - Query relationships between entities
- **Ego Network** - Get N-hop neighborhood graphs
- **Export** - Download graphs in JSON, CSV, GraphML, GEXF formats
- **Statistics** - Graph-level statistics and metrics
- **Quality Metrics** - Per-stage quality metrics
- **Performance Metrics** - Pipeline performance data

## Base URL

```
http://localhost:8081/api
```

## Authentication

Currently no authentication required. All endpoints are publicly accessible.

## Common Parameters

All endpoints accept these query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `db_name` | string | `2025-12` | Database name |
| `limit` | int | 50 | Maximum results (max: 500) |
| `offset` | int | 0 | Pagination offset |

## Endpoints

### Health Check

```
GET /health
```

Returns server status and version.

### Entities

```
GET /entities/search?q=algorithm&type=CONCEPT&limit=50
GET /entities/{entity_id}
```

### Communities

```
GET /communities/search?level=1&min_size=3&sort_by=entity_count
GET /communities/{community_id}
GET /communities/levels
```

### Relationships

```
GET /relationships/search?predicate=RELATES_TO&subject_id=xxx
```

### Ego Network

```
GET /ego/network/{entity_id}?max_hops=2&max_nodes=100
```

### Export

```
GET /export/json?community_id=xxx
GET /export/csv?entity_ids=id1,id2,id3
GET /export/graphml
GET /export/gexf
```

### Statistics & Metrics

```
GET /statistics
GET /statistics/time
GET /metrics
GET /metrics/quality?stage=extraction
GET /metrics/performance
GET /metrics/performance/trends
```

## Error Responses

All errors follow this format:

```json
{
  "error": "Error type",
  "message": "Detailed error message"
}
```

Common HTTP status codes:
- `200` - Success
- `400` - Bad request
- `404` - Resource not found
- `500` - Internal server error

## CORS

All endpoints include CORS headers for browser access:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, HEAD, OPTIONS`

## Related Documentation

- [API Specification](API_SPECIFICATION.md) - Full endpoint documentation
- [Postman Collection](postman_collection.json) - Test requests
- [Architecture](../API_ARCHITECTURE.md) - Design decisions

