"""
Graph Link Prediction Agent

Uses graph structure and entity embeddings to predict missing relationships.
"""

import logging
import numpy as np
from typing import List, Dict, Any, Tuple, Optional
import networkx as nx
from collections import defaultdict
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


class GraphLinkPredictionAgent:
    """
    Agent for predicting missing links in knowledge graph using graph embeddings.
    """

    def __init__(
        self,
        confidence_threshold: float = 0.65,
        max_predictions_per_entity: int = 5,
        use_structural_features: bool = True,
    ):
        """
        Initialize link prediction agent.

        Args:
            confidence_threshold: Minimum confidence for predicted links
            max_predictions_per_entity: Maximum predicted links per entity
            use_structural_features: Use graph structure for prediction
        """
        self.confidence_threshold = confidence_threshold
        self.max_predictions_per_entity = max_predictions_per_entity
        self.use_structural_features = use_structural_features

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def predict_missing_links(
        self,
        entities: List[Dict[str, Any]],
        relationships: List[Dict[str, Any]],
    ) -> List[Tuple[str, str, str, float]]:
        """
        Predict missing links in the graph.

        Args:
            entities: List of entity documents with embeddings
            relationships: List of existing relationship documents

        Returns:
            List of (subject_id, object_id, predicate, confidence) tuples
        """
        logger.info(f"Starting link prediction for {len(entities)} entities")

        # Build NetworkX graph
        G = nx.Graph()

        for entity in entities:
            G.add_node(entity["entity_id"], **entity)

        for rel in relationships:
            G.add_edge(rel["subject_id"], rel["object_id"], **rel)

        logger.info(
            f"Built graph with {G.number_of_nodes()} nodes, {G.number_of_edges()} edges"
        )

        predictions = []

        # Strategy 1: Common neighbors (structural)
        if self.use_structural_features:
            structural_predictions = self._predict_via_common_neighbors(G)
            predictions.extend(structural_predictions)

        # Strategy 2: Embedding similarity (semantic)
        entities_with_embeddings = [e for e in entities if "entity_embedding" in e]
        if entities_with_embeddings:
            semantic_predictions = self._predict_via_embeddings(
                entities_with_embeddings, G
            )
            predictions.extend(semantic_predictions)

        # Deduplicate and rank by confidence
        predictions = self._deduplicate_predictions(predictions)
        predictions = sorted(predictions, key=lambda x: x[3], reverse=True)

        logger.info(f"Generated {len(predictions)} link predictions")

        return predictions

    def _predict_via_common_neighbors(
        self, G: nx.Graph
    ) -> List[Tuple[str, str, str, float]]:
        """
        Predict links based on common neighbors (Adamic-Adar, Jaccard similarity).

        Args:
            G: NetworkX graph

        Returns:
            List of predictions
        """
        predictions = []

        # Use Adamic-Adar index for link prediction
        try:
            # Get non-edges (potential links)
            non_edges = list(nx.non_edges(G))

            # Limit for performance - sample if too many
            if len(non_edges) > 1000:
                import random

                non_edges = random.sample(non_edges, 1000)

            # Calculate Adamic-Adar scores
            aa_scores = nx.adamic_adar_index(G, non_edges)

            for u, v, score in aa_scores:
                if score > 0.5:  # Threshold for structural similarity
                    confidence = min(0.9, score / 10)  # Normalize to 0-0.9

                    # Determine predicate based on node types
                    u_type = G.nodes[u].get("type", "OTHER")
                    v_type = G.nodes[v].get("type", "OTHER")
                    predicate = self._infer_predicate_from_types(u_type, v_type)

                    predictions.append((u, v, predicate, confidence))

        except Exception as e:
            logger.error(f"Failed to compute Adamic-Adar scores: {e}")

        return predictions

    def _predict_via_embeddings(
        self, entities: List[Dict[str, Any]], G: nx.Graph
    ) -> List[Tuple[str, str, str, float]]:
        """
        Predict links based on entity embedding similarity.

        Args:
            entities: Entities with embeddings
            G: NetworkX graph

        Returns:
            List of predictions
        """
        predictions = []

        entity_dict = {e["entity_id"]: e for e in entities}

        # For each entity, find most similar entities without existing connections
        for entity in entities:
            entity_id = entity["entity_id"]
            entity_embedding = np.array(entity["entity_embedding"])

            # Get existing neighbors
            existing_neighbors = (
                set(G.neighbors(entity_id)) if entity_id in G else set()
            )

            similarities = []

            for other_entity in entities:
                other_id = other_entity["entity_id"]

                if other_id == entity_id or other_id in existing_neighbors:
                    continue

                other_embedding = np.array(other_entity["entity_embedding"])

                # Calculate cosine similarity
                similarity = np.dot(entity_embedding, other_embedding) / (
                    np.linalg.norm(entity_embedding) * np.linalg.norm(other_embedding)
                )

                if similarity >= self.confidence_threshold:
                    similarities.append((other_id, similarity))

            # Get top N similar entities
            similarities = sorted(similarities, key=lambda x: x[1], reverse=True)
            top_similar = similarities[: self.max_predictions_per_entity]

            for other_id, similarity in top_similar:
                # Determine predicate
                entity_type = entity.get("type", "OTHER")
                other_type = entity_dict[other_id].get("type", "OTHER")
                predicate = self._infer_predicate_from_types(entity_type, other_type)

                predictions.append((entity_id, other_id, predicate, float(similarity)))

        return predictions

    def _infer_predicate_from_types(self, type1: str, type2: str) -> str:
        """Infer relationship predicate from entity types."""

        type_patterns = {
            ("PERSON", "CONCEPT"): "discusses",
            ("PERSON", "TECHNOLOGY"): "uses",
            ("CONCEPT", "CONCEPT"): "related_to",
            ("CONCEPT", "TECHNOLOGY"): "implemented_in",
            ("TECHNOLOGY", "TECHNOLOGY"): "works_with",
            ("PERSON", "ORGANIZATION"): "affiliated_with",
            ("ORGANIZATION", "TECHNOLOGY"): "develops",
        }

        predicate = type_patterns.get((type1, type2))
        if not predicate:
            predicate = type_patterns.get((type2, type1), "related_to")

        return predicate

    def _deduplicate_predictions(
        self, predictions: List[Tuple[str, str, str, float]]
    ) -> List[Tuple[str, str, str, float]]:
        """Remove duplicate predictions, keeping highest confidence."""

        seen = {}

        for subj, obj, pred, conf in predictions:
            # Create bidirectional key (order-independent)
            key = tuple(sorted([subj, obj])) + (pred,)

            if key not in seen or conf > seen[key][3]:
                seen[key] = (subj, obj, pred, conf)

        return list(seen.values())
