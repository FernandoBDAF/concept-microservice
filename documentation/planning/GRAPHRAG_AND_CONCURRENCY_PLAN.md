# GraphRAG Document Processing & Concurrency Optimization Plan

**Project**: API Service Enhancement with GraphRAG and Concurrency Analysis  
**Date**: January 29, 2026  
**Status**: Planning Phase

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Part 1: GraphRAG Document Processing Feature](#part-1-graphrag-document-processing-feature)
3. [Part 2: Concurrency Opportunities Analysis](#part-2-concurrency-opportunities-analysis)
4. [Implementation Roadmap](#implementation-roadmap)
5. [Success Metrics](#success-metrics)

---

## Executive Summary

This document outlines a comprehensive plan for two major enhancements to the microservices architecture:

### Phase 1: GraphRAG Document Processing
Add document upload and processing capabilities to the api-service, enabling users to upload documents that will be processed by a specialized GraphRAG worker to build knowledge graphs.

### Phase 2: Concurrency Optimization
After implementing GraphRAG processing, conduct a deep analysis to identify and implement concurrency optimizations across all services, focusing on maximizing throughput and reducing latency.

---

# Part 1: GraphRAG Document Processing Feature

## 1.1 Overview

### Feature Description
Enable users to upload documents (PDF, TXT, DOCX, MD) through the api-service, which will be stored and queued for processing by a specialized GraphRAG worker. The worker will extract entities, relationships, and build a knowledge graph for semantic search and retrieval.

### Architecture Flow

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Client    │────▶│ API Service │────▶│   RabbitMQ   │────▶│  GraphRAG       │
│             │     │             │     │              │     │  Worker         │
└─────────────┘     │  Document   │     │ document.    │     │                 │
                    │  Handler    │     │ process      │     │ - Extract       │
                    └─────┬───────┘     │              │     │ - Build Graph   │
                          │             └──────────────┘     │ - Store Vector  │
                          ▼                                  └────────┬────────┘
                    ┌─────────────┐                                  │
                    │   Storage   │◀─────────────────────────────────┘
                    │  (S3/Minio) │
                    │  PostgreSQL │
                    └─────────────┘
```

---

## 1.2 Implementation Plan - High Level Steps

### Step 1: Document Storage Infrastructure (2-3 days)

#### 1.1 Object Storage Setup
**Goal**: Set up MinIO (S3-compatible) for document storage

**Tasks**:
- [ ] Deploy MinIO via Kubernetes or Docker Compose
- [ ] Create buckets: `documents-raw`, `documents-processed`
- [ ] Configure access credentials and policies
- [ ] Set up lifecycle policies for document retention

**Deliverables**:
- MinIO deployment manifest (`k8s/deployment/minio/`)
- Configuration for buckets and policies
- Connection credentials in ConfigMap/Secret

#### 1.2 Database Schema Extension
**Goal**: Add tables for document metadata and GraphRAG entities

**Tasks**:
- [ ] Create migration for documents table
- [ ] Create migration for document_chunks table
- [ ] Create migration for entities table
- [ ] Create migration for relationships table
- [ ] Create migration for embeddings table
- [ ] Add indexes for efficient queries

**Database Schema**:

```sql
-- Migration: 000004_create_graphrag_tables.up.sql

-- Documents table
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    storage_bucket VARCHAR(100) NOT NULL,
    mime_type VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    processing_started_at TIMESTAMP WITH TIME ZONE,
    processing_completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Document chunks (for RAG processing)
CREATE TABLE IF NOT EXISTS document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER,
    embedding_id UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Entities extracted from documents
CREATE TABLE IF NOT EXISTS entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    type VARCHAR(100) NOT NULL, -- person, organization, location, concept, etc.
    description TEXT,
    source_chunk_ids UUID[],
    properties JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Relationships between entities
CREATE TABLE IF NOT EXISTS relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    source_entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relationship_type VARCHAR(100) NOT NULL,
    description TEXT,
    weight FLOAT DEFAULT 1.0,
    source_chunk_ids UUID[],
    properties JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Vector embeddings for semantic search
CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    chunk_id UUID REFERENCES document_chunks(id) ON DELETE CASCADE,
    entity_id UUID REFERENCES entities(id) ON DELETE CASCADE,
    embedding_vector vector(1536), -- OpenAI ada-002 dimension
    model VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Only one of document_id, chunk_id, or entity_id should be set
    CHECK (
        (document_id IS NOT NULL AND chunk_id IS NULL AND entity_id IS NULL) OR
        (document_id IS NULL AND chunk_id IS NOT NULL AND entity_id IS NULL) OR
        (document_id IS NULL AND chunk_id IS NULL AND entity_id IS NOT NULL)
    )
);

-- Indexes for performance
CREATE INDEX idx_documents_profile_id ON documents(profile_id);
CREATE INDEX idx_documents_user_id ON documents(user_id);
CREATE INDEX idx_documents_status ON documents(status);
CREATE INDEX idx_documents_created_at ON documents(created_at DESC);

CREATE INDEX idx_document_chunks_document_id ON document_chunks(document_id);
CREATE INDEX idx_document_chunks_chunk_index ON document_chunks(document_id, chunk_index);

CREATE INDEX idx_entities_document_id ON entities(document_id);
CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_name ON entities(name);

CREATE INDEX idx_relationships_document_id ON relationships(document_id);
CREATE INDEX idx_relationships_source ON relationships(source_entity_id);
CREATE INDEX idx_relationships_target ON relationships(target_entity_id);
CREATE INDEX idx_relationships_type ON relationships(relationship_type);

CREATE INDEX idx_embeddings_document_id ON embeddings(document_id);
CREATE INDEX idx_embeddings_chunk_id ON embeddings(chunk_id);
CREATE INDEX idx_embeddings_entity_id ON embeddings(entity_id);

-- Vector similarity search index (requires pgvector extension)
-- CREATE INDEX idx_embeddings_vector ON embeddings USING ivfflat (embedding_vector vector_cosine_ops);
```

**Deliverables**:
- Migration files in `api-service/migrations/`
- Repository interfaces in Go

---

### Step 2: API Service Document Upload Endpoint (3-4 days)

#### 2.1 Domain Layer - Document Models
**Location**: `api-service/internal/domain/document/`

**Tasks**:
- [ ] Create `model.go` with Document, DocumentChunk, Entity, Relationship structs
- [ ] Create `repository.go` interface
- [ ] Create `service.go` with document business logic
- [ ] Create `storage.go` interface for object storage

**Key Files**:

```go
// api-service/internal/domain/document/model.go
package document

type DocumentStatus string

const (
    StatusPending    DocumentStatus = "pending"
    StatusProcessing DocumentStatus = "processing"
    StatusCompleted  DocumentStatus = "completed"
    StatusFailed     DocumentStatus = "failed"
)

type Document struct {
    ID                     uuid.UUID         `json:"id"`
    ProfileID              uuid.UUID         `json:"profile_id"`
    UserID                 uuid.UUID         `json:"user_id"`
    Filename               string            `json:"filename"`
    OriginalFilename       string            `json:"original_filename"`
    FileType               string            `json:"file_type"`
    FileSize               int64             `json:"file_size"`
    StoragePath            string            `json:"storage_path"`
    StorageBucket          string            `json:"storage_bucket"`
    MimeType               string            `json:"mime_type"`
    Status                 DocumentStatus    `json:"status"`
    ProcessingStartedAt    *time.Time        `json:"processing_started_at,omitempty"`
    ProcessingCompletedAt  *time.Time        `json:"processing_completed_at,omitempty"`
    ErrorMessage           string            `json:"error_message,omitempty"`
    Metadata               map[string]string `json:"metadata"`
    CreatedAt              time.Time         `json:"created_at"`
    UpdatedAt              time.Time         `json:"updated_at"`
}

type Entity struct {
    ID              uuid.UUID         `json:"id"`
    DocumentID      uuid.UUID         `json:"document_id"`
    Name            string            `json:"name"`
    Type            string            `json:"type"`
    Description     string            `json:"description"`
    SourceChunkIDs  []uuid.UUID       `json:"source_chunk_ids"`
    Properties      map[string]interface{} `json:"properties"`
    CreatedAt       time.Time         `json:"created_at"`
    UpdatedAt       time.Time         `json:"updated_at"`
}

type Relationship struct {
    ID               uuid.UUID         `json:"id"`
    DocumentID       uuid.UUID         `json:"document_id"`
    SourceEntityID   uuid.UUID         `json:"source_entity_id"`
    TargetEntityID   uuid.UUID         `json:"target_entity_id"`
    RelationshipType string            `json:"relationship_type"`
    Description      string            `json:"description"`
    Weight           float64           `json:"weight"`
    SourceChunkIDs   []uuid.UUID       `json:"source_chunk_ids"`
    Properties       map[string]interface{} `json:"properties"`
    CreatedAt        time.Time         `json:"created_at"`
}
```

#### 2.2 Infrastructure Layer - Storage Implementation
**Location**: `api-service/internal/infrastructure/storage/`

**Tasks**:
- [ ] Create MinIO client wrapper (`minio_client.go`)
- [ ] Implement document upload/download methods
- [ ] Add presigned URL generation for large files
- [ ] Implement document repository for PostgreSQL

**Key Components**:
- MinIO client with multipart upload support
- Streaming upload for large files
- Document metadata persistence
- Error handling and retry logic

#### 2.3 API Handlers - Document Upload
**Location**: `api-service/internal/api/handlers/document.go`

**Tasks**:
- [ ] Create DocumentHandler with upload endpoint
- [ ] Implement multipart/form-data parsing
- [ ] Add file validation (size, type, virus scanning)
- [ ] Integrate with document service
- [ ] Return document metadata and task ID

**New Endpoints**:

```
POST   /api/v1/profiles/:id/documents          # Upload document
GET    /api/v1/profiles/:id/documents          # List documents
GET    /api/v1/documents/:id                   # Get document details
GET    /api/v1/documents/:id/download          # Download document
GET    /api/v1/documents/:id/entities          # Get extracted entities
GET    /api/v1/documents/:id/relationships     # Get relationships
POST   /api/v1/documents/:id/search            # Semantic search within document
DELETE /api/v1/documents/:id                   # Delete document
```

**Request/Response Examples**:

```bash
# Upload document
curl -X POST http://api-service/api/v1/profiles/{profile_id}/documents \
  -H "Authorization: Bearer {token}" \
  -F "file=@document.pdf" \
  -F "metadata={\"category\":\"research\",\"tags\":[\"ai\",\"graphrag\"]}"

# Response
{
  "document_id": "123e4567-e89b-12d3-a456-426614174000",
  "task_id": "789e0123-e89b-12d3-a456-426614174999",
  "filename": "document.pdf",
  "status": "pending",
  "message": "Document uploaded successfully, processing queued"
}
```

**Deliverables**:
- Document upload handler with validation
- Integration tests for upload flow
- API documentation

---

### Step 3: RabbitMQ Queue Configuration (1 day)

#### 3.1 Add GraphRAG Queue Configuration
**Location**: `api-service/internal/domain/task/model.go`

**Tasks**:
- [ ] Add `document.process` routing key to DefaultRoutingMap
- [ ] Configure queue parameters (TTL, prefetch, DLQ)
- [ ] Set up exchange: `document-tasks`
- [ ] Configure queue: `document-processing`

**Configuration**:

```go
// Add to DefaultRoutingMap in api-service/internal/domain/task/model.go
"document.process": {
    Exchange:      "document-tasks",
    Queue:         "document-processing",
    TTL:           12 * time.Hour,  // Long TTL for large documents
    Prefetch:      1,               // One document at a time (memory intensive)
    Durable:       true,
    AutoDelete:    false,
    Exclusive:     false,
    NoWait:        false,
    DeadLetterTTL: 7 * 24 * time.Hour,
    MaxRetries:    2,               // Limited retries (expensive processing)
    Description:   "Document processing tasks for GraphRAG with long TTL and low prefetch",
},
```

#### 3.2 Message Publishing Integration
**Location**: `api-service/internal/domain/document/service.go`

**Tasks**:
- [ ] Integrate task publisher in document service
- [ ] Create message payload with document metadata
- [ ] Publish to `document.process` queue after upload
- [ ] Handle publishing errors

**Deliverables**:
- Queue configuration
- RabbitMQ deployment updates
- Message publishing integration

---

### Step 4: GraphRAG Worker Implementation (5-7 days)

#### 4.1 Create GraphRAG Worker Service
**Location**: `services/worker-service/services/workers/graphrag-worker/`

**Structure**:
```
services/workers/graphrag-worker/
├── cmd/
│   └── main.go                        # Entry point
├── internal/
│   ├── domain/
│   │   ├── message.go                 # Document message structure
│   │   └── graph.go                   # Graph models
│   ├── processors/
│   │   ├── processor.go               # Main processing orchestrator
│   │   ├── extractor.go               # Text extraction from files
│   │   ├── chunker.go                 # Document chunking
│   │   ├── entity_extractor.go        # Entity recognition (LLM)
│   │   ├── relationship_extractor.go  # Relationship extraction (LLM)
│   │   ├── embedder.go                # Vector embedding generation
│   │   └── graph_builder.go           # Graph construction
│   ├── storage/
│   │   ├── minio_client.go            # Object storage
│   │   └── postgres_client.go         # Metadata storage
│   └── llm/
│       ├── client.go                  # LLM client (OpenAI/local)
│       └── prompts.go                 # Extraction prompts
├── k8s/
│   ├── deployment.yaml
│   ├── hpa.yaml
│   └── service.yaml
├── Dockerfile
├── go.mod
└── README.md
```

#### 4.2 Core Processing Pipeline

**Processing Steps**:

1. **Document Retrieval** (30s - 2min)
   - Download document from MinIO
   - Validate file integrity
   - Update status to "processing"

2. **Text Extraction** (1-5 min)
   - Extract text from PDF/DOCX/TXT
   - Preserve structure (headings, lists)
   - Handle OCR for scanned documents (optional)

3. **Chunking** (1-2 min)
   - Split document into semantic chunks
   - Overlap chunks for context preservation
   - Target: 500-1000 tokens per chunk

4. **Entity Extraction** (5-15 min, LLM-intensive)
   - Process each chunk with LLM
   - Extract entities: people, organizations, locations, concepts
   - Deduplicate and merge entities

5. **Relationship Extraction** (5-15 min, LLM-intensive)
   - Identify relationships between entities
   - Extract relationship types and descriptions
   - Build relationship graph

6. **Embedding Generation** (2-5 min)
   - Generate embeddings for chunks
   - Generate embeddings for entities
   - Store in PostgreSQL with pgvector

7. **Graph Construction** (1-2 min)
   - Build knowledge graph structure
   - Calculate entity importance scores
   - Store graph metadata

8. **Finalization** (30s)
   - Update document status to "completed"
   - Store processing statistics
   - Trigger notifications

**Total Processing Time**: 15-45 minutes per document (depending on size and complexity)

#### 4.3 LLM Integration

**Entity Extraction Prompt**:
```
You are an expert at extracting entities from documents.

Extract all entities from the following text chunk. For each entity, provide:
- name: The entity name
- type: person, organization, location, concept, event, date, or other
- description: Brief description of the entity in context

Text:
{chunk_text}

Respond in JSON format:
{
  "entities": [
    {"name": "...", "type": "...", "description": "..."},
    ...
  ]
}
```

**Relationship Extraction Prompt**:
```
You are an expert at identifying relationships between entities.

Given the following entities and text, identify relationships:

Entities: {entity_list}

Text:
{chunk_text}

For each relationship, provide:
- source: Source entity name
- target: Target entity name
- type: Relationship type (e.g., "works_for", "located_in", "related_to")
- description: Brief description

Respond in JSON format:
{
  "relationships": [
    {"source": "...", "target": "...", "type": "...", "description": "..."},
    ...
  ]
}
```

**LLM Configuration**:
- Model: GPT-4-turbo or GPT-3.5-turbo (cost/speed tradeoff)
- Temperature: 0.1 (deterministic)
- Max tokens: 2000
- Retry logic: 3 attempts with exponential backoff
- Rate limiting: Respect OpenAI limits

#### 4.4 Error Handling & Monitoring

**Error Scenarios**:
- Document download failure → Retry 3 times, then mark failed
- Text extraction failure → Mark failed, store error
- LLM API failure → Retry with backoff, fallback to simpler extraction
- Embedding generation failure → Retry, skip if persistent
- Database write failure → Retry, rollback on persistent failure

**Monitoring Metrics**:
- `graphrag_processing_duration_seconds` (histogram)
- `graphrag_processing_errors_total` (counter)
- `graphrag_processing_success_total` (counter)
- `graphrag_entities_extracted_total` (counter)
- `graphrag_relationships_extracted_total` (counter)
- `graphrag_llm_calls_total` (counter)
- `graphrag_llm_errors_total` (counter)

**Deliverables**:
- Complete GraphRAG worker implementation
- LLM integration with retry logic
- Comprehensive error handling
- Monitoring and metrics

---

### Step 5: Query & Retrieval APIs (2-3 days)

#### 5.1 Semantic Search Endpoint
**Location**: `api-service/internal/api/handlers/document.go`

**Tasks**:
- [ ] Implement semantic search handler
- [ ] Generate query embedding
- [ ] Perform vector similarity search
- [ ] Return ranked results with context

**Search Endpoint**:
```
POST /api/v1/documents/:id/search
Content-Type: application/json

{
  "query": "What are the main findings about AI safety?",
  "max_results": 10,
  "similarity_threshold": 0.7,
  "include_context": true
}

Response:
{
  "query": "What are the main findings about AI safety?",
  "results": [
    {
      "chunk_id": "...",
      "content": "...",
      "similarity_score": 0.92,
      "chunk_index": 5,
      "entities": [...],
      "relationships": [...]
    },
    ...
  ],
  "processing_time_ms": 245
}
```

#### 5.2 Graph Query Endpoints

**Entity Search**:
```
GET /api/v1/documents/:id/entities?type=person&limit=50
```

**Relationship Traversal**:
```
GET /api/v1/entities/:id/relationships?direction=outgoing&depth=2
```

**Graph Visualization Data**:
```
GET /api/v1/documents/:id/graph
Response:
{
  "nodes": [
    {"id": "...", "name": "...", "type": "person", "properties": {...}},
    ...
  ],
  "edges": [
    {"source": "...", "target": "...", "type": "works_for", "weight": 0.8},
    ...
  ]
}
```

**Deliverables**:
- Semantic search implementation
- Graph query APIs
- Integration tests
- API documentation

---

### Step 6: Testing & Validation (2-3 days)

#### 6.1 Unit Tests
- [ ] Document upload validation tests
- [ ] Text extraction tests
- [ ] Entity extraction tests
- [ ] Relationship extraction tests
- [ ] Embedding generation tests

#### 6.2 Integration Tests
- [ ] End-to-end document processing flow
- [ ] RabbitMQ message flow validation
- [ ] MinIO upload/download tests
- [ ] PostgreSQL persistence tests
- [ ] Semantic search accuracy tests

#### 6.3 Performance Tests
- [ ] Load test document uploads (100 concurrent)
- [ ] Stress test worker processing (10 documents simultaneously)
- [ ] Measure processing time for various document sizes
- [ ] Vector search performance benchmarks

#### 6.4 Test Documents
Prepare test corpus:
- [ ] 10 research papers (PDF)
- [ ] 5 technical documentation (MD/DOCX)
- [ ] 3 large documents (>100 pages)
- [ ] 2 multilingual documents
- [ ] 1 scanned document (OCR test)

**Deliverables**:
- Comprehensive test suite
- Performance benchmarks
- Test documentation

---

### Step 7: Deployment & Documentation (1-2 days)

#### 7.1 Kubernetes Deployment
- [ ] Update deployment manifests
- [ ] Configure HPA for GraphRAG worker
- [ ] Set up MinIO in cluster
- [ ] Configure secrets and config maps
- [ ] Deploy to staging environment

#### 7.2 Documentation
- [ ] API documentation (OpenAPI/Swagger)
- [ ] Architecture diagrams
- [ ] Processing flow documentation
- [ ] Operational runbook
- [ ] User guide for document upload

**Deliverables**:
- Production-ready deployment
- Complete documentation
- Operational procedures

---

## 1.3 Technical Considerations

### File Size Limits
- Max upload size: 100 MB (configurable)
- Streaming upload for large files
- Presigned URLs for downloads

### Supported File Types
- **Text**: `.txt`, `.md`
- **Documents**: `.pdf`, `.docx`, `.doc`
- **Future**: `.html`, `.epub`, `.odt`

### Security
- File type validation (MIME type + magic bytes)
- Virus scanning (ClamAV integration - optional)
- Access control: Users can only access their own documents
- Presigned URL expiration: 15 minutes

### Scalability
- Worker auto-scaling based on queue depth (KEDA)
- Resource limits: 2 CPU, 4Gi memory per worker pod
- Replica limits: 1-10 workers
- Object storage: Unlimited (MinIO/S3)

### Cost Optimization
- LLM API costs: ~$0.01-0.05 per document (GPT-3.5-turbo)
- Alternative: Local LLM (Llama 3, Mistral) for reduced costs
- Caching: Cache entity extraction results to avoid reprocessing

---

## 1.4 Dependencies & Libraries

### API Service
```go
// Document processing
github.com/minio/minio-go/v7           // MinIO client
github.com/gabriel-vasile/mimetype     // MIME type detection

// Existing dependencies
github.com/gin-gonic/gin
github.com/google/uuid
github.com/rabbitmq/amqp091-go
```

### GraphRAG Worker
```go
// Text extraction
github.com/ledongthuc/pdf              // PDF text extraction
github.com/nguyenthenguyen/docx        // DOCX processing
code.sajari.com/docconv                // Universal document converter

// LLM integration
github.com/sashabaranov/go-openai      // OpenAI API client

// Chunking & NLP
github.com/pkoukk/tiktoken-go          // Token counting

// Vector operations
github.com/pgvector/pgvector-go        // pgvector support

// Storage
github.com/minio/minio-go/v7           // MinIO
github.com/jmoiron/sqlx                // PostgreSQL

// Existing
github.com/rabbitmq/amqp091-go
go.uber.org/zap
```

### PostgreSQL Extensions
```sql
CREATE EXTENSION IF NOT EXISTS vector;  -- pgvector for embeddings
CREATE EXTENSION IF NOT EXISTS pg_trgm; -- Trigram indexing for text search
```

---

## 1.5 Alternative Approaches & Trade-offs

### Approach 1: Synchronous Processing (Not Recommended)
**Pros**: Simpler implementation, immediate results  
**Cons**: Long API response times (15-45 min), blocking, poor UX  
**Verdict**: ❌ Not suitable for production

### Approach 2: Async with Polling (Recommended)
**Pros**: Non-blocking, scalable, status tracking  
**Cons**: Requires polling mechanism, slightly more complex  
**Verdict**: ✅ **RECOMMENDED** - Best balance of complexity and UX

### Approach 3: Async with WebSocket Notifications
**Pros**: Real-time updates, great UX  
**Cons**: Additional infrastructure (WebSocket server), stateful connections  
**Verdict**: ⚠️ Consider for Phase 2 enhancement

### LLM Choice: Cloud vs Local

#### Cloud LLM (OpenAI GPT-4)
**Pros**:
- High quality entity/relationship extraction
- No GPU infrastructure needed
- Regular model improvements

**Cons**:
- API costs: ~$0.01-0.05 per document
- Rate limits: Potential bottlenecks
- Data privacy concerns

**Cost Estimate**: $10-50 per 1000 documents

#### Local LLM (Llama 3, Mistral)
**Pros**:
- Zero API costs after setup
- Data privacy (stays on-premise)
- No rate limits

**Cons**:
- Requires GPU infrastructure (expensive)
- Lower quality compared to GPT-4
- Slower processing

**Cost Estimate**: GPU instance: $500-2000/month (dedicated)

**Recommendation**: Start with **OpenAI GPT-3.5-turbo** for MVP, evaluate local LLM after 1000 documents processed.

---

# Part 2: Concurrency Opportunities Analysis

## 2.1 Current State Assessment

After implementing the GraphRAG feature, the system will have the following characteristics:

### Current Architecture
```
API Service (api-service)
├── HTTP Request Handlers (Gin)
├── Profile Service (CRUD operations)
├── Task Service (Queue publishing)
├── Document Service (Upload & retrieval)
├── PostgreSQL Client
├── Redis Cache Client
└── RabbitMQ Publisher

Worker Services
├── Email Worker (email processing)
├── Image Worker (image processing)
├── Profile Worker (profile tasks)
└── GraphRAG Worker (document processing)
```

### Current Concurrency State
- ✅ HTTP handlers: Gin handles concurrent requests (goroutines per request)
- ✅ Database connection pooling: sqlx with connection pool
- ✅ Redis connection pooling: go-redis with connection pool
- ⚠️ Worker processing: Sequential message consumption (one at a time)
- ⚠️ Profile service: Sequential cache + DB operations
- ⚠️ Document upload: Sequential file operations
- ⚠️ GraphRAG processing: Sequential pipeline stages

---

## 2.2 Concurrency Opportunities - Detailed Analysis

### Opportunity #1: Parallel Cache Operations in Profile Service

#### Current Implementation
```go
// api-service/internal/domain/profile/service.go
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
    // Sequential operations
    if err := s.repo.Delete(ctx, id); err != nil {
        return err
    }
    if s.cache != nil {
        _ = s.cache.InvalidateProfile(ctx, id)  // ← Happens after DB delete
    }
    return nil
}
```

**Problem**: Cache invalidation waits for DB delete to complete, adding unnecessary latency.

#### Optimized Implementation with Concurrency
```go
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
    var wg sync.WaitGroup
    errChan := make(chan error, 2)
    
    // Delete from DB
    wg.Add(1)
    go func() {
        defer wg.Done()
        if err := s.repo.Delete(ctx, id); err != nil {
            errChan <- err
        }
    }()
    
    // Invalidate cache (fire and forget, but track completion)
    if s.cache != nil {
        wg.Add(1)
        go func() {
            defer wg.Done()
            _ = s.cache.InvalidateProfile(ctx, id)
            // Ignore cache errors (best effort)
        }()
    }
    
    wg.Wait()
    close(errChan)
    
    // Return first error encountered
    select {
    case err := <-errChan:
        return err
    default:
        return nil
    }
}
```

**Benefits**:
- Reduces latency by ~5-10ms (cache RTT)
- Cache and DB operations happen in parallel
- Error handling preserved

**Impact**: Medium latency improvement on delete operations

---

### Opportunity #2: Concurrent Document Chunk Processing

#### Current Implementation (Sequential)
```go
// graphrag-worker/internal/processors/processor.go
func (p *Processor) ProcessDocument(doc *Document) error {
    // 1. Extract text (5 min)
    text, err := p.extractText(doc)
    
    // 2. Chunk text (2 min)
    chunks, err := p.chunkText(text)
    
    // 3. Process chunks sequentially (15 min for 30 chunks)
    for _, chunk := range chunks {
        entities, _ := p.extractEntities(chunk)    // 20s per chunk
        embeddings, _ := p.generateEmbeddings(chunk) // 10s per chunk
    }
    
    // 4. Build graph (2 min)
    graph, err := p.buildGraph(entities)
    
    return nil
}
```

**Problem**: 30 chunks processed sequentially = 30 × 30s = 15 minutes total

#### Optimized Implementation with Worker Pool
```go
func (p *Processor) ProcessDocument(doc *Document) error {
    // 1. Extract text (5 min) - cannot parallelize
    text, err := p.extractText(doc)
    if err != nil {
        return err
    }
    
    // 2. Chunk text (2 min) - cannot parallelize
    chunks, err := p.chunkText(text)
    if err != nil {
        return err
    }
    
    // 3. Process chunks concurrently with worker pool
    numWorkers := 5 // Process 5 chunks at a time
    results := make(chan ProcessedChunk, len(chunks))
    errChan := make(chan error, len(chunks))
    
    // Create worker pool
    var wg sync.WaitGroup
    chunkChan := make(chan *Chunk, len(chunks))
    
    // Start workers
    for w := 0; w < numWorkers; w++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for chunk := range chunkChan {
                processed, err := p.processChunk(ctx, chunk)
                if err != nil {
                    errChan <- fmt.Errorf("worker %d: %w", workerID, err)
                    continue
                }
                results <- processed
            }
        }(w)
    }
    
    // Feed chunks to workers
    for _, chunk := range chunks {
        chunkChan <- chunk
    }
    close(chunkChan)
    
    // Wait for completion
    wg.Wait()
    close(results)
    close(errChan)
    
    // Check for errors
    select {
    case err := <-errChan:
        return fmt.Errorf("chunk processing failed: %w", err)
    default:
    }
    
    // Collect results
    processedChunks := make([]ProcessedChunk, 0, len(chunks))
    for result := range results {
        processedChunks = append(processedChunks, result)
    }
    
    // 4. Build graph (2 min) - can be optimized separately
    graph, err := p.buildGraph(processedChunks)
    
    return nil
}

func (p *Processor) processChunk(ctx context.Context, chunk *Chunk) (ProcessedChunk, error) {
    var result ProcessedChunk
    var wg sync.WaitGroup
    errChan := make(chan error, 2)
    
    // Extract entities and generate embeddings in parallel
    wg.Add(2)
    
    // Entity extraction
    go func() {
        defer wg.Done()
        entities, err := p.extractEntities(ctx, chunk)
        if err != nil {
            errChan <- fmt.Errorf("entity extraction: %w", err)
            return
        }
        result.Entities = entities
    }()
    
    // Embedding generation
    go func() {
        defer wg.Done()
        embedding, err := p.generateEmbeddings(ctx, chunk)
        if err != nil {
            errChan <- fmt.Errorf("embedding generation: %w", err)
            return
        }
        result.Embedding = embedding
    }()
    
    wg.Wait()
    close(errChan)
    
    // Check for errors
    select {
    case err := <-errChan:
        return ProcessedChunk{}, err
    default:
        return result, nil
    }
}
```

**Performance Comparison**:
- **Before**: 30 chunks × 30s = 900s (15 minutes)
- **After**: (30 chunks ÷ 5 workers) × 30s = 180s (3 minutes)
- **Speedup**: **5x faster** (83% reduction in processing time)

**Benefits**:
- Massive time savings on document processing
- Utilizes multiple CPU cores
- Respects LLM rate limits (configurable workers)
- Maintains error handling

**Trade-offs**:
- Increased memory usage (5 chunks in flight simultaneously)
- LLM API rate limits may throttle (monitor and adjust workers)
- Requires careful rate limiting

**Impact**: **HIGH** - Critical for user experience

---

### Opportunity #3: Parallel Profile List Enrichment

#### Current Implementation
```go
// api-service/internal/api/handlers/profile.go
func (h *ProfileHandler) GetProfiles(c *gin.Context) {
    // Get profiles from DB/cache
    profiles, err := h.service.List(ctx, page, pageSize)
    
    // Sequentially enrich each profile with additional data
    for i, profile := range profiles {
        // Fetch document count for each profile
        docCount, _ := h.documentService.GetCount(ctx, profile.ID)
        profiles[i].DocumentCount = docCount
        
        // Fetch entity count
        entityCount, _ := h.documentService.GetEntityCount(ctx, profile.ID)
        profiles[i].EntityCount = entityCount
    }
    
    c.JSON(http.StatusOK, profiles)
}
```

**Problem**: For 50 profiles, this makes 100 sequential DB queries (2 per profile)

#### Optimized Implementation with Batch Queries and Concurrency
```go
func (h *ProfileHandler) GetProfiles(c *gin.Context) {
    // Get profiles from DB/cache
    profiles, err := h.service.List(ctx, page, pageSize)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    profileIDs := make([]uuid.UUID, len(profiles))
    for i, p := range profiles {
        profileIDs[i] = p.ID
    }
    
    // Batch fetch enrichment data concurrently
    var wg sync.WaitGroup
    var docCounts map[uuid.UUID]int
    var entityCounts map[uuid.UUID]int
    var enrichErr error
    
    wg.Add(2)
    
    // Fetch document counts (batch query)
    go func() {
        defer wg.Done()
        var err error
        docCounts, err = h.documentService.GetCountsBatch(ctx, profileIDs)
        if err != nil {
            enrichErr = fmt.Errorf("doc counts: %w", err)
        }
    }()
    
    // Fetch entity counts (batch query)
    go func() {
        defer wg.Done()
        var err error
        entityCounts, err = h.documentService.GetEntityCountsBatch(ctx, profileIDs)
        if err != nil {
            enrichErr = fmt.Errorf("entity counts: %w", err)
        }
    }()
    
    wg.Wait()
    
    if enrichErr != nil {
        // Log error but don't fail the request
        zap.L().Warn("enrichment failed", zap.Error(enrichErr))
    }
    
    // Enrich profiles
    for i := range profiles {
        if docCounts != nil {
            profiles[i].DocumentCount = docCounts[profiles[i].ID]
        }
        if entityCounts != nil {
            profiles[i].EntityCount = entityCounts[profiles[i].ID]
        }
    }
    
    c.JSON(http.StatusOK, profiles)
}

// New batch query methods
func (s *DocumentService) GetCountsBatch(ctx context.Context, profileIDs []uuid.UUID) (map[uuid.UUID]int, error) {
    query := `
        SELECT profile_id, COUNT(*) as count
        FROM documents
        WHERE profile_id = ANY($1)
        GROUP BY profile_id
    `
    rows, err := s.db.QueryContext(ctx, query, pq.Array(profileIDs))
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    counts := make(map[uuid.UUID]int)
    for rows.Next() {
        var id uuid.UUID
        var count int
        if err := rows.Scan(&id, &count); err != nil {
            return nil, err
        }
        counts[id] = count
    }
    
    return counts, nil
}
```

**Performance Comparison**:
- **Before**: 50 profiles × 2 queries × 5ms = 500ms
- **After**: 1 profile query + 2 batch queries (parallel) = ~20ms
- **Speedup**: **25x faster** (96% reduction in query time)

**Benefits**:
- Dramatically reduced DB load
- Lower latency for list endpoints
- Better scalability

**Impact**: **HIGH** - Significant improvement for common operations

---

### Opportunity #4: Worker Message Processing Concurrency

#### Current Implementation
```go
// worker-service/services/workers/common/base/worker.go
func (w *BaseWorker) Start() error {
    // Consumes messages one at a time
    for msg := range messageChan {
        w.processor.Process(msg)  // Blocks until complete
        msg.Ack()
    }
}
```

**Problem**: Workers process one message at a time, even though they could handle multiple

#### Optimized Implementation with Configurable Concurrency
```go
type BaseWorker struct {
    processor   MessageProcessor
    consumer    *queue.Consumer
    concurrency int // Number of messages to process concurrently
    semaphore   chan struct{}
}

func NewBaseWorker(processor MessageProcessor, consumer *queue.Consumer, concurrency int) *BaseWorker {
    return &BaseWorker{
        processor:   processor,
        consumer:    consumer,
        concurrency: concurrency,
        semaphore:   make(chan struct{}, concurrency),
    }
}

func (w *BaseWorker) Start(ctx context.Context) error {
    var wg sync.WaitGroup
    
    for {
        select {
        case <-ctx.Done():
            // Wait for in-flight messages to complete
            wg.Wait()
            return ctx.Err()
            
        case msg := <-w.consumer.Messages():
            // Acquire semaphore slot
            w.semaphore <- struct{}{}
            
            wg.Add(1)
            go func(message queue.Message) {
                defer wg.Done()
                defer func() { <-w.semaphore }() // Release slot
                
                // Process message
                if err := w.processor.Process(ctx, message); err != nil {
                    zap.L().Error("processing failed", zap.Error(err))
                    message.Nack(true) // Requeue
                } else {
                    message.Ack()
                }
            }(msg)
        }
    }
}
```

**Configuration by Worker Type**:

```yaml
# Email Worker (high throughput, I/O bound)
concurrency: 10  # Process 10 emails simultaneously
resources:
  requests: { cpu: 100m, memory: 128Mi }
  
# Image Worker (CPU intensive)
concurrency: 2   # Process 2 images at a time
resources:
  requests: { cpu: 1000m, memory: 512Mi }
  
# GraphRAG Worker (memory + LLM rate limits)
concurrency: 1   # Process 1 document at a time (but chunks are parallel internally)
resources:
  requests: { cpu: 500m, memory: 2Gi }

# Profile Worker
concurrency: 5   # Moderate concurrency
resources:
  requests: { cpu: 200m, memory: 256Mi }
```

**Benefits**:
- **Email Worker**: 10x throughput improvement
- **Image Worker**: 2x throughput improvement
- **Profile Worker**: 5x throughput improvement
- Better resource utilization

**Trade-offs**:
- Increased memory usage
- Need to tune concurrency per worker type
- Risk of overwhelming downstream services

**Impact**: **HIGH** - Major throughput improvement

---

### Opportunity #5: Concurrent Database Batch Inserts

#### Current Implementation
```go
// graphrag-worker: Insert entities sequentially
func (s *GraphStorage) SaveEntities(ctx context.Context, entities []Entity) error {
    for _, entity := range entities {
        _, err := s.db.ExecContext(ctx, 
            "INSERT INTO entities (id, document_id, name, type, ...) VALUES ($1, $2, $3, $4, ...)",
            entity.ID, entity.DocumentID, entity.Name, entity.Type, ...)
        if err != nil {
            return err
        }
    }
    return nil
}
```

**Problem**: 100 entities = 100 sequential INSERT statements = ~500ms

#### Optimized Implementation with Batch Insert
```go
func (s *GraphStorage) SaveEntities(ctx context.Context, entities []Entity) error {
    if len(entities) == 0 {
        return nil
    }
    
    // Build batch insert query
    query := `
        INSERT INTO entities (id, document_id, name, type, description, properties, created_at)
        VALUES
    `
    
    values := make([]interface{}, 0, len(entities)*7)
    placeholders := make([]string, 0, len(entities))
    
    for i, entity := range entities {
        offset := i * 7
        placeholders = append(placeholders, fmt.Sprintf(
            "($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
            offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7,
        ))
        
        values = append(values,
            entity.ID,
            entity.DocumentID,
            entity.Name,
            entity.Type,
            entity.Description,
            entity.Properties,
            time.Now(),
        )
    }
    
    query += strings.Join(placeholders, ", ")
    
    // Execute single batch insert
    _, err := s.db.ExecContext(ctx, query, values...)
    return err
}

// Even better: Use COPY for very large batches
func (s *GraphStorage) SaveEntitiesBulk(ctx context.Context, entities []Entity) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    stmt, err := tx.PrepareContext(ctx, pq.CopyIn(
        "entities",
        "id", "document_id", "name", "type", "description", "properties", "created_at",
    ))
    if err != nil {
        return err
    }
    
    for _, entity := range entities {
        _, err = stmt.ExecContext(ctx,
            entity.ID,
            entity.DocumentID,
            entity.Name,
            entity.Type,
            entity.Description,
            entity.Properties,
            time.Now(),
        )
        if err != nil {
            return err
        }
    }
    
    if err := stmt.Close(); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

**Performance Comparison**:
- **Sequential INSERTs**: 100 entities × 5ms = 500ms
- **Batch INSERT**: Single query = ~20ms
- **COPY command**: Ultra-fast = ~5ms
- **Speedup**: **25-100x faster**

**Benefits**:
- Massive performance improvement for bulk operations
- Reduced DB connection overhead
- Lower transaction count

**Impact**: **HIGH** - Critical for GraphRAG worker performance

---

### Opportunity #6: Parallel File Upload to MinIO

#### Current Implementation
```go
// api-service: Upload single large file
func (s *StorageService) UploadDocument(ctx context.Context, file io.Reader, size int64) error {
    _, err := s.minioClient.PutObject(ctx, "documents-raw", objectName, file, size, 
        minio.PutObjectOptions{ContentType: mimeType})
    return err
}
```

**Problem**: Large files (100MB) upload slowly, single-threaded

#### Optimized Implementation with Multipart Upload
```go
func (s *StorageService) UploadDocumentMultipart(ctx context.Context, file io.Reader, size int64) error {
    const partSize = 10 * 1024 * 1024 // 10 MB parts
    
    // MinIO SDK handles multipart upload automatically if partSize is set
    _, err := s.minioClient.PutObject(ctx, "documents-raw", objectName, file, size, 
        minio.PutObjectOptions{
            ContentType: mimeType,
            PartSize:    partSize, // Enables concurrent part uploads
        })
    return err
}

// For even better control: Manual multipart upload with parallel uploads
func (s *StorageService) UploadDocumentParallel(ctx context.Context, filePath string) error {
    file, err := os.Open(filePath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    stat, err := file.Stat()
    if err != nil {
        return err
    }
    
    const partSize = 10 * 1024 * 1024 // 10 MB
    const concurrency = 5              // Upload 5 parts at a time
    
    numParts := int(math.Ceil(float64(stat.Size()) / float64(partSize)))
    
    // Initialize multipart upload
    uploadID, err := s.minioClient.NewMultipartUpload(ctx, "documents-raw", objectName, 
        minio.PutObjectOptions{ContentType: mimeType})
    if err != nil {
        return err
    }
    
    // Upload parts concurrently
    parts := make([]minio.CompletePart, numParts)
    errChan := make(chan error, numParts)
    sem := make(chan struct{}, concurrency)
    
    var wg sync.WaitGroup
    for partNum := 1; partNum <= numParts; partNum++ {
        wg.Add(1)
        go func(pn int) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire slot
            defer func() { <-sem }() // Release slot
            
            // Calculate part boundaries
            offset := int64(pn-1) * partSize
            size := partSize
            if pn == numParts {
                size = int(stat.Size() - offset)
            }
            
            // Read part
            partData := make([]byte, size)
            _, err := file.ReadAt(partData, offset)
            if err != nil {
                errChan <- fmt.Errorf("part %d: %w", pn, err)
                return
            }
            
            // Upload part
            part, err := s.minioClient.PutObjectPart(ctx, "documents-raw", objectName, 
                uploadID, pn, bytes.NewReader(partData), int64(size), 
                minio.PutObjectPartOptions{})
            if err != nil {
                errChan <- fmt.Errorf("part %d: %w", pn, err)
                return
            }
            
            parts[pn-1] = minio.CompletePart{
                PartNumber: pn,
                ETag:       part.ETag,
            }
        }(partNum)
    }
    
    wg.Wait()
    close(errChan)
    
    // Check for errors
    select {
    case err := <-errChan:
        // Abort multipart upload on error
        s.minioClient.AbortMultipartUpload(ctx, "documents-raw", objectName, uploadID)
        return err
    default:
    }
    
    // Complete multipart upload
    _, err = s.minioClient.CompleteMultipartUpload(ctx, "documents-raw", objectName, 
        uploadID, parts)
    return err
}
```

**Performance Comparison**:
- **Before**: 100MB file @ 10MB/s = 10 seconds
- **After**: 100MB in 10MB parts, 5 parallel uploads = ~2.5 seconds
- **Speedup**: **4x faster**

**Benefits**:
- Faster uploads for large documents
- Better utilization of network bandwidth
- Resilience to part failures (retry individual parts)

**Impact**: **MEDIUM** - Significant for large file uploads

---

### Opportunity #7: Cache Warming with Goroutines

#### Current Implementation
```go
// api-service: Cold cache on startup
func main() {
    // ...services initialized...
    // Cache is empty, first requests are slow
}
```

**Problem**: First requests after startup suffer cache misses, slow response times

#### Optimized Implementation with Cache Warming
```go
type CacheWarmer struct {
    profileService  *profile.Service
    documentService *document.Service
    cache           *redis.Client
}

func (cw *CacheWarmer) WarmCache(ctx context.Context) {
    zap.L().Info("starting cache warming")
    
    var wg sync.WaitGroup
    
    // Warm top profiles concurrently
    wg.Add(1)
    go func() {
        defer wg.Done()
        cw.warmTopProfiles(ctx)
    }()
    
    // Warm frequently accessed documents concurrently
    wg.Add(1)
    go func() {
        defer wg.Done()
        cw.warmTopDocuments(ctx)
    }()
    
    // Warm popular search queries concurrently
    wg.Add(1)
    go func() {
        defer wg.Done()
        cw.warmPopularSearches(ctx)
    }()
    
    wg.Wait()
    zap.L().Info("cache warming completed")
}

func (cw *CacheWarmer) warmTopProfiles(ctx context.Context) {
    // Fetch top 100 most accessed profiles from analytics
    topProfiles, err := cw.getTopProfiles(ctx, 100)
    if err != nil {
        zap.L().Warn("failed to get top profiles", zap.Error(err))
        return
    }
    
    // Warm cache with concurrent requests
    sem := make(chan struct{}, 10) // 10 concurrent cache writes
    var wg sync.WaitGroup
    
    for _, profileID := range topProfiles {
        wg.Add(1)
        go func(id uuid.UUID) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            
            // Fetch and cache profile
            profile, err := cw.profileService.GetByID(ctx, id)
            if err != nil {
                zap.L().Warn("failed to warm profile", zap.String("id", id.String()))
                return
            }
            
            zap.L().Debug("warmed profile cache", zap.String("id", profile.ID.String()))
        }(profileID)
    }
    
    wg.Wait()
}

// Usage in main.go
func main() {
    // ... initialize services ...
    
    // Warm cache in background (non-blocking)
    warmer := NewCacheWarmer(profileService, documentService, redisClient)
    go func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        warmer.WarmCache(ctx)
    }()
    
    // Start HTTP server immediately
    server.ListenAndServe()
}
```

**Benefits**:
- Faster response times for common requests after startup
- Reduced database load during peak times
- Better user experience

**Impact**: **MEDIUM** - Improved startup performance

---

### Opportunity #8: Concurrent Health Checks

#### Current Implementation
```go
// api-service/internal/api/handlers/health.go
func (h *HealthHandler) Readiness(c *gin.Context) {
    // Sequential checks
    if err := h.db.Ping(); err != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "reason": "database"})
        return
    }
    
    if h.redisClient != nil {
        if err := h.redisClient.Ping(c.Request.Context()).Err(); err != nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "reason": "redis"})
            return
        }
    }
    
    if err := h.rabbitmqClient.HealthCheck(); err != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "reason": "rabbitmq"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}
```

**Problem**: Health checks take 3 × RTT (30-60ms), slowing down Kubernetes probes

#### Optimized Implementation with Concurrent Checks
```go
func (h *HealthHandler) Readiness(c *gin.Context) {
    ctx := c.Request.Context()
    
    type healthResult struct {
        component string
        healthy   bool
        err       error
    }
    
    results := make(chan healthResult, 3)
    
    // Check DB
    go func() {
        err := h.db.PingContext(ctx)
        results <- healthResult{"database", err == nil, err}
    }()
    
    // Check Redis
    go func() {
        if h.redisClient == nil {
            results <- healthResult{"redis", true, nil}
            return
        }
        err := h.redisClient.Ping(ctx).Err()
        results <- healthResult{"redis", err == nil, err}
    }()
    
    // Check RabbitMQ
    go func() {
        err := h.rabbitmqClient.HealthCheck()
        results <- healthResult{"rabbitmq", err == nil, err}
    }()
    
    // Collect results
    checks := make(map[string]healthResult)
    for i := 0; i < 3; i++ {
        result := <-results
        checks[result.component] = result
    }
    
    // Determine overall health
    allHealthy := true
    details := make(map[string]interface{})
    
    for component, result := range checks {
        details[component] = map[string]interface{}{
            "healthy": result.healthy,
            "error":   errorString(result.err),
        }
        if !result.healthy {
            allHealthy = false
        }
    }
    
    if allHealthy {
        c.JSON(http.StatusOK, gin.H{
            "status":  "healthy",
            "details": details,
        })
    } else {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "status":  "unhealthy",
            "details": details,
        })
    }
}
```

**Performance Comparison**:
- **Before**: 3 checks × 20ms = 60ms
- **After**: max(20ms, 20ms, 20ms) = 20ms
- **Speedup**: **3x faster**

**Benefits**:
- Faster Kubernetes readiness probes
- Quicker service recovery detection
- Better observability (all components checked)

**Impact**: **LOW** - Nice improvement for operational efficiency

---

## 2.3 Concurrency Implementation Priority Matrix

| Opportunity | Impact | Complexity | Priority | Estimated Effort |
|-------------|--------|------------|----------|------------------|
| #2: Concurrent Document Chunk Processing | **HIGH** | Medium | **P0** | 3-4 days |
| #3: Parallel Profile List Enrichment | **HIGH** | Low | **P0** | 1-2 days |
| #4: Worker Message Processing Concurrency | **HIGH** | Medium | **P0** | 2-3 days |
| #5: Concurrent Database Batch Inserts | **HIGH** | Low | **P1** | 1-2 days |
| #6: Parallel File Upload to MinIO | **MEDIUM** | Medium | **P1** | 2-3 days |
| #1: Parallel Cache Operations | **MEDIUM** | Low | **P2** | 1 day |
| #7: Cache Warming with Goroutines | **MEDIUM** | Low | **P2** | 1-2 days |
| #8: Concurrent Health Checks | **LOW** | Low | **P3** | 0.5 day |

**Priority Legend**:
- **P0**: Critical - Implement immediately after GraphRAG feature
- **P1**: High - Implement in first optimization sprint
- **P2**: Medium - Implement in second optimization sprint
- **P3**: Low - Nice to have, implement when time permits

---

## 2.4 Concurrency Best Practices & Patterns

### Pattern 1: Worker Pool for CPU-Bound Tasks
```go
type WorkerPool struct {
    workers   int
    tasks     chan Task
    results   chan Result
    wg        sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        workers: workers,
        tasks:   make(chan Task, workers*2),
        results: make(chan Result, workers*2),
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker(ctx, i)
    }
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
    defer wp.wg.Done()
    for task := range wp.tasks {
        result := task.Execute(ctx)
        select {
        case wp.results <- result:
        case <-ctx.Done():
            return
        }
    }
}

func (wp *WorkerPool) Submit(task Task) {
    wp.tasks <- task
}

func (wp *WorkerPool) Close() {
    close(wp.tasks)
    wp.wg.Wait()
    close(wp.results)
}
```

### Pattern 2: Semaphore for Rate Limiting
```go
type RateLimiter struct {
    sem       chan struct{}
    rateLimit int
    interval  time.Duration
}

func NewRateLimiter(rateLimit int, interval time.Duration) *RateLimiter {
    return &RateLimiter{
        sem:       make(chan struct{}, rateLimit),
        rateLimit: rateLimit,
        interval:  interval,
    }
}

func (rl *RateLimiter) Acquire(ctx context.Context) error {
    select {
    case rl.sem <- struct{}{}:
        go func() {
            time.Sleep(rl.interval)
            <-rl.sem
        }()
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Usage with LLM API calls
func (p *Processor) extractEntitiesWithRateLimit(ctx context.Context, chunk *Chunk) ([]Entity, error) {
    if err := p.rateLimiter.Acquire(ctx); err != nil {
        return nil, err
    }
    return p.llmClient.ExtractEntities(ctx, chunk.Text)
}
```

### Pattern 3: Fan-Out/Fan-In
```go
func FanOut(ctx context.Context, input <-chan Task, workers int) []<-chan Result {
    outputs := make([]<-chan Result, workers)
    
    for i := 0; i < workers; i++ {
        outputs[i] = worker(ctx, input, i)
    }
    
    return outputs
}

func FanIn(ctx context.Context, inputs ...<-chan Result) <-chan Result {
    output := make(chan Result)
    var wg sync.WaitGroup
    
    for _, input := range inputs {
        wg.Add(1)
        go func(ch <-chan Result) {
            defer wg.Done()
            for result := range ch {
                select {
                case output <- result:
                case <-ctx.Done():
                    return
                }
            }
        }(input)
    }
    
    go func() {
        wg.Wait()
        close(output)
    }()
    
    return output
}

// Usage
func ProcessTasksConcurrently(ctx context.Context, tasks []Task) []Result {
    taskChan := make(chan Task, len(tasks))
    for _, task := range tasks {
        taskChan <- task
    }
    close(taskChan)
    
    // Fan-out to 10 workers
    resultChans := FanOut(ctx, taskChan, 10)
    
    // Fan-in results
    resultChan := FanIn(ctx, resultChans...)
    
    // Collect results
    results := []Result{}
    for result := range resultChan {
        results = append(results, result)
    }
    
    return results
}
```

### Pattern 4: Context-Aware Cancellation
```go
func (p *Processor) ProcessWithTimeout(parentCtx context.Context, task *Task) error {
    // Create timeout context
    ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
    defer cancel()
    
    // Use errgroup for concurrent operations with cancellation
    g, gCtx := errgroup.WithContext(ctx)
    
    // Step 1: Extract text
    var text string
    g.Go(func() error {
        var err error
        text, err = p.extractText(gCtx, task.Document)
        return err
    })
    
    // Wait for text extraction before proceeding
    if err := g.Wait(); err != nil {
        return fmt.Errorf("text extraction failed: %w", err)
    }
    
    // Step 2: Process chunks concurrently
    chunks := p.chunkText(text)
    results := make([]ProcessedChunk, len(chunks))
    
    for i, chunk := range chunks {
        i, chunk := i, chunk // Capture loop variables
        g.Go(func() error {
            result, err := p.processChunk(gCtx, chunk)
            if err != nil {
                return err
            }
            results[i] = result
            return nil
        })
    }
    
    // Wait for all chunks, cancel on first error
    if err := g.Wait(); err != nil {
        return fmt.Errorf("chunk processing failed: %w", err)
    }
    
    // Step 3: Build graph
    return p.buildGraph(gCtx, results)
}
```

### Pattern 5: Buffered Channels for Backpressure
```go
type Pipeline struct {
    input    chan Task
    output   chan Result
    capacity int
}

func NewPipeline(capacity int) *Pipeline {
    return &Pipeline{
        input:    make(chan Task, capacity),
        output:   make(chan Result, capacity),
        capacity: capacity,
    }
}

func (p *Pipeline) Start(ctx context.Context, workers int) {
    for i := 0; i < workers; i++ {
        go func() {
            for {
                select {
                case task := <-p.input:
                    result := task.Process()
                    select {
                    case p.output <- result:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
}

// Backpressure: If output channel is full, workers block
```

### Concurrency Safety Checklist

- [ ] Use `sync.Mutex` or `sync.RWMutex` for shared state
- [ ] Avoid data races (use `go run -race` to detect)
- [ ] Use `sync.WaitGroup` to wait for goroutines to complete
- [ ] Always close channels when done sending
- [ ] Use `context.Context` for cancellation propagation
- [ ] Handle goroutine panics with `recover()`
- [ ] Set timeouts on all blocking operations
- [ ] Limit goroutine creation (use worker pools)
- [ ] Monitor goroutine leaks (use pprof)
- [ ] Test with high concurrency loads

---

## 2.5 Monitoring & Observability for Concurrency

### Key Metrics to Track

**System Metrics**:
- `go_goroutines`: Number of active goroutines
- `go_threads`: Number of OS threads
- `process_cpu_seconds_total`: CPU time consumed
- `process_resident_memory_bytes`: Memory usage

**Application Metrics**:
```go
// Track worker pool utilization
workerPoolUtilization := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "worker_pool_active_workers",
        Help: "Number of active workers in the pool",
    },
    []string{"pool_name"},
)

// Track concurrent operations
concurrentOperations := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "concurrent_operations",
        Help: "Number of concurrent operations in progress",
    },
    []string{"operation_type"},
)

// Track queue depths
queueDepth := prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "queue_depth",
        Help: "Number of items waiting in queue",
    },
    []string{"queue_name"},
)
```

### Profiling for Concurrency Issues

```bash
# Enable pprof endpoint
import _ "net/http/pprof"

# CPU profiling
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Goroutine profiling
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof

# Heap profiling
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Trace execution
curl http://localhost:6060/debug/pprof/trace?seconds=5 > trace.out
go tool trace trace.out
```

### Alerting Rules

```yaml
# Alert on goroutine leak
- alert: GoroutineLeakDetected
  expr: go_goroutines > 1000
  for: 5m
  annotations:
    summary: "Possible goroutine leak in {{ $labels.service }}"
    description: "Goroutine count is {{ $value }}, which is abnormally high"

# Alert on high worker pool saturation
- alert: WorkerPoolSaturated
  expr: worker_pool_active_workers / worker_pool_size > 0.9
  for: 5m
  annotations:
    summary: "Worker pool {{ $labels.pool_name }} is saturated"
    description: "Pool utilization is {{ $value * 100 }}%"

# Alert on queue backlog
- alert: QueueBacklogGrowing
  expr: rate(queue_depth[5m]) > 0
  for: 10m
  annotations:
    summary: "Queue {{ $labels.queue_name }} backlog is growing"
    description: "Queue depth is increasing at {{ $value }} items/second"
```

---

# Implementation Roadmap

## Phase 1: GraphRAG Document Processing (3-4 weeks)

### Week 1: Foundation
- [ ] Set up MinIO for object storage
- [ ] Create database migrations for document tables
- [ ] Implement document domain models
- [ ] Create document repository interface

### Week 2: API Service Integration
- [ ] Implement document upload endpoint
- [ ] Add document listing and retrieval endpoints
- [ ] Integrate with RabbitMQ for task publishing
- [ ] Create document service with business logic

### Week 3: GraphRAG Worker Implementation
- [ ] Create GraphRAG worker structure
- [ ] Implement text extraction pipeline
- [ ] Implement LLM integration for entity/relationship extraction
- [ ] Implement embedding generation

### Week 4: Testing & Documentation
- [ ] Write unit and integration tests
- [ ] Perform load testing
- [ ] Create API documentation
- [ ] Deploy to staging environment

**Milestone**: GraphRAG document processing feature is live in staging

---

## Phase 2: Concurrency Optimization - P0 (2-3 weeks)

### Week 5: High-Impact Optimizations
- [ ] **#2: Concurrent Document Chunk Processing**
  - Implement worker pool for chunk processing
  - Add rate limiting for LLM API calls
  - Test performance improvements
  
- [ ] **#3: Parallel Profile List Enrichment**
  - Implement batch query methods
  - Add concurrent enrichment logic
  - Measure latency improvements

### Week 6: Worker Optimization
- [ ] **#4: Worker Message Processing Concurrency**
  - Add configurable concurrency to BaseWorker
  - Configure optimal concurrency per worker type
  - Test throughput improvements
  - Monitor resource usage

**Milestone**: 5x performance improvement on document processing, 25x improvement on list operations

---

## Phase 3: Concurrency Optimization - P1 (2 weeks)

### Week 7: Additional Optimizations
- [ ] **#5: Concurrent Database Batch Inserts**
  - Implement batch insert methods
  - Add COPY command support for bulk operations
  - Measure database load reduction
  
- [ ] **#6: Parallel File Upload to MinIO**
  - Implement multipart upload with concurrency
  - Test large file upload performance
  - Monitor network utilization

**Milestone**: Database operations 25x faster, file uploads 4x faster

---

## Phase 4: Concurrency Optimization - P2 (1 week)

### Week 8: Nice-to-Have Optimizations
- [ ] **#1: Parallel Cache Operations**
  - Refactor cache operations for concurrency
  - Add concurrent invalidation
  
- [ ] **#7: Cache Warming with Goroutines**
  - Implement cache warming logic
  - Add startup cache preloading
  
- [ ] **#8: Concurrent Health Checks**
  - Refactor health check handlers
  - Add parallel health checks

**Milestone**: All P0-P2 concurrency optimizations complete

---

# Success Metrics

## GraphRAG Feature Success Metrics

| Metric | Target | Measurement Method |
|--------|--------|-------------------|
| Document upload success rate | >99% | Monitor upload failures |
| Processing completion rate | >95% | Track status transitions |
| Average processing time | <30 min | Time from upload to completion |
| P95 processing time | <45 min | 95th percentile processing time |
| Entity extraction accuracy | >80% | Manual review of sample documents |
| Semantic search relevance | >70% | User feedback / rating system |
| API response time (upload) | <500ms | Prometheus metrics |
| API response time (search) | <200ms | Prometheus metrics |
| System availability | >99.5% | Uptime monitoring |

## Concurrency Optimization Success Metrics

| Optimization | Baseline | Target | Success Criteria |
|--------------|----------|--------|------------------|
| Document chunk processing | 15 min | 3 min | 5x speedup achieved |
| Profile list enrichment | 500ms | 20ms | 25x speedup achieved |
| Worker throughput | 1 msg/worker | 5-10 msg/worker | 5-10x improvement |
| Database batch inserts | 500ms | 20ms | 25x speedup achieved |
| File uploads (100MB) | 10s | 2.5s | 4x speedup achieved |
| Health check latency | 60ms | 20ms | 3x speedup achieved |
| Overall system throughput | Baseline | 2-3x | Measured with load tests |
| P95 API latency | Baseline | 30-50% reduction | Prometheus metrics |

---

## Risk Assessment & Mitigation

### GraphRAG Feature Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| LLM API costs exceed budget | Medium | High | Implement cost monitoring, set daily limits, consider local LLM |
| LLM rate limits cause bottlenecks | High | Medium | Implement exponential backoff, queue depth monitoring, alert on delays |
| Large documents cause memory issues | Medium | High | Stream processing, chunk-by-chunk, set file size limits |
| Embedding storage grows too large | Medium | Medium | Implement data retention policies, archive old embeddings |
| Vector search performance degrades | Low | Medium | Use pgvector indexes, consider dedicated vector DB (Qdrant) if needed |
| Document processing fails silently | Low | High | Comprehensive error handling, dead letter queue, monitoring alerts |

### Concurrency Optimization Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Goroutine leaks | Medium | High | Rigorous testing with race detector, goroutine monitoring, timeouts |
| Deadlocks | Low | High | Use context cancellation, avoid circular dependencies, timeout all operations |
| Race conditions | Medium | High | Use `go run -race` in tests, mutex protection for shared state |
| Resource exhaustion | Medium | High | Set goroutine limits (worker pools), monitor system resources, load testing |
| Downstream service overload | Medium | Medium | Rate limiting, backpressure mechanisms, circuit breakers |
| Database connection pool exhaustion | Low | Medium | Configure appropriate pool sizes, monitor connection usage |
| Degraded error handling | Low | Medium | Ensure all goroutines have error handling, use errgroup pattern |

---

## Conclusion

This comprehensive plan outlines a structured approach to implementing GraphRAG document processing and optimizing concurrency across the microservices architecture. The implementation is divided into clear phases with measurable success criteria.

### Summary of Benefits

**GraphRAG Feature**:
- Enables semantic search and knowledge graph creation from documents
- Provides intelligent entity and relationship extraction
- Creates new value for users through advanced document understanding

**Concurrency Optimizations**:
- **5-25x performance improvements** across critical operations
- **Reduced API latency** by 30-50% (P95)
- **Increased system throughput** by 2-3x overall
- **Better resource utilization** (CPU, memory, network)
- **Improved scalability** for future growth

### Next Steps

1. **Review and approve** this plan
2. **Prioritize** features based on business needs
3. **Allocate resources** (development team, infrastructure)
4. **Begin Phase 1** implementation (GraphRAG feature)
5. **Iterate and optimize** based on metrics and feedback

---

**Document Version**: 1.0  
**Last Updated**: January 29, 2026  
**Author**: Microservices Architecture Team
