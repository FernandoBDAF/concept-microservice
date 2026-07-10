"""
Community Summarization Agent

This module implements community summarization using LLM to generate
comprehensive summaries of detected communities.
"""

import logging
from src.lib.retry import retry_llm_call
from src.lib.logging import log_exception
from src.lib.error_handling.decorators import handle_errors
from typing import Dict, List, Any, Optional, Tuple
from openai import OpenAI
from src.core.models.graphrag import ResolvedEntity, ResolvedRelationship, CommunitySummary
import networkx as nx

import logging

logger = logging.getLogger(__name__)

# Achievement 2.1: Exact token counting with tiktoken
try:
    import tiktoken

    TIKTOKEN_AVAILABLE = True
except ImportError:
    TIKTOKEN_AVAILABLE = False
    logger.warning(
        "tiktoken not available. Install with: pip install tiktoken. "
        "Falling back to estimation."
    )


class CommunitySummarizationAgent:
    """
    Agent for generating summaries of detected communities using LLM.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",  # Use gpt-4o-mini for all communities
        temperature: float = 0.2,
        max_tokens: Optional[int] = None,
        max_summary_length: int = 2000,
        min_summary_length: int = 100,
        large_model_name: str = "gpt-4o-mini",  # Same model (no switching complexity)
        max_context_tokens: Optional[int] = None,  # Auto-detect based on model
    ):
        """
        Initialize the Community Summarization Agent.

        DESIGN DECISIONS & TESTING NOTES (2024-11-04):
        ==============================================

        1. MODEL SELECTION:
           - Current: gpt-4o-mini for ALL communities
           - Why: Simple, proven, cost-effective, no permission issues
           - Tested alternatives that failed:
             * gpt-5-nano: Doesn't exist in OpenAI API (empty responses)
             * o1-mini: Requires special handling (no system messages, 401 permission errors)
           - Future improvements to test:
             * gpt-4o: Better quality but 10× cost
             * Claude models: May handle large contexts better
             * Model switching: Small model for small communities, large model for huge ones

        2. CONTEXT WINDOW HANDLING:
           - Current: 128k limit (gpt-4o-mini)
           - Truncation threshold: 60k estimated tokens (vs 120k actual limit)
           - Why so conservative: Token estimation is 2× inaccurate (60k estimated ≈ 120k actual)
           - Future improvements to test:
             * Better token counting (tiktoken library for accurate counting)
             * Hierarchical summarization for huge communities
             * Extract top-k most important entities using centrality metrics

        3. TRUNCATION LIMITS:
           - Normal large communities (60k-120k estimated): 30 entities, 50 relationships
           - Huge communities (>240k estimated): 15 entities, 25 relationships
           - Why so aggressive: Testing showed even 80/120 exceeded 128k limit
           - Actual token usage: ~1600 tokens per entity+relationship (vs estimated 200-300)
           - Future improvements to test:
             * Use PageRank/betweenness centrality to select most important entities
             * Stratified sampling to preserve community diversity
             * Multi-pass summarization (summarize sub-communities, then combine)

        4. CONCURRENCY:
           - Current: 300 workers, 800k TPM, 15k RPM
           - Why lower than other stages: Community summarization uses more tokens per request
           - Future improvements to test:
             * Dynamic TPM based on community size distribution
             * Separate pools for small vs large communities

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for summarization (default for normal communities)
            temperature: Temperature for LLM generation
            max_tokens: Maximum tokens for generation
            max_summary_length: Maximum length of summary
            min_summary_length: Minimum length of summary
            large_model_name: Model to use for large communities (default: gpt-4o-mini)
            max_context_tokens: Maximum tokens before truncation (auto-detect if None)
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.large_model_name = large_model_name
        self.temperature = temperature
        self.max_tokens = max_tokens
        self.max_summary_length = max_summary_length
        self.min_summary_length = min_summary_length

        # Achievement 2.1: Initialize tiktoken encoder for exact token counting
        self._token_encoder = None
        if TIKTOKEN_AVAILABLE:
            try:
                # Map model names to tiktoken encoding names
                encoding_map = {
                    "gpt-4o-mini": "o200k_base",  # o200k_base for GPT-4o models
                    "gpt-4o": "o200k_base",
                    "gpt-4-turbo": "cl100k_base",  # GPT-4 Turbo uses cl100k_base
                    "gpt-4": "cl100k_base",
                    "gpt-3.5-turbo": "cl100k_base",
                }
                encoding_name = encoding_map.get(
                    (
                        self.model_name.split("-")[0]
                        + "-"
                        + self.model_name.split("-")[1]
                        if "-" in self.model_name
                        else self.model_name
                    ),
                    "cl100k_base",  # Default to cl100k_base
                )
                self._token_encoder = tiktoken.get_encoding(encoding_name)
                logger.info(
                    f"Initialized tiktoken encoder: {encoding_name} for {self.model_name}"
                )
            except Exception as e:
                logger.warning(f"Failed to initialize tiktoken encoder: {e}")
                self._token_encoder = None

        # Auto-detect context window based on model
        if max_context_tokens is None:
            # Model context windows (conservative estimates with safety margin)
            # Format: model_name -> (context_window, safe_input_limit)
            # safe_input_limit = context_window - system_prompt - output_buffer
            context_limits = {
                # 128k models: use ~120k for input (5k system + 3k output buffer)
                "gpt-4o": 120000,
                "gpt-4o-mini": 120000,
                "gpt-4-turbo": 120000,
                # o1 models: 128k context, but be more conservative (no streaming, reasoning tokens)
                "o1-mini": 110000,  # 128k limit - 18k safety margin for reasoning
                "o1-preview": 110000,  # 128k limit - 18k safety margin
                "o3-mini": 110000,  # 128k limit - 18k safety margin
                # Older models
                "gpt-4": 7000,  # 8k limit
                "gpt-3.5-turbo": 14000,  # 16k limit
            }

            # Find matching model (supports partial matches like "gpt-4o-2024")
            detected_limit = 100000  # Default conservative fallback
            for model_key, limit in context_limits.items():
                if model_key in large_model_name.lower():
                    detected_limit = limit
                    break

            self.max_context_tokens = detected_limit
            logger.info(
                f"Auto-detected context limit for {large_model_name}: "
                f"{self.max_context_tokens:,} tokens "
                f"(will preserve full context up to this limit)"
            )
        else:
            self.max_context_tokens = max_context_tokens

        # Small model context limit - use VERY conservative threshold
        #
        # CRITICAL LEARNING: Token estimation is catastrophically inaccurate!
        # - Estimated: ~200-300 tokens per entity/relationship
        # - Actual: ~1600 tokens per entity/relationship (8× underestimate!)
        # - Result: Communities estimated at 100k often use 128k+ actual tokens
        #
        # Solution: Trigger truncation at 60k estimated (≈120k actual) to stay under 128k limit
        #
        # Future improvements to test:
        # - Use tiktoken library for accurate token counting before sending to API
        # - Profile actual token usage per community size to build better estimation model
        # - Consider model's tokenizer differences (gpt-4o vs gpt-4o-mini may differ)
        small_model_limits = {
            "gpt-4o-mini": 60000,  # Very conservative (was 120k, but estimation is 2× off)
            "gpt-4o": 60000,
            "gpt-4-turbo": 60000,
            "gpt-4": 5000,
            "gpt-3.5-turbo": 10000,
        }
        self.small_model_limit = 60000  # Default - very conservative
        for model_key, limit in small_model_limits.items():
            if model_key in model_name.lower():
                self.small_model_limit = limit
                break

        # System prompt for community summarization
        self.summarization_prompt = """
        You are an expert at creating comprehensive summaries of communities of related entities from YouTube content.

        Your task is to create a clear, informative summary that captures the key aspects of a community of entities and their relationships.

        ## Instructions:

        1. **Community Overview**: Provide a clear overview of what the community represents
        2. **Key Entities**: Identify and describe the main entities in the community
        3. **Relationships**: Explain the key relationships between entities
        4. **Context**: Provide context about how these entities relate to YouTube content
        5. **Technical Focus**: Focus on technical, educational, or informational aspects
        6. **Clarity**: Write in clear, accessible language
        7. **Completeness**: Include all important information from the provided data

        ## Guidelines:
        - Start with a brief title that captures the community's essence
        - Use bullet points or structured format for clarity
        - Include specific entity names and relationship types
        - Explain the significance of the relationships
        - Keep the summary informative but concise
        - Focus on actionable insights

        ## Output Format:
        Provide a structured summary with:
        - Title: Brief descriptive title
        - Summary: Comprehensive summary paragraph
        - Key Entities: List of main entities
        - Key Relationships: List of important relationships
        - Context: How this relates to YouTube content

        ## Output:
        Provide only the structured summary, nothing else.
        """

        logger.info(f"Initialized CommunitySummarizationAgent with model {model_name}")

    @handle_errors(fallback={}, log_traceback=True, reraise=False)
    def summarize_communities(
        self,
        communities: Dict[int, Dict[str, Any]],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
        concurrent: bool = False,
        max_workers: int = 300,
    ) -> Dict[str, CommunitySummary]:
        """
        Generate summaries for all detected communities.

        Args:
            communities: Organized communities by level
            entities: List of resolved entities
            relationships: List of resolved relationships
            concurrent: If True, use concurrent processing with TPM tracking
            max_workers: Maximum number of concurrent workers (only used if concurrent=True)

        Returns:
            Dictionary mapping community IDs to CommunitySummary objects
        """
        total_communities = sum(
            len(level_communities) for level_communities in communities.values()
        )
        logger.info(
            f"Summarizing {total_communities} communities (concurrent={concurrent})"
        )

        entity_map = {entity.entity_id: entity for entity in entities}
        relationship_map = {rel.relationship_id: rel for rel in relationships}

        if concurrent:
            return self._summarize_communities_concurrent(
                communities, entity_map, relationship_map, max_workers
            )
        else:
            return self._summarize_communities_sequential(
                communities, entity_map, relationship_map
            )

    def _summarize_communities_sequential(
        self,
        communities: Dict[int, Dict[str, Any]],
        entity_map: Dict[str, ResolvedEntity],
        relationship_map: Dict[str, ResolvedRelationship],
    ) -> Dict[str, CommunitySummary]:
        """Sequential summarization (original implementation)."""
        community_summaries = {}

        for level, level_communities in communities.items():
            for community_id, community_data in level_communities.items():
                try:
                    summary = self._summarize_single_community(
                        community_data, entity_map, relationship_map
                    )
                    if summary:
                        community_summaries[community_id] = summary
                except Exception as e:
                    logger.error(f"Failed to summarize community {community_id}: {e}")
                    continue

        logger.info(f"Successfully summarized {len(community_summaries)} communities")
        return community_summaries

    def _summarize_communities_concurrent(
        self,
        communities: Dict[int, Dict[str, Any]],
        entity_map: Dict[str, ResolvedEntity],
        relationship_map: Dict[str, ResolvedRelationship],
        max_workers: int = 300,
    ) -> Dict[str, CommunitySummary]:
        """Concurrent summarization with TPM tracking."""
        from src.lib.concurrency import run_concurrent_with_tpm
        import os

        # Prepare all communities for processing
        community_items = []
        for level, level_communities in communities.items():
            for community_id, community_data in level_communities.items():
                community_items.append((community_id, community_data, level))

        if not community_items:
            return {}

        # Define processor function
        def process_community(item):
            """Summarize a single community."""
            community_id, community_data, level = item
            summary = self._summarize_single_community(
                community_data, entity_map, relationship_map
            )
            return community_id, summary

        # Define token estimator
        def estimate_tokens(item):
            """Estimate tokens for community summarization."""
            _, community_data, _ = item
            entity_count = len(community_data.get("entities", []))
            rel_count = len(community_data.get("relationships", []))
            # Each entity ~200 tokens, each relationship ~300 tokens, output ~2000 tokens
            estimated = entity_count * 200 + rel_count * 300 + 2000
            return max(estimated, 500)

        # Use generic concurrent processor
        # Lower TPM for community summarization to avoid rate limits (mixing gpt-4o-mini and gpt-4o)
        target_tpm = int(
            os.getenv("GRAPHRAG_COMMUNITY_SUMMARY_TPM", "800000")
        )  # Lower for safety
        target_rpm = int(
            os.getenv("GRAPHRAG_COMMUNITY_SUMMARY_RPM", "15000")
        )  # Lower for safety

        results = run_concurrent_with_tpm(
            items=community_items,
            processor_fn=process_community,
            estimate_tokens_fn=estimate_tokens,
            max_workers=max_workers,
            target_tpm=target_tpm,
            target_rpm=target_rpm,
            limiter_name="community_summarization",
            progress_name="communities",
        )

        # Convert results to dictionary
        # results contains (item, result) tuples where:
        # - item is the original input: (community_id, community_data, level)
        # - result is what process_community returned: (community_id, summary)
        community_summaries = {}
        for original_item, result in results:
            if result is None:
                continue
            # result is (community_id, summary) tuple from process_community
            if isinstance(result, tuple) and len(result) == 2:
                community_id, summary = result
                if summary:
                    community_summaries[community_id] = summary
            else:
                logger.warning(
                    f"Unexpected result format from process_community: {type(result)}"
                )

        logger.info(
            f"Successfully summarized {len(community_summaries)}/{len(community_items)} communities"
        )
        return community_summaries

    def _summarize_single_community(
        self,
        community_data: Dict[str, Any],
        entity_map: Dict[str, ResolvedEntity],
        relationship_map: Dict[str, ResolvedRelationship],
    ) -> Optional[CommunitySummary]:
        """
        Generate summary for a single community.

        Args:
            community_data: Community data
            entity_map: Map of entity IDs to entities
            relationship_map: Map of relationship IDs to relationships

        Returns:
            CommunitySummary object or None if summarization fails
        """
        community_id = community_data["community_id"]
        level = community_data["level"]

        # Get entities and relationships for this community
        community_entities = []
        community_relationships = []

        for entity_id in community_data["entities"]:
            if entity_id in entity_map:
                community_entities.append(entity_map[entity_id])

        for rel_id in community_data["relationships"]:
            if rel_id in relationship_map:
                community_relationships.append(relationship_map[rel_id])

        if not community_entities:
            logger.warning(f"No entities found for community {community_id}")
            return None

        # Estimate tokens before processing
        estimated_tokens = self._estimate_tokens_for_community(
            community_entities, community_relationships
        )

        # If community exceeds small model limit, use large model with smart truncation
        # Compare against small model limit (120k) not large model limit (950k)
        if estimated_tokens > self.small_model_limit:
            # Calculate truncation needed to stay safely under limit
            # Target: use 50% of limit to ensure we don't exceed even with estimation errors
            # This is EXTREMELY conservative to account for:
            # - Token estimation inaccuracies (actual usage is 8x higher than estimates!)
            # - System prompt + output tokens (~5k)
            # - Safety margin for edge cases
            target_tokens = int(self.max_context_tokens * 0.5)

            # Calculate how many entities/relationships we can fit
            # Rough estimate: ~200 tokens per entity, ~300 tokens per relationship
            # Plus ~2000 tokens for system prompt and output
            available_tokens = target_tokens - 2000  # Reserve for system + output

            # Estimate tokens per entity/relationship
            if len(community_entities) > 0:
                avg_entity_tokens = estimated_tokens / (
                    len(community_entities) + len(community_relationships) * 1.5
                )
            else:
                avg_entity_tokens = 200

            # Calculate max entities/relationships that fit
            # Use ratio of entities to relationships from original
            entity_ratio = len(community_entities) / (
                len(community_entities) + len(community_relationships)
                if (len(community_entities) + len(community_relationships)) > 0
                else 1
            )

            # Calculate max items that fit
            max_items = int(available_tokens / avg_entity_tokens)
            max_entities = max(
                1, min(int(max_items * entity_ratio), len(community_entities))
            )
            max_relationships = max(
                1,
                min(int(max_items * (1 - entity_ratio)), len(community_relationships)),
            )

            # Apply ABSOLUTE MINIMUM caps - empirically tested to avoid context errors
            #
            # TESTING HISTORY:
            # - Tried 150 entities, 250 relationships → FAILED (exceeded 128k)
            # - Tried 80 entities, 120 relationships → FAILED (still exceeded 128k)
            # - Tried 40 entities, 60 relationships → FAILED (still exceeded on some)
            # - Current: 30 entities, 50 relationships → SUCCESS (stays under 128k)
            #
            # MATH VALIDATION:
            # - 30 entities + 50 relationships = 80 items
            # - 80 items × 1600 tokens/item (actual measured) = ~128k tokens (at limit)
            # - With 50% safety target: 40 items × 1600 = ~64k tokens ✓ SAFE
            #
            # Future improvements to test:
            # - Use centrality metrics (PageRank, betweenness) to select MOST important entities
            # - Cluster entities into sub-communities, summarize each, then combine
            # - Use extractive summarization instead of generative for huge communities

            # ABSOLUTE MINIMUM caps to guarantee we stay well under 128k limit
            max_entities = min(
                max_entities, 30, len(community_entities)
            )  # Tested: 30 works, 40 sometimes fails
            max_relationships = min(
                max_relationships, 50, len(community_relationships)
            )  # Tested: 50 works, 60 sometimes fails

            # For extremely large communities (>2× limit), use BARE MINIMUM
            if estimated_tokens > self.max_context_tokens * 2:
                # Very large community (e.g., 4804 entities) - use minimal caps
                # Trade-off: Loses context but guarantees completion
                max_entities = min(max_entities, 15)
                max_relationships = min(max_relationships, 25)

            logger.info(
                f"Community {community_id} exceeds small model limit "
                f"({estimated_tokens:,} > {self.small_model_limit:,} tokens). "
                f"Using {self.large_model_name} (limit: {self.max_context_tokens:,}) with truncation "
                f"({max_entities}/{len(community_entities)} entities, "
                f"{max_relationships}/{len(community_relationships)} relationships)."
            )

            return self.summarize_large_community(
                community_data,
                entity_map,
                relationship_map,
                max_entities=max_entities,
                max_relationships=max_relationships,
            )

        # Generate summary using LLM (normal size community)
        summary_text = self._generate_summary_text(
            community_entities, community_relationships
        )

        # If generation failed (e.g., context length error), fallback to truncation
        if not summary_text:
            logger.warning(
                f"Failed to generate summary for community {community_id}, "
                f"falling back to truncation with {self.large_model_name}"
            )
            return self.summarize_large_community(
                community_data,
                entity_map,
                relationship_map,
                max_entities=min(100, len(community_entities)),
                max_relationships=min(150, len(community_relationships)),
            )

        # Create CommunitySummary object
        return CommunitySummary(
            community_id=community_id,
            level=level,
            title=self._extract_title(summary_text),
            summary=summary_text,
            entities=community_data["entities"],
            entity_count=community_data["entity_count"],
            relationship_count=community_data["relationship_count"],
            coherence_score=community_data["coherence_score"],
        )

    def _count_tokens_exact(self, text: str) -> int:
        """
        Count tokens exactly using tiktoken (Achievement 2.1).

        Args:
            text: Text to count tokens for

        Returns:
            Exact token count
        """
        if self._token_encoder:
            return len(self._token_encoder.encode(text))
        else:
            # Fallback to rough estimation (~4 chars per token)
            return len(text) // 4

    def _estimate_tokens_for_community(
        self, entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
    ) -> int:
        """
        Estimate token count for a community (rough approximation: ~4 chars per token).

        Args:
            entities: Entities in the community
            relationships: Relationships in the community

        Returns:
            Estimated token count
        """
        # Estimate entity text
        entity_text = "\n".join(
            [
                f"- {entity.name} ({entity.type.value}): {entity.description}"
                for entity in entities
            ]
        )

        # Estimate relationship text
        relationship_text = "\n".join(
            [
                f"- {rel.subject_id} -> {rel.object_id} ({rel.predicate}): {rel.description}"
                for rel in relationships
            ]
        )

        # Count tokens exactly using tiktoken (Achievement 2.1)
        input_text = entity_text + "\n" + relationship_text
        input_tokens = self._count_tokens_exact(input_text)

        # Count system prompt tokens exactly
        system_tokens = self._count_tokens_exact(self.summarization_prompt)

        # Output estimate (we don't know actual output until generation)
        output_estimate = self.max_tokens or 2000

        # Total tokens
        total_tokens = system_tokens + input_tokens + output_estimate

        return total_tokens

    def _generate_summary_text(
        self, entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
    ) -> Optional[str]:
        """
        Generate summary text using LLM.

        Args:
            entities: Entities in the community
            relationships: Relationships in the community

        Returns:
            Generated summary text or None if generation fails
        """
        # Prepare entity information
        entities_text = "\n".join(
            [
                f"- {entity.name} ({entity.type.value}): {entity.description}"
                for entity in entities
            ]
        )

        # Prepare relationship information
        relationships_text = "\n".join(
            [
                f"- {rel.subject_id} -> {rel.object_id} ({rel.predicate}): {rel.description}"
                for rel in relationships
            ]
        )

        # Achievement 2.3: Compute predicate profile and add to prompt
        predicate_profile = self._compute_predicate_profile(relationships)

        # Create input text with predicate profile
        predicate_profile_text = ""
        if predicate_profile:
            top_predicates = ", ".join(predicate_profile[:5])  # Top 5 predicates
            predicate_profile_text = f"\n\nThis community focuses on these relationship types: {top_predicates}"

        input_text = f"""
        Entities in this community:
        {entities_text}

        Relationships in this community:
        {relationships_text}{predicate_profile_text}

        Please create a comprehensive summary of this community.
        """

        try:
            summary_text = self._generate_with_llm(input_text)

            if len(summary_text) < self.min_summary_length:
                logger.warning(
                    f"Generated summary too short: {len(summary_text)} characters"
                )
                return None

            if len(summary_text) > self.max_summary_length:
                summary_text = summary_text[: self.max_summary_length] + "..."

            return summary_text

        except Exception as e:
            # If context length error, try with truncation
            error_msg = str(e)
            if (
                "context_length_exceeded" in error_msg
                or "context length" in error_msg.lower()
            ):
                logger.warning(
                    f"Context length exceeded, falling back to truncation: {error_msg[:200]}"
                )
                # Return None to trigger fallback to summarize_large_community
                return None

            log_exception(logger, "All summary generation attempts failed", e)
            return None

    @retry_llm_call(max_attempts=3)
    def _generate_with_llm(self, input_text: str, use_large_model: bool = False) -> str:
        """Generate community summary with automatic retry.

        Args:
            input_text: Input text for summarization
            use_large_model: If True, use large_model_name instead of model_name

        Returns:
            Generated summary text
        """
        # Select model based on use_large_model flag
        model = self.large_model_name if use_large_model else self.model_name

        # Check if this is a restricted model that requires max_completion_tokens
        is_restricted_model = any(
            prefix in model.lower()
            for prefix in ["gpt-5", "o1", "o3", "o4"]
        )

        # Prepare request parameters
        request_params = {
            "model": model,
            "messages": [
                {"role": "system", "content": self.summarization_prompt},
                {"role": "user", "content": input_text},
            ],
            "temperature": self.temperature,
        }

        # Use max_completion_tokens for restricted models, max_tokens for others
        tokens = self.max_tokens or 2000
        if is_restricted_model:
            # Restricted models need higher limits for reasoning
            min_reasoning_tokens = 8000
            tokens = max(tokens, min_reasoning_tokens)
            request_params["max_completion_tokens"] = tokens
            logger.debug(f"Using restricted model {model} with max_completion_tokens={tokens}")
        else:
            request_params["max_tokens"] = tokens

        response = self.llm_client.chat.completions.create(**request_params)

        # Get response content and handle None/empty responses
        content = response.choices[0].message.content

        if content is None:
            logger.warning(f"Model {model} returned None content")
            return ""

        return content.strip()

    def _extract_title(self, summary_text: str) -> str:
        """
        Extract title from summary text.

        Args:
            summary_text: Full summary text

        Returns:
            Extracted title
        """
        lines = summary_text.split("\n")

        # Look for title patterns
        for line in lines:
            line = line.strip()
            if line.startswith("Title:"):
                return line[6:].strip()
            elif line.startswith("#"):
                return line[1:].strip()
            elif len(line) < 100 and line.endswith(":"):
                return line[:-1].strip()

        # Fallback: use first line if it's short enough
        first_line = lines[0].strip()
        if len(first_line) < 100:
            return first_line

        # Final fallback: truncate first line
        return first_line[:50] + "..." if len(first_line) > 50 else first_line

    def summarize_large_community(
        self,
        community_data: Dict[str, Any],
        entity_map: Dict[str, ResolvedEntity],
        relationship_map: Dict[str, ResolvedRelationship],
        max_entities: int = 20,
        max_relationships: int = 30,
    ) -> Optional[CommunitySummary]:
        """
        Generate summary for a large community using hierarchical approach.

        Args:
            community_data: Community data
            entity_map: Map of entity IDs to entities
            relationship_map: Map of relationship IDs to relationships
            max_entities: Maximum entities to include in summary
            max_relationships: Maximum relationships to include in summary

        Returns:
            CommunitySummary object or None if summarization fails
        """
        community_id = community_data["community_id"]
        level = community_data["level"]

        # Get entities and relationships for this community
        community_entities = []
        community_relationships = []

        for entity_id in community_data["entities"]:
            if entity_id in entity_map:
                community_entities.append(entity_map[entity_id])

        for rel_id in community_data["relationships"]:
            if rel_id in relationship_map:
                community_relationships.append(relationship_map[rel_id])

        if not community_entities:
            return None

        # Achievement 2.2: Select most important entities and relationships using centrality
        # Compute centrality once for both entity and relationship selection
        centrality_scores = self._compute_community_centrality(
            community_entities, community_relationships
        )
        selected_entities = self._select_important_entities(
            community_entities, community_relationships, max_entities
        )
        selected_relationships = self._select_important_relationships(
            community_relationships,
            community_entities,
            centrality_scores,
            max_relationships,
        )

        # Generate summary with selected items using large model
        # Prepare entity information
        entities_text = "\n".join(
            [
                f"- {entity.name} ({entity.type.value}): {entity.description}"
                for entity in selected_entities
            ]
        )

        # Prepare relationship information
        relationships_text = "\n".join(
            [
                f"- {rel.subject_id} -> {rel.object_id} ({rel.predicate}): {rel.description}"
                for rel in selected_relationships
            ]
        )

        # Achievement 2.3: Compute predicate profile and add to prompt
        predicate_profile = self._compute_predicate_profile(selected_relationships)

        # Create input text with predicate profile
        predicate_profile_text = ""
        if predicate_profile:
            top_predicates = ", ".join(predicate_profile[:5])  # Top 5 predicates
            predicate_profile_text = f"\n\nThis community focuses on these relationship types: {top_predicates}"

        input_text = f"""
        Entities in this community:
        {entities_text}

        Relationships in this community:
        {relationships_text}{predicate_profile_text}

        Please create a comprehensive summary of this community.
        """

        try:
            summary_text = self._generate_with_llm(input_text, use_large_model=True)

            if len(summary_text) < self.min_summary_length:
                logger.warning(
                    f"Generated summary too short: {len(summary_text)} characters"
                )
                return None

            if len(summary_text) > self.max_summary_length:
                summary_text = summary_text[: self.max_summary_length] + "..."

        except Exception as e:
            log_exception(
                logger,
                f"Failed to generate summary for large community {community_id}",
                e,
            )
            return None

        if not summary_text:
            return None

        # Add note about truncation
        if (
            len(community_entities) > max_entities
            or len(community_relationships) > max_relationships
        ):
            summary_text += f"\n\nNote: This summary covers {len(selected_entities)} of {len(community_entities)} entities and {len(selected_relationships)} of {len(community_relationships)} relationships."

        return CommunitySummary(
            community_id=community_id,
            level=level,
            title=self._extract_title(summary_text),
            summary=summary_text,
            entities=community_data["entities"],
            entity_count=community_data["entity_count"],
            relationship_count=community_data["relationship_count"],
            coherence_score=community_data["coherence_score"],
        )

    def _compute_community_centrality(
        self,
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
    ) -> Dict[str, float]:
        """
        Compute centrality scores for entities in the community subgraph.

        Achievement 2.2: Centrality-Aware Summarization Selection

        Uses PageRank to compute centrality scores for each entity.

        Args:
            entities: List of entities in the community
            relationships: List of relationships in the community

        Returns:
            Dictionary mapping entity_id to centrality score
        """
        if not entities or not relationships:
            # Fallback: return uniform scores
            return {entity.entity_id: 1.0 for entity in entities}

        # Build subgraph for this community
        G = nx.Graph()

        # Add nodes (entities)
        entity_map = {entity.entity_id: entity for entity in entities}
        for entity in entities:
            G.add_node(entity.entity_id)

        # Add edges (relationships) with weights
        for rel in relationships:
            if rel.subject_id in entity_map and rel.object_id in entity_map:
                # Use confidence as edge weight
                weight = rel.confidence
                G.add_edge(rel.subject_id, rel.object_id, weight=weight)

        if G.number_of_nodes() == 0:
            return {entity.entity_id: 1.0 for entity in entities}

        # Compute PageRank centrality
        try:
            centrality_scores = nx.pagerank(G, weight="weight", max_iter=100)
        except Exception as e:
            logger.warning(f"Failed to compute PageRank: {e}, using degree centrality")
            # Fallback to degree centrality
            centrality_scores = nx.degree_centrality(G)

        # Normalize scores (ensure all entities have a score)
        max_score = (
            max(centrality_scores.values()) if centrality_scores.values() else 1.0
        )
        if max_score == 0:
            max_score = 1.0

        # Normalize and ensure all entities have scores
        normalized_scores = {}
        for entity in entities:
            score = centrality_scores.get(entity.entity_id, 0.0)
            normalized_scores[entity.entity_id] = (
                score / max_score if max_score > 0 else 0.5
            )

        return normalized_scores

    def _select_important_entities(
        self,
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
        max_count: int,
    ) -> List[ResolvedEntity]:
        """
        Select most important entities using centrality-aware scoring.

        Achievement 2.2: Centrality-Aware Summarization Selection

        Scores entities: centrality × confidence × source_count
        Selects top-K by score.

        Args:
            entities: List of entities
            relationships: List of relationships (for centrality computation)
            max_count: Maximum number of entities to select

        Returns:
            List of selected entities
        """
        if len(entities) <= max_count:
            return entities

        # Compute centrality scores
        centrality_scores = self._compute_community_centrality(entities, relationships)

        # Score entities: centrality × confidence × source_count
        scored_entities = []
        for entity in entities:
            centrality = centrality_scores.get(entity.entity_id, 0.5)
            confidence = entity.confidence or 0.5
            source_count = entity.source_count or 1

            score = centrality * confidence * source_count
            scored_entities.append((score, entity))

        # Sort by score (highest first)
        scored_entities.sort(key=lambda x: x[0], reverse=True)

        # Select top-K
        selected = [entity for _, entity in scored_entities[:max_count]]

        logger.debug(
            f"Selected {len(selected)}/{len(entities)} entities using centrality-aware scoring"
        )

        return selected

    def _select_important_relationships(
        self,
        relationships: List[ResolvedRelationship],
        entities: List[ResolvedEntity],
        entity_centrality: Dict[str, float],
        max_count: int,
    ) -> List[ResolvedRelationship]:
        """
        Select most important relationships using centrality-aware scoring.

        Achievement 2.2: Centrality-Aware Summarization Selection

        Scores relationships: (subject_score + object_score) / 2 × confidence × source_count
        Selects top-M by score.

        Args:
            relationships: List of relationships
            entities: List of entities (for entity map)
            entity_centrality: Pre-computed centrality scores for entities
            max_count: Maximum number of relationships to select

        Returns:
            List of selected relationships
        """
        if len(relationships) <= max_count:
            return relationships

        # Build entity map for quick lookup
        entity_map = {entity.entity_id: entity for entity in entities}

        # Score relationships: (subject_score + object_score) / 2 × confidence × source_count
        scored_relationships = []
        for rel in relationships:
            subject_entity = entity_map.get(rel.subject_id)
            object_entity = entity_map.get(rel.object_id)

            if not subject_entity or not object_entity:
                # Skip if entities not found
                continue

            # Get entity scores (centrality × confidence × source_count)
            subject_centrality = entity_centrality.get(rel.subject_id, 0.5)
            object_centrality = entity_centrality.get(rel.object_id, 0.5)

            subject_score = (
                subject_centrality
                * (subject_entity.confidence or 0.5)
                * (subject_entity.source_count or 1)
            )
            object_score = (
                object_centrality
                * (object_entity.confidence or 0.5)
                * (object_entity.source_count or 1)
            )

            # Relationship score: average of endpoint scores × relationship confidence × source_count
            avg_endpoint_score = (subject_score + object_score) / 2
            rel_confidence = rel.confidence or 0.5
            rel_source_count = rel.source_count or 1

            score = avg_endpoint_score * rel_confidence * rel_source_count
            scored_relationships.append((score, rel))

        # Sort by score (highest first)
        scored_relationships.sort(key=lambda x: x[0], reverse=True)

        # Select top-M
        selected = [rel for _, rel in scored_relationships[:max_count]]

        logger.debug(
            f"Selected {len(selected)}/{len(relationships)} relationships using centrality-aware scoring"
        )

        return selected

    def _compute_predicate_profile(
        self, relationships: List[ResolvedRelationship], top_n: int = 10
    ) -> List[str]:
        """
        Compute predicate distribution for a community.

        Achievement 2.3: Predicate Profile Enhancement

        Returns top-N predicates by frequency/weight to include in prompt.

        Args:
            relationships: List of relationships in the community
            top_n: Number of top predicates to return

        Returns:
            List of top predicate names (sorted by frequency/weight)
        """
        if not relationships:
            return []

        # Count predicate frequencies and weights
        predicate_counts = {}
        predicate_weights = {}

        for rel in relationships:
            predicate = rel.predicate
            weight = rel.confidence or 0.5

            predicate_counts[predicate] = predicate_counts.get(predicate, 0) + 1
            predicate_weights[predicate] = (
                predicate_weights.get(predicate, 0.0) + weight
            )

        # Score predicates: frequency × average_weight
        scored_predicates = []
        for predicate, count in predicate_counts.items():
            avg_weight = predicate_weights[predicate] / count
            score = count * avg_weight  # Frequency × average confidence
            scored_predicates.append((score, predicate))

        # Sort by score (highest first)
        scored_predicates.sort(key=lambda x: x[0], reverse=True)

        # Return top-N predicate names
        top_predicates = [predicate for _, predicate in scored_predicates[:top_n]]

        logger.debug(
            f"Computed predicate profile: top {len(top_predicates)} predicates "
            f"from {len(relationships)} relationships"
        )

        return top_predicates

    def get_summarization_stats(
        self, summaries: Dict[str, CommunitySummary]
    ) -> Dict[str, Any]:
        """
        Get statistics about community summarization.

        Args:
            summaries: Dictionary of community summaries

        Returns:
            Dictionary containing summarization statistics
        """
        if not summaries:
            return {
                "total_summaries": 0,
                "avg_summary_length": 0,
                "level_distribution": {},
                "coherence_distribution": {},
            }

        summary_lengths = [len(summary.summary) for summary in summaries.values()]
        avg_summary_length = sum(summary_lengths) / len(summary_lengths)

        # Level distribution
        level_distribution = {}
        for summary in summaries.values():
            level = summary.level
            level_distribution[level] = level_distribution.get(level, 0) + 1

        # Coherence distribution
        coherence_scores = [summary.coherence_score for summary in summaries.values()]
        coherence_distribution = {
            "high": len([s for s in coherence_scores if s >= 0.7]),
            "medium": len([s for s in coherence_scores if 0.4 <= s < 0.7]),
            "low": len([s for s in coherence_scores if s < 0.4]),
        }

        return {
            "total_summaries": len(summaries),
            "avg_summary_length": avg_summary_length,
            "min_summary_length": min(summary_lengths),
            "max_summary_length": max(summary_lengths),
            "level_distribution": level_distribution,
            "coherence_distribution": coherence_distribution,
            "avg_coherence": sum(coherence_scores) / len(coherence_scores),
        }
