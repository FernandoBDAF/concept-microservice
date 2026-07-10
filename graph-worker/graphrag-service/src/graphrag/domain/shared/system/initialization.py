"""
System Database Initialization

Automatically ensures system_data database has required collections, indexes,
and default data. Safe to call multiple times (idempotent).

This is called automatically when the application starts or when
get_mongo_client() is first called.
"""

import logging
from typing import Dict, Any, List, Tuple
from datetime import datetime
from pymongo import ASCENDING, DESCENDING
from pymongo.errors import OperationFailure

from src.core.config.paths import CONSTANT_DB_NAME

logger = logging.getLogger(__name__)

# Track if initialization has already been done this session
_initialized = False


# =============================================================================
# Index Definitions for System Collections
# =============================================================================

SYSTEM_INDEXES: Dict[str, List[Tuple[List[Tuple[str, int]], Dict[str, Any]]]] = {
    "raw_videos": [
        ([("video_id", ASCENDING)], {"unique": True, "name": "video_id_unique"}),
        ([("channel_id", ASCENDING)], {"name": "channel_id"}),
        ([("playlist_id", ASCENDING)], {"name": "playlist_id", "sparse": True}),
        ([("published_at", DESCENDING)], {"name": "published_at_desc"}),
        ([("engagement_score", DESCENDING)], {"name": "engagement_score_desc"}),
        ([("channel_id", ASCENDING), ("published_at", DESCENDING)], {"name": "channel_date"}),
    ],
    "pipeline_input_filters": [
        ([("name", ASCENDING)], {"name": "name"}),
        ([("created_at", DESCENDING)], {"name": "created_at_desc"}),
    ],
    "agent_prompts": [
        ([("prompt_id", ASCENDING)], {"unique": True, "name": "prompt_id_unique"}),
        ([("agent_type", ASCENDING)], {"name": "agent_type"}),
        ([("agent_type", ASCENDING), ("is_default", ASCENDING)], {"name": "agent_type_default"}),
    ],
    "transformation_logs": [
        ([("run_id", ASCENDING)], {"name": "run_id"}),
        ([("stage", ASCENDING)], {"name": "stage"}),
        ([("timestamp", DESCENDING)], {"name": "timestamp_desc"}),
    ],
    "pipeline_executions": [
        ([("execution_id", ASCENDING)], {"unique": True, "name": "execution_id_unique"}),
        ([("status", ASCENDING)], {"name": "status"}),
        ([("started_at", DESCENDING)], {"name": "started_at_desc"}),
    ],
}


# =============================================================================
# Default Prompts
# =============================================================================

DEFAULT_PROMPTS: List[Dict[str, Any]] = [
    {
        "prompt_id": "clean_transcript_default",
        "agent_type": "TranscriptCleanAgent",
        "version": "1.0",
        "name": "Default Transcript Cleaning",
        "description": "Standard cleaning with filler removal and paragraph formatting.",
        "is_default": True,
        "system_prompt": """You are CleanerAgent — an expert linguistic editor specialized in transforming raw or auto-generated YouTube transcripts into fluent, readable text while preserving meaning and language fidelity.

Your goal: normalize and clean the transcript while keeping all technical content, mathematical notation, and speaker cues intact.

Guiding principles:
• Fidelity: Never paraphrase, summarize, or add new information.
• Structure: Produce coherent paragraphs with natural flow.
• Integrity: Preserve all code, formulas, numbers, and symbols exactly as written.
• Speaker Cues: Keep or standardize '[Name]:' or '[Speaker X]:' markers.
• Clarity: Fix casing, punctuation, and spacing; remove fillers and noise.
• Output: Clean plain text only — no markdown, no commentary, no labels.""",
        "user_prompt_template": """Clean and normalize this transcript into fluent, readable text.

Rules:
1. Merge broken lines into full sentences
2. Remove fillers ('uh', 'um', 'you know') and stage cues ([APPLAUSE], [MUSIC])
3. Preserve speaker cues as [Speaker N]:
4. Preserve all technical content, code, and math exactly
5. Group sentences into paragraphs (6-10 sentences each)
6. Keep the same language - do not translate
7. Output ONLY the cleaned text - no markdown, no JSON

{raw_text}""",
        "tags": ["cleaning", "standard"],
        "active": True,
    },
    {
        "prompt_id": "clean_transcript_concise",
        "agent_type": "TranscriptCleanAgent",
        "version": "1.0",
        "name": "Concise Cleaning",
        "description": "Faster cleaning with aggressive filler removal. Good for high-volume processing.",
        "is_default": False,
        "system_prompt": """You are CleanerAgent. Clean transcripts efficiently while preserving meaning.

Rules:
• Keep all technical content, code, math exactly as written
• Remove fillers, stage cues, and noise
• Standardize speaker cues as [Speaker N]:
• Output plain text only, no markdown
• Be concise""",
        "user_prompt_template": """Clean this transcript. Remove fillers, fix formatting, preserve technical content. Output clean text only.

{raw_text}""",
        "tags": ["cleaning", "concise", "fast"],
        "active": True,
    },
]


# =============================================================================
# Initialization Functions
# =============================================================================

def ensure_system_indexes(client) -> Dict[str, Any]:
    """
    Ensure all indexes exist on system_data collections.
    
    Safe to call multiple times - skips existing indexes.
    
    Args:
        client: MongoDB client
        
    Returns:
        Summary of indexes created/skipped
    """
    results = {"created": [], "skipped": [], "errors": []}
    db = client[CONSTANT_DB_NAME]
    
    for collection_name, index_defs in SYSTEM_INDEXES.items():
        coll = db[collection_name]
        
        try:
            existing = set(coll.index_information().keys())
        except Exception:
            existing = set()
        
        for index_spec, index_options in index_defs:
            index_name = index_options.get("name", str(index_spec))
            
            if index_name in existing:
                results["skipped"].append(f"{collection_name}.{index_name}")
                continue
            
            try:
                coll.create_index(index_spec, **index_options)
                results["created"].append(f"{collection_name}.{index_name}")
                logger.debug(f"Created index: {collection_name}.{index_name}")
            except OperationFailure as e:
                error_msg = f"{collection_name}.{index_name}: {e}"
                results["errors"].append(error_msg)
                logger.warning(f"Failed to create index: {error_msg}")
    
    if results["created"]:
        logger.info(f"System indexes created: {len(results['created'])}")
    
    return results


def ensure_default_prompts(client) -> Dict[str, Any]:
    """
    Ensure default prompts exist in agent_prompts collection.
    
    Safe to call multiple times - skips existing prompts.
    
    Args:
        client: MongoDB client
        
    Returns:
        Summary of prompts created/skipped
    """
    results = {"created": [], "skipped": [], "errors": []}
    coll = client[CONSTANT_DB_NAME]["agent_prompts"]
    
    now = datetime.utcnow()
    
    for prompt in DEFAULT_PROMPTS:
        prompt_id = prompt["prompt_id"]
        
        try:
            existing = coll.find_one({"prompt_id": prompt_id}, {"_id": 1})
            
            if existing:
                results["skipped"].append(prompt_id)
                continue
            
            # Insert new prompt with timestamps
            doc = {
                **prompt,
                "created_at": now,
                "updated_at": now,
            }
            coll.insert_one(doc)
            results["created"].append(prompt_id)
            logger.debug(f"Created default prompt: {prompt_id}")
            
        except Exception as e:
            error_msg = f"{prompt_id}: {e}"
            results["errors"].append(error_msg)
            logger.warning(f"Failed to create prompt: {error_msg}")
    
    if results["created"]:
        logger.info(f"Default prompts created: {len(results['created'])}")
    
    return results


def ensure_system_database_initialized(client) -> Dict[str, Any]:
    """
    Ensure system_data database is fully initialized.
    
    This is the main entry point - it ensures:
    1. All required indexes exist
    2. Default prompts are seeded
    
    Safe to call multiple times (idempotent). Uses a session flag to avoid
    repeated checks within the same application run.
    
    Args:
        client: MongoDB client
        
    Returns:
        Summary of initialization actions
    """
    global _initialized
    
    if _initialized:
        return {"status": "already_initialized", "skipped": True}
    
    logger.info(f"Initializing system database: {CONSTANT_DB_NAME}")
    
    results = {
        "database": CONSTANT_DB_NAME,
        "indexes": ensure_system_indexes(client),
        "prompts": ensure_default_prompts(client),
        "status": "initialized",
    }
    
    _initialized = True
    
    total_created = len(results["indexes"]["created"]) + len(results["prompts"]["created"])
    if total_created > 0:
        logger.info(f"System database initialized: {total_created} items created")
    else:
        logger.debug("System database already initialized")
    
    return results


def reset_initialization_flag():
    """Reset the initialization flag (for testing)."""
    global _initialized
    _initialized = False

