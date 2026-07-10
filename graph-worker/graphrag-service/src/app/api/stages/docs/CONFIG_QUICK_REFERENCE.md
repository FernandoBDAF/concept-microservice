# Configuration Quick Reference

**For full details, see:** [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md)

---

## TL;DR

✅ **Configuration is well-structured and 83% centralized**
- Base config: `core/models/config.py`
- GraphRAG configs: `core/config/graphrag.py` (centralized ✅)
- Ingestion configs: `business/stages/ingestion/*.py` (distributed ⚠️)
- Field metadata: `app/stages_api/field_metadata.py` (centralized ✅)

---

## Quick File Lookup

Need to modify a stage configuration? Here's where to find it:

### Ingestion Pipeline
```
business/stages/ingestion/
├── ingest.py           → IngestConfig
├── clean.py            → CleanConfig
├── chunk.py            → ChunkConfig
├── enrich.py           → EnrichConfig
├── embed.py            → EmbedConfig
├── redundancy.py       → RedundancyConfig
├── trust.py            → TrustConfig
├── compress.py         → CompressConfig
└── backfill_transcript.py → BackfillTranscriptConfig
```

### GraphRAG Pipeline
```
core/config/graphrag.py
├── GraphExtractionConfig
├── EntityResolutionConfig
├── GraphConstructionConfig
└── CommunityDetectionConfig
```

---

## Adding a New Configuration Field

### Step 1: Add to Config Class

**Example:** Adding `chunk_strategy` to `CleanConfig`

```python
# File: business/stages/ingestion/clean.py

@dataclass
class CleanConfig(BaseStageConfig):
    use_llm: bool = True
    # ... existing fields ...
    chunk_strategy: str = "semantic"  # ← NEW FIELD
```

### Step 2: Add Environment Variable Support

```python
@classmethod
def from_args_env(cls, args, env, default_db):
    base = BaseStageConfig.from_args_env(args, env, default_db)
    return cls(
        **vars(base),
        use_llm=bool(getattr(args, "llm", True)),
        chunk_strategy=env.get("CLEAN_CHUNK_STRATEGY", "semantic"),  # ← NEW
    )
```

### Step 3: Add Field Metadata (Optional but Recommended)

```python
# File: app/stages_api/field_metadata.py

FIELD_METADATA: Dict[str, Dict[str, Any]] = {
    # ... existing metadata ...
    "chunk_strategy": {
        "description": "Strategy for chunking during cleaning",
        "ui_type": "select",
        "options": ["semantic", "fixed", "paragraph"],
        "category": "Processing",
        "recommended": "semantic",
    },
}
```

### Step 4: Test

```bash
# 1. Restart backend
cd GraphRAG && python -m app.stages_api.server --port 8080

# 2. Check schema
curl http://localhost:8080/api/v1/stages/clean/config | jq '.fields[] | select(.name=="chunk_strategy")'

# 3. Test in UI
# Field should appear in the form with select dropdown
```

---

## Common Tasks

### Update Default Value

**File:** Config class definition (e.g., `clean.py`)

```python
@dataclass
class CleanConfig(BaseStageConfig):
    use_llm: bool = False  # Changed from True → False
```

### Add Environment Variable

**Files:** 
1. Config's `from_args_env()` method
2. Optionally: `env.example` (for documentation)

```python
llm_retries=int(env.get("CLEAN_LLM_RETRIES", "3"))  # NEW: CLEAN_LLM_RETRIES
```

### Change UI Widget Type

**File:** `app/stages_api/field_metadata.py`

```python
"temperature": {
    "ui_type": "slider",  # Change from "number" to "slider"
    "min": 0.0,
    "max": 2.0,
    "step": 0.1,
}
```

### Add Field Category

**File:** `app/stages_api/constants.py`

```python
CATEGORY_PATTERNS = {
    "LLM Settings": ["model", "temperature", "token", "llm"],
    "My New Category": ["pattern1", "pattern2"],  # ← NEW CATEGORY
}
```

---

## Environment Variable Naming Convention

| Stage Type | Prefix Pattern | Example |
|------------|----------------|---------|
| Common | `LLM_*` | `LLM_RETRIES` |
| Ingestion Stage | `{STAGE}_*` | `CLEAN_WITH_LLM` |
| GraphRAG Stage | `GRAPHRAG_{STAGE}_*` | `GRAPHRAG_EXTRACTION_MODEL` |
| Global | `DB_*`, `MONGODB_*` | `DB_NAME`, `MONGODB_URI` |

---

## Testing Configuration Changes

### 1. Schema Introspection Test

```bash
# Verify field appears in schema
curl http://localhost:8080/api/v1/stages/clean/config | \
  jq '.fields[] | select(.name=="your_field")'
```

### 2. Defaults Test

```bash
# Verify default value
curl http://localhost:8080/api/v1/stages/clean/defaults | \
  jq '.your_field'
```

### 3. Execution Test

```bash
# Test in actual pipeline
curl -X POST http://localhost:8080/api/v1/pipelines/execute \
  -H "Content-Type: application/json" \
  -d '{
    "pipeline": "ingestion",
    "stages": ["clean"],
    "config": {
      "clean": {
        "max": 2,
        "your_field": "test_value"
      }
    }
  }'
```

### 4. Environment Variable Test

```bash
# Set env var and test
export CLEAN_YOUR_FIELD="env_value"
python -c "
from business.stages.ingestion.clean import CleanConfig
import argparse
config = CleanConfig.from_args_env(
    argparse.Namespace(),
    {'CLEAN_YOUR_FIELD': 'env_value'},
    'test_db'
)
print(f'Value: {config.your_field}')
"
```

---

## Troubleshooting

### Field Not Appearing in UI

**Checklist:**
1. ✓ Added to dataclass?
2. ✓ Backend restarted?
3. ✓ Field metadata added? (optional but improves UX)
4. ✓ Frontend refreshed?

### Default Value Not Working

**Check:**
1. Dataclass default: `field: type = default_value`
2. Frontend caches defaults - clear browser cache
3. Check `/stages/{stage}/defaults` endpoint

### Environment Variable Not Loading

**Debug:**
```python
import os
from dotenv import load_dotenv
load_dotenv('.env')
print(f'Value: {os.getenv("YOUR_VAR", "NOT SET")}')
```

### UI Shows Wrong Widget Type

**Fix:** Add/update in `field_metadata.py`:
```python
"field_name": {
    "ui_type": "number|slider|checkbox|select|text"
}
```

---

## Key Files Reference

| What You Need | Where To Look |
|---------------|---------------|
| Add new field | Config class (see File Lookup above) |
| Change default | Config class dataclass definition |
| Add env var | Config's `from_args_env()` method |
| UI customization | `app/stages_api/field_metadata.py` |
| Field categories | `app/stages_api/constants.py` |
| Stage registry | `business/pipelines/runner.py` |
| Collection names | `core/config/paths.py` |
| API endpoints | `app/stages_api/api.py` |
| Base config | `core/models/config.py` |

---

## Need More Details?

See [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md) for:
- Complete configuration reference for all 14 stages
- Full environment variable list (60+ variables)
- Configuration flow diagrams
- Centralization assessment
- Architecture recommendations

---

**Last Updated:** December 9, 2025

