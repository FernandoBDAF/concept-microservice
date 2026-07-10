"""
Graph Extraction Agent

This module implements LLM-based entity and relationship extraction from text chunks.
Uses OpenAI's structured output capabilities with Pydantic models for reliable extraction.
"""

import logging
import os
import re
from typing import Dict, List, Any, Optional
from openai import OpenAI
from src.core.models.graphrag import (
    KnowledgeModel,
    RelationshipModel,
    EntityType,
)
from src.lib.retry import retry_llm_call
from src.lib.logging import log_exception
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)

# Try to import unidecode for predicate normalization
try:
    from unidecode import unidecode
except ImportError:
    # Fallback: use str as-is if unidecode not available
    def unidecode(s: str) -> str:
        return s


class GraphExtractionAgent:
    """
    Agent for extracting entities and relationships from text chunks using LLM.
    """

    def __init__(
        self,
        llm_client: OpenAI,
        model_name: str = "gpt-4o-mini",
        temperature: float = 0.1,
        max_tokens: Optional[int] = None,
    ):
        """
        Initialize the Graph Extraction Agent.

        Args:
            llm_client: OpenAI client instance
            model_name: Model to use for extraction (default: gpt-4o-mini)
            temperature: Temperature for LLM generation
            max_tokens: Maximum tokens for response (default: 4000 if None)
        """
        self.llm_client = llm_client
        self.model_name = model_name
        self.temperature = temperature
        self.max_tokens = max_tokens if max_tokens is not None else 4000

        # Load ontology for predicate canonicalization and filtering
        self.ontology = self._load_ontology()

        # Cache for LLM normalization results (to avoid repeated calls)
        self._normalization_cache: Dict[str, str] = {}

        # Build base extraction prompt
        base_prompt = """
        You are an advanced **Entity and Relationship Extraction Agent** for a GraphRAG (Graph-based Retrieval-Augmented Generation) pipeline.

        Your goal is to transform each input text chunk into a **structured semantic graph fragment** — capturing entities, their types, and all meaningful relationships between them, aligned with a canonical ontology.

        ---

        ## 🎯 OBJECTIVE

        For each text chunk:
        - Identify **entities** (concepts, people, technologies, etc.)
        - Extract **relationships** (functional, structural, semantic, or hierarchical)
        - Map all predicates to canonical ontology forms defined in configuration files
        - Produce high-quality structured data following the `KnowledgeModel` schema

        ---

        ## 🧩 ENTITY EXTRACTION

        Extract every **distinct, semantically relevant entity** clearly mentioned or implied by the text.

        Each entity must include:
        - **name** — canonical, human-readable (no pronouns, abbreviations, or vague terms)
        - **type** — one of the allowed ontology types:
        - PERSON — individuals, researchers, instructors, authors
        - ORGANIZATION — companies, universities, teams, departments
        - CONCEPT — abstract ideas, topics, or general notions
        - METHOD — systematic procedures or algorithms used to achieve a goal
        - TECHNOLOGY — software, hardware, frameworks, languages, or tools
        - PROCESS — multi-step flows or pipelines
        - TASK — defined objectives or problems (e.g., "object detection", "sorting")
        - THEORY — established scientific or conceptual frameworks
        - LAW — formal rules or principles (e.g., "Ohm’s Law")
        - FORMULA — explicit mathematical or symbolic expressions
        - EXPERIMENT — trials, tests, or empirical procedures
        - DATASTRUCTURE — specific data organization models (e.g., "stack", "graph")
        - ALGORITHM — specific computational procedures (e.g., "Dijkstra’s algorithm")
        - MODEL — predictive or mathematical models (e.g., "Transformer model")
        - METRIC — evaluative measures or performance scores (e.g., "accuracy")
        - COURSE — named courses, curricula, or educational units
        - EVENT — lectures, releases, hackathons, competitions
        - LOCATION — physical or digital places (e.g., “Stanford”, “GitHub”)
        - MATERIAL — physical or conceptual resources used in teaching or research
        - OTHER — any valid entity not fitting above categories
        - **description** — concise contextual explanation (1–2 sentences)
        - **confidence** — numeric score (0.0–1.0)

        ⚙️ Guidelines:
        - Use context for disambiguation and specificity.
        - Only extract entities with clear referents.
        - Prefer named concepts over generic ones (“QuickSort algorithm” instead of “Algorithm”).
        - Avoid pronouns, placeholders, or non-conceptual terms.

        ---

        ## 🔗 RELATIONSHIP EXTRACTION

        Identify every **meaningful relationship** between entities.

        Each relationship must include:
        - **source_entity**
        - **target_entity**
        - **predicate** — verb or relation term (canonicalized if possible)
        - **description** — short explanation in natural language
        - **confidence** — numeric score (0.0–1.0)

        ### 🧠 Canonical Predicate Mapping
        Use canonical forms (from the ontology) whenever possible — e.g.:
        - structural: `is_a`, `part_of`, `component_of`
        - functional: `uses`, `depends_on`, `integrates_with`, `implements`
        - causal: `requires`, `affects`, `influences`, `results_in`
        - didactic: `teaches`, `explains`, `demonstrates`
        - organizational: `works_at`, `developed_by`, `affiliated_with`, `hosts`
        - evaluative: `evaluated_on`, `measured_by`, `improves_on`
        - comparative: `related_to`, `similar_to`, `contrasts_with`
        - data/flow: `feeds_into`, `processed_by`, `trained_on`

        If a predicate is not an exact match, select the semantically closest canonical form.

        ---

        ### 🔄 Relationship Principles

        1. **Multiplicity**
        - If multiple relations apply between two entities, extract all (2–5 typical).
        - Example: “Algorithm uses Data Structure” → `uses`, `depends_on`, `applies_to`.

        2. **Directionality**
        - Prefer active voice: subject → object.
        - Add inverse relations (e.g., `used_by`, `taught_by`) where contextually valid.

        3. **Hierarchy**
        - Capture conceptual hierarchies:
            - “Binary Tree is a Data Structure” → `is_a`
            - “Sorting Step is part of Algorithm” → `part_of`

        4. **Symmetry**
        - For symmetric predicates (e.g., `related_to`, `similar_to`, `collaborates_with`),
            record a single directed instance; normalization will mirror it automatically.

        5. **Semantic and Functional**
        - Include inferred conceptual connections such as:
            - “Model trained_on Dataset”
            - “Metric evaluates Model”
            - “Algorithm applies_to Problem”

        6. **Evaluation and Metrics**
        - Include relationships expressing performance or validation:
            - “Model evaluated_on Dataset”
            - “Metric measures Accuracy”

        7. **Cross-Type Relations**
        - Cross categories are valid (e.g., PERSON → teaches → COURSE,
            TECHNOLOGY → used_in → PROCESS, MODEL → implemented_in → TECHNOLOGY)

        ---

        ## 🧮 QUALITY RULES

        - Extract only well-defined entities and relationships.
        - Avoid duplicates or trivial edges (e.g., “Concept related_to Concept” with no context).
        - Prefer **high-confidence**, **high-specificity** extractions.
        - Strive for consistent predicate naming aligned with canonical ontology files:
        - `canonical_predicates.yml`
        - `predicate_map.yml`
        - `types.yml`

        ---

        ## 💡 EXAMPLES

        **Example 1**
        > “Professor John teaches Dynamic Programming, which is used in Optimization Algorithms.”

        Entities:
        - John → PERSON
        - Dynamic Programming → CONCEPT
        - Optimization Algorithms → CONCEPT

        Relationships:
        - John → teaches → Dynamic Programming
        - Dynamic Programming → used_in → Optimization Algorithms
        - Optimization Algorithms → depends_on → Dynamic Programming

        ---

        **Example 2**
        > “TensorFlow integrates with NumPy and PyTorch. It was developed by Google.”

        Entities:
        - TensorFlow → TECHNOLOGY
        - NumPy → TECHNOLOGY
        - PyTorch → TECHNOLOGY
        - Google → ORGANIZATION

        Relationships:
        - TensorFlow → integrates_with → NumPy
        - TensorFlow → integrates_with → PyTorch
        - TensorFlow → developed_by → Google
        - Google → works_on → TensorFlow

        ---

        **Example 3**
        > “Ohm’s Law defines the relationship between voltage, current, and resistance.”

        Entities:
        - Ohm’s Law → LAW
        - Voltage → CONCEPT
        - Current → CONCEPT
        - Resistance → CONCEPT

        Relationships:
        - Ohm’s Law → defines → Voltage
        - Ohm’s Law → defines → Current
        - Ohm’s Law → defines → Resistance

        ---

        ## 🧱 OUTPUT FORMAT

        Return a structured JSON object following the `KnowledgeModel` schema:
        - `entities`: list of EntityModel objects (with name, type, description, confidence)
        - `relationships`: list of RelationshipModel objects (with predicate, description, confidence)
        - No commentary, explanations, or formatting outside this structure

        ---

        ## ✅ SUMMARY

        You are not just extracting names—you are **building a semantic graph**.
        Your output must reflect:
        - Ontology consistency  
        - Predicate canonicalization  
        - Type correctness  
        - Symmetric and hierarchical reasoning  
        - Rich connectivity (2–5 meaningful relations per connected entity pair)
        """

        # Build ontology context and inject into system prompt
        ontology_context = self._build_ontology_context()
        self.system_prompt = f"""{base_prompt}

        {ontology_context}
        """

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def extract_from_chunk(self, chunk: Dict[str, Any]) -> Optional[KnowledgeModel]:
        """
        Extract entities and relationships from a single text chunk.

        Args:
            chunk: Dictionary containing chunk data with 'text' field

        Returns:
            KnowledgeModel containing extracted entities and relationships, or None if extraction fails
        """
        if not chunk.get("chunk_text") or not chunk["chunk_text"].strip():
            logger.warning(
                f"Empty or missing text in chunk {chunk.get('chunk_id', 'unknown')}"
            )
            return None

        text = chunk["chunk_text"].strip()
        chunk_id = chunk.get("chunk_id", "unknown")

        logger.debug(f"Extracting entities from chunk {chunk_id} (length: {len(text)})")

        try:
            knowledge_model = self._extract_with_llm(text)

            # Validate and enhance the extracted knowledge
            validated_model = self._validate_and_enhance(knowledge_model, chunk)

            logger.debug(
                f"Successfully extracted {len(validated_model.entities)} entities "
                f"and {len(validated_model.relationships)} relationships from chunk {chunk_id}"
            )

            return validated_model

        except Exception as e:
            # Handle ValidationError for empty entities gracefully
            # This happens when LLM correctly returns empty entity list for chunks with no extractable entities
            # Pydantic raises ValidationError (not ValueError) when KnowledgeModel.validate_entities() fails
            error_msg = str(e)
            error_type = type(e).__name__

            # Check if this is the empty entity validation error
            # The error message contains "At least one entity must be identified"
            # Pydantic ValidationError has the message in str(e) format
            is_empty_entity_error = (
                "At least one entity must be identified" in error_msg
            )

            # Also check Pydantic ValidationError.errors() structure if available
            # This provides more robust detection for edge cases
            if not is_empty_entity_error:
                # Check error type name (ValidationError from pydantic_core)
                if "ValidationError" in error_type:
                    # Try to get detailed error info
                    if hasattr(e, "errors"):
                        try:
                            # Pydantic ValidationError has errors() method that returns list
                            errors_list = e.errors()
                            is_empty_entity_error = any(
                                "At least one entity must be identified"
                                in str(err.get("msg", ""))
                                for err in errors_list
                            )
                        except (TypeError, AttributeError, Exception):
                            # If errors() fails, fall back to string check (already done above)
                            pass

                    # Also check if error message contains validation error pattern
                    if not is_empty_entity_error:
                        # Check for the pattern in the full error string representation
                        error_repr = repr(e)
                        is_empty_entity_error = (
                            "At least one entity must be identified" in error_repr
                        )

            if is_empty_entity_error:
                logger.debug(
                    f"Chunk {chunk_id} has no extractable entities (LLM returned empty list). "
                    "This is expected for fragments/noise chunks."
                )
                # Return None to signal "no entities" case (not a failure)
                return None

            # Check for quota errors - don't log as exception, just return None
            # Quota errors are terminal and won't succeed on retry
            if self._is_quota_error(e):
                logger.error(
                    f"OpenAI quota exceeded for chunk {chunk_id}. "
                    "Stopping extraction. Please check your OpenAI billing and quota limits."
                )
                # Don't retry quota errors - they won't succeed
                return None

            # All other errors (including other ValidationErrors)
            log_exception(
                logger, f"All extraction attempts failed for chunk {chunk_id}", e
            )
            return None

    def _is_quota_error(self, error: Exception) -> bool:
        """
        Check if error is a quota/rate limit error that won't succeed on retry.

        Uses the shared quota detection logic from retry decorators.

        Args:
            error: Exception to check

        Returns:
            True if this is a quota error that shouldn't be retried
        """
        from src.lib.retry.decorators import _is_quota_error

        return _is_quota_error(error)

    @retry_llm_call(max_attempts=3)
    def _extract_with_llm(self, text: str) -> KnowledgeModel:
        """Extract entities and relationships with automatic retry.

        Args:
            text: Text to extract from

        Returns:
            KnowledgeModel with entities and relationships
        """
        # Check if this is a newer model with restrictions (no temperature, uses max_completion_tokens)
        is_restricted_model = any(prefix in self.model_name for prefix in ["gpt-5", "o1", "o3"])
        
        # Build request parameters
        request_params = {
            "model": self.model_name,
            "messages": [
                {"role": "system", "content": self.system_prompt},
                {"role": "user", "content": f"Text: {text}"},
            ],
            "response_format": KnowledgeModel,
        }
        
        # Newer models (gpt-5.x, o1, o3) don't support custom temperature - omit it
        if not is_restricted_model:
            request_params["temperature"] = self.temperature
        
        # Use max_completion_tokens for newer models, max_tokens for older models
        # Reasoning models (gpt-5, o1, o3) need higher limits as they use tokens for reasoning
        if is_restricted_model:
            # Force minimum 16000 for reasoning models to allow for reasoning + structured output
            # The configured max_tokens is usually too low for reasoning models
            min_reasoning_tokens = 16000
            tokens = max(self.max_tokens, min_reasoning_tokens) if self.max_tokens else min_reasoning_tokens
            request_params["max_completion_tokens"] = tokens
            logger.info(f"[extraction] Using restricted model {self.model_name} with max_completion_tokens={tokens}")
        elif self.max_tokens:
            request_params["max_tokens"] = self.max_tokens
        
        logger.debug(f"[extraction] Request params (excluding messages): model={request_params['model']}, "
                    f"max_completion_tokens={request_params.get('max_completion_tokens')}, "
                    f"max_tokens={request_params.get('max_tokens')}, "
                    f"temperature={request_params.get('temperature')}")
        
        response = self.llm_client.beta.chat.completions.parse(**request_params)

        return response.choices[0].message.parsed

    def _validate_and_enhance(
        self, knowledge_model: KnowledgeModel, chunk: Dict[str, Any]
    ) -> KnowledgeModel:
        """
        Validate and enhance the extracted knowledge model.

        This method applies ontology-based filtering:
        1. Filters low-confidence entities/relationships
        2. Canonicalizes predicates using ontology mapping
        3. Validates type-pair constraints
        4. Normalizes symmetric relations
        5. Validates entity references

        Args:
            knowledge_model: Extracted knowledge model
            chunk: Original chunk data

        Returns:
            Validated and enhanced knowledge model
        """
        # 1. Filter out low-confidence entities
        filtered_entities = [
            entity
            for entity in knowledge_model.entities
            if entity.confidence >= 0.3  # Minimum confidence threshold
        ]

        # 2. Filter out low-confidence relationships
        filtered_relationships = [
            rel
            for rel in knowledge_model.relationships
            if rel.confidence >= 0.3  # Minimum confidence threshold
        ]

        # 3. Canonicalize predicates and filter relationships
        canonicalized_rels = []
        dropped_count = 0
        canonicalization_stats = {
            "canonicalized": 0,
            "dropped_not_found": 0,
            "dropped_explicit": 0,
            "type_constraint_violations": 0,
        }

        for rel in filtered_relationships:
            # Canonicalize predicate (pass confidence for soft-keep logic)
            canonical_pred = self._canonicalize_predicate(rel.relation, rel.confidence)
            if canonical_pred is None:
                dropped_count += 1
                # Track why it was dropped
                normalized = self._normalize_predicate_string(rel.relation)
                if normalized in self.ontology.get("predicate_map", {}):
                    if self.ontology["predicate_map"][normalized] == "__DROP__":
                        canonicalization_stats["dropped_explicit"] += 1
                    else:
                        canonicalization_stats["dropped_not_found"] += 1
                else:
                    canonicalization_stats["dropped_not_found"] += 1
                continue

            # Validate type-pair constraints
            if not self._validate_type_pair(
                canonical_pred, rel.source_entity.type, rel.target_entity.type
            ):
                dropped_count += 1
                canonicalization_stats["type_constraint_violations"] += 1
                logger.debug(
                    f"Type constraint violation: {canonical_pred} not allowed for "
                    f"{rel.source_entity.type.value} -> {rel.target_entity.type.value}"
                )
                continue

            # Update relation to canonical form
            rel.relation = canonical_pred
            canonicalization_stats["canonicalized"] += 1
            canonicalized_rels.append(rel)

        # 4. Normalize symmetric relations
        normalized_rels = [
            self._normalize_symmetric_relation(rel) for rel in canonicalized_rels
        ]

        # 5. Validate relationships have valid entities
        valid_entity_names = {entity.name for entity in filtered_entities}
        validated_relationships = []

        for rel in normalized_rels:
            if (
                rel.source_entity.name in valid_entity_names
                and rel.target_entity.name in valid_entity_names
            ):
                validated_relationships.append(rel)
            else:
                logger.debug(
                    f"Skipping relationship {rel.source_entity.name} -> {rel.target_entity.name} "
                    f"due to missing entities"
                )

        # Add chunk metadata to entities
        for entity in filtered_entities:
            entity.confidence = max(entity.confidence, 0.1)  # Ensure minimum confidence

        # Add chunk metadata to relationships
        for rel in validated_relationships:
            rel.confidence = max(rel.confidence, 0.1)  # Ensure minimum confidence

        # Log statistics
        if canonicalization_stats["canonicalized"] > 0 or dropped_count > 0:
            logger.debug(
                f"Ontology filtering: {canonicalization_stats['canonicalized']} canonicalized, "
                f"{dropped_count} dropped ({canonicalization_stats['dropped_explicit']} explicit, "
                f"{canonicalization_stats['dropped_not_found']} not found, "
                f"{canonicalization_stats['type_constraint_violations']} type violations)"
            )

        return KnowledgeModel(
            entities=filtered_entities, relationships=validated_relationships
        )

    def _load_ontology(self) -> Dict[str, Any]:
        """
        Load ontology files with fallback.

        Returns:
            Dictionary containing ontology data (canonical_predicates, predicate_map, etc.)
        """
        from src.lib.ontology.loader import load_ontology

        return load_ontology()

    def _build_ontology_context(self) -> str:
        """
        Build a compact ontology context summary for injection into the system prompt.

        Extracts:
        - Up to 50 canonical predicates (sorted alphabetically)
        - All allowed entity types (from ontology or fallback to defaults)

        Returns:
            Formatted string containing ontology context
        """
        # Get canonical predicates (limit to 50 for prompt size)
        canonical_predicates = sorted(
            list(self.ontology.get("canonical_predicates", set()))
        )[:50]

        # Build predicate preview
        if canonical_predicates:
            predicate_preview = ", ".join(canonical_predicates)
            if len(self.ontology.get("canonical_predicates", set())) > 200:
                predicate_preview += ", ..."
        else:
            # Fallback: use common predicates if ontology not loaded
            predicate_preview = (
                "uses, depends_on, integrates_with, teaches, applies_to, "
                "part_of, is_a, requires, results_in, evaluated_on, "
                "related_to, similar_to, created_by, implemented_in"
            )

        # Get entity types from ontology or fallback
        entity_types = self._get_entity_types_list()
        type_preview = ", ".join(entity_types)

        # Log loaded counts for debugging
        canonical_count = len(self.ontology.get("canonical_predicates", set()))
        logger.debug(
            f"Ontology context built: {canonical_count} canonical predicates, "
            f"{len(entity_types)} entity types"
        )

        return f"""---

        ### 📚 Ontology Context (auto-loaded from configuration files)

        **Canonical Predicates** (use these forms when extracting relationships):
        {predicate_preview}

        **Allowed Entity Types**:
        {type_preview}

        **Guidance**:
        - If a relationship predicate doesn't match exactly, choose the semantically closest canonical form above.
        - If a new entity type appears, map it to the closest allowed type from the list above.
        - Predicate normalization will automatically handle variants (e.g., "utilizes" → "uses", "teaches" → "teach").
        """

    def _get_entity_types_list(self) -> List[str]:
        """
        Get the list of allowed entity types from ontology or fallback to defaults.

        Tries to load types.yml directly to get canonical_types, falls back to defaults.

        Returns:
            List of entity type strings
        """
        # Try to load types.yml directly to get canonical_types
        try:
            import yaml
            from pathlib import Path
            import os

            ontology_dir = os.getenv("GRAPHRAG_ONTOLOGY_DIR", "ontology")
            ontology_path = Path(ontology_dir)

            if not ontology_path.is_absolute():
                # Relative to project root
                loader_file = Path(__file__).resolve()
                project_root = loader_file.parent.parent.parent.parent
                ontology_path = project_root / ontology_dir

            # Try types.yml first, then entity_types.yml
            types_file = ontology_path / "types.yml"
            entity_types_file = ontology_path / "entity_types.yml"

            for file_path in [types_file, entity_types_file]:
                if file_path.exists():
                    try:
                        with open(file_path, "r", encoding="utf-8") as f:
                            data = yaml.safe_load(f) or {}

                        # Check for canonical_types
                        if "canonical_types" in data and isinstance(
                            data["canonical_types"], list
                        ):
                            types_list = [
                                t for t in data["canonical_types"] if isinstance(t, str)
                            ]
                            if types_list:
                                logger.debug(
                                    f"Loaded {len(types_list)} entity types from {file_path.name}"
                                )
                                return types_list
                    except Exception as e:
                        logger.debug(f"Failed to load types from {file_path}: {e}")
                        continue

        except Exception as e:
            logger.debug(f"Failed to load entity types from ontology files: {e}")

        # Fallback to comprehensive default list
        extended_types = [
            "PERSON",
            "ORGANIZATION",
            "TECHNOLOGY",
            "CONCEPT",
            "LOCATION",
            "EVENT",
            "METHOD",
            "PROCESS",
            "TASK",
            "THEORY",
            "LAW",
            "FORMULA",
            "EXPERIMENT",
            "DATASTRUCTURE",
            "ALGORITHM",
            "MODEL",
            "METRIC",
            "COURSE",
            "MATERIAL",
            "OTHER",
        ]

        return extended_types

    def _normalize_predicate_string(self, predicate: str) -> str:
        """
        Normalize predicate string (lowercase, snake_case, strip gerunds).
        Uses hybrid approach: pure logic for clear cases, LLM for ambiguous cases.

        Args:
            predicate: Raw predicate string

        Returns:
            Normalized predicate string

        Note:
            Avoids over-stemming that would create bad stems like:
            - "uses" -> "use" (not "us")
            - "has" -> "has" (not "ha")
            - "applies_to" -> "apply_to" (not "appli_to")
            - "classes" -> "classes" (not "class")

            For ambiguous cases (overlapping patterns), uses LLM for morphological disambiguation.
        """
        p = unidecode(predicate.strip().lower())
        p = re.sub(r"[^a-z0-9]+", "_", p)
        p = re.sub(r"_+", "_", p).strip("_")

        # Guard against over-stemming short words
        # Words <= 3 chars should not be stemmed
        if len(p) <= 3:
            return p

        # Token-wise stemming: only stem individual tokens
        # Split by underscores, stem each token, rejoin
        tokens = p.split("_")
        stemmed_tokens = []

        for token in tokens:
            # Check if token is ambiguous (requires LLM)
            if self._is_ambiguous_token(token):
                # Use LLM for ambiguous cases
                normalized_token = self._normalize_predicate_with_llm(token)
                stemmed_tokens.append(normalized_token)
            else:
                # Use pure logic for clear cases
                normalized_token = self._normalize_with_logic(token)
                stemmed_tokens.append(normalized_token)

        return "_".join(stemmed_tokens)

    def _is_ambiguous_token(self, token: str) -> bool:
        """
        Detect if token requires LLM normalization.

        Strategy: Use logic only for 100% guaranteed patterns (obvious cases).
        Send everything else to LLM for accurate morphological analysis.

        Clear cases (handled by pure logic - 100% guaranteed):
        - Short words (len <= 3) - keep as-is
        - Words ending in "ing" - remove "ing" (teaching → teach)
        - Words ending in "ies" - convert to "y" (applies → apply)
        - Known special plurals - keep as-is (classes, phases, bases, cases)

        Everything else → LLM (cost is low, accuracy is high)

        Args:
            token: Token to check

        Returns:
            True if token requires LLM normalization, False otherwise
        """
        if len(token) <= 3:
            # Short words: keep as-is (100% guaranteed)
            return False

        # Obvious patterns: remove "ing" suffix (100% guaranteed)
        if token.endswith("ing") and len(token) > 4:
            return False

        # Obvious patterns: convert "ies" to "y" (100% guaranteed)
        if token.endswith("ies") and len(token) > 4:
            return False

        # Known special plurals: keep as-is (100% guaranteed)
        if token in ["classes", "phases", "bases", "cases"]:
            return False

        # Everything else → LLM
        # This includes:
        # - Words ending in "s" (uses, includes, teaches, boxes, etc.)
        # - Words ending in "es" (all variations)
        # - Any other morphological patterns
        return True

    def _normalize_with_logic(self, token: str) -> str:
        """
        Normalize token using pure logic (fast path for clear cases).

        Args:
            token: Token to normalize

        Returns:
            Normalized token
        """
        if len(token) <= 3:
            # Keep short tokens as-is (e.g., "has", "use")
            return token
        elif token.endswith("ing") and len(token) > 4:
            # teaches -> teach, applying -> apply
            return token[:-3]
        elif token.endswith("ies") and len(token) > 4:
            # applies -> apply, studies -> study
            # Convert "ies" -> "y"
            return token[:-3] + "y"
        # Check special plurals BEFORE any suffix rules
        # This prevents "classes" from being caught by "ses" rule
        elif token in ["classes", "phases", "bases", "cases"]:
            return token  # Keep as-is
        elif (
            token.endswith("ses")
            or token.endswith("xes")
            or token.endswith("zes")
            or token.endswith("ches")
            or token.endswith("shes")
        ) and len(token) > 4:
            # Words ending in "ses", "xes", "zes", "ches", "shes" should remove just "s"
            # e.g., "includes" -> "include", "uses" -> "use", "reaches" -> "reach"
            # Check this BEFORE the general "es" rule
            if token not in ["has", "is", "was", "as"]:
                return token[:-1]
            else:
                return token
        elif token.endswith("es") and len(token) > 4:
            # Remove "es" for other cases (e.g., "boxes" -> "box")
            return token[:-2]
        elif token.endswith("s") and len(token) > 2:
            # uses -> use, but keep words like "has", "is"
            # Note: "uses" is len=4, so it won't match "es" rule (len > 4), will match here
            if token not in ["has", "is", "was", "as"]:
                return token[:-1]
            else:
                return token
        else:
            return token

    @retry_llm_call(max_attempts=2)
    def _normalize_predicate_with_llm(self, token: str) -> str:
        """
        Use LLM to normalize ambiguous predicate token.

        Only called for ambiguous cases where pure logic fails due to overlapping patterns.
        Uses lightweight, fast LLM call with caching for performance.

        Args:
            token: Ambiguous token to normalize

        Returns:
            Normalized token (base form)

        Examples:
            "teaches" → "teach" (verb, remove "s" from "ches")
            "includes" → "include" (verb, remove "s" from "ses")
            "classes" → "classes" (special plural, keep as-is)
            "boxes" → "box" (noun, remove "es")
        """
        # Check cache first
        cache_key = f"normalize:{token}"
        if cache_key in self._normalization_cache:
            logger.debug(
                f"Using cached normalization for '{token}': {self._normalization_cache[cache_key]}"
            )
            return self._normalization_cache[cache_key]

        # Lightweight LLM call for morphological normalization
        prompt = f"""Normalize this English predicate to its base form.

Normalization rules:
- For verbs ending in "s" or "es": remove just "s" (teaches → teach, includes → include, uses → use)
- For plural nouns ending in "es": remove "es" (boxes → box)
- Keep special plurals as-is (classes → classes, phases → phases, bases → bases, cases → cases)
- For other patterns, normalize to the most common base form

Predicate: {token}
Return only the normalized word (single word, no explanation):"""

        try:
            response = self.llm_client.chat.completions.create(
                model="gpt-4o-mini",  # Fast, cheap model
                messages=[
                    {
                        "role": "system",
                        "content": "You are a linguistic normalization assistant. Return only the normalized word, no explanation.",
                    },
                    {"role": "user", "content": prompt},
                ],
                max_tokens=10,
                temperature=0.0,  # Deterministic
            )

            # Get content - ensure it's a string, not a MagicMock
            content = response.choices[0].message.content
            if not isinstance(content, str):
                # If it's a MagicMock or other object, convert to string
                content = str(content) if content else ""

            base_form = content.strip().lower()

            # Clean up response (remove any punctuation, newlines, or extra text)
            # Keep only alphanumeric and underscores
            base_form = re.sub(r"[^a-z0-9_]+", "", base_form)

            # Extract just the first word if LLM returned multiple words
            base_form = base_form.split()[0] if base_form else ""

            # Validate result
            if not base_form or len(base_form) < 1:
                logger.warning(
                    f"LLM returned invalid normalization for '{token}': '{response.choices[0].message.content}', using fallback"
                )
                return self._normalize_with_fallback_logic(token)

            # Cache result
            self._normalization_cache[cache_key] = base_form

            logger.debug(f"LLM normalized '{token}' → '{base_form}'")
            return base_form

        except Exception as e:
            logger.warning(
                f"LLM normalization failed for '{token}': {e}, using fallback"
            )
            # Fallback to best-effort logic
            return self._normalize_with_fallback_logic(token)

    def _normalize_with_fallback_logic(self, token: str) -> str:
        """
        Fallback normalization logic when LLM fails.

        Uses best-effort approach: remove "s" from longer patterns (ches, ses, etc.)
        This is a safety net, not ideal but prevents total failure.

        Args:
            token: Token to normalize

        Returns:
            Best-effort normalized token
        """
        # Best-effort: if ends with longer pattern, remove "s"
        if (
            token.endswith("ches")
            or token.endswith("ses")
            or token.endswith("shes")
            or token.endswith("xes")
            or token.endswith("zes")
        ):
            if len(token) > 4:
                return token[:-1]  # Remove "s"

        # Otherwise, remove "es" if ends with "es"
        if token.endswith("es") and len(token) > 4:
            return token[:-2]

        # Otherwise, remove "s" if ends with "s"
        if token.endswith("s") and len(token) > 2:
            if token not in ["has", "is", "was", "as"]:
                return token[:-1]

        return token

    def _canonicalize_predicate(
        self, predicate: str, confidence: float = 0.0
    ) -> Optional[str]:
        """
        Canonicalize predicate using ontology mapping.

        Args:
            predicate: Raw predicate string from LLM
            confidence: Confidence score for the relationship (used for soft-keep logic)

        Returns:
            Canonical predicate name, or None if should be dropped
        """
        # First, normalize predicate (lowercase, snake_case) - basic normalization only
        normalized_raw = unidecode(predicate.strip().lower())
        normalized_raw = re.sub(r"[^a-z0-9]+", "_", normalized_raw)
        normalized_raw = re.sub(r"_+", "_", normalized_raw).strip("_")

        # Check if already canonical BEFORE morphological normalization (preserve canonical predicates)
        if normalized_raw in self.ontology.get("canonical_predicates", set()):
            return normalized_raw

        # Check predicate_map with raw normalized form FIRST (before morphological normalization)
        # This handles cases like "utiliz" → "uses" where the map key is the raw form
        predicate_map = self.ontology.get("predicate_map", {})
        canonical_from_raw = predicate_map.get(normalized_raw)
        if canonical_from_raw == "__DROP__":
            return None  # Drop noisy predicates
        if canonical_from_raw:
            return canonical_from_raw

        # Now normalize predicate morphologically (this may change it, e.g., "utiliz" → "utilize")
        normalized = self._normalize_predicate_string(predicate)

        # Check predicate_map with morphologically normalized form (in case map uses normalized form)
        canonical = predicate_map.get(normalized)
        if canonical == "__DROP__":
            return None  # Drop noisy predicates
        if canonical:
            return canonical

        # Check if already canonical (after normalization)
        if normalized in self.ontology.get("canonical_predicates", set()):
            return normalized

        # Not found - check if we should soft-keep unknown predicates
        keep_unknown = (
            os.getenv("GRAPHRAG_KEEP_UNKNOWN_PREDICATES", "false").lower() == "true"
        )
        if keep_unknown:
            # Soft-keep if confidence >= 0.85 and predicate length >= 4
            # Use normalized form (after morphological normalization) for the check
            if confidence >= 0.85 and len(normalized) >= 4:
                logger.debug(
                    f"Soft-keeping unknown predicate '{normalized}' "
                    f"(confidence={confidence:.2f}, length={len(normalized)})"
                )
                return normalized  # Return normalized form (may differ from original)

        # Not found - return None (will be filtered)
        return None

    def _validate_type_pair(
        self, predicate: str, src_type: EntityType, tgt_type: EntityType
    ) -> bool:
        """
        Validate that predicate is allowed for this entity type pair.

        Args:
            predicate: Canonical predicate name (should already be normalized)
            src_type: Source entity type
            tgt_type: Target entity type

        Returns:
            True if valid, False otherwise
        """
        # Normalize predicate to match constraint keys (which are stored in canonical form)
        normalized_pred = unidecode(predicate.strip().lower())
        normalized_pred = re.sub(r"[^a-z0-9]+", "_", normalized_pred)
        normalized_pred = re.sub(r"_+", "_", normalized_pred).strip("_")

        constraints = self.ontology.get("predicate_type_constraints", {}).get(
            normalized_pred, []
        )
        if not constraints:
            return True  # No constraints = allow all

        # Get type strings - handle both enum values and direct strings
        # EntityType enum only has 7 types, but ontology constraints may reference more types
        # So we compare string values directly (case-insensitive)
        src_str = src_type.value if hasattr(src_type, "value") else str(src_type)
        tgt_str = tgt_type.value if hasattr(tgt_type, "value") else str(tgt_type)

        # Check if (src_type, tgt_type) is in allowed pairs
        for allowed_pair in constraints:
            if len(allowed_pair) == 2:
                allowed_src, allowed_tgt = allowed_pair
                # Direct match (case-insensitive comparison)
                # This handles cases where EntityType enum doesn't have all types
                # (e.g., ALGORITHM, METHOD, MODEL are in constraints but not in enum)
                if (
                    src_str.upper() == allowed_src.upper()
                    and tgt_str.upper() == allowed_tgt.upper()
                ):
                    return True

        return False

    def _normalize_symmetric_relation(
        self, rel: RelationshipModel
    ) -> RelationshipModel:
        """
        Normalize symmetric relations by sorting endpoints.

        For symmetric predicates, ensure consistent ordering:
        - Sort entity names alphabetically
        - Keep predicate as-is (already canonical)

        Args:
            rel: Relationship model to normalize

        Returns:
            Normalized relationship model
        """
        # Check if predicate is symmetric
        # symmetric_predicates are stored in canonical form (e.g., "related_to")
        # The predicate should already be canonicalized when it reaches here
        symmetric_predicates = self.ontology.get("symmetric_predicates", set())

        if not symmetric_predicates:
            return rel  # No symmetric predicates configured

        # Check if predicate is symmetric
        # rel.relation is already lowercased and stripped by RelationshipModel validator
        # symmetric_predicates from YAML are already lowercase strings (e.g., "related_to")

        # Get relation string (already validated by RelationshipModel)
        relation_str = str(rel.relation).strip().lower()

        # Log for debugging
        logger.debug(
            f"Symmetric normalization check: predicate='{relation_str}', "
            f"symmetric_predicates={sorted(list(symmetric_predicates))[:5]}"
        )

        # Check if predicate matches any symmetric predicate
        # Use multiple strategies to be robust
        is_symmetric = relation_str in symmetric_predicates

        if is_symmetric:
            logger.debug(f"✓ Direct membership match found for '{relation_str}'")
        else:
            logger.debug(f"✗ Direct membership check failed for '{relation_str}'")

            # Strategy 2: Iterate and compare strings explicitly
            for sym_pred in symmetric_predicates:
                sym_str = str(sym_pred).strip().lower()
                if relation_str == sym_str:
                    is_symmetric = True
                    logger.debug(
                        f"✓ String comparison match found: '{relation_str}' == '{sym_str}'"
                    )
                    break

            # Strategy 3: Normalized comparison (handles unicode/special chars)
            if not is_symmetric:
                relation_normalized = unidecode(relation_str)
                relation_normalized = re.sub(r"[^a-z0-9]+", "_", relation_normalized)
                relation_normalized = re.sub(r"_+", "_", relation_normalized).strip("_")

                for sym_pred in symmetric_predicates:
                    sym_str = str(sym_pred).strip().lower()
                    sym_normalized = unidecode(sym_str)
                    sym_normalized = re.sub(r"[^a-z0-9]+", "_", sym_normalized)
                    sym_normalized = re.sub(r"_+", "_", sym_normalized).strip("_")

                    if relation_normalized == sym_normalized:
                        is_symmetric = True
                        logger.debug(
                            f"✓ Normalized comparison match found: "
                            f"'{relation_normalized}' == '{sym_normalized}'"
                        )
                        break

        # If not symmetric, return original relationship unchanged
        if not is_symmetric:
            logger.debug(
                f"✗ Predicate '{relation_str}' not recognized as symmetric. "
                f"Returning original relationship unchanged."
            )
            return rel

        logger.debug(
            f"✓ Predicate '{relation_str}' is symmetric. Proceeding with endpoint normalization."
        )

        # Sort endpoints alphabetically (case-insensitive)
        src_name = rel.source_entity.name.lower()
        tgt_name = rel.target_entity.name.lower()

        logger.debug(
            f"Endpoint comparison: src='{src_name}' ({rel.source_entity.name}), "
            f"tgt='{tgt_name}' ({rel.target_entity.name}), "
            f"src > tgt: {src_name > tgt_name}"
        )

        if src_name > tgt_name:
            # Swap endpoints to ensure alphabetical order
            logger.debug(
                f"Swapping endpoints: {rel.source_entity.name} -> {rel.target_entity.name} "
                f"→ {rel.target_entity.name} -> {rel.source_entity.name}"
            )
            return RelationshipModel(
                source_entity=rel.target_entity,
                target_entity=rel.source_entity,
                relation=rel.relation,
                description=rel.description,
                confidence=rel.confidence,
            )

        logger.debug(
            f"Endpoints already in correct alphabetical order: "
            f"{rel.source_entity.name} -> {rel.target_entity.name}"
        )
        return rel  # Already in correct order

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def extract_batch(
        self, chunks: List[Dict[str, Any]]
    ) -> List[Optional[KnowledgeModel]]:
        """
        Extract entities and relationships from a batch of chunks.

        Args:
            chunks: List of chunk dictionaries

        Returns:
            List of KnowledgeModel objects (or None for failed extractions)
        """
        logger.info(f"Extracting entities from batch of {len(chunks)} chunks")

        results = []
        for i, chunk in enumerate(chunks):
            logger.debug(f"Processing chunk {i + 1}/{len(chunks)}")
            result = self.extract_from_chunk(chunk)
            results.append(result)

        successful_extractions = sum(1 for r in results if r is not None)
        logger.info(
            f"Batch extraction completed: {successful_extractions}/{len(chunks)} successful"
        )

        return results

    def get_extraction_stats(
        self, results: List[Optional[KnowledgeModel]]
    ) -> Dict[str, Any]:
        """
        Get statistics about extraction results.

        Args:
            results: List of extraction results

        Returns:
            Dictionary containing extraction statistics
        """
        successful_results = [r for r in results if r is not None]

        if not successful_results:
            return {
                "total_chunks": len(results),
                "successful_extractions": 0,
                "total_entities": 0,
                "total_relationships": 0,
                "avg_entities_per_chunk": 0,
                "avg_relationships_per_chunk": 0,
                "entity_type_distribution": {},
                "success_rate": 0.0,
            }

        total_entities = sum(len(r.entities) for r in successful_results)
        total_relationships = sum(len(r.relationships) for r in successful_results)

        # Calculate entity type distribution
        entity_type_counts = {}
        for result in successful_results:
            for entity in result.entities:
                entity_type_counts[entity.type] = (
                    entity_type_counts.get(entity.type, 0) + 1
                )

        return {
            "total_chunks": len(results),
            "successful_extractions": len(successful_results),
            "total_entities": total_entities,
            "total_relationships": total_relationships,
            "avg_entities_per_chunk": total_entities / len(successful_results),
            "avg_relationships_per_chunk": total_relationships
            / len(successful_results),
            "entity_type_distribution": entity_type_counts,
            "success_rate": len(successful_results) / len(results),
        }
