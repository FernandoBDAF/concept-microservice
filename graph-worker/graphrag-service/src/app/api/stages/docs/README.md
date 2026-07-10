# Stages API Documentation

**Last Updated:** December 9, 2025  
**Status:** Complete & Operational

---

## Quick Start

```bash
# 1. Start the server
cd /Users/fernandobarroso/repo/KnowledgeManager/GraphRAG
source .venv/bin/activate
python -m app.stages_api.server --port 8080

# 2. Test it
curl http://localhost:8080/api/v1/health
curl http://localhost:8080/api/v1/stages
```

**Frontend UI:** See `/Users/fernandobarroso/repo/KnowledgeManager/StagesUI/`

---

## Documentation Index

### 🚀 Start Here

| Document | Purpose | When to Use |
|----------|---------|-------------|
| **[SESSION_SUMMARY.md](./SESSION_SUMMARY.md)** | Current status & recent changes | Starting a new session |
| **[CONFIG_QUICK_REFERENCE.md](./CONFIG_QUICK_REFERENCE.md)** | Quick config lookup & tasks | Adding/modifying config fields |

### 📖 Comprehensive References

| Document | Purpose | Lines |
|----------|---------|-------|
| **[CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md)** | Complete config system analysis | 987 |
| **[API_DESIGN_SPECIFICATION.md](./API_DESIGN_SPECIFICATION.md)** | Full API specification | 2,675 |
| **[IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md)** | Implementation guide | 2,151 |

### 🎨 UI & Integration

| Document | Purpose | Lines |
|----------|---------|-------|
| **[UI_DESIGN_SPECIFICATION.md](./UI_DESIGN_SPECIFICATION.md)** | Frontend UI design | 1,346 |
| **[postman_collection.json](./postman_collection.json)** | API testing collection | 349 |

### 🏗️ Architecture

| Document | Purpose | Lines |
|----------|---------|-------|
| **[STAGES_API_TECHNICAL_FOUNDATION.md](./STAGES_API_TECHNICAL_FOUNDATION.md)** | Technical foundation | 1,464 |

---

## Document Purposes

### SESSION_SUMMARY.md
- **What:** Latest session status and recent changes
- **Use for:** Understanding current implementation state
- **Updated:** Every session

### CONFIGURATION_ARCHITECTURE.md
- **What:** Complete analysis of all 14 stage configurations
- **Use for:** Understanding config structure, finding env vars, adding fields
- **Includes:** 
  - All config classes with code samples
  - 60+ environment variables documented
  - Centralization assessment
  - File location matrix

### CONFIG_QUICK_REFERENCE.md
- **What:** Quick lookup guide for config tasks
- **Use for:** Day-to-day config work
- **Includes:**
  - File lookup table
  - "How to add a field" guide
  - Common tasks
  - Troubleshooting

### API_DESIGN_SPECIFICATION.md
- **What:** Detailed API design & implementation
- **Use for:** Understanding API contracts, adding endpoints
- **Includes:**
  - All 12 endpoint specifications
  - Request/response schemas
  - Implementation code samples
  - MongoDB persistence details

### IMPLEMENTATION_PLAN.md
- **What:** Step-by-step implementation guide
- **Use for:** Historical reference for implementation phases
- **Status:** All phases complete ✅

### UI_DESIGN_SPECIFICATION.md
- **What:** Frontend UI design specification
- **Use for:** Frontend development
- **Includes:**
  - Component architecture
  - UI mockups and flows
  - Technology stack
  - Implementation phases

### STAGES_API_TECHNICAL_FOUNDATION.md
- **What:** Original technical foundation and analysis
- **Use for:** Understanding the problem domain and design decisions
- **Includes:**
  - Stage system analysis
  - Configuration patterns
  - API design rationale

---

## Quick Navigation

### I want to...

**Add a new configuration field**
→ [CONFIG_QUICK_REFERENCE.md](./CONFIG_QUICK_REFERENCE.md) § "Adding a New Configuration Field"

**Understand all stage configs**
→ [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md) § "Ingestion Pipeline Stages" & "GraphRAG Pipeline Stages"

**Find environment variables**
→ [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md) § "Environment Variables"

**Add a new API endpoint**
→ [API_DESIGN_SPECIFICATION.md](./API_DESIGN_SPECIFICATION.md) § "API Endpoints Specification"

**Understand MongoDB persistence**
→ [API_DESIGN_SPECIFICATION.md](./API_DESIGN_SPECIFICATION.md) § "MongoDB Persistence Layer"

**See what changed recently**
→ [SESSION_SUMMARY.md](./SESSION_SUMMARY.md) § "Recent Fixes & Enhancements"

**Test the API**
→ [postman_collection.json](./postman_collection.json) (import to Postman/Insomnia)

---

## Implementation Status

### Backend (GraphRAG)
✅ **Complete** - All 12 endpoints operational
- MongoDB persistence working
- .env file loading
- Health monitoring
- Selective stage execution

### Frontend (StagesUI)
✅ **Complete** - Fully functional UI
- Located at `/Users/fernandobarroso/repo/KnowledgeManager/StagesUI/`
- See `StagesUI/STAGES_UI_IMPLEMENTATION_GUIDE.md`

### Integration
✅ **Working** - Backend + Frontend integrated
- API: `http://localhost:8080`
- UI: `http://localhost:3001` (or 3000/3002 if ports busy)
- MongoDB: Cloud Atlas or localhost

---

## Key Changes (December 9, 2025)

### What Was Added
1. `repository.py` - MongoDB persistence layer
2. `.env` loading in `server.py`
3. Health endpoint (`GET /health`)
4. Enhanced history endpoint (more fields)
5. Response transformation layer in `api.py`
6. Run individual stages (not full pipeline)

### What Was Fixed
1. API contract mismatches (validation errors format)
2. Defaults endpoint returning wrapped structure
3. Database name from env (`MONGODB_DB` support)
4. Frontend 404 handling during polling
5. Error state management in UI

---

## Environment Setup

```bash
# 1. Copy .env template
cp env.example .env

# 2. Configure database
MONGODB_URI=mongodb://localhost:27017  # or your Atlas URI
MONGODB_DB=2025-12                     # your database name

# 3. Add OpenAI key (for LLM stages)
OPENAI_API_KEY=your_key_here

# 4. Start server
python -m app.stages_api.server --port 8080
```

---

## Dependencies

```bash
# Already in requirements.txt
pymongo>=4.0
python-dotenv>=1.0
```

---

## Next Steps

The Stages API is complete and operational. Future work could include:

- WebSocket support for real-time updates (instead of polling)
- Pipeline log streaming
- Batch pipeline execution
- Stage retry mechanism
- Resource monitoring (CPU/memory)

See [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md) § "Future Improvements" for details.

---

**Questions?** Start with [SESSION_SUMMARY.md](./SESSION_SUMMARY.md) for the latest status.

**Last Review:** December 9, 2025

