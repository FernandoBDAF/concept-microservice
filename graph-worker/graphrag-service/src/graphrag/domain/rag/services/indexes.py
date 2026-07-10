from typing import Final, Dict, Any
import os
import time
from pymongo.collection import Collection
from pymongo.errors import OperationFailure
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize index service metrics
_rag_index_calls = Counter(
    "rag_index_calls", "Number of index operations", labels=["operation"]
)
_rag_index_errors = Counter(
    "rag_index_errors", "Number of index operation errors", labels=["operation"]
)
_rag_index_duration = Histogram(
    "rag_index_duration_seconds", "Index operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_index_calls)
_registry.register(_rag_index_errors)
_registry.register(_rag_index_duration)


# Canonical Vector Index configuration (single source of truth)
INDEX_NAME: Final[str] = os.getenv("VECTOR_INDEX_NAME", "vector_index_text") if "os" in globals() else "vector_index_text"  # type: ignore
VECTOR_PATH: Final[str] = "embedding"
# Default to env override if provided, fallback to original 1024
import os
VECTOR_DIM: Final[int] = int(os.getenv("ATLAS_EMBEDDING_DIM", 1024))
VECTOR_SIMILARITY: Final[str] = "cosine"
SEARCH_INDEX_NAME: Final[str] = os.getenv("SEARCH_INDEX_NAME", "search_index_text") if "os" in globals() else "search_index_text"  # type: ignore


def get_vector_index_name() -> str:
    return INDEX_NAME


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def ensure_vector_search_index(collection: Collection) -> None:
    """Ensure an Atlas Vector Search index exists for $vectorSearch.

    Creates a vectorSearch index via the createSearchIndexes command when missing.
    Definition shape (per Atlas docs):
      {
        name: INDEX_NAME,
        type: 'vectorSearch',
        definition: {
          fields: [ { type: 'vector', path: VECTOR_PATH, numDimensions: VECTOR_DIM, similarity: VECTOR_SIMILARITY } ]
        }
      }
    """
    start_time = time.time()
    labels = {"operation": "ensure_vector_search_index"}
    _rag_index_calls.inc(labels=labels)
    
    try:
        db = collection.database
        # Probe existing search indexes (Atlas returns both search and vectorSearch here)
        cur = collection.list_search_indexes()
        for idx in cur:
            if idx.get("name") == INDEX_NAME:
                # Silent return if exists (no need to spam terminal)
                duration = time.time() - start_time
                _rag_index_duration.observe(duration, labels=labels)
                return
    except Exception:
        pass

    try:
        cmd: Dict[str, Any] = {
            "createSearchIndexes": collection.name,
            "indexes": [
                {
                    "name": INDEX_NAME,
                    "type": "vectorSearch",
                    "definition": {
                        "fields": [
                            {
                                "type": "vector",
                                "path": VECTOR_PATH,
                                "numDimensions": VECTOR_DIM,
                                "similarity": VECTOR_SIMILARITY,
                            },
                            # Declare filterable fields for $vectorSearch.filter
                            # Updated: added entities.name, relations.subject
                            # Removed: metadata.tags (empty), relations.predicate, relations.object (not useful)
                            {"type": "filter", "path": "context.tags"},
                            {"type": "filter", "path": "concepts.name"},
                            {"type": "filter", "path": "entities.name"},
                            {"type": "filter", "path": "relations.subject"},
                            {"type": "filter", "path": "metadata.age_days"},
                            {"type": "filter", "path": "published_at"},
                            {"type": "filter", "path": "trust_score"},
                        ]
                    },
                }
            ],
        }
        res = db.command(cmd)
        print(f"createSearchIndexes result: {res}")
        # Brief wait to allow readiness
        time.sleep(2)
        duration = time.time() - start_time
        _rag_index_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_index_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_index_duration.observe(duration, labels=labels)
        print(
            "Warning: Failed to create vectorSearch index via createSearchIndexes. "
            f"Please create it in Atlas UI. Error: {e}"
        )


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def ensure_hybrid_search_index(collection: Collection) -> None:
    """Ensure an Atlas Search (type: 'search') index for hybrid/keyword.

    Your cluster expects the Search index embedding mapping as knnVector with
    dimensions and similarity. We lock to that to avoid regressions.

    IMPORTANT: 'search' != 'vectorSearch'.
    - search.mappings.fields.embedding → { type: 'knnVector', dimensions, similarity }
    - vectorSearch.definition.fields    → { type: 'vector', path, numDimensions, similarity }

    Docs: https://www.mongodb.com/docs/atlas/atlas-vector-search/vector-search-overview/
    """
    start_time = time.time()
    labels = {"operation": "ensure_hybrid_search_index"}
    _rag_index_calls.inc(labels=labels)
    
    try:
        db = collection.database
        # Check if search index exists and is queryable; if failed/non-queryable, drop it
        cur = list(collection.list_search_indexes())
        for idx in cur:
            if idx.get("name") == SEARCH_INDEX_NAME:
                # Atlas returns additional metadata; "queryable": True means healthy
                queryable = idx.get("queryable")
                status = idx.get("status") or idx.get("state")
                if queryable is True:
                    # Silent return if exists and queryable
                    duration = time.time() - start_time
                    _rag_index_duration.observe(duration, labels=labels)
                    return
                # Non-queryable or failed → drop and recreate
                try:
                    collection.drop_search_index(SEARCH_INDEX_NAME)  # PyMongo helper
                    print(
                        f"Dropped non-queryable search index '{SEARCH_INDEX_NAME}' (status={status})."
                    )
                except Exception:
                    # Fallback to command API
                    try:
                        collection.database.command(
                            {
                                "dropSearchIndex": collection.name,
                                "name": SEARCH_INDEX_NAME,
                            }
                        )
                        print(
                            f"Dropped search index via command: '{SEARCH_INDEX_NAME}'."
                        )
                    except Exception:
                        pass
                break
    except Exception:
        pass

    # Locked mapping for your cluster: knnVector + dimensions + similarity
    candidate_knn: Dict[str, Any] = {
        "mappings": {
            "dynamic": True,
            "fields": {
                VECTOR_PATH: {
                    "type": "knnVector",
                    "dimensions": VECTOR_DIM,
                    "similarity": VECTOR_SIMILARITY,
                },
                "text": {"type": "string"},
                "display_text": {"type": "string"},
                "metadata": {
                    "type": "document",
                    "fields": {
                        "age_days": {"type": "number"},
                        "channel_id": {"type": "token"},
                    },
                },
                "context": {"type": "document", "fields": {"tags": {"type": "token"}}},
                "concepts": {"type": "document", "fields": {"name": {"type": "token"}}},
                "entities": {"type": "document", "fields": {"name": {"type": "token"}}},
                "relations": {
                    "type": "document",
                    "fields": {
                        "subject": {"type": "token"},
                    },
                },
                "published_at": {"type": "date"},
                "trust_score": {"type": "number"},
            },
        }
    }

    try:
        # Create with knnVector mapping
        cmd_knn: Dict[str, Any] = {
            "createSearchIndexes": collection.name,
            "indexes": [
                {
                    "name": SEARCH_INDEX_NAME,
                    "type": "search",
                    "definition": candidate_knn,
                }
            ],
        }
        res = db.command(cmd_knn)
        print("createSearchIndexes (search: knnVector+similarity) result:", res)
        time.sleep(2)
        duration = time.time() - start_time
        _rag_index_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_index_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_index_duration.observe(duration, labels=labels)
        print(
            "Warning: Failed to create search index for hybrid via createSearchIndexes. "
            f"Please create it in Atlas UI. Error: {e}"
        )


# Backwards-compatible alias used elsewhere in the codebase
def setup_vector_search_index(collection: Collection, *_args, **_kwargs) -> None:
    ensure_vector_search_index(collection)
