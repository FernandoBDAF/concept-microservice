"""
GraphRAG Retrieval Functions

This module implements retrieval functions for GraphRAG, including entity search,
relationship traversal, community retrieval, and hybrid search combining
multiple retrieval strategies.
"""

import logging
import time
from typing import Dict, List, Any, Optional, Set, Tuple
from pymongo.database import Database
from pymongo.collection import Collection
from src.domain.services.graphrag.indexes import get_graphrag_collections
from src.lib.caching import cached  # Cache entity lookups for performance
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

logger = logging.getLogger(__name__)

# Initialize GraphRAG retrieval metrics
_graphrag_retrieval_calls = Counter(
    "graphrag_retrieval_calls", "Number of GraphRAG retrieval calls", labels=["method"]
)
_graphrag_retrieval_errors = Counter(
    "graphrag_retrieval_errors", "Number of GraphRAG retrieval errors", labels=["method"]
)
_graphrag_retrieval_duration = Histogram(
    "graphrag_retrieval_duration_seconds", "GraphRAG retrieval call duration", labels=["method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_graphrag_retrieval_calls)
_registry.register(_graphrag_retrieval_errors)
_registry.register(_graphrag_retrieval_duration)


class GraphRAGRetrievalEngine:
    """
    Engine for GraphRAG retrieval operations including entity search,
    relationship traversal, and community retrieval.
    """

    def __init__(
        self,
        db: Database,
        entity_search_limit: int = 20,
        relationship_search_limit: int = 50,
        community_search_limit: int = 10,
        max_traversal_depth: int = 2,
        max_traversal_width: int = 20,
    ):
        """
        Initialize the GraphRAG Retrieval Engine.

        Args:
            db: MongoDB database instance
            entity_search_limit: Maximum entities to return from search
            relationship_search_limit: Maximum relationships to return from search
            community_search_limit: Maximum communities to return from search
            max_traversal_depth: Maximum depth for graph traversal
            max_traversal_width: Maximum width for graph traversal
        """
        self.db = db
        self.entity_search_limit = entity_search_limit
        self.relationship_search_limit = relationship_search_limit
        self.community_search_limit = community_search_limit
        self.max_traversal_depth = max_traversal_depth
        self.max_traversal_width = max_traversal_width

        # Get GraphRAG collections
        self.collections = get_graphrag_collections(db)

        logger.info("Initialized GraphRAGRetrievalEngine")

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def entity_search(
        self,
        query_entities: List[str],
        entity_types: Optional[List[str]] = None,
        min_confidence: float = 0.0,
        min_trust_score: float = 0.0,
    ) -> List[Dict[str, Any]]:
        """
        Search for entities by name, type, and other criteria.

        Args:
            query_entities: List of entity names to search for
            entity_types: Optional list of entity types to filter by
            min_confidence: Minimum confidence score
            min_trust_score: Minimum trust score

        Returns:
            List of matching entities
        """
        start_time = time.time()
        labels = {"method": "entity_search"}
        _graphrag_retrieval_calls.inc(labels=labels)
        
        try:
            logger.info(f"Searching entities: {query_entities}")

            entities_collection = self.collections["entities"]

            # Build search query
            search_conditions = []

            for entity in query_entities:
                search_conditions.append(
                    {
                        "$or": [
                            {"name": {"$regex": entity, "$options": "i"}},
                            {"canonical_name": {"$regex": entity, "$options": "i"}},
                            {"aliases": {"$regex": entity, "$options": "i"}},
                        ]
                    }
                )

            if not search_conditions:
                duration = time.time() - start_time
                _graphrag_retrieval_duration.observe(duration, labels=labels)
                return []

            # Base query
            query = {
                "$or": search_conditions,
                "confidence": {"$gte": min_confidence},
                "trust_score": {"$gte": min_trust_score},
            }

            # Add entity type filter
            if entity_types:
                query["type"] = {"$in": entity_types}

            # Execute search
            entities = list(
                entities_collection.find(query)
                .sort([("trust_score", -1), ("centrality_score", -1), ("confidence", -1)])
                .limit(self.entity_search_limit)
            )

            logger.info(f"Found {len(entities)} entities")
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            return entities
        except Exception as e:
            _graphrag_retrieval_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            raise

    def relationship_traversal(
        self,
        entity_ids: List[str],
        max_depth: Optional[int] = None,
        relationship_types: Optional[List[str]] = None,
        min_confidence: float = 0.0,
    ) -> List[Dict[str, Any]]:
        """
        Traverse relationships from given entities using MongoDB aggregation.

        Args:
            entity_ids: List of entity IDs to start traversal from
            max_depth: Maximum traversal depth (default: self.max_traversal_depth)
            relationship_types: Optional list of relationship types to filter by
            min_confidence: Minimum confidence score for relationships

        Returns:
            List of related entities found through traversal
        """
        logger.info(f"Traversing relationships from {len(entity_ids)} entities")

        if not entity_ids:
            return []

        max_depth = max_depth or self.max_traversal_depth
        relations_collection = self.collections["relations"]
        entities_collection = self.collections["entities"]

        # Build aggregation pipeline for graph traversal
        pipeline = [
            # Match relationships involving the starting entities
            {
                "$match": {
                    "$or": [
                        {"subject_id": {"$in": entity_ids}},
                        {"object_id": {"$in": entity_ids}},
                    ],
                    "confidence": {"$gte": min_confidence},
                }
            }
        ]

        # Add relationship type filter
        if relationship_types:
            pipeline[0]["$match"]["predicate"] = {"$in": relationship_types}

        # Get all related entity IDs
        pipeline.extend(
            [
                {
                    "$project": {
                        "related_entities": {
                            "$concatArrays": ["$subject_id", "$object_id"]
                        }
                    }
                },
                {"$unwind": "$related_entities"},
                {"$group": {"_id": "$related_entities"}},
                {"$limit": self.max_traversal_width},
            ]
        )

        # Execute traversal
        traversal_results = list(relations_collection.aggregate(pipeline))
        related_entity_ids = [result["_id"] for result in traversal_results]

        # Remove original entity IDs
        related_entity_ids = [
            eid for eid in related_entity_ids if eid not in entity_ids
        ]

        # Get related entities
        if related_entity_ids:
            related_entities = list(
                entities_collection.find({"entity_id": {"$in": related_entity_ids}})
                .sort([("trust_score", -1), ("centrality_score", -1)])
                .limit(self.max_traversal_width)
            )
        else:
            related_entities = []

        logger.info(f"Found {len(related_entities)} related entities through traversal")
        return related_entities

    def community_retrieval(
        self,
        entity_ids: List[str],
        min_coherence_score: float = 0.0,
        max_level: Optional[int] = None,
    ) -> List[Dict[str, Any]]:
        """
        Retrieve communities containing the given entities.

        Args:
            entity_ids: List of entity IDs
            min_coherence_score: Minimum coherence score for communities
            max_level: Maximum community level to include

        Returns:
            List of relevant communities
        """
        start_time = time.time()
        labels = {"method": "community_retrieval"}
        _graphrag_retrieval_calls.inc(labels=labels)
        
        try:
            logger.info(f"Retrieving communities for {len(entity_ids)} entities")

            communities_collection = self.collections["communities"]

            # Build query
            query = {
                "entities": {"$in": entity_ids},
                "coherence_score": {"$gte": min_coherence_score},
            }

            if max_level is not None:
                query["level"] = {"$lte": max_level}

            # Execute search
            communities = list(
                communities_collection.find(query)
                .sort([("coherence_score", -1), ("entity_count", -1)])
                .limit(self.community_search_limit)
            )

            logger.info(f"Found {len(communities)} relevant communities")
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            return communities
        except Exception as e:
            _graphrag_retrieval_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            raise

    @handle_errors(fallback={"entities": [], "communities": [], "context": ""}, log_traceback=True, reraise=False)
    def hybrid_graphrag_search(
        self, query: str, query_entities: List[str], top_k: int = 10
    ) -> Dict[str, Any]:
        """
        Perform hybrid GraphRAG search combining entity search with community context.

        Args:
            query: Original query text
            query_entities: Entities extracted from query
            top_k: Maximum number of results to return

        Returns:
            Dictionary containing search results
        """
        start_time = time.time()
        labels = {"method": "hybrid_graphrag_search"}
        _graphrag_retrieval_calls.inc(labels=labels)
        
        try:
            logger.info(f"Performing hybrid GraphRAG search for: {query}")

            # 1. Search for entities
            entities = self.entity_search(query_entities)

            # 2. Get related entities via graph traversal
            entity_ids = [entity["entity_id"] for entity in entities]
            related_entities = self.relationship_traversal(entity_ids)

            # 3. Get community summaries
            all_entity_ids = entity_ids + [
                entity["entity_id"] for entity in related_entities
            ]
            communities = self.community_retrieval(all_entity_ids)

            # 4. Build context
            context_parts = []

            # Add entity information
            if entities:
                context_parts.append("## Relevant Entities:")
                for entity in entities[:5]:
                    context_parts.append(
                        f"- {entity['name']} ({entity['type']}): {entity['description']}"
                    )

            # Add related entities
            if related_entities:
                context_parts.append("\n## Related Entities:")
                for entity in related_entities[:5]:
                    context_parts.append(
                        f"- {entity['name']} ({entity['type']}): {entity['description']}"
                    )

            # Add community summaries
            if communities:
                context_parts.append("\n## Community Context:")
                for community in communities:
                    context_parts.append(f"### {community['title']}")
                    context_parts.append(community["summary"])

            context = "\n".join(context_parts)

            execution_time = time.time() - start_time

            logger.info(f"Hybrid search completed in {execution_time:.2f} seconds")

            result = {
                "entities": entities,
                "related_entities": related_entities,
                "communities": communities,
                "context": context,
                "execution_time": execution_time,
                "total_entities": len(entities) + len(related_entities),
                "total_communities": len(communities),
            }
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            return result
        except Exception as e:
            _graphrag_retrieval_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_retrieval_duration.observe(duration, labels=labels)
            raise

    def get_entity_relationships(
        self, entity_id: str, relationship_types: Optional[List[str]] = None
    ) -> List[Dict[str, Any]]:
        """
        Get all relationships for a specific entity.

        Args:
            entity_id: Entity ID
            relationship_types: Optional list of relationship types to filter by

        Returns:
            List of relationships
        """
        logger.info(f"Getting relationships for entity: {entity_id}")

        relations_collection = self.collections["relations"]

        query = {"$or": [{"subject_id": entity_id}, {"object_id": entity_id}]}

        if relationship_types:
            query["predicate"] = {"$in": relationship_types}

        relationships = list(
            relations_collection.find(query).sort(
                [("confidence", -1), ("source_count", -1)]
            )
        )

        logger.info(f"Found {len(relationships)} relationships for entity {entity_id}")
        return relationships

    def get_entity_neighbors(
        self, entity_id: str, max_neighbors: int = 10
    ) -> List[Dict[str, Any]]:
        """
        Get neighboring entities for a specific entity.

        Args:
            entity_id: Entity ID
            max_neighbors: Maximum number of neighbors to return

        Returns:
            List of neighboring entities
        """
        logger.info(f"Getting neighbors for entity: {entity_id}")

        # Get relationships
        relationships = self.get_entity_relationships(entity_id)

        # Collect neighbor entity IDs
        neighbor_ids = set()
        for rel in relationships:
            if rel["subject_id"] != entity_id:
                neighbor_ids.add(rel["subject_id"])
            if rel["object_id"] != entity_id:
                neighbor_ids.add(rel["object_id"])

        # Get neighbor entities
        if neighbor_ids:
            entities_collection = self.collections["entities"]
            neighbors = list(
                entities_collection.find({"entity_id": {"$in": list(neighbor_ids)}})
                .sort([("trust_score", -1), ("centrality_score", -1)])
                .limit(max_neighbors)
            )
        else:
            neighbors = []

        logger.info(f"Found {len(neighbors)} neighbors for entity {entity_id}")
        return neighbors

    def search_by_entity_type(
        self, entity_type: str, min_trust_score: float = 0.0, limit: int = 20
    ) -> List[Dict[str, Any]]:
        """
        Search for entities by type.

        Args:
            entity_type: Type of entities to search for
            min_trust_score: Minimum trust score
            limit: Maximum number of results

        Returns:
            List of entities of the specified type
        """
        logger.info(f"Searching entities by type: {entity_type}")

        entities_collection = self.collections["entities"]

        entities = list(
            entities_collection.find(
                {"type": entity_type, "trust_score": {"$gte": min_trust_score}}
            )
            .sort([("trust_score", -1), ("centrality_score", -1), ("source_count", -1)])
            .limit(limit)
        )

        logger.info(f"Found {len(entities)} entities of type {entity_type}")
        return entities

    def get_retrieval_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the retrieval engine.

        Returns:
            Dictionary containing retrieval statistics
        """
        entities_collection = self.collections["entities"]
        relations_collection = self.collections["relations"]
        communities_collection = self.collections["communities"]

        # Count documents
        total_entities = entities_collection.count_documents({})
        total_relationships = relations_collection.count_documents({})
        total_communities = communities_collection.count_documents({})

        # Count by type
        entity_type_pipeline = [
            {"$group": {"_id": "$type", "count": {"$sum": 1}}},
            {"$sort": {"count": -1}},
        ]
        entity_types = list(entities_collection.aggregate(entity_type_pipeline))

        # Count by level
        community_level_pipeline = [
            {"$group": {"_id": "$level", "count": {"$sum": 1}}},
            {"$sort": {"_id": 1}},
        ]
        community_levels = list(
            communities_collection.aggregate(community_level_pipeline)
        )

        return {
            "total_entities": total_entities,
            "total_relationships": total_relationships,
            "total_communities": total_communities,
            "entity_type_distribution": {
                item["_id"]: item["count"] for item in entity_types
            },
            "community_level_distribution": {
                str(item["_id"]): item["count"] for item in community_levels
            },
            "retrieval_limits": {
                "entity_search_limit": self.entity_search_limit,
                "relationship_search_limit": self.relationship_search_limit,
                "community_search_limit": self.community_search_limit,
                "max_traversal_depth": self.max_traversal_depth,
                "max_traversal_width": self.max_traversal_width,
            },
        }
