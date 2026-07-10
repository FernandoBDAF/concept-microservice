# Graph Data API Specification

Version: 1.0.0

## Table of Contents

1. [Entity Endpoints](#entity-endpoints)
2. [Community Endpoints](#community-endpoints)
3. [Relationship Endpoints](#relationship-endpoints)
4. [Ego Network Endpoints](#ego-network-endpoints)
5. [Export Endpoints](#export-endpoints)
6. [Statistics Endpoints](#statistics-endpoints)
7. [Metrics Endpoints](#metrics-endpoints)
8. [Query Endpoints](#query-endpoints)
9. [Response Schemas](#response-schemas)

---

## Entity Endpoints

### Search Entities

Search entities with optional filters.

**Request:**
```
GET /api/entities/search
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `q` | string | No | Search query (name, canonical_name, aliases) |
| `type` | string | No | Filter by entity type (PERSON, CONCEPT, etc.) |
| `min_confidence` | float | No | Minimum confidence score (0.0-1.0) |
| `min_source_count` | int | No | Minimum source count |
| `limit` | int | No | Max results (default: 50, max: 500) |
| `offset` | int | No | Pagination offset (default: 0) |
| `db_name` | string | No | Database name (default: 2025-12) |

**Response:**
```json
{
  "entities": [
    {
      "entity_id": "ent_abc123",
      "name": "Machine Learning",
      "canonical_name": "machine_learning",
      "type": "CONCEPT",
      "description": "A branch of artificial intelligence...",
      "confidence": 0.95,
      "source_count": 42,
      "aliases": ["ML", "machine learning"],
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-20T14:22:00Z"
    }
  ],
  "total": 156,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

### Get Entity Details

Get detailed information about a specific entity including relationships.

**Request:**
```
GET /api/entities/{entity_id}
```

**Response:**
```json
{
  "entity_id": "ent_abc123",
  "name": "Machine Learning",
  "canonical_name": "machine_learning",
  "type": "CONCEPT",
  "description": "A branch of artificial intelligence...",
  "confidence": 0.95,
  "source_count": 42,
  "aliases": ["ML", "machine learning"],
  "source_chunks": ["chunk_1", "chunk_2"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-20T14:22:00Z",
  "relationships": [
    {
      "relationship_id": "rel_xyz",
      "direction": "outgoing",
      "predicate": "RELATED_TO",
      "object_id": "ent_def456",
      "object_name": {
        "name": "Deep Learning",
        "type": "CONCEPT"
      },
      "confidence": 0.88,
      "source_count": 15
    }
  ],
  "relationship_count": 12
}
```

---

## Community Endpoints

### Search Communities

**Request:**
```
GET /api/communities/search
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `level` | int | No | Community hierarchy level |
| `min_size` | int | No | Minimum entity count |
| `max_size` | int | No | Maximum entity count |
| `min_coherence` | float | No | Minimum coherence score |
| `sort_by` | string | No | Sort field (entity_count, coherence_score, level) |
| `limit` | int | No | Max results (default: 50) |
| `offset` | int | No | Pagination offset |

**Response:**
```json
{
  "communities": [
    {
      "community_id": "comm_abc123",
      "level": 1,
      "title": "Machine Learning Concepts",
      "summary": "Core concepts related to machine learning...",
      "entity_count": 24,
      "relationship_count": 58,
      "coherence_score": 0.82,
      "entities": ["ent_1", "ent_2"],
      "source_chunks": ["chunk_1"],
      "created_at": "2024-01-15T10:30:00Z",
      "run_id": "run_xyz",
      "params_hash": "abc123"
    }
  ],
  "total": 45,
  "limit": 50,
  "offset": 0,
  "has_more": false
}
```

### Get Community Details

**Request:**
```
GET /api/communities/{community_id}
```

**Response:**
Includes full entity details and relationships within the community.

### Get Community Levels

**Request:**
```
GET /api/communities/levels
```

**Response:**
```json
{
  "levels": [
    {
      "level": 0,
      "count": 120,
      "avg_size": 3.5,
      "avg_coherence": 0.75,
      "total_entities": 420
    },
    {
      "level": 1,
      "count": 25,
      "avg_size": 15.2,
      "avg_coherence": 0.68,
      "total_entities": 380
    }
  ]
}
```

---

## Relationship Endpoints

### Search Relationships

**Request:**
```
GET /api/relationships/search
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `predicate` | string | No | Filter by relationship type |
| `type` | string | No | Filter by entity type (subject or object) |
| `min_confidence` | float | No | Minimum confidence |
| `subject_id` | string | No | Filter by subject entity |
| `object_id` | string | No | Filter by object entity |
| `limit` | int | No | Max results |
| `offset` | int | No | Pagination offset |

**Response:**
```json
{
  "relationships": [
    {
      "relationship_id": "rel_xyz",
      "subject_id": "ent_abc",
      "object_id": "ent_def",
      "subject_name": {"name": "ML", "type": "CONCEPT"},
      "object_name": {"name": "DL", "type": "CONCEPT"},
      "predicate": "RELATED_TO",
      "description": "Machine learning relates to deep learning...",
      "confidence": 0.92,
      "source_count": 28,
      "source_chunks": ["chunk_1", "chunk_2"],
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "total": 1250,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

---

## Ego Network Endpoints

### Get Ego Network

Get N-hop neighborhood graph around an entity.

**Request:**
```
GET /api/ego/network/{entity_id}
```

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_hops` | int | 2 | Maximum hops from center entity |
| `max_nodes` | int | 100 | Maximum nodes to return |

**Response:**
```json
{
  "center_entity": {
    "entity_id": "ent_abc",
    "name": "Machine Learning",
    "type": "CONCEPT"
  },
  "nodes": [
    {
      "entity_id": "ent_abc",
      "name": "Machine Learning",
      "canonical_name": "machine_learning",
      "type": "CONCEPT",
      "description": "...",
      "confidence": 0.95,
      "source_count": 42,
      "hop_level": 0,
      "is_center": true
    }
  ],
  "links": [
    {
      "source": "ent_abc",
      "target": "ent_def",
      "predicate": "RELATED_TO",
      "confidence": 0.88,
      "hop": 1
    }
  ],
  "max_hops": 2,
  "total_nodes": 45,
  "total_links": 120
}
```

---

## Export Endpoints

### Export Graph

Export graph data in various formats.

**Request:**
```
GET /api/export/{format}
```

**Path Parameters:**
| Parameter | Values | Description |
|-----------|--------|-------------|
| `format` | json, csv, graphml, gexf | Export format |

**Query Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `entity_ids` | string | Comma-separated entity IDs for subgraph |
| `community_id` | string | Export community subgraph |

**Response (JSON format):**
```json
{
  "nodes": [...],
  "links": [...],
  "metadata": {
    "total_nodes": 50,
    "total_links": 120,
    "export_type": "community"
  }
}
```

**Response (CSV format):**
```
Content-Type: text/csv

# NODES
id,name,type,confidence,source_count
ent_abc,Machine Learning,CONCEPT,0.95,42
...

# LINKS
source,target,predicate,confidence,source_count
ent_abc,ent_def,RELATED_TO,0.88,15
...
```

---

## Statistics Endpoints

### Get Graph Statistics

**Request:**
```
GET /api/statistics
```

**Response:**
```json
{
  "total_entities": 1250,
  "total_relationships": 3400,
  "graph_density": 0.0043,
  "edge_to_node_ratio": 2.72,
  "isolated_entities": 45,
  "connected_entities": 1205,
  "avg_degree": 5.44,
  "max_degree": 142,
  "min_degree": 1,
  "type_distribution": [
    {"type": "CONCEPT", "count": 520},
    {"type": "PERSON", "count": 380}
  ],
  "predicate_distribution": [
    {"predicate": "RELATED_TO", "count": 1200},
    {"predicate": "CREATED_BY", "count": 450}
  ],
  "degree_distribution": [
    {"degree": 0, "count": 45},
    {"degree": 5, "count": 320}
  ]
}
```

---

## Metrics Endpoints

### Get Quality Metrics

**Request:**
```
GET /api/metrics/quality?stage=extraction
```

**Response:**
```json
{
  "extraction": {
    "completion_rate": 0.98,
    "failure_rate": 0.02,
    "total_chunks": 5000,
    "processed_chunks": 4900,
    "failed_chunks": 100,
    "canonical_ratio": 0.95
  }
}
```

### Get Performance Metrics

**Request:**
```
GET /api/metrics/performance
```

**Response:**
```json
{
  "pipeline_id": "run_abc123",
  "status": "completed",
  "started_at": "2024-01-15T10:00:00Z",
  "completed_at": "2024-01-15T11:30:00Z",
  "duration_seconds": 5400,
  "total_chunks": 5000,
  "throughput": {
    "chunks_per_sec": 0.926,
    "chunks_per_min": 55.56
  },
  "stages": {
    "graph_extraction": {...},
    "entity_resolution": {...}
  }
}
```

---

## Query Endpoints

### Execute Query

Execute a natural language query using GraphRAG to generate answers based on the knowledge graph.

**Request:**
```
POST /api/query/execute
```

**Request Body:**
```json
{
  "query": "What is machine learning and how does it relate to deep learning?",
  "mode": "global",
  "options": {
    "top_k": 10,
    "include_sources": true,
    "include_communities": true,
    "model": "gpt-4o-mini",
    "temperature": 0.3
  }
}
```

**Body Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | Yes | Natural language question |
| `mode` | string | No | Query mode: "local", "global", or "hybrid" (default: "global") |
| `options.top_k` | int | No | Maximum number of results (default: 10) |
| `options.include_sources` | bool | No | Include source chunks (default: true) |
| `options.include_communities` | bool | No | Include community context (default: true) |
| `options.model` | string | No | LLM model to use (default: "gpt-4o-mini") |
| `options.temperature` | float | No | LLM temperature (default: 0.3) |

**Query Modes:**
- **local**: Focus on specific entity neighborhoods. Best for questions about specific entities.
- **global**: Use community summaries for broad context. Best for general/thematic questions.
- **hybrid**: Combine local and global approaches. Best for complex questions.

**Response (200 OK):**
```json
{
  "answer": "Machine learning is a branch of artificial intelligence that enables systems to learn from data...",
  "confidence": 0.85,
  "entities": [
    {
      "id": "ent_abc123",
      "name": "Machine Learning",
      "type": "CONCEPT",
      "description": "A branch of artificial intelligence...",
      "confidence": 0.95,
      "trust_score": 0.88,
      "source_count": 42
    }
  ],
  "communities": [
    {
      "id": "comm_xyz789",
      "title": "AI and Machine Learning Concepts",
      "summary": "Core concepts related to AI and ML...",
      "level": 1,
      "entity_count": 24,
      "coherence_score": 0.82
    }
  ],
  "sources": [],
  "meta": {
    "query_id": "query_a1b2c3d4e5f6",
    "processing_time_ms": 2340,
    "mode_used": "global",
    "model": "gpt-4o-mini",
    "entity_count": 5,
    "community_count": 2
  }
}
```

**Error Response (400 Bad Request):**
```json
{
  "error": "Query text is required",
  "field": "query"
}
```

**Error Response (400 Invalid Mode):**
```json
{
  "error": "Invalid mode: invalid_mode",
  "valid_modes": ["local", "global", "hybrid"],
  "query_id": "query_a1b2c3d4e5f6"
}
```

**Error Response (500 Configuration Error):**
```json
{
  "error": "Configuration error",
  "message": "OPENAI_API_KEY environment variable not set",
  "query_id": "query_a1b2c3d4e5f6"
}
```

### Get Query Modes

Get available query modes and their descriptions.

**Request:**
```
GET /api/query/modes
```

**Response:**
```json
{
  "modes": [
    {
      "name": "local",
      "description": "Focus on specific entity neighborhoods. Best for questions about specific entities."
    },
    {
      "name": "global",
      "description": "Use community summaries for broad context. Best for general/thematic questions."
    },
    {
      "name": "hybrid",
      "description": "Combine local and global approaches. Best for complex questions."
    }
  ],
  "default": "global"
}
```

---

## Response Schemas

### Error Response

```json
{
  "error": "Error type",
  "message": "Detailed error description",
  "field": "optional field name"
}
```

### Pagination

All list endpoints return:
```json
{
  "items": [...],
  "total": 1250,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

