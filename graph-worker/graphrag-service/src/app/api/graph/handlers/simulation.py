"""
Query Simulation Handler - Business Logic

Step-by-step query simulation for the Query Simulation Panel.
Allows UI to visualize how an agent queries the knowledge graph.

Steps:
1. Entity Extraction - Extract entities from query text
2. Entity Search - Find matching entities in the graph
3. Relationship Traversal - Expand to related entities
4. Community Context - Get community summaries
5. Context Assembly - Build final LLM context
"""

import logging
import os
import re
import sys
import time
from typing import Dict, Any, List, Optional, Set

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections

logger = logging.getLogger(__name__)

# Common stop words to filter out during entity extraction
STOP_WORDS = {
    "the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for",
    "of", "with", "by", "is", "are", "was", "were", "be", "been", "have",
    "has", "had", "do", "does", "did", "will", "would", "could", "should",
    "may", "might", "can", "what", "how", "when", "where", "why", "who",
    "which", "that", "this", "these", "those", "it", "its", "i", "you",
    "we", "they", "he", "she", "my", "your", "our", "their", "all", "any",
    "some", "no", "not", "more", "most", "other", "into", "from", "about",
}


def simulate_step(
    db_name: str,
    query_text: str,
    step: int,
    previous_results: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """
    Execute a single simulation step.

    Args:
        db_name: Database name
        query_text: User's natural language query
        step: Step number (1-5)
        previous_results: Results from previous steps

    Returns:
        Dictionary with step results and metadata
    """
    start_time = time.time()
    previous_results = previous_results or {}

    try:
        if step == 1:
            result = _step_entity_extraction(query_text)
        elif step == 2:
            result = _step_entity_search(db_name, query_text, previous_results)
        elif step == 3:
            result = _step_relationship_traversal(db_name, previous_results)
        elif step == 4:
            result = _step_community_context(db_name, previous_results)
        elif step == 5:
            result = _step_context_assembly(db_name, previous_results)
        else:
            return {
                "error": f"Invalid step: {step}. Valid steps are 1-5.",
                "valid_steps": [1, 2, 3, 4, 5],
            }

        duration_ms = int((time.time() - start_time) * 1000)
        result["duration_ms"] = duration_ms
        result["status"] = "completed"

        return result

    except Exception as e:
        logger.exception(f"Error in simulation step {step}")
        return {
            "step": step,
            "status": "error",
            "error": str(e),
            "duration_ms": int((time.time() - start_time) * 1000),
        }


def simulate_all(
    db_name: str,
    query_text: str,
) -> Dict[str, Any]:
    """
    Run all simulation steps and return complete results.

    Args:
        db_name: Database name
        query_text: User's natural language query

    Returns:
        Dictionary with all step results
    """
    start_time = time.time()
    results = {}

    # Step 1: Entity Extraction
    step1 = simulate_step(db_name, query_text, 1, {})
    results["step_1"] = step1

    # Step 2: Entity Search
    step2 = simulate_step(db_name, query_text, 2, step1.get("results", {}))
    results["step_2"] = step2

    # Step 3: Relationship Traversal
    step3 = simulate_step(db_name, query_text, 3, {**step1.get("results", {}), **step2.get("results", {})})
    results["step_3"] = step3

    # Step 4: Community Context
    step4 = simulate_step(db_name, query_text, 4, {**step2.get("results", {}), **step3.get("results", {})})
    results["step_4"] = step4

    # Step 5: Context Assembly
    all_results = {
        **step1.get("results", {}),
        **step2.get("results", {}),
        **step3.get("results", {}),
        **step4.get("results", {}),
    }
    step5 = simulate_step(db_name, query_text, 5, all_results)
    results["step_5"] = step5

    total_duration_ms = int((time.time() - start_time) * 1000)

    return {
        "query_text": query_text,
        "steps": results,
        "total_duration_ms": total_duration_ms,
        "status": "completed",
    }


def _step_entity_extraction(query_text: str) -> Dict[str, Any]:
    """
    Step 1: Extract potential entities from query text.
    Uses simple keyword extraction (LLM-based extraction would be used in production).
    """
    # Tokenize and clean
    words = re.findall(r'\b[a-zA-Z][a-zA-Z0-9_-]*\b', query_text.lower())
    
    # Filter stop words and short words
    keywords = [w for w in words if w not in STOP_WORDS and len(w) > 2]
    
    # Also extract potential multi-word entities (capitalized phrases)
    capitalized_phrases = re.findall(r'\b[A-Z][a-zA-Z]+(?:\s+[A-Z][a-zA-Z]+)*\b', query_text)
    
    # Combine and deduplicate
    extracted_entities = list(set(keywords + [p.lower() for p in capitalized_phrases]))
    
    return {
        "step": 1,
        "name": "Entity Extraction",
        "results": {
            "extracted_entities": extracted_entities,
            "capitalized_phrases": capitalized_phrases,
            "keywords": keywords,
        },
        "highlight": {
            "node_ids": [],  # No graph highlighting yet
        },
    }


def _step_entity_search(
    db_name: str,
    query_text: str,
    previous_results: Dict[str, Any],
) -> Dict[str, Any]:
    """
    Step 2: Search for entities matching the extracted keywords.
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]

    extracted_entities = previous_results.get("extracted_entities", [])
    
    if not extracted_entities:
        # Fallback: use query text directly
        extracted_entities = [query_text]

    # Build search query
    search_conditions = []
    for entity in extracted_entities:
        search_conditions.append({
            "$or": [
                {"name": {"$regex": entity, "$options": "i"}},
                {"canonical_name": {"$regex": entity, "$options": "i"}},
                {"aliases": {"$regex": entity, "$options": "i"}},
                {"description": {"$regex": entity, "$options": "i"}},
            ]
        })

    if not search_conditions:
        return {
            "step": 2,
            "name": "Entity Search",
            "results": {
                "matched_entities": [],
                "match_count": 0,
            },
            "highlight": {
                "node_ids": [],
            },
        }

    # Execute search
    matched_docs = list(
        entities_collection.find(
            {"$or": search_conditions},
            {
                "entity_id": 1,
                "name": 1,
                "canonical_name": 1,
                "type": 1,
                "description": 1,
                "confidence": 1,
            }
        )
        .sort([("confidence", -1), ("source_count", -1)])
        .limit(20)
    )

    matched_entities = [
        {
            "entity_id": doc.get("entity_id"),
            "name": doc.get("name") or doc.get("canonical_name"),
            "type": doc.get("type", "OTHER"),
            "description": doc.get("description", "")[:200],
            "confidence": doc.get("confidence", 0.0),
        }
        for doc in matched_docs
    ]

    entity_ids = [e["entity_id"] for e in matched_entities]

    return {
        "step": 2,
        "name": "Entity Search",
        "results": {
            "matched_entities": matched_entities,
            "match_count": len(matched_entities),
            "search_terms": extracted_entities,
        },
        "highlight": {
            "node_ids": entity_ids,
        },
    }


def _step_relationship_traversal(
    db_name: str,
    previous_results: Dict[str, Any],
) -> Dict[str, Any]:
    """
    Step 3: Traverse relationships to find related entities.
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    relations_collection = collections["relations"]
    entities_collection = collections["entities"]

    matched_entities = previous_results.get("matched_entities", [])
    seed_entity_ids = [e["entity_id"] for e in matched_entities if e.get("entity_id")]

    if not seed_entity_ids:
        return {
            "step": 3,
            "name": "Relationship Traversal",
            "results": {
                "related_entities": [],
                "relationships": [],
                "expansion_count": 0,
            },
            "highlight": {
                "node_ids": [],
                "edge_ids": [],
            },
        }

    # Find relationships involving seed entities
    relationships = list(
        relations_collection.find(
            {
                "$or": [
                    {"subject_id": {"$in": seed_entity_ids}},
                    {"object_id": {"$in": seed_entity_ids}},
                ]
            },
            {
                "relationship_id": 1,
                "subject_id": 1,
                "object_id": 1,
                "predicate": 1,
                "confidence": 1,
            }
        )
        .sort("confidence", -1)
        .limit(100)
    )

    # Collect related entity IDs
    related_ids: Set[str] = set()
    for rel in relationships:
        related_ids.add(rel.get("subject_id"))
        related_ids.add(rel.get("object_id"))

    # Remove seed entities
    related_ids -= set(seed_entity_ids)

    # Get related entity details
    related_docs = list(
        entities_collection.find(
            {"entity_id": {"$in": list(related_ids)}},
            {
                "entity_id": 1,
                "name": 1,
                "canonical_name": 1,
                "type": 1,
                "description": 1,
            }
        ).limit(50)
    )

    related_entities = [
        {
            "entity_id": doc.get("entity_id"),
            "name": doc.get("name") or doc.get("canonical_name"),
            "type": doc.get("type", "OTHER"),
            "description": doc.get("description", "")[:100],
        }
        for doc in related_docs
    ]

    relationship_data = [
        {
            "relationship_id": rel.get("relationship_id"),
            "subject_id": rel.get("subject_id"),
            "object_id": rel.get("object_id"),
            "predicate": rel.get("predicate"),
        }
        for rel in relationships[:30]  # Limit for response size
    ]

    all_node_ids = seed_entity_ids + [e["entity_id"] for e in related_entities]
    edge_ids = [rel.get("relationship_id") for rel in relationships[:30]]

    return {
        "step": 3,
        "name": "Relationship Traversal",
        "results": {
            "related_entities": related_entities,
            "relationships": relationship_data,
            "expansion_count": len(related_entities),
            "seed_count": len(seed_entity_ids),
        },
        "highlight": {
            "node_ids": all_node_ids,
            "edge_ids": edge_ids,
        },
    }


def _step_community_context(
    db_name: str,
    previous_results: Dict[str, Any],
) -> Dict[str, Any]:
    """
    Step 4: Get community summaries for relevant entities.
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    communities_collection = collections["communities"]

    # Collect all entity IDs from previous steps
    matched_entities = previous_results.get("matched_entities", [])
    related_entities = previous_results.get("related_entities", [])

    all_entity_ids = [e["entity_id"] for e in matched_entities if e.get("entity_id")]
    all_entity_ids += [e["entity_id"] for e in related_entities if e.get("entity_id")]

    if not all_entity_ids:
        return {
            "step": 4,
            "name": "Community Context",
            "results": {
                "communities": [],
                "community_count": 0,
            },
            "highlight": {
                "community_ids": [],
            },
        }

    # Find communities containing these entities
    communities = list(
        communities_collection.find(
            {"entities": {"$in": all_entity_ids}},
            {
                "community_id": 1,
                "title": 1,
                "summary": 1,
                "level": 1,
                "entity_count": 1,
                "coherence_score": 1,
            }
        )
        .sort([("coherence_score", -1), ("entity_count", -1)])
        .limit(10)
    )

    community_data = [
        {
            "community_id": c.get("community_id"),
            "title": c.get("title", "Untitled Community"),
            "summary": c.get("summary", "")[:300],
            "level": c.get("level", 0),
            "entity_count": c.get("entity_count", 0),
            "coherence_score": c.get("coherence_score", 0.0),
        }
        for c in communities
    ]

    community_ids = [c["community_id"] for c in community_data]

    return {
        "step": 4,
        "name": "Community Context",
        "results": {
            "communities": community_data,
            "community_count": len(community_data),
        },
        "highlight": {
            "community_ids": community_ids,
        },
    }


def _step_context_assembly(
    db_name: str,
    previous_results: Dict[str, Any],
) -> Dict[str, Any]:
    """
    Step 5: Assemble final context for LLM.
    """
    matched_entities = previous_results.get("matched_entities", [])
    related_entities = previous_results.get("related_entities", [])
    communities = previous_results.get("communities", [])

    context_parts = []

    # Add entity information
    if matched_entities:
        context_parts.append("## Relevant Entities:")
        for entity in matched_entities[:5]:
            context_parts.append(
                f"- **{entity.get('name')}** ({entity.get('type')}): {entity.get('description', '')[:150]}"
            )

    # Add related entities
    if related_entities:
        context_parts.append("\n## Related Entities:")
        for entity in related_entities[:5]:
            context_parts.append(
                f"- **{entity.get('name')}** ({entity.get('type')}): {entity.get('description', '')[:100]}"
            )

    # Add community summaries
    if communities:
        context_parts.append("\n## Community Context:")
        for community in communities[:3]:
            context_parts.append(f"\n### {community.get('title')}")
            context_parts.append(community.get("summary", "")[:200])

    context = "\n".join(context_parts)
    
    # Estimate token count (rough approximation: ~1.3 tokens per word)
    word_count = len(context.split())
    token_estimate = int(word_count * 1.3)

    return {
        "step": 5,
        "name": "Context Assembly",
        "results": {
            "context": context,
            "token_estimate": token_estimate,
            "word_count": word_count,
            "breakdown": {
                "entity_count": len(matched_entities),
                "related_count": len(related_entities),
                "community_count": len(communities),
            },
        },
        "highlight": {
            "node_ids": [],  # Context assembly doesn't add new highlights
        },
    }


def get_step_info() -> Dict[str, Any]:
    """
    Get information about available simulation steps.
    """
    return {
        "steps": [
            {
                "step": 1,
                "name": "Entity Extraction",
                "description": "Extract potential entities and keywords from the query text",
            },
            {
                "step": 2,
                "name": "Entity Search",
                "description": "Search for matching entities in the knowledge graph",
            },
            {
                "step": 3,
                "name": "Relationship Traversal",
                "description": "Expand from seed entities to find related entities via relationships",
            },
            {
                "step": 4,
                "name": "Community Context",
                "description": "Retrieve community summaries containing relevant entities",
            },
            {
                "step": 5,
                "name": "Context Assembly",
                "description": "Assemble the final context that would be sent to the LLM",
            },
        ],
        "total_steps": 5,
    }

