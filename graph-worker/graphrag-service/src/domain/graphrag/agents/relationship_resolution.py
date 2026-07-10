"""
Relationship Resolution Agent

This module implements relationship resolution to canonicalize relationships
extracted from different chunks. It groups similar relationships and resolves
their descriptions using LLM-based summarization.
"""

import logging
from src.lib.retry import retry_llm_call
from src.lib.logging import log_exception
from src.lib.error_handling.decorators import handle_errors
from typing import Dict, List, Any, Optional, Set
from collections import defaultdict
from openai import OpenAI
from src.core.models.graphrag import ResolvedRelationship

logger = logging.getLogger(__name__)


class RelationshipResolutionAgent:
    """
    Agent for resolving and canonicalizing relationships across multiple chunks.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",
        temperature: float = 0.1,
    ):
        """
        Initialize the Relationship Resolution Agent.

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for resolution
            temperature: Temperature for LLM generation
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.temperature = temperature
        
        # Check if this is a newer model with API restrictions (no temperature, use max_completion_tokens)
        self._is_restricted_model = any(
            prefix in self.model_name for prefix in ["gpt-5", "o1", "o3"]
        )

        # System prompt for relationship description resolution
        self.resolution_prompt = """
        You are an expert at resolving and summarizing relationship descriptions from multiple sources.

        Your task is to create a comprehensive, coherent description by combining multiple descriptions of the same relationship.

        ## Instructions:

        1. **Combine Information**: Merge all provided descriptions into a single, comprehensive description
        2. **Resolve Contradictions**: If descriptions contradict each other, choose the most accurate or recent information
        3. **Maintain Context**: Keep the relationship context clear throughout the description
        4. **Be Concise**: Create a well-structured description that captures all important aspects
        5. **Third Person**: Write in third person perspective
        6. **YouTube Context**: Consider that this is from YouTube content, so focus on technical and educational aspects

        ## Guidelines:
        - Include all unique information from the descriptions
        - Remove redundant or repetitive information
        - Maintain technical accuracy
        - Keep the description informative but concise
        - Ensure the description flows naturally
        - Focus on the specific relationship between the entities

        ## Output:
        Provide only the resolved description, nothing else.
        """

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def resolve_relationships(
        self,
        extracted_data: List[Dict[str, Any]],
        entity_name_to_id: Optional[Dict[str, str]] = None,
    ) -> List[ResolvedRelationship]:
        """
        Resolve relationships across all extracted data.

        Args:
            extracted_data: List of extraction results from chunks
            entity_name_to_id: Optional mapping of entity names to entity IDs (32-char MD5 hashes)

        Returns:
            List of resolved relationships
        """
        logger.info(
            f"Resolving relationships from {len(extracted_data)} extraction results"
        )

        # Group relationships by (subject, object, predicate) tuple
        relationship_groups = self._group_relationships_by_tuple(extracted_data)

        logger.info(f"Found {len(relationship_groups)} unique relationship groups")

        resolved_relationships = []

        for relationship_key, relationship_group in relationship_groups.items():
            try:
                resolved_relationship = self._resolve_relationship_group(
                    relationship_key, relationship_group, entity_name_to_id
                )
                if resolved_relationship:
                    resolved_relationships.append(resolved_relationship)
            except Exception as e:
                logger.error(
                    f"Failed to resolve relationship group '{relationship_key}': {e}"
                )
                continue

        logger.info(
            f"Successfully resolved {len(resolved_relationships)} relationships"
        )
        return resolved_relationships

    def _group_relationships_by_tuple(
        self, extracted_data: List[Dict[str, Any]]
    ) -> Dict[str, List[Dict[str, Any]]]:
        """
        Group relationships by (subject, object, predicate) tuple.

        Args:
            extracted_data: List of extraction results

        Returns:
            Dictionary mapping relationship keys to relationship groups
        """
        relationship_groups = defaultdict(list)

        for extraction in extracted_data:
            if not extraction or "relationships" not in extraction:
                continue

            for relationship_data in extraction["relationships"]:
                # Create relationship key
                subject_name = (
                    relationship_data["source_entity"]["name"].lower().strip()
                )
                object_name = relationship_data["target_entity"]["name"].lower().strip()
                predicate = relationship_data["relation"].lower().strip()

                relationship_key = f"{subject_name}:{object_name}:{predicate}"

                # Add source information
                relationship_with_source = relationship_data.copy()
                relationship_with_source["source_chunk"] = extraction.get(
                    "chunk_id", "unknown"
                )

                relationship_groups[relationship_key].append(relationship_with_source)

        return dict(relationship_groups)

    def _resolve_relationship_group(
        self,
        relationship_key: str,
        relationship_group: List[Dict[str, Any]],
        entity_name_to_id: Optional[Dict[str, str]] = None,
    ) -> Optional[ResolvedRelationship]:
        """
        Resolve a group of relationships into a single canonical relationship.

        Args:
            relationship_key: Relationship key (subject:object:predicate)
            relationship_group: List of relationship instances to resolve
            entity_name_to_id: Optional mapping of entity names to entity IDs

        Returns:
            Resolved relationship or None if resolution fails
        """
        if not relationship_group:
            return None

        # Parse relationship key
        subject_name, object_name, predicate = relationship_key.split(":")

        # Look up entity IDs if mapping provided
        subject_id = self._lookup_entity_id(subject_name, entity_name_to_id)
        object_id = self._lookup_entity_id(object_name, entity_name_to_id)

        # If entity IDs not found, skip this relationship
        if not subject_id or not object_id:
            logger.debug(
                f"Skipping relationship {subject_name} -> {object_name}: "
                f"entity IDs not found (subject_id={bool(subject_id)}, object_id={bool(object_id)})"
            )
            return None

        # If only one relationship, use it directly
        if len(relationship_group) == 1:
            relationship_data = relationship_group[0]
            return self._create_resolved_relationship_from_single(
                relationship_data, subject_id, object_id, predicate
            )

        # Multiple relationships - need to resolve descriptions
        return self._resolve_multiple_relationships(
            relationship_group, subject_id, object_id, predicate
        )

    def _create_resolved_relationship_from_single(
        self,
        relationship_data: Dict[str, Any],
        subject_id: str,
        object_id: str,
        predicate: str,
    ) -> ResolvedRelationship:
        """
        Create a resolved relationship from a single relationship instance.

        Args:
            relationship_data: Single relationship data
            subject_id: Subject entity ID (32-char MD5 hash)
            object_id: Object entity ID (32-char MD5 hash)
            predicate: Relationship predicate

        Returns:
            Resolved relationship
        """
        # Generate relationship ID
        relationship_id = ResolvedRelationship.generate_relationship_id(
            subject_id, object_id, predicate
        )

        return ResolvedRelationship(
            relationship_id=relationship_id,
            subject_id=subject_id,
            object_id=object_id,
            predicate=predicate,
            description=relationship_data["description"],
            confidence=relationship_data["confidence"],
            source_count=1,
        )

    def _resolve_multiple_relationships(
        self,
        relationship_group: List[Dict[str, Any]],
        subject_id: str,
        object_id: str,
        predicate: str,
    ) -> Optional[ResolvedRelationship]:
        """
        Resolve multiple relationship instances into a single canonical relationship.

        Args:
            relationship_group: List of relationship instances
            subject_id: Subject entity ID (32-char MD5 hash)
            object_id: Object entity ID (32-char MD5 hash)
            predicate: Relationship predicate

        Returns:
            Resolved relationship or None if resolution fails
        """
        # Get entity names for LLM description resolution (lookup from first relationship)
        subject_name = (
            relationship_group[0].get("source_entity", {}).get("name", "subject")
        )
        object_name = (
            relationship_group[0].get("target_entity", {}).get("name", "object")
        )

        # Resolve description using LLM
        resolved_description = self._resolve_descriptions(
            relationship_group, subject_name, object_name, predicate
        )

        if not resolved_description:
            logger.warning(
                f"Failed to resolve description for relationship '{subject_id} -> {object_id}'"
            )
            return None

        # Calculate overall confidence
        overall_confidence = self._calculate_overall_confidence(relationship_group)

        # Generate relationship ID
        relationship_id = ResolvedRelationship.generate_relationship_id(
            subject_id, object_id, predicate
        )

        return ResolvedRelationship(
            relationship_id=relationship_id,
            subject_id=subject_id,
            object_id=object_id,
            predicate=predicate,
            description=resolved_description,
            confidence=overall_confidence,
            source_count=len(relationship_group),
        )

    def _lookup_entity_id(
        self, entity_name: str, entity_name_to_id: Optional[Dict[str, str]] = None
    ) -> Optional[str]:
        """
        Look up entity ID from entity name.

        Args:
            entity_name: Entity name to look up
            entity_name_to_id: Optional mapping of entity names to entity IDs

        Returns:
            Entity ID (32-char MD5 hash) or None if not found
        """
        if not entity_name_to_id:
            # If no mapping provided, generate ID from canonical name
            # This matches how ResolvedEntity.generate_entity_id works
            from src.core.models.graphrag import ResolvedEntity

            return ResolvedEntity.generate_entity_id(entity_name.lower().strip())

        # Normalize entity name for lookup (lowercase, stripped)
        normalized_name = entity_name.lower().strip()

        # Try exact match first
        if normalized_name in entity_name_to_id:
            return entity_name_to_id[normalized_name]

        # Try case-insensitive match
        for name, entity_id in entity_name_to_id.items():
            if name.lower() == normalized_name:
                return entity_id

        # Entity not found in mapping
        return None

    def _resolve_descriptions(
        self,
        relationship_group: List[Dict[str, Any]],
        subject_name: str,
        object_name: str,
        predicate: str,
    ) -> Optional[str]:
        """
        Resolve relationship descriptions using LLM summarization.

        Args:
            relationship_group: List of relationship instances
            subject_name: Subject entity name
            object_name: Object entity name
            predicate: Relationship predicate

        Returns:
            Resolved description or None if resolution fails
        """
        descriptions = [rel["description"] for rel in relationship_group]

        if len(descriptions) == 1:
            return descriptions[0]

        # Combine descriptions for LLM processing
        combined_descriptions = "\n\n".join(
            [f"Description {i+1}: {desc}" for i, desc in enumerate(descriptions)]
        )

        try:
            resolved_description = self._resolve_with_llm(
                subject_name, object_name, predicate, combined_descriptions
            )

            if len(resolved_description) < 10:
                logger.warning(
                    f"Resolved description too short for relationship '{subject_name} -> {object_name}'"
                )
                return None

            return resolved_description

        except Exception as e:
            log_exception(
                logger,
                f"All description resolution attempts failed for relationship '{subject_name} -> {object_name}'",
                e,
            )
            return None

    @retry_llm_call(max_attempts=3)
    def _resolve_with_llm(
        self,
        subject_name: str,
        object_name: str,
        predicate: str,
        combined_descriptions: str,
    ) -> str:
        """Resolve relationship descriptions with automatic retry.

        Args:
            subject_name: Subject entity name
            object_name: Object entity name
            predicate: Relationship predicate
            combined_descriptions: Combined descriptions to resolve

        Returns:
            Resolved description
        """
        # Build request params with appropriate parameters for model
        request_params = {
            "model": self.model_name,
            "messages": [
                {"role": "system", "content": self.resolution_prompt},
                {
                    "role": "user",
                    "content": f"Subject: {subject_name}\nObject: {object_name}\nPredicate: {predicate}\n\nDescriptions:\n{combined_descriptions}",
                },
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

        return response.choices[0].message.content.strip()

    def _calculate_overall_confidence(
        self, relationship_group: List[Dict[str, Any]]
    ) -> float:
        """
        Calculate overall confidence for a group of relationships.

        Args:
            relationship_group: List of relationship instances

        Returns:
            Overall confidence score
        """
        confidences = [rel["confidence"] for rel in relationship_group]

        # Use weighted average based on source count
        total_weight = len(confidences)
        weighted_sum = sum(confidences)

        return weighted_sum / total_weight if total_weight > 0 else 0.0

    def validate_entity_existence(
        self, resolved_relationships: List[ResolvedRelationship], entity_ids: Set[str]
    ) -> List[ResolvedRelationship]:
        """
        Validate that relationships reference existing entities.

        Args:
            resolved_relationships: List of resolved relationships
            entity_ids: Set of existing entity IDs

        Returns:
            List of validated relationships
        """
        validated_relationships = []

        for relationship in resolved_relationships:
            # Check if both subject and object entities exist
            if (
                relationship.subject_id in entity_ids
                and relationship.object_id in entity_ids
            ):
                validated_relationships.append(relationship)
            else:
                logger.debug(
                    f"Skipping relationship {relationship.subject_id} -> {relationship.object_id} "
                    f"due to missing entities"
                )

        logger.info(
            f"Validated {len(validated_relationships)}/{len(resolved_relationships)} relationships"
        )
        return validated_relationships

    def get_resolution_stats(
        self, resolved_relationships: List[ResolvedRelationship]
    ) -> Dict[str, Any]:
        """
        Get statistics about relationship resolution.

        Args:
            resolved_relationships: List of resolved relationships

        Returns:
            Dictionary containing resolution statistics
        """
        if not resolved_relationships:
            return {
                "total_relationships": 0,
                "single_instance_relationships": 0,
                "multi_instance_relationships": 0,
                "avg_source_count": 0,
                "avg_confidence": 0,
                "predicate_distribution": {},
            }

        single_instance = sum(1 for r in resolved_relationships if r.source_count == 1)
        multi_instance = len(resolved_relationships) - single_instance

        avg_source_count = sum(r.source_count for r in resolved_relationships) / len(
            resolved_relationships
        )
        avg_confidence = sum(r.confidence for r in resolved_relationships) / len(
            resolved_relationships
        )

        # Predicate distribution
        predicate_counts = defaultdict(int)
        for relationship in resolved_relationships:
            predicate_counts[relationship.predicate] += 1

        return {
            "total_relationships": len(resolved_relationships),
            "single_instance_relationships": single_instance,
            "multi_instance_relationships": multi_instance,
            "avg_source_count": avg_source_count,
            "avg_confidence": avg_confidence,
            "predicate_distribution": dict(predicate_counts),
        }
