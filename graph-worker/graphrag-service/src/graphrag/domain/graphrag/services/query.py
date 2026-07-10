"""
GraphRAG Query Processing

This module implements query processing for GraphRAG, including entity extraction
from queries, graph traversal, and community context retrieval.
"""

import logging
import time
from typing import Dict, List, Any, Optional, Set
from collections import defaultdict
from openai import OpenAI
from src.core.models.graphrag import GraphRAGQuery, GraphRAGResponse, KeywordsModel
from src.domain.services.graphrag.indexes import get_graphrag_collections
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

logger = logging.getLogger(__name__)

# Initialize GraphRAG query metrics
_graphrag_query_calls = Counter(
    "graphrag_query_calls", "Number of GraphRAG query calls", labels=["method"]
)
_graphrag_query_errors = Counter(
    "graphrag_query_errors", "Number of GraphRAG query errors", labels=["method"]
)
_graphrag_query_duration = Histogram(
    "graphrag_query_duration_seconds", "GraphRAG query call duration", labels=["method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_graphrag_query_calls)
_registry.register(_graphrag_query_errors)
_registry.register(_graphrag_query_duration)


class GraphRAGQueryProcessor:
    """
    Processor for GraphRAG queries that extracts entities and retrieves relevant context.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",
        temperature: float = 0.1,
        max_query_entities: int = 10,
        max_related_entities: int = 50,
        max_community_context: int = 5,
    ):
        """
        Initialize the GraphRAG Query Processor.

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for query processing
            temperature: Temperature for LLM generation
            max_query_entities: Maximum entities to extract from query
            max_related_entities: Maximum related entities to retrieve
            max_community_context: Maximum community summaries to include
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.temperature = temperature
        self.max_query_entities = max_query_entities
        self.max_related_entities = max_related_entities
        self.max_community_context = max_community_context
        
        # Check if this is a newer model with API restrictions (no temperature, use max_completion_tokens)
        self._is_restricted_model = any(
            prefix in self.model_name for prefix in ["gpt-5", "o1", "o3"]
        )

        # System prompt for query entity extraction
        self.entity_extraction_prompt = """
You are an expert at extracting entities from user queries for GraphRAG processing.

Your task is to identify entities mentioned in the user's query that can be used to search the knowledge graph.

## Instructions:

1. **Entity Identification**: Identify all entities mentioned in the query
2. **Entity Types**: Classify entities into types (PERSON, ORGANIZATION, TECHNOLOGY, CONCEPT, LOCATION, EVENT, OTHER)
3. **Synonyms**: Generate synonyms and related terms for each entity
4. **Query Intent**: Determine the intent of the query (factual, comparative, procedural, etc.)

## Guidelines:
- Extract both explicit and implicit entities
- Include technical terms, proper nouns, and concepts
- Generate relevant synonyms and variations
- Focus on entities that would be useful for knowledge graph search
- Limit to the most important entities (max 10)

## Output Format:
Return a structured response with:
- entities: List of entity names
- entity_types: List of corresponding entity types
- synonyms: List of synonyms for each entity
- intent: Query intent classification

## Output:
Provide only the structured response, nothing else.
"""

        logger.info(f"Initialized GraphRAGQueryProcessor with model {model_name}")

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def process_query(self, query_text: str, db) -> GraphRAGQuery:
        """
        Process a user query to extract entities and determine intent.

        Args:
            query_text: User's query text
            db: MongoDB database instance

        Returns:
            GraphRAGQuery object
        """
        start_time = time.time()
        labels = {"method": "process_query"}
        _graphrag_query_calls.inc(labels=labels)
        
        try:
            logger.info(f"Processing query: {query_text}")
            # Extract entities from query
            extracted_entities = self._extract_query_entities(query_text)

            # Generate keywords for entity search
            keywords = self._generate_query_keywords(query_text, extracted_entities)

            # Determine query intent
            intent = self._determine_query_intent(query_text)

            processing_time = time.time() - start_time

            query_obj = GraphRAGQuery(
                query_text=query_text,
                extracted_entities=extracted_entities,
                query_intent=intent,
                keywords=keywords,
            )

            logger.info(f"Query processed in {processing_time:.2f} seconds")
            logger.info(
                f"Extracted {len(extracted_entities)} entities: {extracted_entities}"
            )

            duration = time.time() - start_time
            _graphrag_query_duration.observe(duration, labels=labels)
            return query_obj

        except Exception as e:
            _graphrag_query_errors.inc(labels=labels)
            duration = time.time() - start_time
            _graphrag_query_duration.observe(duration, labels=labels)
            logger.error(f"Error processing query: {e}")
            # Return basic query object on error
            return GraphRAGQuery(
                query_text=query_text,
                extracted_entities=[],
                query_intent="general",
                keywords=[],
            )

    def _extract_query_entities(self, query_text: str) -> List[str]:
        """
        Extract entities from query text using LLM.

        Args:
            query_text: Query text

        Returns:
            List of extracted entity names
        """
        try:
            # Build request params with appropriate parameters for model
            request_params = {
                "model": self.model_name,
                "messages": [
                    {"role": "system", "content": self.entity_extraction_prompt},
                    {"role": "user", "content": f"Query: {query_text}"},
                ],
            }
            # Newer models don't support custom temperature
            if not self._is_restricted_model:
                request_params["temperature"] = self.temperature
            # Use appropriate token parameter - reasoning models need higher limits
            if self._is_restricted_model:
                request_params["max_completion_tokens"] = 4000  # Higher for reasoning models
            else:
                request_params["max_tokens"] = 1000
                
            response = self.llm_client.chat.completions.create(**request_params)

            # Parse response (simplified - in production, use structured output)
            content = response.choices[0].message.content.strip()

            # Extract entities from response (basic parsing)
            entities = []
            lines = content.split("\n")

            for line in lines:
                if "entities:" in line.lower() or "entity:" in line.lower():
                    # Extract entity names from the line
                    parts = line.split(":")
                    if len(parts) > 1:
                        entity_text = parts[1].strip()
                        # Split by common delimiters
                        for entity in entity_text.split(","):
                            entity = entity.strip().strip("[]\"'")
                            if entity and len(entity) > 1:
                                entities.append(entity)

            # Limit to max entities
            entities = entities[: self.max_query_entities]

            return entities

        except Exception as e:
            logger.warning(f"Failed to extract entities from query: {e}")
            # Fallback: simple keyword extraction
            return self._fallback_entity_extraction(query_text)

    def _fallback_entity_extraction(self, query_text: str) -> List[str]:
        """
        Fallback entity extraction using simple keyword extraction.

        Args:
            query_text: Query text

        Returns:
            List of extracted keywords
        """
        # Simple keyword extraction
        words = query_text.lower().split()

        # Filter out common stop words
        stop_words = {
            "the",
            "a",
            "an",
            "and",
            "or",
            "but",
            "in",
            "on",
            "at",
            "to",
            "for",
            "of",
            "with",
            "by",
            "is",
            "are",
            "was",
            "were",
            "be",
            "been",
            "have",
            "has",
            "had",
            "do",
            "does",
            "did",
            "will",
            "would",
            "could",
            "should",
            "may",
            "might",
            "can",
            "what",
            "how",
            "when",
            "where",
            "why",
            "who",
        }

        keywords = [word for word in words if word not in stop_words and len(word) > 2]

        return keywords[: self.max_query_entities]

    def _generate_query_keywords(
        self, query_text: str, entities: List[str]
    ) -> List[str]:
        """
        Generate additional keywords for entity search.

        Args:
            query_text: Original query text
            entities: Extracted entities

        Returns:
            List of keywords
        """
        try:
            response = self.llm_client.beta.chat.completions.parse(
                model=self.model_name,
                messages=[
                    {
                        "role": "system",
                        "content": "Generate synonyms and related keywords for the given entities. Return only a list of keywords.",
                    },
                    {"role": "user", "content": f"Entities: {', '.join(entities)}"},
                ],
                response_format=KeywordsModel,
                temperature=self.temperature,
            )

            keywords_model = response.choices[0].message.parsed
            return keywords_model.keywords

        except Exception as e:
            logger.warning(f"Failed to generate keywords: {e}")
            return entities  # Fallback to entities

    def _determine_query_intent(self, query_text: str) -> str:
        """
        Determine the intent of the query.

        Args:
            query_text: Query text

        Returns:
            Query intent classification
        """
        # Simple intent classification based on keywords
        query_lower = query_text.lower()

        if any(
            word in query_lower
            for word in ["what", "what is", "what are", "define", "definition"]
        ):
            return "factual"
        elif any(
            word in query_lower
            for word in ["how", "how to", "how do", "steps", "process"]
        ):
            return "procedural"
        elif any(
            word in query_lower
            for word in ["compare", "difference", "vs", "versus", "better"]
        ):
            return "comparative"
        elif any(word in query_lower for word in ["why", "reason", "because", "cause"]):
            return "explanatory"
        elif any(word in query_lower for word in ["when", "time", "date", "schedule"]):
            return "temporal"
        elif any(
            word in query_lower for word in ["where", "location", "place", "find"]
        ):
            return "locational"
        else:
            return "general"

    def search_entities(self, query: GraphRAGQuery, db) -> List[Dict[str, Any]]:
        """
        Search for entities matching the query.

        Args:
            query: Processed query object
            db: MongoDB database instance

        Returns:
            List of matching entities
        """
        logger.info(f"Searching entities for query: {query.query_text}")

        entities_collection = get_graphrag_collections(db)["entities"]

        # Build search query
        search_conditions = []

        # Search by entity names
        for entity in query.extracted_entities:
            search_conditions.append(
                {
                    "$or": [
                        {"name": {"$regex": entity, "$options": "i"}},
                        {"canonical_name": {"$regex": entity, "$options": "i"}},
                        {"aliases": {"$regex": entity, "$options": "i"}},
                    ]
                }
            )

        # Search by keywords
        for keyword in query.keywords:
            search_conditions.append(
                {
                    "$or": [
                        {"name": {"$regex": keyword, "$options": "i"}},
                        {"canonical_name": {"$regex": keyword, "$options": "i"}},
                        {"description": {"$regex": keyword, "$options": "i"}},
                    ]
                }
            )

        if not search_conditions:
            return []

        # Combine search conditions
        search_query = {"$or": search_conditions}

        # Execute search
        entities = list(
            entities_collection.find(search_query).limit(self.max_query_entities)
        )

        logger.info(f"Found {len(entities)} matching entities")
        return entities

    def get_related_entities(
        self, entities: List[Dict[str, Any]], db, max_depth: int = 2
    ) -> List[Dict[str, Any]]:
        """
        Get entities related to the found entities through graph traversal.

        Args:
            entities: List of found entities
            db: MongoDB database instance
            max_depth: Maximum traversal depth

        Returns:
            List of related entities
        """
        logger.info(f"Getting related entities for {len(entities)} entities")

        relations_collection = get_graphrag_collections(db)["relations"]
        entities_collection = get_graphrag_collections(db)["entities"]

        related_entity_ids = set()

        for entity in entities:
            entity_id = entity["entity_id"]

            # Get direct relationships
            relationships = list(
                relations_collection.find(
                    {"$or": [{"subject_id": entity_id}, {"object_id": entity_id}]}
                )
            )

            # Collect related entity IDs
            for rel in relationships:
                if rel["subject_id"] != entity_id:
                    related_entity_ids.add(rel["subject_id"])
                if rel["object_id"] != entity_id:
                    related_entity_ids.add(rel["object_id"])

        # Get related entities
        if related_entity_ids:
            related_entities = list(
                entities_collection.find(
                    {"entity_id": {"$in": list(related_entity_ids)}}
                ).limit(self.max_related_entities)
            )
        else:
            related_entities = []

        logger.info(f"Found {len(related_entities)} related entities")
        return related_entities

    def get_community_context(
        self, entities: List[Dict[str, Any]], db
    ) -> List[Dict[str, Any]]:
        """
        Get community summaries for the entities.

        Args:
            entities: List of entities
            db: MongoDB database instance

        Returns:
            List of community summaries
        """
        logger.info(f"Getting community context for {len(entities)} entities")

        communities_collection = get_graphrag_collections(db)["communities"]

        # Get communities that contain any of the entities
        entity_ids = [entity["entity_id"] for entity in entities]

        communities = list(
            communities_collection.find({"entities": {"$in": entity_ids}}).limit(
                self.max_community_context
            )
        )

        logger.info(f"Found {len(communities)} relevant communities")
        return communities

    def build_context(self, query: GraphRAGQuery, db) -> str:
        """
        Build context string from entities and communities.

        Args:
            query: Processed query object
            db: MongoDB database instance

        Returns:
            Context string for answer generation
        """
        logger.info(f"Building context for query: {query.query_text}")

        # Search for entities
        entities = self.search_entities(query, db)

        # Get related entities
        related_entities = self.get_related_entities(entities, db)

        # Get community context
        communities = self.get_community_context(entities + related_entities, db)

        # Build context string
        context_parts = []

        # Add entity information
        if entities:
            context_parts.append("## Relevant Entities:")
            for entity in entities[:5]:  # Limit to top 5 entities
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

        logger.info(
            f"Built context with {len(entities)} entities and {len(communities)} communities"
        )
        return context

    def get_query_stats(self, query: GraphRAGQuery) -> Dict[str, Any]:
        """
        Get statistics about the processed query.

        Args:
            query: Processed query object

        Returns:
            Dictionary containing query statistics
        """
        return {
            "query_text": query.query_text,
            "extracted_entities": len(query.extracted_entities),
            "keywords": len(query.keywords),
            "query_intent": query.query_intent,
            "entities": query.extracted_entities,
            "keywords_list": query.keywords,
        }
