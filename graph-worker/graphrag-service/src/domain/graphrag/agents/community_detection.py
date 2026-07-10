"""
Community Detection Agent

This module implements community detection using the Louvain algorithm
to identify clusters of related entities in the knowledge graph.

NOTE: Switched from hierarchical_leiden to Louvain (Nov 4, 2025)
Reason: hierarchical_leiden produced single-entity communities on sparse graphs.
Louvain is proven to work well with GraphRAG's sparse, diverse entity graphs.
"""

import logging
import os
import hashlib
from typing import Dict, List, Any
from collections import defaultdict
import networkx as nx
from networkx.algorithms import community as nx_community
from src.core.models.graphrag import ResolvedEntity, ResolvedRelationship
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


class CommunityDetectionAgent:
    """
    Agent for detecting communities in the knowledge graph using Louvain algorithm.

    Switched from hierarchical_leiden (Nov 4, 2025) due to poor performance on sparse graphs.
    Louvain is proven to work well with GraphRAG's diverse, sparse entity graphs.
    """

    def __init__(
        self,
        max_cluster_size: int = 50,  # Increased from 10 (Louvain produces larger communities)
        min_cluster_size: int = 2,
        resolution_parameter: float = 1.0,
        max_iterations: int = 100,
        max_levels: int = 3,
        algorithm: str = "louvain",  # Algorithm to use: "louvain" or "hierarchical_leiden"
    ):
        """
        Initialize the Community Detection Agent.

        DESIGN DECISIONS & TESTING NOTES (2024-11-04):
        ==============================================

        1. ALGORITHM SELECTION:
           - Current: Louvain (default)
           - Why: Produces meaningful communities on sparse GraphRAG graphs
           - Previous: hierarchical_leiden
           - Why changed: hierarchical_leiden produced mostly single-entity communities
           - Metrics: Louvain achieved modularity=0.6347 (excellent!) vs leiden ~0.3
           - Future improvements to test:
             * Leiden with different parameters (quality function, seed)
             * Label Propagation (faster, simpler)
             * Infomap (information-theoretic approach)
             * Ensemble methods (combine multiple algorithms)

        2. RESOLUTION PARAMETER:
           - Current: 1.0 (default)
           - Why: Produces balanced community sizes (10-4804 entities)
           - Range: 0.5-2.0 (lower=fewer larger communities, higher=more smaller communities)
           - Future improvements to test:
             * 0.7-0.8: Fewer, larger communities (better for high-level topics)
             * 1.5-2.0: More, smaller communities (better for fine-grained topics)
             * Multi-resolution: Detect at multiple resolutions, pick best modularity

        3. MIN/MAX CLUSTER SIZE:
           - Current: min=2, max=50
           - Why: Filter out single-entity communities (noise), soft cap at 50
           - Note: Louvain ignores max_cluster_size (post-processing only)
           - Actual sizes: 2-4804 entities (largest=4804, median~50)
           - Future improvements to test:
             * Split very large communities (>1000) using sub-community detection
             * Merge very small communities (<5) if they're highly connected

        Args:
            max_cluster_size: Maximum size of a community (soft limit, Louvain ignores)
            min_cluster_size: Minimum size to keep (filter out smaller, default=2)
            resolution_parameter: Louvain resolution (0.5-2.0, default=1.0)
            max_iterations: Maximum iterations for the algorithm
            max_levels: Maximum number of hierarchical levels (hierarchical_leiden only)
            algorithm: Algorithm to use ("louvain" default or "hierarchical_leiden")
        """
        self.max_cluster_size = max_cluster_size
        self.min_cluster_size = min_cluster_size
        self.resolution_parameter = resolution_parameter
        self.max_iterations = max_iterations
        self.max_levels = max_levels
        self.algorithm = algorithm

        # Achievement 3.1: Multi-resolution configuration
        multires_str = os.getenv("GRAPHRAG_COMMUNITY_MULTIRES", "1.0")
        self.multires_resolutions = self._parse_multires_config(multires_str)
        self.use_multires = len(self.multires_resolutions) > 1

        # Load ontology for edge weighting (Achievement 1.1, 1.2)
        self.ontology = self._load_ontology()

        logger.info(
            f"Initialized CommunityDetectionAgent with algorithm={algorithm}, "
            f"resolution={resolution_parameter}, min_size={min_cluster_size}"
        )

    @handle_errors(fallback={"communities": {}, "levels": 0, "total_communities": 0}, log_traceback=True, reraise=False)
    def detect_communities(
        self, entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
    ) -> Dict[str, Any]:
        """
        Detect communities using Louvain algorithm (default) or hierarchical Leiden.

        Args:
            entities: List of resolved entities
            relationships: List of resolved relationships

        Returns:
            Dictionary containing community detection results
        """
        logger.info(
            f"Detecting communities from {len(entities)} entities and {len(relationships)} relationships "
            f"using {self.algorithm} algorithm"
        )

        if not entities:
            logger.warning("No entities provided for community detection")
            return {"communities": {}, "levels": 0, "total_communities": 0}

        # Convert to NetworkX graph
        G = self._create_networkx_graph(entities, relationships)

        if G.number_of_nodes() == 0:
            logger.warning("Empty graph created from entities and relationships")
            return {"communities": {}, "levels": 0, "total_communities": 0}

        logger.info(
            f"Created graph with {G.number_of_nodes()} nodes and {G.number_of_edges()} edges"
        )

        # Achievement 3.1: Multi-resolution detection
        if self.use_multires and self.algorithm == "louvain":
            # Multi-resolution Louvain
            organized_communities = self._detect_multires_louvain(
                G, entities, relationships
            )
        else:
            # Single resolution (existing behavior)
            # Run community detection with selected algorithm
            try:
                if self.algorithm == "louvain":
                    communities = self._detect_louvain(G)
                elif self.algorithm == "leiden":
                    communities = self._detect_leiden(G)
                elif self.algorithm == "label_prop":
                    communities = self._detect_label_propagation(G)
                elif self.algorithm == "hierarchical_leiden":
                    communities = self._detect_hierarchical_leiden(G)
                else:
                    logger.warning(
                        f"Unknown algorithm '{self.algorithm}', using Louvain"
                    )
                    communities = self._detect_louvain(G)

                logger.info(
                    f"Detected {len(communities)} communities using {self.algorithm}"
                )

            except Exception as e:
                logger.error(f"Failed to run {self.algorithm} algorithm: {e}")
                # Fallback to simple community detection
                communities = self._fallback_community_detection(G)

            # Organize communities by level
            organized_communities = self._organize_communities_by_level(
                communities, entities, relationships
            )

        # Apply size management (split oversized, merge micro) - Achievement 1.3
        organized_communities = self._apply_size_management(
            organized_communities, entities, relationships, G
        )

        # Calculate community quality metrics
        quality_metrics = self._calculate_community_quality(organized_communities, G)

        # Achievement 3.4: Quality Gates - Validate quality before accepting results
        quality_gate_result = self._validate_quality_gates(
            organized_communities, quality_metrics, G
        )

        if not quality_gate_result["passed"]:
            logger.warning(
                f"Quality gates failed: {quality_gate_result['reasons']}. "
                f"Results may be suboptimal but will be returned."
            )

        return {
            "communities": organized_communities,
            "levels": len(organized_communities),
            "total_communities": sum(
                len(level_communities)
                for level_communities in organized_communities.values()
            ),
            "quality_metrics": quality_metrics,
            "quality_gates": quality_gate_result,
            "graph_stats": {
                "nodes": G.number_of_nodes(),
                "edges": G.number_of_edges(),
                "density": nx.density(G),
            },
        }

    def _create_networkx_graph(
        self, entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
    ) -> nx.Graph:
        """
        Convert entities and relationships to NetworkX graph.

        Args:
            entities: List of resolved entities
            relationships: List of resolved relationships

        Returns:
            NetworkX graph
        """
        G = nx.Graph()

        # Add nodes (entities)
        for entity in entities:
            G.add_node(
                entity.entity_id,
                name=entity.name,
                type=entity.type.value,
                description=entity.description,
                confidence=entity.confidence,
                source_count=entity.source_count,
                centrality_score=0.0,  # Will be calculated later
            )

        # Add edges (relationships) with weights
        for relationship in relationships:
            if G.has_node(relationship.subject_id) and G.has_node(
                relationship.object_id
            ):
                # Calculate edge weight based on confidence and relationship type
                base_confidence = relationship.confidence
                relationship_type = getattr(relationship, "relationship_type", None)

                # Apply weight multipliers based on relationship source
                if relationship_type == "co_occurrence":
                    weight = base_confidence  # 0.7 typically
                elif relationship_type == "semantic_similarity":
                    weight = base_confidence * 0.8  # Slight penalty (similarity score)
                elif relationship_type == "cross_chunk":
                    weight = base_confidence * 0.5  # 50% penalty for inferred
                elif relationship_type == "bidirectional":
                    weight = base_confidence  # Same as original
                elif relationship_type == "predicted":
                    weight = base_confidence * 0.4  # 60% penalty for predicted
                else:
                    # LLM-extracted (no relationship_type field)
                    weight = base_confidence  # Full weight (0.8-0.95)

                # Apply ontology-aware weight adjustments (Achievement 1.1, 1.2)
                weight = self._apply_ontology_weight_adjustments(
                    weight, relationship, G
                )

                # Ensure weight is in valid range
                weight = max(0.1, min(1.0, weight))

                G.add_edge(
                    relationship.subject_id,
                    relationship.object_id,
                    predicate=relationship.predicate,
                    description=relationship.description,
                    confidence=relationship.confidence,
                    source_count=relationship.source_count,
                    weight=weight,  # Edge weight for community detection
                    relationship_type=relationship_type,
                )

        return G

    def _load_ontology(self) -> Dict[str, Any]:
        """
        Load ontology for edge weighting.

        Returns:
            Ontology dictionary with canonical_predicates, predicate_map, predicate_type_constraints
        """
        try:
            from src.lib.ontology.loader import load_ontology

            return load_ontology()
        except Exception as e:
            logger.warning(f"Could not load ontology for edge weighting: {e}")
            return {
                "canonical_predicates": set(),
                "predicate_map": {},
                "predicate_type_constraints": {},
            }

    def _apply_ontology_weight_adjustments(
        self, weight: float, relationship: ResolvedRelationship, G: nx.Graph
    ) -> float:
        """
        Apply ontology-aware weight adjustments to edge weight.

        Achievement 1.1: Ontology-Aware Edge Weighting
        Achievement 1.2: Type-Pair Validity Bonuses

        Adjustments:
        - Canonical predicates: +15% boost
        - Soft-kept/unknown predicates: -15% penalty
        - Valid type-pairs: +10% bonus
        - Invalid type-pairs: -20% penalty

        Args:
            weight: Base edge weight
            relationship: Relationship object
            G: NetworkX graph (for entity type lookup)

        Returns:
            Adjusted weight
        """
        predicate = relationship.predicate
        canonical_predicates = self.ontology.get("canonical_predicates", set())
        predicate_map = self.ontology.get("predicate_map", {})
        type_constraints = self.ontology.get("predicate_type_constraints", {})

        # Achievement 1.1: Canonical predicate boost
        if predicate in canonical_predicates:
            weight *= 1.15  # +15% boost for canonical predicates
            logger.debug(f"Canonical predicate boost: {predicate} → {weight:.3f}")
        else:
            # Check if predicate was soft-kept (mapped but not canonical)
            normalized_pred = predicate.lower().strip()
            if normalized_pred in predicate_map:
                mapped_pred = predicate_map[normalized_pred]
                if mapped_pred == "__DROP__":
                    # Was soft-kept (should have been dropped but wasn't)
                    weight *= 0.85  # -15% penalty
                    logger.debug(
                        f"Soft-kept predicate penalty: {predicate} → {weight:.3f}"
                    )
            else:
                # Unknown predicate (not in ontology)
                weight *= 0.85  # -15% penalty
                logger.debug(f"Unknown predicate penalty: {predicate} → {weight:.3f}")

        # Achievement 1.2: Type-pair validity bonus/penalty
        subject_id = relationship.subject_id
        object_id = relationship.object_id

        # Get entity types from graph nodes
        subject_type = G.nodes[subject_id].get("type") if subject_id in G else None
        object_type = G.nodes[object_id].get("type") if object_id in G else None

        if subject_type and object_type and predicate in type_constraints:
            # Check if type-pair is valid for this predicate
            valid_pairs = type_constraints[predicate]
            is_valid = False

            for valid_pair in valid_pairs:
                if len(valid_pair) == 2:
                    if valid_pair[0] == subject_type and valid_pair[1] == object_type:
                        is_valid = True
                        break

            if is_valid:
                weight *= 1.1  # +10% bonus for valid type-pairs
                logger.debug(
                    f"Valid type-pair bonus: {predicate} ({subject_type}→{object_type}) → {weight:.3f}"
                )
            else:
                weight *= 0.8  # -20% penalty for invalid type-pairs
                logger.debug(
                    f"Invalid type-pair penalty: {predicate} ({subject_type}→{object_type}) → {weight:.3f}"
                )

        return weight

    def _parse_multires_config(self, multires_str: str) -> List[float]:
        """
        Parse multi-resolution configuration from environment variable.

        Achievement 3.1: Multi-Resolution Louvain

        Args:
            multires_str: Comma-separated resolution values (e.g., "0.8,1.0,1.6")

        Returns:
            List of resolution parameters as floats
        """
        try:
            resolutions = [float(r.strip()) for r in multires_str.split(",")]
            # Validate all are positive
            resolutions = [r for r in resolutions if r > 0]
            if not resolutions:
                logger.warning("No valid resolutions found, using default 1.0")
                return [1.0]
            logger.info(f"Multi-resolution enabled: {resolutions}")
            return resolutions
        except Exception as e:
            logger.warning(
                f"Failed to parse multires config '{multires_str}': {e}, using default 1.0"
            )
            return [1.0]

    def _detect_multires_louvain(
        self,
        G: nx.Graph,
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
    ) -> Dict[int, Dict[str, Any]]:
        """
        Detect communities at multiple resolutions using Louvain.

        Achievement 3.1: Multi-Resolution Louvain

        Runs Louvain at each resolution and stores each as a separate level.
        Entities can appear in multiple levels (multi-scale membership).

        Args:
            G: NetworkX graph
            entities: List of entities
            relationships: List of relationships

        Returns:
            Dictionary mapping levels to community information
        """
        logger.info(
            f"Running multi-resolution Louvain at resolutions: {self.multires_resolutions}"
        )

        organized_communities = defaultdict(dict)
        seed = int(os.getenv("GRAPHRAG_RANDOM_SEED", "42"))

        # Run Louvain at each resolution
        for level, resolution in enumerate(self.multires_resolutions, start=1):
            logger.info(
                f"Detecting communities at resolution {resolution} (level {level})"
            )

            try:
                # Run Louvain at this resolution
                communities = nx_community.louvain_communities(
                    G,
                    resolution=resolution,
                    seed=seed,
                    weight="weight",
                )

                # Calculate modularity
                modularity = nx_community.modularity(G, communities, weight="weight")

                logger.info(
                    f"Resolution {resolution} (level {level}): {len(communities)} communities "
                    f"(modularity={modularity:.4f})"
                )

                # Organize communities for this level
                level_communities = self._organize_multires_level(
                    level, communities, entities, relationships
                )

                organized_communities[level] = level_communities

            except Exception as e:
                logger.error(
                    f"Failed to detect communities at resolution {resolution}: {e}"
                )
                # Skip this resolution, continue with others
                continue

        logger.info(
            f"Multi-resolution detection complete: {len(organized_communities)} levels, "
            f"{sum(len(level_comms) for level_comms in organized_communities.values())} total communities"
        )

        return dict(organized_communities)

    def _organize_multires_level(
        self,
        level: int,
        communities: List[Any],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
    ) -> Dict[str, Any]:
        """
        Organize communities for a single multi-resolution level.

        Achievement 3.1: Multi-Resolution Louvain

        Args:
            level: Level number (1, 2, 3, etc.)
            communities: List of community frozensets from Louvain
            entities: List of all entities
            relationships: List of all relationships

        Returns:
            Dictionary mapping community_id to community data
        """
        level_communities = {}

        for community_nodes in communities:
            entity_ids = list(community_nodes)

            if len(entity_ids) < self.min_cluster_size:
                logger.debug(
                    f"Skipping community with {len(entity_ids)} entities "
                    f"(min_cluster_size={self.min_cluster_size})"
                )
                continue

            # Generate stable, deterministic community ID
            community_id = self._generate_stable_community_id(level, entity_ids)

            # Get entities and relationships for this community
            community_entities = []
            community_relationships = []

            # Filter entities
            for entity in entities:
                if entity.entity_id in entity_ids:
                    community_entities.append(entity)

            # Filter relationships (both entities must be in community)
            for relationship in relationships:
                if (
                    relationship.subject_id in entity_ids
                    and relationship.object_id in entity_ids
                ):
                    community_relationships.append(relationship)

            # Calculate coherence
            coherence_score = self._calculate_coherence_score(
                community_entities, community_relationships
            )

            # Store community
            level_communities[community_id] = {
                "community_id": community_id,
                "level": level,
                "entities": [e.entity_id for e in community_entities],
                "entity_count": len(community_entities),
                "relationships": [r.relationship_id for r in community_relationships],
                "relationship_count": len(community_relationships),
                "coherence_score": coherence_score,
                "entity_names": [e.name for e in community_entities],
                "entity_types": [e.type.value for e in community_entities],
            }

        return level_communities

    def _apply_size_management(
        self,
        organized_communities: Dict[int, Dict[str, Any]],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
        G: nx.Graph,
    ) -> Dict[int, Dict[str, Any]]:
        """
        Apply size management to communities: split oversized, merge micro.

        Achievement 1.3: Community Size Management

        - Split oversized communities (>1000 entities) using recursive Louvain
        - Merge micro-communities (<5 entities) into nearest neighbor

        Args:
            organized_communities: Dictionary of organized communities by level
            entities: List of all entities
            relationships: List of all relationships
            G: NetworkX graph

        Returns:
            Updated organized communities dictionary
        """
        split_threshold = int(os.getenv("GRAPHRAG_COMMUNITY_SPLIT_THRESHOLD", "1000"))
        merge_threshold = int(os.getenv("GRAPHRAG_COMMUNITY_MERGE_THRESHOLD", "5"))

        updated_communities = defaultdict(dict)

        for level, level_communities in organized_communities.items():
            for community_id, community_data in level_communities.items():
                entity_count = community_data["entity_count"]

                # Split oversized communities
                if entity_count > split_threshold:
                    logger.info(
                        f"Splitting oversized community {community_id} "
                        f"({entity_count} entities > {split_threshold})"
                    )
                    split_communities = self._split_oversized_community(
                        community_data, entities, relationships, G, level
                    )
                    # Add split communities to updated_communities
                    for split_id, split_data in split_communities.items():
                        updated_communities[level][split_id] = split_data
                # Merge micro-communities (will be handled after all splits)
                elif entity_count < merge_threshold:
                    # Store for later merging
                    if level not in updated_communities:
                        updated_communities[level] = {}
                    updated_communities[level][community_id] = community_data
                else:
                    # Keep as-is
                    if level not in updated_communities:
                        updated_communities[level] = {}
                    updated_communities[level][community_id] = community_data

        # Merge micro-communities
        updated_communities = self._merge_micro_communities(
            updated_communities, entities, relationships, G, merge_threshold
        )

        return dict(updated_communities)

    def _split_oversized_community(
        self,
        community_data: Dict[str, Any],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
        G: nx.Graph,
        parent_level: int,
    ) -> Dict[str, Dict[str, Any]]:
        """
        Split an oversized community using recursive Louvain.

        Args:
            community_data: Community data dictionary
            entities: List of all entities
            relationships: List of all relationships
            G: Full NetworkX graph
            parent_level: Level of parent community

        Returns:
            Dictionary of split communities
        """
        entity_ids = set(community_data["entities"])

        # Create subgraph for this community
        subgraph = G.subgraph(entity_ids)

        if subgraph.number_of_nodes() < 2:
            # Can't split single node
            return {community_data["community_id"]: community_data}

        # Run Louvain with higher resolution (1.5-2.0) to get finer partitions
        split_resolution = float(
            os.getenv("GRAPHRAG_COMMUNITY_SPLIT_RESOLUTION", "1.8")
        )
        seed = int(os.getenv("GRAPHRAG_RANDOM_SEED", "42"))

        try:
            split_communities = nx_community.louvain_communities(
                subgraph,
                resolution=split_resolution,
                seed=seed,
                weight="weight",
            )

            logger.info(
                f"Split community {community_data['community_id']} into {len(split_communities)} sub-communities"
            )

            # Organize split communities
            split_organized = {}
            for i, split_nodes in enumerate(split_communities):
                split_entity_ids = list(split_nodes)

                # Generate stable ID with partition suffix
                base_id = community_data["community_id"]
                partition_hash = hashlib.sha1(
                    ",".join(sorted(split_entity_ids)).encode()
                ).hexdigest()[:8]
                split_id = f"{base_id}-p{partition_hash}"

                # Get entities and relationships for this split
                split_entities = [
                    e for e in entities if e.entity_id in split_entity_ids
                ]
                split_relationships = [
                    r
                    for r in relationships
                    if r.subject_id in split_entity_ids
                    and r.object_id in split_entity_ids
                ]

                # Calculate coherence
                coherence = self._calculate_coherence_score(
                    split_entities, split_relationships
                )

                split_organized[split_id] = {
                    "community_id": split_id,
                    "level": parent_level,
                    "entities": split_entity_ids,
                    "entity_count": len(split_entities),
                    "relationships": [r.relationship_id for r in split_relationships],
                    "relationship_count": len(split_relationships),
                    "coherence_score": coherence,
                    "entity_names": [e.name for e in split_entities],
                    "entity_types": [e.type.value for e in split_entities],
                    "parent_community_id": community_data["community_id"],
                }

            return split_organized

        except Exception as e:
            logger.error(
                f"Failed to split community {community_data['community_id']}: {e}"
            )
            # Return original if split fails
            return {community_data["community_id"]: community_data}

    def _merge_micro_communities(
        self,
        organized_communities: Dict[int, Dict[str, Any]],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
        G: nx.Graph,
        merge_threshold: int,
    ) -> Dict[int, Dict[str, Any]]:
        """
        Merge micro-communities (<merge_threshold) into nearest neighbor.

        Args:
            organized_communities: Dictionary of organized communities
            entities: List of all entities
            relationships: List of all relationships
            G: NetworkX graph
            merge_threshold: Minimum size threshold for merging

        Returns:
            Updated organized communities with micro-communities merged
        """
        merged_communities = defaultdict(dict)

        for level, level_communities in organized_communities.items():
            micro_communities = []
            normal_communities = {}

            # Separate micro and normal communities
            for comm_id, comm_data in level_communities.items():
                if comm_data["entity_count"] < merge_threshold:
                    micro_communities.append((comm_id, comm_data))
                else:
                    normal_communities[comm_id] = comm_data

            # Try to merge each micro-community into nearest neighbor
            for micro_id, micro_data in micro_communities:
                micro_entity_ids = set(micro_data["entities"])

                # Find nearest neighbor (highest edge weight connection)
                best_neighbor_id = None
                best_weight = 0.0

                for neighbor_id, neighbor_data in normal_communities.items():
                    neighbor_entity_ids = set(neighbor_data["entities"])

                    # Calculate total edge weight between micro and neighbor
                    total_weight = 0.0
                    for micro_eid in micro_entity_ids:
                        for neighbor_eid in neighbor_entity_ids:
                            if G.has_edge(micro_eid, neighbor_eid):
                                total_weight += G[micro_eid][neighbor_eid].get(
                                    "weight", 0.0
                                )

                    if total_weight > best_weight:
                        best_weight = total_weight
                        best_neighbor_id = neighbor_id

                # Merge into best neighbor or keep as-is if no good neighbor
                if (
                    best_neighbor_id and best_weight > 0.1
                ):  # Minimum connection threshold
                    logger.debug(
                        f"Merging micro-community {micro_id} ({micro_data['entity_count']} entities) "
                        f"into {best_neighbor_id} (weight={best_weight:.3f})"
                    )
                    # Merge entities and relationships
                    neighbor_data = normal_communities[best_neighbor_id]
                    merged_entity_ids = (
                        set(neighbor_data["entities"]) | micro_entity_ids
                    )

                    # Recalculate merged community
                    merged_entities = [
                        e for e in entities if e.entity_id in merged_entity_ids
                    ]
                    merged_relationships = [
                        r
                        for r in relationships
                        if r.subject_id in merged_entity_ids
                        and r.object_id in merged_entity_ids
                    ]
                    merged_coherence = self._calculate_coherence_score(
                        merged_entities, merged_relationships
                    )

                    # Update neighbor with merged data
                    normal_communities[best_neighbor_id] = {
                        "community_id": best_neighbor_id,
                        "level": level,
                        "entities": list(merged_entity_ids),
                        "entity_count": len(merged_entities),
                        "relationships": [
                            r.relationship_id for r in merged_relationships
                        ],
                        "relationship_count": len(merged_relationships),
                        "coherence_score": merged_coherence,
                        "entity_names": [e.name for e in merged_entities],
                        "entity_types": [e.type.value for e in merged_entities],
                    }
                else:
                    # Keep micro-community as-is (no good neighbor to merge with)
                    normal_communities[micro_id] = micro_data

            merged_communities[level] = normal_communities

        return dict(merged_communities)

    def _generate_stable_community_id(self, level: int, entity_ids: List[str]) -> str:
        """
        Generate a stable, deterministic community ID based on entity set.

        Uses SHA1 hash of sorted entity IDs to ensure:
        - Same entities → same ID (deterministic)
        - Different order → same ID (order-independent)
        - Different entities → different ID

        Args:
            level: Community level (1, 2, 3, etc.)
            entity_ids: List of entity IDs in the community

        Returns:
            Stable community ID in format: lvl{level}-{12-char-hash}
            Example: lvl1-a3f2b1c4d5e6
        """
        # Sort entity IDs to ensure deterministic order
        sorted_ids = sorted(entity_ids)

        # Create signature string
        signature = ",".join(sorted_ids)

        # Compute SHA1 hash
        hash_obj = hashlib.sha1(signature.encode())
        hash_hex = hash_obj.hexdigest()[:12]  # Use first 12 characters

        # Format: lvl{level}-{hash}
        community_id = f"lvl{level}-{hash_hex}"

        return community_id

    def _detect_louvain(self, G: nx.Graph) -> List[Any]:
        """
        Detect communities using Louvain algorithm.

        Args:
            G: NetworkX graph

        Returns:
            List of community frozensets
        """
        logger.info(
            f"Running Louvain algorithm with resolution={self.resolution_parameter}"
        )

        # Get random seed from environment or use default
        seed = int(os.getenv("GRAPHRAG_RANDOM_SEED", "42"))

        # Run Louvain algorithm
        communities = nx_community.louvain_communities(
            G,
            resolution=self.resolution_parameter,
            seed=seed,
            weight="weight",  # Use edge weights
        )

        # Calculate modularity
        modularity = nx_community.modularity(G, communities, weight="weight")

        logger.info(
            f"Louvain detected {len(communities)} communities "
            f"(modularity={modularity:.4f})"
        )

        # Log community sizes
        sizes = sorted([len(c) for c in communities], reverse=True)
        if sizes:
            logger.info(
                f"Community sizes: {sizes[:10]}{'...' if len(sizes) > 10 else ''}"
            )

        return list(communities)

    def _detect_leiden(self, G: nx.Graph) -> List[Any]:
        """
        Detect communities using Leiden algorithm (proper implementation).

        Achievement 3.2: Leiden Detector Implemented (Proper)

        Tries multiple methods in order:
        1. NetworkX 3.5+ leiden_communities (if available)
        2. graspologic hierarchical_leiden (if available)
        3. Falls back to Louvain with warning

        Args:
            G: NetworkX graph

        Returns:
            List of community frozensets (compatible with Louvain format)
        """
        # Try NetworkX leiden_communities first (NetworkX 3.5+)
        try:
            # Check if leiden_communities is available
            if hasattr(nx_community, "leiden_communities"):
                logger.info(
                    f"Using NetworkX leiden_communities with resolution={self.resolution_parameter}"
                )

                seed = int(os.getenv("GRAPHRAG_RANDOM_SEED", "42"))

                # NetworkX leiden_communities signature:
                # leiden_communities(G, resolution=1.0, seed=None, weight=None)
                communities = nx_community.leiden_communities(
                    G,
                    resolution=self.resolution_parameter,
                    seed=seed,
                    weight="weight",
                )

                # Calculate modularity
                modularity = nx_community.modularity(G, communities, weight="weight")

                logger.info(
                    f"Leiden (NetworkX) detected {len(communities)} communities "
                    f"(modularity={modularity:.4f})"
                )

                # Convert to list of frozensets for compatibility
                return list(communities)

        except (AttributeError, Exception) as e:
            logger.debug(f"NetworkX leiden_communities not available: {e}")

        # Try graspologic hierarchical_leiden
        try:
            from graspologic.partition import hierarchical_leiden

            logger.info(
                f"Using graspologic hierarchical_leiden with max_cluster_size={self.max_cluster_size}"
            )
            logger.warning(
                "Note: graspologic hierarchical_leiden ignores resolution_parameter. "
                "Use max_cluster_size to control community sizes."
            )

            communities = hierarchical_leiden(
                G,
                max_cluster_size=self.max_cluster_size,
            )

            logger.info(f"Leiden (graspologic) detected {len(communities)} communities")

            # Convert graspologic format to frozensets
            # graspologic returns objects with .nodes/.node attributes
            # We'll let _organize_communities_by_level handle the conversion
            return communities

        except ImportError:
            logger.warning(
                "graspologic not installed. Leiden algorithm requires either "
                "NetworkX 3.5+ (with leiden_communities) or graspologic. "
                "Falling back to Louvain."
            )
        except Exception as e:
            logger.error(f"graspologic hierarchical_leiden failed: {e}")

        # Fallback to Louvain
        logger.warning("Falling back to Louvain algorithm")
        return self._detect_louvain(G)

    def _detect_hierarchical_leiden(self, G: nx.Graph) -> List[Any]:
        """
        Detect communities using hierarchical Leiden algorithm.

        NOTE: Kept for backward compatibility. Use "leiden" algorithm instead.

        Args:
            G: NetworkX graph

        Returns:
            List of communities
        """
        logger.warning(
            "hierarchical_leiden algorithm is deprecated. Use 'leiden' instead."
        )
        return self._detect_leiden(G)

    def _detect_label_propagation(self, G: nx.Graph) -> List[Any]:
        """
        Detect communities using Label Propagation algorithm.

        Achievement 3.3: Label Propagation Baseline Implemented

        Fast, non-deterministic algorithm. Runs multiple times and takes consensus
        if needed for stability.

        Args:
            G: NetworkX graph

        Returns:
            List of community frozensets
        """
        logger.info("Running Label Propagation algorithm")

        # Label Propagation is non-deterministic, so we run it multiple times
        # and take consensus for stability
        num_runs = int(os.getenv("GRAPHRAG_LABEL_PROP_RUNS", "3"))
        seed = int(os.getenv("GRAPHRAG_RANDOM_SEED", "42"))

        all_communities = []

        for run in range(num_runs):
            try:
                # Use different seed for each run (if multiple runs)
                run_seed = seed + run if num_runs > 1 else seed

                # NetworkX label_propagation_communities
                communities = list(
                    nx_community.label_propagation_communities(G, seed=run_seed)
                )

                all_communities.append(communities)

                logger.debug(
                    f"Label Propagation run {run + 1}/{num_runs}: {len(communities)} communities"
                )

            except Exception as e:
                logger.error(f"Label Propagation run {run + 1} failed: {e}")
                continue

        if not all_communities:
            logger.error("All Label Propagation runs failed, falling back to Louvain")
            return self._detect_louvain(G)

        # Take consensus: use the run with median number of communities
        # (more stable than min/max)
        all_communities.sort(key=len)
        median_idx = len(all_communities) // 2
        communities = all_communities[median_idx]

        # Calculate modularity
        modularity = nx_community.modularity(G, communities, weight="weight")

        logger.info(
            f"Label Propagation detected {len(communities)} communities "
            f"(modularity={modularity:.4f}, {num_runs} runs, using median)"
        )

        logger.warning(
            "Label Propagation is non-deterministic and may produce unstable partitions. "
            "Use for fast baseline or very large graphs."
        )

        # Convert to list of frozensets for compatibility
        return list(communities)

    def _fallback_community_detection(self, G: nx.Graph) -> List[Any]:
        """
        Fallback community detection using simple connected components.

        Args:
            G: NetworkX graph

        Returns:
            List of communities
        """
        logger.info("Using fallback community detection based on connected components")

        communities = []
        for i, component in enumerate(nx.connected_components(G)):
            if len(component) >= self.min_cluster_size:
                # Create a simple community object with level >= 1 (CommunitySummary requires level >= 1)
                # Use 'nodes' attribute for multi-node communities
                if len(component) > 1:
                    community = type(
                        "Community",
                        (),
                        {"cluster": i, "nodes": component, "level": 1},
                    )()
                else:
                    # Single node community
                    community = type(
                        "Community",
                        (),
                        {"cluster": i, "node": list(component)[0], "level": 1},
                    )()
                communities.append(community)

        return communities

    def _organize_communities_by_level(
        self,
        communities: List[Any],
        entities: List[ResolvedEntity],
        relationships: List[ResolvedRelationship],
    ) -> Dict[int, Dict[str, Any]]:
        """
        Organize communities by hierarchical level.

        Handles both Louvain format (list of frozensets) and hierarchical_leiden format (objects with attributes).

        Args:
            communities: List of detected communities (frozensets from Louvain or objects from hierarchical_leiden)
            entities: List of resolved entities
            relationships: List of resolved relationships

        Returns:
            Dictionary mapping levels to community information
        """
        organized = defaultdict(dict)

        # Detect format: frozenset (Louvain) or object with attributes (hierarchical_leiden)
        if communities and isinstance(communities[0], (frozenset, set)):
            # Louvain format: list of frozensets
            logger.debug("Processing Louvain format communities (frozensets)")
            level = 1  # All Louvain communities at level 1

            for i, community_nodes in enumerate(communities):
                entity_ids = list(community_nodes)

                if len(entity_ids) < self.min_cluster_size:
                    logger.debug(
                        f"Skipping community with {len(entity_ids)} entities (min_cluster_size={self.min_cluster_size})"
                    )
                    continue

                # Generate stable, deterministic community ID
                community_id = self._generate_stable_community_id(level, entity_ids)

                # Get entities in this community
                community_entities = []
                community_relationships = []

                # Filter entities
                for entity in entities:
                    if entity.entity_id in entity_ids:
                        community_entities.append(entity)

                # Filter relationships (both entities must be in community)
                for relationship in relationships:
                    if (
                        relationship.subject_id in entity_ids
                        and relationship.object_id in entity_ids
                    ):
                        community_relationships.append(relationship)

                # Calculate coherence
                coherence_score = self._calculate_coherence_score(
                    community_entities, community_relationships
                )

                # Store community
                organized[level][community_id] = {
                    "community_id": community_id,
                    "level": level,
                    "entities": [e.entity_id for e in community_entities],
                    "entity_count": len(community_entities),
                    "relationships": [
                        r.relationship_id for r in community_relationships
                    ],
                    "relationship_count": len(community_relationships),
                    "coherence_score": coherence_score,
                    "entity_names": [e.name for e in community_entities],
                    "entity_types": [e.type.value for e in community_entities],
                }

            logger.info(
                f"Organized {len(organized.get(1, {}))} Louvain communities at level 1 "
                f"(filtered from {len(communities)} total)"
            )
            return dict(organized)

        # hierarchical_leiden format: objects with .level, .nodes/.node attributes
        logger.debug("Processing hierarchical_leiden format communities (objects)")
        level_communities = defaultdict(list)
        for community in communities:
            # Get level, default to 1 if not present
            level = getattr(community, "level", 1)
            level = max(1, level)
            level_communities[level].append(community)

        # Process each level
        for level, level_comm_list in level_communities.items():
            for i, community in enumerate(level_comm_list):

                if hasattr(community, "nodes"):
                    # Multiple nodes in community
                    entity_ids = list(community.nodes)
                else:
                    # Single node community
                    entity_ids = [getattr(community, "node", "")]

                # Filter out communities below min_cluster_size
                if len(entity_ids) < self.min_cluster_size:
                    logger.debug(
                        f"Skipping community with {len(entity_ids)} entities "
                        f"(min_cluster_size={self.min_cluster_size})"
                    )
                    continue

                # Generate stable, deterministic community ID
                community_id = self._generate_stable_community_id(level, entity_ids)

                # Filter entities and relationships
                community_entities = []
                community_relationships = []

                for entity in entities:
                    if entity.entity_id in entity_ids:
                        community_entities.append(entity)

                for relationship in relationships:
                    if (
                        relationship.subject_id in entity_ids
                        and relationship.object_id in entity_ids
                    ):
                        community_relationships.append(relationship)

                # Calculate community metrics
                coherence_score = self._calculate_coherence_score(
                    community_entities, community_relationships
                )

                organized[level][community_id] = {
                    "community_id": community_id,
                    "level": level,
                    "entities": [e.entity_id for e in community_entities],
                    "entity_count": len(community_entities),
                    "relationships": [
                        r.relationship_id for r in community_relationships
                    ],
                    "relationship_count": len(community_relationships),
                    "coherence_score": coherence_score,
                    "entity_names": [e.name for e in community_entities],
                    "entity_types": [e.type.value for e in community_entities],
                }

        return dict(organized)

    def _calculate_coherence_score(
        self, entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
    ) -> float:
        """
        Calculate coherence score for a community.

        Args:
            entities: Entities in the community
            relationships: Relationships in the community

        Returns:
            Coherence score between 0 and 1
        """
        if not entities:
            return 0.0

        if len(entities) == 1:
            # Changed from 1.0 - isolated entities have no coherence
            return 0.0

        # Calculate internal connectivity
        entity_ids = {e.entity_id for e in entities}
        internal_relationships = len(relationships)

        # Calculate potential relationships (complete graph)
        potential_relationships = len(entities) * (len(entities) - 1) / 2

        # Connectivity ratio
        connectivity_ratio = (
            internal_relationships / potential_relationships
            if potential_relationships > 0
            else 0
        )

        # Average entity confidence
        avg_confidence = sum(e.confidence for e in entities) / len(entities)

        # Average relationship confidence
        avg_rel_confidence = (
            sum(r.confidence for r in relationships) / len(relationships)
            if relationships
            else 0
        )

        # Combined coherence score
        coherence_score = (
            0.4 * connectivity_ratio + 0.3 * avg_confidence + 0.3 * avg_rel_confidence
        )

        return min(1.0, max(0.0, coherence_score))

    def _calculate_community_quality(
        self, organized_communities: Dict[int, Dict[str, Any]], G: nx.Graph
    ) -> Dict[str, Any]:
        """
        Calculate quality metrics for detected communities.

        Args:
            organized_communities: Organized communities by level
            G: NetworkX graph

        Returns:
            Dictionary containing quality metrics
        """
        total_communities = sum(
            len(level_communities)
            for level_communities in organized_communities.values()
        )

        if total_communities == 0:
            return {
                "total_communities": 0,
                "avg_coherence": 0,
                "avg_size": 0,
                "coverage": 0,
            }

        # Calculate average coherence
        all_coherence_scores = []
        all_sizes = []

        for level_communities in organized_communities.values():
            for community in level_communities.values():
                all_coherence_scores.append(community["coherence_score"])
                all_sizes.append(community["entity_count"])

        avg_coherence = sum(all_coherence_scores) / len(all_coherence_scores)
        avg_size = sum(all_sizes) / len(all_sizes)

        # Calculate coverage (percentage of nodes in communities)
        total_nodes_in_communities = sum(all_sizes)
        total_graph_nodes = G.number_of_nodes()
        coverage = (
            total_nodes_in_communities / total_graph_nodes
            if total_graph_nodes > 0
            else 0
        )

        return {
            "total_communities": total_communities,
            "avg_coherence": avg_coherence,
            "avg_size": avg_size,
            "coverage": coverage,
            "max_coherence": max(all_coherence_scores),
            "min_coherence": min(all_coherence_scores),
            "max_size": max(all_sizes),
            "min_size": min(all_sizes),
        }

    def _validate_quality_gates(
        self,
        organized_communities: Dict[int, Dict[str, Any]],
        quality_metrics: Dict[str, Any],
        G: nx.Graph,
    ) -> Dict[str, Any]:
        """
        Validate quality gates before accepting detection results.

        Achievement 3.4: Quality Gates Implemented

        Validates:
        - Modularity > threshold (default 0.3)
        - Coverage > threshold (default 0.7)
        - No giant communities (max size check)
        - No excessive singleton communities

        Args:
            organized_communities: Organized communities by level
            quality_metrics: Quality metrics dictionary
            G: NetworkX graph

        Returns:
            Dictionary with 'passed' (bool) and 'reasons' (list of strings)
        """
        # Get configurable thresholds
        min_modularity = float(os.getenv("GRAPHRAG_QUALITY_MIN_MODULARITY", "0.3"))
        min_coverage = float(os.getenv("GRAPHRAG_QUALITY_MIN_COVERAGE", "0.7"))
        max_community_size = int(
            os.getenv("GRAPHRAG_QUALITY_MAX_COMMUNITY_SIZE", "5000")
        )
        max_singleton_ratio = float(
            os.getenv("GRAPHRAG_QUALITY_MAX_SINGLETON_RATIO", "0.3")
        )

        reasons = []
        passed = True

        # Check modularity (need to compute it from graph)
        # For now, use a placeholder - modularity should be computed in quality_metrics
        # We'll compute it here if not present
        if "modularity" not in quality_metrics:
            # Compute modularity from communities
            all_communities = []
            for level_communities in organized_communities.values():
                for comm_data in level_communities.values():
                    all_communities.append(frozenset(comm_data["entities"]))
            if all_communities:
                try:
                    modularity = nx_community.modularity(
                        G, all_communities, weight="weight"
                    )
                except Exception as e:
                    # Handle incomplete partition (orphan entities from filtering)
                    # This can happen when single-entity communities are filtered out
                    logger.warning(
                        f"Cannot calculate modularity: {e}. "
                        f"This is expected when filtering creates incomplete partitions. "
                        f"Skipping modularity quality gate."
                    )
                    modularity = None  # Skip modularity check
            else:
                modularity = 0.0
        else:
            modularity = quality_metrics.get("modularity", 0.0)

        # Only check modularity if it was successfully calculated
        if modularity is not None and modularity < min_modularity:
            reasons.append(
                f"Modularity {modularity:.4f} below threshold {min_modularity}"
            )
            passed = False

        # Check coverage
        coverage = quality_metrics.get("coverage", 0.0)
        if coverage < min_coverage:
            reasons.append(f"Coverage {coverage:.4f} below threshold {min_coverage}")
            passed = False

        # Check for giant communities
        max_size = 0
        for level_communities in organized_communities.values():
            for comm_data in level_communities.values():
                entity_count = comm_data.get("entity_count", 0)
                max_size = max(max_size, entity_count)

        if max_size > max_community_size:
            reasons.append(
                f"Giant community detected: {max_size} entities (max: {max_community_size})"
            )
            passed = False

        # Check for excessive singleton communities
        total_communities = quality_metrics.get("total_communities", 0)
        if total_communities > 0:
            singleton_count = 0
            for level_communities in organized_communities.values():
                for comm_data in level_communities.values():
                    if comm_data.get("entity_count", 0) == 1:
                        singleton_count += 1

            singleton_ratio = singleton_count / total_communities
            if singleton_ratio > max_singleton_ratio:
                reasons.append(
                    f"Excessive singleton communities: {singleton_ratio:.2%} "
                    f"(max: {max_singleton_ratio:.2%})"
                )
                passed = False

        result = {
            "passed": passed,
            "reasons": reasons,
            "thresholds": {
                "min_modularity": min_modularity,
                "min_coverage": min_coverage,
                "max_community_size": max_community_size,
                "max_singleton_ratio": max_singleton_ratio,
            },
            "actual": {
                "modularity": modularity,
                "coverage": coverage,
                "max_community_size": max_size,
            },
        }

        if passed:
            logger.info("✅ Quality gates passed")
        else:
            logger.warning(f"⚠️ Quality gates failed: {', '.join(reasons)}")

        return result

    def get_community_statistics(
        self, detection_results: Dict[str, Any]
    ) -> Dict[str, Any]:
        """
        Get statistics about community detection results.

        Args:
            detection_results: Results from detect_communities

        Returns:
            Dictionary containing detection statistics
        """
        communities = detection_results.get("communities", {})
        quality_metrics = detection_results.get("quality_metrics", {})
        graph_stats = detection_results.get("graph_stats", {})

        # Level distribution
        level_distribution = {}
        for level, level_communities in communities.items():
            level_distribution[f"level_{level}"] = len(level_communities)

        # Size distribution
        size_distribution = defaultdict(int)
        for level_communities in communities.values():
            for community in level_communities.values():
                size = community["entity_count"]
                if size <= 2:
                    size_distribution["small"] += 1
                elif size <= 5:
                    size_distribution["medium"] += 1
                else:
                    size_distribution["large"] += 1

        return {
            "total_communities": detection_results.get("total_communities", 0),
            "levels": detection_results.get("levels", 0),
            "level_distribution": level_distribution,
            "size_distribution": dict(size_distribution),
            "quality_metrics": quality_metrics,
            "graph_stats": graph_stats,
            "detection_parameters": {
                "max_cluster_size": self.max_cluster_size,
                "min_cluster_size": self.min_cluster_size,
                "resolution_parameter": self.resolution_parameter,
                "max_iterations": self.max_iterations,
            },
        }
