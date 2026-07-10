"""
Quality Metrics Service

This module provides comprehensive quality metrics calculation for each GraphRAG pipeline stage.
Metrics are calculated from transformation logs and intermediate data collections.

Achievement 0.4: Per-Stage Quality Metrics Implementation
"""

import logging
from typing import Dict, List, Any, Optional
from datetime import datetime, timezone
from pymongo.database import Database
from pymongo.collection import Collection
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


class QualityMetricsService:
    """
    Service for calculating and storing quality metrics at each pipeline stage.
    
    Calculates 23 metrics across 4 stages:
    - Extraction: 6 metrics
    - Resolution: 6 metrics
    - Construction: 6 metrics
    - Detection: 5 metrics
    
    Metrics are stored in:
    - graphrag_runs collection (per-run snapshot)
    - quality_metrics collection (time-series)
    """

    # Collection names
    GRAPHRAG_RUNS = "graphrag_runs"
    QUALITY_METRICS = "quality_metrics"
    
    # Healthy ranges for metrics
    HEALTHY_RANGES = {
        "extraction": {
            "entity_count_avg": (8, 15),
            "relationship_count_avg": (5, 12),
            "predicate_diversity": (0.6, 0.9),
            "type_coverage": (0.7, 1.0),
            "confidence_avg": (0.75, 0.95),
            "canonical_predicate_coverage": (0.8, 1.0),
        },
        "resolution": {
            "merge_rate": (0.15, 0.35),
            "confidence_preservation": (0.95, 1.0),
            "cross_video_linking_rate": (0.10, 0.30),
            "false_positive_estimate": (0.0, 0.05),
            "false_negative_estimate": (0.0, 0.10),
        },
        "construction": {
            "graph_density": (0.15, 0.25),
            "average_degree": (3, 8),
        },
        "detection": {
            "modularity": (0.3, 0.7),
            "coherence_avg": (0.65, 0.95),
            "singleton_rate": (0.0, 0.10),
            "coverage": (0.85, 1.0),
        },
    }

    def __init__(self, db: Database, enabled: bool = True):
        """
        Initialize the Quality Metrics Service.

        Args:
            db: MongoDB database instance
            enabled: Whether metrics calculation is enabled
        """
        self.db = db
        self.enabled = enabled
        
        if self.enabled:
            self._ensure_indexes()
        
        logger.info(f"Initialized QualityMetricsService (enabled={enabled})")

    def _ensure_indexes(self):
        """Create indexes for fast querying."""
        try:
            # Indexes for graphrag_runs
            runs_collection = self.db[self.GRAPHRAG_RUNS]
            runs_collection.create_index("trace_id", unique=True)
            runs_collection.create_index("timestamp")
            
            # Indexes for quality_metrics (time-series)
            metrics_collection = self.db[self.QUALITY_METRICS]
            metrics_collection.create_index([("trace_id", 1), ("stage", 1)])
            metrics_collection.create_index("timestamp")
            metrics_collection.create_index("stage")
            
            logger.info("Quality metrics indexes ensured")
        except Exception as e:
            logger.error(f"Error creating quality metrics indexes: {e}")

    @handle_errors(log_traceback=True, reraise=False)
    def calculate_extraction_metrics(self, trace_id: str) -> Dict[str, Any]:
        """
        Calculate extraction quality metrics.
        
        Metrics:
        - entity_count_avg: Average entities per chunk
        - relationship_count_avg: Average relationships per chunk
        - predicate_diversity: Unique predicates / total relationships
        - type_coverage: % of entity types represented
        - confidence_avg: Average confidence score
        - canonical_predicate_coverage: % using canonical predicates
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Dictionary with extraction metrics
        """
        if not self.enabled:
            return {}
        
        try:
            entities_raw = self.db.entities_raw
            relations_raw = self.db.relations_raw
            
            # Get all raw entities for this trace
            raw_entities = list(entities_raw.find({"trace_id": trace_id}))
            raw_relations = list(relations_raw.find({"trace_id": trace_id}))
            
            if not raw_entities and not raw_relations:
                logger.warning(f"No extraction data found for trace_id={trace_id}")
                return {}
            
            # Calculate entity count per chunk
            chunk_entity_counts = {}
            for entity in raw_entities:
                chunk_id = entity.get("chunk_id", "unknown")
                chunk_entity_counts[chunk_id] = chunk_entity_counts.get(chunk_id, 0) + 1
            
            entity_count_avg = (
                sum(chunk_entity_counts.values()) / len(chunk_entity_counts)
                if chunk_entity_counts else 0
            )
            
            # Calculate relationship count per chunk
            chunk_relation_counts = {}
            for relation in raw_relations:
                chunk_id = relation.get("chunk_id", "unknown")
                chunk_relation_counts[chunk_id] = chunk_relation_counts.get(chunk_id, 0) + 1
            
            relationship_count_avg = (
                sum(chunk_relation_counts.values()) / len(chunk_relation_counts)
                if chunk_relation_counts else 0
            )
            
            # Calculate predicate diversity
            predicates = set(r.get("predicate", "") for r in raw_relations if r.get("predicate"))
            predicate_diversity = (
                len(predicates) / len(raw_relations)
                if raw_relations else 0
            )
            
            # Calculate type coverage
            entity_types = set(e.get("type", "") for e in raw_entities if e.get("type"))
            # Assume we want to see at least 5 common types (person, organization, location, concept, event)
            expected_types = 5
            type_coverage = min(len(entity_types) / expected_types, 1.0) if entity_types else 0
            
            # Calculate confidence average
            confidences = [e.get("confidence", 0) for e in raw_entities if "confidence" in e]
            confidence_avg = sum(confidences) / len(confidences) if confidences else 0
            
            # Calculate canonical predicate coverage
            # Canonical predicates are those that follow standard naming (lowercase, underscored)
            canonical_count = sum(
                1 for r in raw_relations
                if r.get("predicate", "").islower() and "_" in r.get("predicate", "")
            )
            canonical_predicate_coverage = (
                canonical_count / len(raw_relations)
                if raw_relations else 0
            )
            
            metrics = {
                "entity_count_avg": round(entity_count_avg, 2),
                "relationship_count_avg": round(relationship_count_avg, 2),
                "predicate_diversity": round(predicate_diversity, 3),
                "type_coverage": round(type_coverage, 3),
                "confidence_avg": round(confidence_avg, 3),
                "canonical_predicate_coverage": round(canonical_predicate_coverage, 3),
                "total_entities": len(raw_entities),
                "total_relationships": len(raw_relations),
                "unique_chunks": len(chunk_entity_counts),
            }
            
            logger.info(f"Calculated extraction metrics for trace_id={trace_id}: {metrics}")
            return metrics
            
        except Exception as e:
            logger.error(f"Error calculating extraction metrics: {e}")
            return {}

    @handle_errors(log_traceback=True, reraise=False)
    def calculate_resolution_metrics(self, trace_id: str) -> Dict[str, Any]:
        """
        Calculate resolution quality metrics.
        
        Metrics:
        - merge_rate: (raw entities - resolved entities) / raw entities
        - duplicate_reduction: Number of duplicates merged
        - confidence_preservation: Average confidence before/after
        - cross_video_linking_rate: Entities appearing in multiple videos
        - false_positive_estimate: High-confidence merges with low similarity
        - false_negative_estimate: High similarity but didn't merge
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Dictionary with resolution metrics
        """
        if not self.enabled:
            return {}
        
        try:
            entities_raw = self.db.entities_raw
            entities_resolved = self.db.entities_resolved
            transformation_logs = self.db.transformation_logs
            
            # Count entities before and after resolution
            raw_count = entities_raw.count_documents({"trace_id": trace_id})
            resolved_count = entities_resolved.count_documents({"trace_id": trace_id})
            
            if raw_count == 0:
                logger.warning(f"No raw entities found for trace_id={trace_id}")
                return {}
            
            # Calculate merge rate
            merge_rate = (raw_count - resolved_count) / raw_count if raw_count > 0 else 0
            duplicate_reduction = raw_count - resolved_count
            
            # Calculate confidence preservation
            raw_entities = list(entities_raw.find({"trace_id": trace_id}))
            resolved_entities = list(entities_resolved.find({"trace_id": trace_id}))
            
            raw_confidences = [e.get("confidence", 0) for e in raw_entities if "confidence" in e]
            resolved_confidences = [e.get("confidence", 0) for e in resolved_entities if "confidence" in e]
            
            raw_conf_avg = sum(raw_confidences) / len(raw_confidences) if raw_confidences else 0
            resolved_conf_avg = sum(resolved_confidences) / len(resolved_confidences) if resolved_confidences else 0
            
            confidence_preservation = (
                resolved_conf_avg / raw_conf_avg if raw_conf_avg > 0 else 1.0
            )
            
            # Calculate cross-video linking rate
            # Count entities that appear in multiple videos (have multiple chunk_ids from different videos)
            entity_videos = {}
            for entity in resolved_entities:
                entity_id = entity.get("entity_id", "")
                video_id = entity.get("video_id", "")
                if entity_id and video_id:
                    if entity_id not in entity_videos:
                        entity_videos[entity_id] = set()
                    entity_videos[entity_id].add(video_id)
            
            cross_video_entities = sum(1 for videos in entity_videos.values() if len(videos) > 1)
            cross_video_linking_rate = (
                cross_video_entities / len(entity_videos)
                if entity_videos else 0
            )
            
            # Estimate false positives from transformation logs
            # High-confidence merges (confidence > 0.9) with low similarity (< 0.7)
            merge_logs = list(transformation_logs.find({
                "trace_id": trace_id,
                "operation": "entity_merge",
                "confidence": {"$gt": 0.9},
            }))
            
            # Check if similarity is low (would need similarity in logs)
            # For now, use a heuristic: very different entity names
            false_positive_count = 0
            for log in merge_logs:
                before = log.get("before", {})
                after = log.get("after", {})
                before_name = before.get("name", "").lower()
                after_name = after.get("name", "").lower()
                
                # If names are very different (no common words), might be false positive
                before_words = set(before_name.split())
                after_words = set(after_name.split())
                common_words = before_words & after_words
                
                if len(common_words) == 0 and before_name and after_name:
                    false_positive_count += 1
            
            false_positive_estimate = (
                false_positive_count / len(merge_logs)
                if merge_logs else 0
            )
            
            # Estimate false negatives
            # Entities with high similarity but didn't merge (would need similarity matrix)
            # For now, use a heuristic: entities with same type and very similar names
            false_negative_count = 0
            for i, e1 in enumerate(resolved_entities):
                for e2 in resolved_entities[i+1:]:
                    if e1.get("type") == e2.get("type"):
                        name1 = e1.get("name", "").lower()
                        name2 = e2.get("name", "").lower()
                        
                        # Simple similarity: check if one name contains the other
                        if name1 and name2 and (name1 in name2 or name2 in name1):
                            false_negative_count += 1
                            if false_negative_count >= 10:  # Limit to avoid long computation
                                break
                if false_negative_count >= 10:
                    break
            
            false_negative_estimate = (
                false_negative_count / resolved_count
                if resolved_count > 0 else 0
            )
            
            metrics = {
                "merge_rate": round(merge_rate, 3),
                "duplicate_reduction": duplicate_reduction,
                "confidence_preservation": round(confidence_preservation, 3),
                "cross_video_linking_rate": round(cross_video_linking_rate, 3),
                "false_positive_estimate": round(false_positive_estimate, 3),
                "false_negative_estimate": round(min(false_negative_estimate, 1.0), 3),
                "raw_entity_count": raw_count,
                "resolved_entity_count": resolved_count,
            }
            
            logger.info(f"Calculated resolution metrics for trace_id={trace_id}: {metrics}")
            return metrics
            
        except Exception as e:
            logger.error(f"Error calculating resolution metrics: {e}")
            return {}

    @handle_errors(log_traceback=True, reraise=False)
    def calculate_construction_metrics(self, trace_id: str) -> Dict[str, Any]:
        """
        Calculate graph construction quality metrics.
        
        Metrics:
        - graph_density: Current density and optimal range
        - average_degree: Current avg degree and healthy range
        - degree_distribution: Power law, random, or small-world
        - relationship_type_balance: % LLM vs co-occurrence vs semantic
        - post_processing_contribution: Relationships added by each method
        - density_safeguard_triggers: How often hit max density
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Dictionary with construction metrics
        """
        if not self.enabled:
            return {}
        
        try:
            entities_resolved = self.db.entities_resolved
            relations_raw = self.db.relations_raw
            relations_final = self.db.relations_final
            transformation_logs = self.db.transformation_logs
            
            # Count entities and relationships
            node_count = entities_resolved.count_documents({"trace_id": trace_id})
            edge_count_raw = relations_raw.count_documents({"trace_id": trace_id})
            edge_count_final = relations_final.count_documents({"trace_id": trace_id})
            
            if node_count == 0:
                logger.warning(f"No resolved entities found for trace_id={trace_id}")
                return {}
            
            # Calculate graph density
            max_edges = node_count * (node_count - 1)
            graph_density = edge_count_final / max_edges if max_edges > 0 else 0
            
            # Calculate average degree
            # Degree = number of edges connected to a node (count both directions)
            final_relations = list(relations_final.find({"trace_id": trace_id}))
            
            degree_counts = {}
            for relation in final_relations:
                source = relation.get("source_id", "")
                target = relation.get("target_id", "")
                
                degree_counts[source] = degree_counts.get(source, 0) + 1
                degree_counts[target] = degree_counts.get(target, 0) + 1
            
            average_degree = (
                sum(degree_counts.values()) / len(degree_counts)
                if degree_counts else 0
            )
            
            # Analyze degree distribution
            degrees = list(degree_counts.values())
            if degrees:
                max_degree = max(degrees)
                min_degree = min(degrees)
                
                # Simple heuristic for distribution type
                # Power law: few nodes with very high degree
                high_degree_nodes = sum(1 for d in degrees if d > average_degree * 2)
                if high_degree_nodes / len(degrees) < 0.1:
                    degree_distribution_type = "power_law"
                elif max_degree - min_degree < average_degree:
                    degree_distribution_type = "random"
                else:
                    degree_distribution_type = "small_world"
            else:
                degree_distribution_type = "unknown"
                max_degree = 0
            
            # Calculate relationship type balance
            # Count relationships by source (LLM extraction, co-occurrence, semantic similarity)
            relationship_sources = {}
            for relation in final_relations:
                source = relation.get("source", "llm")  # Default to LLM
                relationship_sources[source] = relationship_sources.get(source, 0) + 1
            
            total_rels = len(final_relations)
            relationship_type_balance = {
                source: round(count / total_rels, 3)
                for source, count in relationship_sources.items()
            } if total_rels > 0 else {}
            
            # Calculate post-processing contribution
            post_processing_contribution = {
                "co_occurrence": relationship_sources.get("co_occurrence", 0),
                "semantic_similarity": relationship_sources.get("semantic_similarity", 0),
                "total_added": edge_count_final - edge_count_raw,
            }
            
            # Count density safeguard triggers from logs
            safeguard_logs = transformation_logs.count_documents({
                "trace_id": trace_id,
                "operation": "relationship_filter",
                "reason": {"$regex": "density.*safeguard", "$options": "i"},
            })
            
            metrics = {
                "graph_density": round(graph_density, 4),
                "average_degree": round(average_degree, 2),
                "max_degree": max_degree,
                "degree_distribution_type": degree_distribution_type,
                "relationship_type_balance": relationship_type_balance,
                "post_processing_contribution": post_processing_contribution,
                "density_safeguard_triggers": safeguard_logs,
                "node_count": node_count,
                "edge_count_raw": edge_count_raw,
                "edge_count_final": edge_count_final,
            }
            
            logger.info(f"Calculated construction metrics for trace_id={trace_id}: {metrics}")
            return metrics
            
        except Exception as e:
            logger.error(f"Error calculating construction metrics: {e}")
            return {}

    @handle_errors(log_traceback=True, reraise=False)
    def calculate_detection_metrics(self, trace_id: str) -> Dict[str, Any]:
        """
        Calculate community detection quality metrics.
        
        Metrics:
        - modularity: Modularity score (higher = better communities)
        - community_count: Number of communities detected
        - community_size_distribution: Size stats (avg, p50, p95)
        - coherence_avg: Average coherence score per community
        - singleton_rate: % of single-entity communities
        - coverage: % of entities in meaningful communities
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Dictionary with detection metrics
        """
        if not self.enabled:
            return {}
        
        try:
            entities_resolved = self.db.entities_resolved
            communities = self.db.communities
            
            # Get all communities for this trace
            community_docs = list(communities.find({"trace_id": trace_id}))
            
            if not community_docs:
                logger.warning(f"No communities found for trace_id={trace_id}")
                return {}
            
            # Count total entities
            total_entities = entities_resolved.count_documents({"trace_id": trace_id})
            
            # Calculate community count and sizes
            community_count = len(community_docs)
            community_sizes = [len(c.get("entity_ids", [])) for c in community_docs]
            
            # Calculate size distribution
            community_sizes.sort()
            size_avg = sum(community_sizes) / len(community_sizes) if community_sizes else 0
            size_p50 = community_sizes[len(community_sizes) // 2] if community_sizes else 0
            size_p95 = community_sizes[int(len(community_sizes) * 0.95)] if community_sizes else 0
            
            # Calculate coherence average
            coherences = [c.get("coherence_score", 0) for c in community_docs if "coherence_score" in c]
            coherence_avg = sum(coherences) / len(coherences) if coherences else 0
            
            # Calculate singleton rate
            singleton_count = sum(1 for size in community_sizes if size == 1)
            singleton_rate = singleton_count / community_count if community_count > 0 else 0
            
            # Calculate coverage (entities in communities with size > 1)
            entities_in_meaningful_communities = sum(
                size for size in community_sizes if size > 1
            )
            coverage = (
                entities_in_meaningful_communities / total_entities
                if total_entities > 0 else 0
            )
            
            # Calculate modularity (if available in community docs)
            # Otherwise estimate from community structure
            modularity = 0
            for community in community_docs:
                if "modularity" in community:
                    modularity = community["modularity"]
                    break
            
            # If no modularity stored, use a simple estimate
            # Modularity ≈ (edges within communities / total edges) - (expected random)
            # For simplicity, use coherence as a proxy
            if modularity == 0:
                modularity = coherence_avg * 0.8  # Rough estimate
            
            metrics = {
                "modularity": round(modularity, 3),
                "community_count": community_count,
                "community_size_avg": round(size_avg, 2),
                "community_size_p50": size_p50,
                "community_size_p95": size_p95,
                "coherence_avg": round(coherence_avg, 3),
                "singleton_rate": round(singleton_rate, 3),
                "coverage": round(coverage, 3),
                "total_entities": total_entities,
            }
            
            logger.info(f"Calculated detection metrics for trace_id={trace_id}: {metrics}")
            return metrics
            
        except Exception as e:
            logger.error(f"Error calculating detection metrics: {e}")
            return {}

    @handle_errors(log_traceback=True, reraise=False)
    def calculate_all_metrics(self, trace_id: str) -> Dict[str, Any]:
        """
        Calculate all quality metrics for a pipeline run.
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Dictionary with all metrics organized by stage
        """
        if not self.enabled:
            return {}
        
        logger.info(f"Calculating all quality metrics for trace_id={trace_id}")
        
        metrics = {
            "trace_id": trace_id,
            "timestamp": datetime.now(timezone.utc),
            "extraction": self.calculate_extraction_metrics(trace_id),
            "resolution": self.calculate_resolution_metrics(trace_id),
            "construction": self.calculate_construction_metrics(trace_id),
            "detection": self.calculate_detection_metrics(trace_id),
        }
        
        return metrics

    @handle_errors(log_traceback=True, reraise=False)
    def store_metrics(self, trace_id: str, metrics: Dict[str, Any]) -> bool:
        """
        Store calculated metrics in MongoDB collections.
        
        Stores in two collections:
        - graphrag_runs: Per-run snapshot
        - quality_metrics: Time-series data
        
        Args:
            trace_id: Pipeline run trace ID
            metrics: Calculated metrics dictionary
            
        Returns:
            True if successful, False otherwise
        """
        if not self.enabled:
            return False
        
        try:
            timestamp = metrics.get("timestamp", datetime.now(timezone.utc))
            
            # Store in graphrag_runs (per-run snapshot)
            runs_collection = self.db[self.GRAPHRAG_RUNS]
            runs_collection.update_one(
                {"trace_id": trace_id},
                {"$set": {
                    "trace_id": trace_id,
                    "timestamp": timestamp,
                    "metrics": metrics,
                }},
                upsert=True
            )
            
            # Store in quality_metrics (time-series)
            metrics_collection = self.db[self.QUALITY_METRICS]
            
            # Performance Optimization (Achievement 7.2):
            # Batch all metrics into a list and use insert_many() instead of multiple insert_one() calls
            metric_documents = []
            
            # Store each stage's metrics separately for time-series queries
            for stage in ["extraction", "resolution", "construction", "detection"]:
                stage_metrics = metrics.get(stage, {})
                if not stage_metrics:
                    continue
                
                # Collect all metric documents for this stage
                for metric_name, metric_value in stage_metrics.items():
                    # Skip non-numeric metrics
                    if not isinstance(metric_value, (int, float)):
                        continue
                    
                    # Check if metric is in healthy range
                    healthy_range = self.HEALTHY_RANGES.get(stage, {}).get(metric_name)
                    in_range = None
                    if healthy_range:
                        min_val, max_val = healthy_range
                        in_range = min_val <= metric_value <= max_val
                    
                    metric_documents.append({
                        "trace_id": trace_id,
                        "timestamp": timestamp,
                        "stage": stage,
                        "metric_name": metric_name,
                        "metric_value": metric_value,
                        "healthy_range": healthy_range,
                        "in_range": in_range,
                    })
            
            # Batch insert all metrics at once
            if metric_documents:
                metrics_collection.insert_many(metric_documents, ordered=False)
                logger.info(f"Stored {len(metric_documents)} metrics for trace_id={trace_id}")
            
            return True
            
        except Exception as e:
            logger.error(f"Error storing metrics: {e}")
            return False

    @handle_errors(log_traceback=True, reraise=False)
    def get_metrics(self, trace_id: str) -> Optional[Dict[str, Any]]:
        """
        Retrieve stored metrics for a pipeline run.
        
        Args:
            trace_id: Pipeline run trace ID
            
        Returns:
            Metrics dictionary or None if not found
        """
        if not self.enabled:
            return None
        
        try:
            runs_collection = self.db[self.GRAPHRAG_RUNS]
            run_doc = runs_collection.find_one({"trace_id": trace_id})
            
            if run_doc:
                return run_doc.get("metrics")
            
            return None
            
        except Exception as e:
            logger.error(f"Error retrieving metrics: {e}")
            return None

    @handle_errors(log_traceback=True, reraise=False)
    def get_metrics_time_series(
        self,
        stage: str,
        metric_name: str,
        limit: int = 50
    ) -> List[Dict[str, Any]]:
        """
        Retrieve time-series data for a specific metric.
        
        Args:
            stage: Stage name (extraction, resolution, construction, detection)
            metric_name: Metric name
            limit: Maximum number of data points
            
        Returns:
            List of metric data points sorted by timestamp
        """
        if not self.enabled:
            return []
        
        try:
            metrics_collection = self.db[self.QUALITY_METRICS]
            
            cursor = metrics_collection.find({
                "stage": stage,
                "metric_name": metric_name,
            }).sort("timestamp", -1).limit(limit)
            
            data_points = list(cursor)
            data_points.reverse()  # Chronological order
            
            return data_points
            
        except Exception as e:
            logger.error(f"Error retrieving time-series data: {e}")
            return []

    @handle_errors(log_traceback=True, reraise=False)
    def check_healthy_ranges(self, metrics: Dict[str, Any]) -> Dict[str, List[str]]:
        """
        Check which metrics are outside healthy ranges.
        
        Args:
            metrics: Calculated metrics dictionary
            
        Returns:
            Dictionary with warnings for out-of-range metrics
        """
        warnings = {
            "extraction": [],
            "resolution": [],
            "construction": [],
            "detection": [],
        }
        
        for stage in ["extraction", "resolution", "construction", "detection"]:
            stage_metrics = metrics.get(stage, {})
            healthy_ranges = self.HEALTHY_RANGES.get(stage, {})
            
            for metric_name, (min_val, max_val) in healthy_ranges.items():
                metric_value = stage_metrics.get(metric_name)
                
                if metric_value is None:
                    continue
                
                if metric_value < min_val:
                    warnings[stage].append(
                        f"{metric_name}={metric_value:.3f} below healthy range [{min_val}, {max_val}]"
                    )
                elif metric_value > max_val:
                    warnings[stage].append(
                        f"{metric_name}={metric_value:.3f} above healthy range [{min_val}, {max_val}]"
                    )
        
        return warnings


