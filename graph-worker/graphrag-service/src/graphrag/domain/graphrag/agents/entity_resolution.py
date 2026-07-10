"""
Entity Resolution Agent

This module implements multi-strategy entity resolution to canonicalize entities
extracted from different chunks. It groups similar entities and resolves their
descriptions using LLM-based summarization.
"""

import logging
import re
from src.lib.retry import retry_llm_call
from src.lib.logging import log_exception
from src.lib.error_handling.decorators import handle_errors
import hashlib
from typing import Dict, List, Any, Optional, Set, Tuple
from collections import defaultdict
from difflib import SequenceMatcher
from openai import OpenAI
from src.core.models.graphrag import ResolvedEntity, EntityModel, EntityType

try:
    from rapidfuzz import fuzz

    HAS_RAPIDFUZZ = True
except ImportError:
    HAS_RAPIDFUZZ = False
    # Fallback to difflib if rapidfuzz not available
    from difflib import SequenceMatcher

try:
    import jellyfish

    HAS_JELLYFISH = True
except ImportError:
    HAS_JELLYFISH = False

logger = logging.getLogger(__name__)


class EntityResolutionAgent:
    """
    Agent for resolving and canonicalizing entities across multiple chunks.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",
        temperature: float = 0.1,
        similarity_threshold: float = 0.85,
        max_input_tokens: Optional[int] = None,
    ):
        """
        Initialize the Entity Resolution Agent.

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for resolution
            temperature: Temperature for LLM generation
            similarity_threshold: Threshold for entity similarity matching
            max_input_tokens: Optional token budget for input (None = disabled, preserves quality)
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.temperature = temperature
        self.similarity_threshold = similarity_threshold
        self.max_input_tokens = max_input_tokens
        
        # Check if this is a newer model with API restrictions (no temperature, use max_completion_tokens)
        self._is_restricted_model = any(
            prefix in self.model_name for prefix in ["gpt-5", "o1", "o3"]
        )

        # System prompt for entity description resolution
        self.resolution_prompt = """
        You are an expert at resolving and summarizing entity descriptions from multiple sources.

        Your task is to create a comprehensive, coherent description by combining multiple descriptions of the same entity.

        ## Instructions:

        1. **Combine Information**: Merge all provided descriptions into a single, comprehensive description
        2. **Resolve Contradictions**: If descriptions contradict each other, choose the most accurate or recent information
        3. **Maintain Context**: Keep the entity name and context clear throughout the description
        4. **Be Concise**: Create a well-structured description that captures all important aspects
        5. **Third Person**: Write in third person perspective
        6. **YouTube Context**: Consider that this is from YouTube content, so focus on technical and educational aspects

        ## Guidelines:
        - Include all unique information from the descriptions
        - Remove redundant or repetitive information
        - Maintain technical accuracy
        - Keep the description informative but concise
        - Ensure the description flows naturally

        ## Output:
        Provide only the resolved description, nothing else.
        """

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def resolve_entities(
        self, extracted_data: List[Dict[str, Any]]
    ) -> List[ResolvedEntity]:
        """
        Resolve entities across all extracted data.

        Args:
            extracted_data: List of extraction results from chunks

        Returns:
            List of resolved entities
        """
        logger.info(f"Resolving entities from {len(extracted_data)} extraction results")

        # Group entities by normalized name
        entity_groups = self._group_entities_by_name(extracted_data)

        logger.info(f"Found {len(entity_groups)} unique entity groups")

        resolved_entities = []

        for normalized_name, entity_group in entity_groups.items():
            try:
                resolved_entity = self._resolve_entity_group(
                    normalized_name, entity_group
                )
                if resolved_entity:
                    resolved_entities.append(resolved_entity)
            except Exception as e:
                logger.error(f"Failed to resolve entity group '{normalized_name}': {e}")
                continue

        logger.info(f"Successfully resolved {len(resolved_entities)} entities")
        return resolved_entities

    def _group_entities_by_name(
        self, extracted_data: List[Dict[str, Any]]
    ) -> Dict[str, List[Dict[str, Any]]]:
        """
        Group entities by normalized name using multiple strategies.

        Args:
            extracted_data: List of extraction results

        Returns:
            Dictionary mapping normalized names to entity groups
        """
        entity_groups = defaultdict(list)

        for extraction in extracted_data:
            if not extraction or "entities" not in extraction:
                continue

            for entity_data in extraction["entities"]:
                entity_name = entity_data["name"]
                normalized_name = self._normalize_entity_name(entity_name)

                # Add source information
                entity_with_source = entity_data.copy()
                entity_with_source["source_chunk"] = extraction.get(
                    "chunk_id", "unknown"
                )

                entity_groups[normalized_name].append(entity_with_source)

        return dict(entity_groups)

    def _normalize_entity_name(self, name: str) -> str:
        """
        Normalize entity name for grouping.

        Args:
            name: Original entity name

        Returns:
            Normalized entity name
        """
        # Convert to lowercase and strip whitespace
        normalized = name.lower().strip()

        # Remove common prefixes/suffixes
        prefixes_to_remove = ["mr.", "ms.", "dr.", "prof.", "the "]
        suffixes_to_remove = [" inc.", " corp.", " ltd.", " llc.", " co."]

        for prefix in prefixes_to_remove:
            if normalized.startswith(prefix):
                normalized = normalized[len(prefix) :].strip()

        for suffix in suffixes_to_remove:
            if normalized.endswith(suffix):
                normalized = normalized[: -len(suffix)].strip()

        return normalized

    def _blocking_keys(self, name: str) -> List[str]:
        """
        Generate blocking keys for efficient candidate search.

        Blocking keys are used to quickly find potential matches in the database
        without scanning all entities. Multiple keys are generated to maximize
        recall while maintaining efficiency.

        Generates:
        - Normalized name (lowercase, stripped)
        - Alnum-only key (alphanumeric characters only)
        - Acronym key (first letter of each word)
        - Optional: Soundex key (phonetic matching)
        - Optional: Metaphone key (phonetic matching)

        Args:
            name: Original entity name

        Returns:
            List of blocking keys
        """
        normalized = self._normalize_entity_name(name)

        keys = set()
        keys.add(normalized)  # Normalized name

        # Alnum-only key (remove all non-alphanumeric except spaces)
        alnum_only = "".join(ch for ch in normalized if ch.isalnum() or ch.isspace())
        alnum_only = re.sub(r"\s+", " ", alnum_only).strip()
        if alnum_only:
            keys.add(alnum_only)

        # Acronym key (first letter of each word, excluding stop words)
        words = normalized.split()
        if len(words) > 1:
            # Filter out common stop words for better acronyms (e.g., "MIT" not "MIOT")
            stop_words = {
                "of",
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
                "with",
                "by",
            }
            filtered_words = [w for w in words if w not in stop_words]
            if len(filtered_words) > 1:
                acronym = "".join(w[0] for w in filtered_words if w)
            else:
                # If filtering removes too many words, use all words
                acronym = "".join(w[0] for w in words if w)
            if acronym:
                keys.add(acronym)

        # Phonetic keys (optional, for better matching of similar-sounding names)
        if HAS_JELLYFISH:
            try:
                # Soundex: phonetic algorithm for English names
                # "Smith" and "Smyth" will have same Soundex
                soundex_key = jellyfish.soundex(normalized)
                if soundex_key:
                    keys.add(f"soundex:{soundex_key}")

                # Metaphone: more accurate than Soundex, handles more cases
                # "Smith" and "Smyth" will have same Metaphone
                metaphone_key = jellyfish.metaphone(normalized)
                if metaphone_key:
                    keys.add(f"metaphone:{metaphone_key}")
            except Exception as e:
                # Graceful degradation if phonetic encoding fails
                logger.debug(f"Phonetic encoding failed for '{name}': {e}")

        return list(keys)

    def _string_score(self, a: str, b: str) -> float:
        """
        Calculate string similarity score between two entity names.

        Uses RapidFuzz for fast, accurate fuzzy matching. Returns a score
        between 0.0 (completely different) and 1.0 (identical).

        Args:
            a: First entity name
            b: Second entity name

        Returns:
            Similarity score between 0.0 and 1.0
        """
        if not a or not b:
            return 0.0

        if a == b:
            return 1.0

        # Normalize both strings before comparison
        a_norm = self._normalize_entity_name(a)
        b_norm = self._normalize_entity_name(b)

        if a_norm == b_norm:
            return 1.0

        if HAS_RAPIDFUZZ:
            # Use RapidFuzz WRatio for best results
            # WRatio combines multiple algorithms and handles word order
            score = fuzz.WRatio(a_norm, b_norm) / 100.0
        else:
            # Fallback to SequenceMatcher if RapidFuzz not available
            score = SequenceMatcher(None, a_norm, b_norm).ratio()

        return max(0.0, min(1.0, score))  # Clamp to [0.0, 1.0]

    def _token_score(self, a: str, b: str) -> float:
        """
        Calculate token overlap score using Jaccard similarity.

        Tokenizes both names into word sets and calculates Jaccard similarity.
        Useful for names with word order variations (e.g., "John Smith" vs "Smith, John").

        Args:
            a: First entity name
            b: Second entity name

        Returns:
            Jaccard similarity score between 0.0 and 1.0
        """
        if not a or not b:
            return 0.0

        def tokenize(name: str) -> Set[str]:
            """Tokenize name into word set (lowercase, no punctuation)."""
            normalized = self._normalize_entity_name(name)
            # Split on whitespace and filter out very short words
            words = [w for w in normalized.split() if len(w) > 1]
            return set(words)

        tokens_a = tokenize(a)
        tokens_b = tokenize(b)

        if not tokens_a and not tokens_b:
            return 1.0
        if not tokens_a or not tokens_b:
            return 0.0

        intersection = len(tokens_a & tokens_b)
        union = len(tokens_a | tokens_b)

        jaccard = intersection / union if union > 0 else 0.0
        return max(0.0, min(1.0, jaccard))

    def _multi_strategy_score(
        self, a: str, b: str, use_embeddings: bool = False
    ) -> float:
        """
        Calculate combined similarity score using multiple strategies.

        Combines:
        - String similarity (RapidFuzz): 0.5 weight
        - Token overlap (Jaccard): 0.3 weight
        - Embedding similarity (optional): 0.2 weight

        If embeddings unavailable, weights are adjusted to:
        - String similarity: 0.6 weight
        - Token overlap: 0.4 weight

        Args:
            a: First entity name
            b: Second entity name
            use_embeddings: Whether to use embedding similarity (future enhancement)

        Returns:
            Combined similarity score between 0.0 and 1.0
        """
        # String similarity (already implemented)
        string_score = self._string_score(a, b)

        # Token overlap (Jaccard)
        token_score = self._token_score(a, b)

        # Embedding similarity (optional, stub for future)
        embedding_score = 0.0
        if use_embeddings:
            # TODO: Implement embedding similarity using sentence-transformers
            # For now, use token score as proxy (can be enhanced later)
            embedding_score = token_score
            logger.debug("Embedding similarity not yet implemented, using token score")

        # Combine scores with weights
        if use_embeddings and embedding_score > 0:
            # Full weights: 0.5*string + 0.3*token + 0.2*embedding
            combined = 0.5 * string_score + 0.3 * token_score + 0.2 * embedding_score
        else:
            # Adjusted weights: 0.6*string + 0.4*token (sums to 1.0)
            combined = 0.6 * string_score + 0.4 * token_score

        return max(0.0, min(1.0, combined))

    def _resolve_entity_group(
        self, normalized_name: str, entity_group: List[Dict[str, Any]]
    ) -> Optional[ResolvedEntity]:
        """
        Resolve a group of entities into a single canonical entity.

        Args:
            normalized_name: Normalized name for the entity group
            entity_group: List of entity instances to resolve

        Returns:
            Resolved entity or None if resolution fails
        """
        if not entity_group:
            return None

        # If only one entity, use it directly
        if len(entity_group) == 1:
            entity_data = entity_group[0]
            return self._create_resolved_entity_from_single(
                entity_data, normalized_name
            )

        # Multiple entities - need to resolve descriptions
        return self._resolve_multiple_entities(normalized_name, entity_group)

    def _create_resolved_entity_from_single(
        self, entity_data: Dict[str, Any], normalized_name: str
    ) -> ResolvedEntity:
        """
        Create a resolved entity from a single entity instance.

        Args:
            entity_data: Single entity data
            normalized_name: Normalized name

        Returns:
            Resolved entity
        """
        canonical_name = entity_data["name"]
        entity_type = EntityType(entity_data["type"])
        entity_id = ResolvedEntity.generate_entity_id(canonical_name, entity_type)

        return ResolvedEntity(
            entity_id=entity_id,
            canonical_name=canonical_name,
            name=entity_data["name"],
            type=entity_type,
            description=entity_data["description"],
            confidence=entity_data["confidence"],
            source_count=1,
            resolution_methods=["single_instance"],
            aliases=[entity_data["name"]],
        )

    def _resolve_multiple_entities(
        self, normalized_name: str, entity_group: List[Dict[str, Any]]
    ) -> Optional[ResolvedEntity]:
        """
        Resolve multiple entity instances into a single canonical entity.

        Args:
            normalized_name: Normalized name for the entity group
            entity_group: List of entity instances

        Returns:
            Resolved entity or None if resolution fails
        """
        # Determine canonical name (most common or highest confidence)
        canonical_name = self._determine_canonical_name(entity_group)

        # Determine entity type using weighted voting
        # Note: existing_type could be passed from candidate lookup, but for now
        # we resolve from entity_group only
        entity_type = self._determine_entity_type(entity_group)

        # Resolve description using LLM
        resolved_description = self._resolve_descriptions(entity_group, canonical_name)

        if not resolved_description:
            logger.warning(
                f"Failed to resolve description for entity '{canonical_name}'"
            )
            return None

        # Calculate overall confidence
        overall_confidence = self._calculate_overall_confidence(entity_group)

        # Generate entity ID
        entity_id = ResolvedEntity.generate_entity_id(canonical_name, entity_type)

        # Collect aliases
        aliases = list(set(entity["name"] for entity in entity_group))

        # Determine resolution methods used
        resolution_methods = ["llm_summary", "name_grouping"]
        if len(entity_group) > 1:
            resolution_methods.append("multi_instance")

        return ResolvedEntity(
            entity_id=entity_id,
            canonical_name=canonical_name,
            name=canonical_name,
            type=entity_type,
            description=resolved_description,
            confidence=overall_confidence,
            source_count=len(entity_group),
            resolution_methods=resolution_methods,
            aliases=aliases,
        )

    def _determine_canonical_name(self, entity_group: List[Dict[str, Any]]) -> str:
        """
        Determine the canonical name for a group of entities.

        Args:
            entity_group: List of entity instances

        Returns:
            Canonical name
        """
        # Count name occurrences
        name_counts = defaultdict(int)
        name_confidences = defaultdict(list)

        for entity in entity_group:
            name = entity["name"]
            confidence = entity["confidence"]
            name_counts[name] += 1
            name_confidences[name].append(confidence)

        # Choose name with highest count, then highest average confidence
        best_name = max(
            name_counts.keys(),
            key=lambda name: (
                name_counts[name],
                sum(name_confidences[name]) / len(name_confidences[name]),
            ),
        )

        return best_name

    def _determine_entity_type(
        self,
        entity_group: List[Dict[str, Any]],
        existing_type: Optional[EntityType] = None,
    ) -> EntityType:
        """
        Determine the entity type for a group of entities using weighted voting.

        Uses weighted scoring: type_score = confidence × source_count
        Prefers existing DB type for stability (tie-breaker).

        Args:
            entity_group: List of entity instances
            existing_type: Existing entity type from database (for stability)

        Returns:
            Selected entity type
        """
        type_scores = defaultdict(float)
        type_counts = defaultdict(int)

        for entity in entity_group:
            entity_type_str = entity.get("type", "OTHER")
            confidence = float(entity.get("confidence", 0.5))

            # Weighted score: confidence × 1 (each entity is one source)
            type_scores[entity_type_str] += confidence
            type_counts[entity_type_str] += 1

        if not type_scores:
            return EntityType.OTHER

        # Find type with highest weighted score
        best_type_str = max(type_scores.keys(), key=lambda t: type_scores[t])
        best_score = type_scores[best_type_str]

        # Check for ties (within 0.1 of best score)
        tied_types = [
            t for t, score in type_scores.items() if abs(score - best_score) < 0.1
        ]

        # If tie and existing type is one of the tied types, prefer it
        if len(tied_types) > 1 and existing_type:
            existing_type_str = (
                existing_type.value
                if isinstance(existing_type, EntityType)
                else str(existing_type)
            )
            if existing_type_str in tied_types:
                logger.debug(
                    f"Type tie detected, preferring existing type '{existing_type_str}' "
                    f"over alternatives: {tied_types}"
                )
                return EntityType(existing_type_str)

        return EntityType(best_type_str)

    def _are_types_compatible(self, type1: EntityType, type2: EntityType) -> bool:
        """
        Check if two entity types are compatible for merging.

        Some type pairs should never be merged:
        - PERSON vs ORG
        - PERSON vs TECHNOLOGY
        - Add more rules as needed

        Args:
            type1: First entity type
            type2: Second entity type

        Returns:
            True if types are compatible, False otherwise
        """
        # Normalize types to EntityType enum
        if isinstance(type1, str):
            type1 = EntityType(type1)
        if isinstance(type2, str):
            type2 = EntityType(type2)

        # Same type is always compatible
        if type1 == type2:
            return True

        # Incompatible type pairs
        incompatible_pairs = [
            (EntityType.PERSON, EntityType.ORG),
            (EntityType.PERSON, EntityType.TECHNOLOGY),
            (EntityType.ORG, EntityType.PERSON),
            (EntityType.TECHNOLOGY, EntityType.PERSON),
        ]

        pair = (type1, type2)
        if pair in incompatible_pairs:
            logger.warning(
                f"Incompatible types detected: {type1.value} vs {type2.value} - "
                "these should not be merged"
            )
            return False

        # All other combinations are compatible (for now)
        return True

    def _resolve_descriptions(
        self, entity_group: List[Dict[str, Any]], entity_name: str
    ) -> Optional[str]:
        """
        Resolve entity descriptions using LLM summarization or local merge.

        Checks description similarity first. If descriptions are near-duplicates
        (similarity >= 0.8), performs local merge without LLM call. Only calls
        LLM for genuinely divergent descriptions.

        Args:
            entity_group: List of entity instances
            entity_name: Name of the entity

        Returns:
            Resolved description or None if resolution fails
        """
        # Extract and clean descriptions
        descriptions = [
            d.strip()
            for d in (entity.get("description", "") for entity in entity_group)
            if d and d.strip()
        ]

        if not descriptions:
            return None

        if len(descriptions) == 1:
            return descriptions[0]

        # De-dup near-identical texts first (exact matches)
        unique_descriptions = []
        seen_exact = set()
        for desc in descriptions:
            desc_lower = desc.lower()
            if desc_lower not in seen_exact:
                seen_exact.add(desc_lower)
                unique_descriptions.append(desc)

        if len(unique_descriptions) == 1:
            return unique_descriptions[0]

        # Check if descriptions are near-duplicates using Jaccard similarity
        similarity = self._description_similarity(unique_descriptions)
        llm_gate_threshold = 0.8  # Configurable threshold for LLM gating

        if similarity >= llm_gate_threshold:
            # Descriptions are similar enough, do local merge without LLM
            logger.debug(
                f"Descriptions for '{entity_name}' are similar (Jaccard={similarity:.3f}), "
                "using local merge instead of LLM"
            )
            return self._merge_descriptions_locally(unique_descriptions)

        # Descriptions diverge significantly, use LLM for proper resolution
        logger.debug(
            f"Descriptions for '{entity_name}' diverge (Jaccard={similarity:.3f}), "
            "using LLM for resolution"
        )
        combined_descriptions = "\n\n".join(
            [f"Description {i+1}: {desc}" for i, desc in enumerate(unique_descriptions)]
        )

        # Apply token budget management if enabled
        if self.max_input_tokens is not None:
            estimated_tokens = self._estimate_tokens(combined_descriptions)
            if estimated_tokens > self.max_input_tokens:
                logger.debug(
                    f"Input token budget exceeded ({estimated_tokens} > {self.max_input_tokens}) "
                    f"for entity '{entity_name}', applying smart truncation"
                )
                combined_descriptions = self._truncate_descriptions_smartly(
                    combined_descriptions, self.max_input_tokens
                )
                logger.debug(
                    f"Truncated to {self._estimate_tokens(combined_descriptions)} tokens"
                )

        try:
            resolved_description = self._resolve_with_llm(
                entity_name, combined_descriptions
            )

            if len(resolved_description) < 10:
                logger.warning(
                    f"Resolved description too short for entity '{entity_name}'"
                )
                return None

            return resolved_description

        except Exception as e:
            log_exception(
                logger,
                f"All description resolution attempts failed for entity '{entity_name}'",
                e,
            )
            return None

    def _description_similarity(self, descriptions: List[str]) -> float:
        """
        Calculate average pairwise Jaccard similarity of descriptions.

        Args:
            descriptions: List of description strings

        Returns:
            Average pairwise Jaccard similarity (0.0 to 1.0)
        """
        if len(descriptions) < 2:
            return 1.0

        def tokens(text: str) -> Set[str]:
            """Tokenize text into word set (lowercase, no punctuation)."""
            # Simple tokenization: lowercase, split on whitespace
            words = text.lower().split()
            # Remove very short words (likely noise)
            return {w for w in words if len(w) > 2}

        similarities = []
        for i in range(len(descriptions)):
            for j in range(i + 1, len(descriptions)):
                tokens_a = tokens(descriptions[i])
                tokens_b = tokens(descriptions[j])

                if not tokens_a and not tokens_b:
                    similarity = 1.0
                elif not tokens_a or not tokens_b:
                    similarity = 0.0
                else:
                    intersection = len(tokens_a & tokens_b)
                    union = len(tokens_a | tokens_b)
                    jaccard = intersection / union if union > 0 else 0.0
                    similarities.append(jaccard)

        return sum(similarities) / len(similarities) if similarities else 0.0

    def _merge_descriptions_locally(self, descriptions: List[str]) -> str:
        """
        Merge near-duplicate descriptions locally without LLM.

        Extracts unique sentences and concatenates them, preserving key information.

        Args:
            descriptions: List of similar descriptions

        Returns:
            Merged description (max 1200 characters)
        """
        sentences = []
        seen_sentences = set()

        for desc in descriptions:
            # Split on sentence boundaries (period, exclamation, question mark)
            for sentence in re.split(r"[.!?]+", desc):
                sentence = sentence.strip()
                if not sentence:
                    continue

                sentence_lower = sentence.lower()
                # Skip if we've seen this sentence before (exact match)
                if sentence_lower in seen_sentences:
                    continue

                seen_sentences.add(sentence_lower)
                sentences.append(sentence)

        # Join sentences with period and space
        merged = ". ".join(sentences)
        if merged and not merged.endswith((".", "!", "?")):
            merged += "."

        # Truncate to 1200 characters to avoid overly long descriptions
        if len(merged) > 1200:
            merged = merged[:1197] + "..."

        return merged

    def _estimate_tokens(self, text: str) -> int:
        """
        Estimate token count for a text string.

        Uses simple approximation: ~4 characters per token.
        For more accuracy, tiktoken could be used in the future.

        Args:
            text: Text to estimate tokens for

        Returns:
            Estimated token count
        """
        # Simple approximation: average ~4 characters per token
        # This is a rough estimate but sufficient for budget management
        # For gpt-4o-mini, actual tokens per character varies but ~4 is reasonable
        return len(text) // 4

    def _truncate_descriptions_smartly(
        self, combined_descriptions: str, max_tokens: int
    ) -> str:
        """
        Intelligently truncate descriptions to fit within token budget.

        Prioritizes informative sentences (longer, more content) over short ones.
        Preserves sentence boundaries and order when possible.

        Args:
            combined_descriptions: Combined descriptions to truncate
            max_tokens: Maximum token budget

        Returns:
            Truncated descriptions within token budget
        """
        # Split into sentences (preserve sentence boundaries)
        sentences = []
        current_sentence = ""

        for char in combined_descriptions:
            current_sentence += char
            if char in ".!?":
                sentence = current_sentence.strip()
                if sentence:
                    sentences.append(sentence)
                current_sentence = ""

        # Add any remaining text
        if current_sentence.strip():
            sentences.append(current_sentence.strip())

        # Score sentences by informativeness (length, word count)
        scored_sentences = []
        for sentence in sentences:
            # Score based on length and word count (longer = more informative)
            word_count = len(sentence.split())
            length_score = len(sentence)
            # Combined score: prioritize sentences with both length and content
            score = length_score + (word_count * 10)
            scored_sentences.append((score, sentence))

        # Sort by score (highest first) to prioritize informative sentences
        scored_sentences.sort(key=lambda x: x[0], reverse=True)

        # Select sentences until we fit within token budget
        selected_sentences = []
        current_tokens = 0

        for score, sentence in scored_sentences:
            sentence_tokens = self._estimate_tokens(sentence)
            if current_tokens + sentence_tokens <= max_tokens:
                selected_sentences.append(sentence)
                current_tokens += sentence_tokens
            else:
                # Can't fit full sentence, but try to fit partial if we have room
                remaining_tokens = max_tokens - current_tokens
                if remaining_tokens > 50:  # Only if meaningful space left
                    # Truncate sentence to fit
                    max_chars = remaining_tokens * 4
                    truncated = sentence[:max_chars]
                    if truncated:
                        selected_sentences.append(truncated + "...")
                break

        # If we have selected sentences, join them; otherwise return truncated original
        if selected_sentences:
            # Try to preserve original order if possible
            # Otherwise just return sorted by score
            result = " ".join(selected_sentences)
        else:
            # Fallback: truncate original text directly
            max_chars = max_tokens * 4
            result = combined_descriptions[:max_chars]
            if len(combined_descriptions) > max_chars:
                result += "..."

        return result

    @retry_llm_call(max_attempts=3)
    def _resolve_with_llm(self, entity_name: str, combined_descriptions: str) -> str:
        """Resolve entity descriptions with automatic retry.

        Args:
            entity_name: Name of the entity
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
                    "content": f"Entity: {entity_name}\n\nDescriptions:\n{combined_descriptions}",
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
        self, entity_group: List[Dict[str, Any]]
    ) -> float:
        """
        Calculate overall confidence for a group of entities using weighted model.

        Uses formula: confidence = clamp(μ + 0.1*log10(1+source_count) + 0.05*agreement, 0, 1)
        Where:
        - μ = mean confidence from entity_group
        - source_count = number of sources (entities)
        - agreement = average pairwise similarity of descriptions

        This rewards multi-source agreement and higher source counts.

        Args:
            entity_group: List of entity instances

        Returns:
            Overall confidence score (0.0 to 1.0)
        """
        import math

        if not entity_group:
            return 0.0

        # Calculate mean confidence
        confidences = [float(entity.get("confidence", 0.5)) for entity in entity_group]
        mean_confidence = sum(confidences) / len(confidences) if confidences else 0.5

        # Source count (number of entities)
        source_count = len(entity_group)

        # Calculate agreement (average pairwise similarity of descriptions)
        descriptions = [
            entity.get("description", "").strip()
            for entity in entity_group
            if entity.get("description", "").strip()
        ]

        if len(descriptions) >= 2:
            agreement = self._description_similarity(descriptions)
        else:
            # Single or no descriptions: assume perfect agreement
            agreement = 1.0

        # Calculate weighted confidence
        # Formula: μ + 0.1*log10(1+source_count) + 0.05*agreement
        confidence = (
            mean_confidence + 0.1 * math.log10(1 + source_count) + 0.05 * agreement
        )

        # Clamp to [0.0, 1.0]
        return max(0.0, min(1.0, confidence))

    def get_resolution_stats(
        self, resolved_entities: List[ResolvedEntity]
    ) -> Dict[str, Any]:
        """
        Get statistics about entity resolution.

        Args:
            resolved_entities: List of resolved entities

        Returns:
            Dictionary containing resolution statistics
        """
        if not resolved_entities:
            return {
                "total_entities": 0,
                "single_instance_entities": 0,
                "multi_instance_entities": 0,
                "avg_source_count": 0,
                "avg_confidence": 0,
                "entity_type_distribution": {},
                "resolution_method_distribution": {},
            }

        single_instance = sum(1 for e in resolved_entities if e.source_count == 1)
        multi_instance = len(resolved_entities) - single_instance

        avg_source_count = sum(e.source_count for e in resolved_entities) / len(
            resolved_entities
        )
        avg_confidence = sum(e.confidence for e in resolved_entities) / len(
            resolved_entities
        )

        # Entity type distribution
        type_counts = defaultdict(int)
        for entity in resolved_entities:
            type_counts[entity.type.value] += 1

        # Resolution method distribution
        method_counts = defaultdict(int)
        for entity in resolved_entities:
            for method in entity.resolution_methods:
                method_counts[method] += 1

        return {
            "total_entities": len(resolved_entities),
            "single_instance_entities": single_instance,
            "multi_instance_entities": multi_instance,
            "avg_source_count": avg_source_count,
            "avg_confidence": avg_confidence,
            "entity_type_distribution": dict(type_counts),
            "resolution_method_distribution": dict(method_counts),
        }
