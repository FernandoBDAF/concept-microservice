import math
import logging
import time
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Set
import argparse
from collections import defaultdict

from dotenv import load_dotenv
from pymongo import MongoClient
import os

try:
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", "..")))
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
from src.core.config.paths import DB_NAME, COLL_CHUNKS
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


def sigmoid(x: float) -> float:
    return 1.0 / (1.0 + math.exp(-x))


def compute_trust_score(chunk: Dict[str, Any]) -> float:
    redundancy = float(chunk.get("redundancy_score", 0.0) or 0.0)
    is_redundant = bool(chunk.get("is_redundant", False))
    engagement = float(chunk.get("metadata", {}).get("engagement_norm", 0.0) or 0.0)
    recency_days = float(chunk.get("metadata", {}).get("age_days", 365) or 365)

    consensus = redundancy if is_redundant else 0.5 * redundancy
    recency_component = sigmoid(max(-6.0, min(6.0, (180.0 - recency_days) / 60.0)))
    engagement_component = max(0.0, min(1.0, engagement))

    w1, w2, w3, w4 = 0.4, 0.3, 0.2, 0.1
    code_valid = 1.0 if chunk.get("metadata", {}).get("code_present") else 0.6
    score = w1 * consensus + w2 * recency_component + w3 * engagement_component + w4 * code_valid
    return max(0.0, min(1.0, score))


@dataclass
class TrustConfig(BaseStageConfig):
    use_llm: bool = False
    auto_llm: bool = True
    band_low: float = 0.40
    band_high: float = 0.70
    neighbors: int = 2

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        use_llm = bool(getattr(args, "llm", False) or (env.get("TRUST_WITH_LLM") == "1"))
        auto_llm = (env.get("TRUST_LLM_AUTO", "true") or "true").lower() in {
            "1",
            "true",
            "yes",
            "on",
        }

        def _getf(key: str, default: float) -> float:
            try:
                return float(env.get(key, str(default)))
            except Exception:
                return default

        band_low = _getf("TRUST_LLM_BAND_LOW", 0.40)
        band_high = _getf("TRUST_LLM_BAND_HIGH", 0.70)

        def _geti(key: str, default: int) -> int:
            try:
                return int(env.get(key, str(default)))
            except Exception:
                return default

        neighbors = _geti("TRUST_LLM_NEIGHBORS", 2)
        return cls(
            **vars(base),
            use_llm=use_llm,
            auto_llm=auto_llm,
            band_low=band_low,
            band_high=band_high,
            neighbors=neighbors,
        )


class TrustStage(BaseStage):
    name = "trust"
    description = "Compute trust scores with heuristic base and optional LLM for borderline cases"
    ConfigCls = TrustConfig

    def iter_docs(self) -> List[Dict[str, Any]]:
        src_db = self.config.read_db_name or self.config.db_name
        coll = self.get_collection(self.config.read_coll or COLL_CHUNKS, io="read", db_name=src_db)
        
        # Build query with video filtering
        q: Dict[str, Any] = {}
        if self.config.video_id:
            # Single video filter (CLI argument)
            q["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            q["video_id"] = {"$in": self.config.input_video_ids}
            print(f"[trust] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        docs = list(
            coll.find(
                q,
                {
                    "video_id": 1,
                    "chunk_id": 1,
                    "metadata": 1,
                    "is_redundant": 1,
                    "redundancy_score": 1,
                    "text": 1,
                    "embedding": 1,
                },
            )
        )
        print(f"[trust] Selected {len(docs)} chunk(s) for trust scoring")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, c: Dict[str, Any]) -> None:
        dst_db = self.config.write_db_name or self.config.db_name
        coll = self.get_collection(
            self.config.write_coll or COLL_CHUNKS, io="write", db_name=dst_db
        )
        if self.config.use_llm:
            try:
                from src.domain.agents.ingestion.trust import TrustRankAgent

                payload = {
                    "chunk_text": c.get("text", ""),
                    "similar_chunks": [],
                    "channel_metrics": {},
                    "published_at": None,
                    "code_valid": bool(c.get("metadata", {}).get("code_present")),
                }
                heuristic_score = compute_trust_score(c)
                do_llm = self.config.auto_llm and (
                    self.config.band_low
                    <= float(c.get("redundancy_score", 0.0) or 0.0)
                    <= self.config.band_high
                    or bool(c.get("metadata", {}).get("code_present"))
                    or float(c.get("metadata", {}).get("age_days", 365) or 365) < 30
                )
                score = heuristic_score
                method = "heuristic"
                if do_llm:
                    try:
                        from math import sqrt

                        vid = c.get("video_id")
                        base = c.get("embedding", [])
                        neigh_docs = list(
                            coll.find(
                                {"video_id": vid},
                                {"chunk_id": 1, "text": 1, "embedding": 1},
                            ).limit(50)
                        )
                        sims = []

                        def _cos(a, b):
                            if not a or not b or len(a) != len(b):
                                return 0.0
                            s = sum(x * y for x, y in zip(a, b))
                            da = sqrt(sum(x * x for x in a)) or 1.0
                            db = sqrt(sum(y * y for y in b)) or 1.0
                            return s / (da * db)

                        for d in neigh_docs:
                            if d.get("chunk_id") == c.get("chunk_id"):
                                continue
                            sims.append((d, _cos(base, d.get("embedding", []))))
                        sims.sort(key=lambda x: x[1], reverse=True)
                        topn = []
                        for d, _ in sims[: max(0, self.config.neighbors)]:
                            topn.append(
                                {
                                    "chunk_id": d.get("chunk_id"),
                                    "text": (d.get("text", "")[:500]),
                                }
                            )
                        payload["similar_chunks"] = topn
                    except Exception:
                        pass
                    agent = TrustRankAgent()
                    out = agent.score(payload)
                    s = out.get("trust_score")
                    if s is not None:
                        score = s
                        method = "llm"
            except Exception:
                score = compute_trust_score(c)
                method = "heuristic"
        else:
            score = compute_trust_score(c)
            method = "heuristic"
        chunk_id = c.get("chunk_id")
        video_id = c.get("video_id")

        if not self.config.upsert_existing:
            existing = coll.find_one({"_id": c["_id"]}, {"trust_score": 1})
            if existing and "trust_score" in existing:
                logger.info(
                    f"[trust] Skipping {chunk_id} (video={video_id}): "
                    f"already has trust_score={existing.get('trust_score'):.3f}"
                )
                self.stats["skipped"] += 1
                return

        logger.info(
            f"[trust] Updating {chunk_id} (video={video_id}): "
            f"trust_score={score:.3f}, method={method}"
        )

        coll.update_one(
            {"_id": c["_id"]},
            {"$set": {"trust_score": float(score), "trust_method": method}},
        )
        self.stats["updated"] += 1

    def get_entity_trust_scores(self, video_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Export entity trust scores for GraphRAG.

        This method provides trust scores for entities based on their source chunks
        and can be used by the GraphRAG pipeline to weight entity importance.

        Args:
            video_id: Optional video ID to filter by

        Returns:
            Dictionary containing entity trust scores
        """
        try:
            src_db = self.config.read_db_name or self.config.db_name
            coll = self.get_collection(
                self.config.read_coll or COLL_CHUNKS, io="read", db_name=src_db
            )

            # Query for chunks with trust scores
            query = {
                "trust_score": {"$exists": True},
                "graphrag_resolution.status": "completed",
            }
            if video_id:
                query["video_id"] = video_id

            chunks = list(
                coll.find(
                    query,
                    {
                        "video_id": 1,
                        "chunk_id": 1,
                        "trust_score": 1,
                        "trust_method": 1,
                        "graphrag_resolution.resolved_entities": 1,
                    },
                )
            )

            # Calculate entity trust scores
            entity_trust_scores = defaultdict(list)
            entity_source_counts = defaultdict(int)

            for chunk in chunks:
                chunk_trust_score = chunk.get("trust_score", 0.0)
                chunk_method = chunk.get("trust_method", "heuristic")
                resolved_entities = chunk.get("graphrag_resolution", {}).get("resolved_entities", 0)

                # If chunk has resolved entities, distribute trust score
                if resolved_entities > 0:
                    # For now, we'll use a simple approach - in a real implementation,
                    # you might want to track which specific entities are in each chunk
                    chunk_id = chunk.get("chunk_id")

                    # Store chunk-level trust information
                    entity_trust_scores[chunk_id] = {
                        "trust_score": chunk_trust_score,
                        "trust_method": chunk_method,
                        "resolved_entities": resolved_entities,
                        "video_id": chunk.get("video_id"),
                    }

                    entity_source_counts[chunk_id] += 1

            # Calculate aggregated entity trust scores
            aggregated_scores = {}
            for chunk_id, trust_info in entity_trust_scores.items():
                source_count = entity_source_counts[chunk_id]
                avg_trust_score = trust_info["trust_score"]

                # Weight by source count and trust score
                weighted_score = avg_trust_score * min(source_count, 5) / 5.0  # Cap at 5 sources

                aggregated_scores[chunk_id] = {
                    "trust_score": weighted_score,
                    "source_count": source_count,
                    "trust_method": trust_info["trust_method"],
                    "video_id": trust_info["video_id"],
                }

            logger.info(f"Generated entity trust scores for {len(aggregated_scores)} entities")

            return {
                "entity_trust_scores": aggregated_scores,
                "total_entities": len(aggregated_scores),
                "avg_trust_score": (
                    sum(s["trust_score"] for s in aggregated_scores.values())
                    / len(aggregated_scores)
                    if aggregated_scores
                    else 0
                ),
                "high_trust_entities": len(
                    [s for s in aggregated_scores.values() if s["trust_score"] >= 0.7]
                ),
                "generated_at": time.time(),
            }

        except Exception as e:
            logger.error(f"Failed to generate entity trust scores: {e}")
            return {
                "entity_trust_scores": {},
                "total_entities": 0,
                "avg_trust_score": 0,
                "high_trust_entities": 0,
                "error": str(e),
                "generated_at": time.time(),
            }

    def propagate_trust_to_entities(self, video_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Propagate trust scores from chunks to entities in the GraphRAG collections.

        Args:
            video_id: Optional video ID to filter by

        Returns:
            Dictionary containing propagation results
        """
        try:
            # Get entity trust scores
            trust_data = self.get_entity_trust_scores(video_id)
            entity_trust_scores = trust_data.get("entity_trust_scores", {})

            if not entity_trust_scores:
                logger.warning("No entity trust scores available for propagation")
                return {"propagated_entities": 0, "error": "no_trust_scores"}

            # Get GraphRAG collections
            from src.domain.services.graphrag.indexes import get_graphrag_collections

            graphrag_collections = get_graphrag_collections(self.db)
            entities_collection = graphrag_collections["entities"]
            entity_mentions_collection = graphrag_collections["entity_mentions"]

            propagated_count = 0

            # Update entities with trust scores based on their source chunks
            for entity_doc in entities_collection.find({}):
                entity_id = entity_doc.get("entity_id")
                source_chunks = entity_doc.get("source_chunks", [])

                if not source_chunks:
                    continue

                # Calculate entity trust score from source chunks
                chunk_trust_scores = []
                for chunk_id in source_chunks:
                    if chunk_id in entity_trust_scores:
                        chunk_trust_scores.append(entity_trust_scores[chunk_id]["trust_score"])

                if chunk_trust_scores:
                    # Use average trust score from source chunks
                    avg_trust_score = sum(chunk_trust_scores) / len(chunk_trust_scores)

                    # Update entity with trust score
                    entities_collection.update_one(
                        {"entity_id": entity_id},
                        {
                            "$set": {
                                "trust_score": avg_trust_score,
                                "trust_source_count": len(chunk_trust_scores),
                                "trust_updated_at": time.time(),
                            }
                        },
                    )

                    propagated_count += 1

            logger.info(f"Propagated trust scores to {propagated_count} entities")

            return {
                "propagated_entities": propagated_count,
                "total_entities": len(entity_trust_scores),
                "propagation_rate": (
                    propagated_count / len(entity_trust_scores) if entity_trust_scores else 0
                ),
                "propagated_at": time.time(),
            }

        except Exception as e:
            logger.error(f"Failed to propagate trust scores to entities: {e}")
            return {
                "propagated_entities": 0,
                "error": str(e),
                "propagated_at": time.time(),
            }

    def get_trust_statistics(self, video_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Get comprehensive trust statistics for GraphRAG.

        Args:
            video_id: Optional video ID to filter by

        Returns:
            Dictionary containing trust statistics
        """
        try:
            src_db = self.config.read_db_name or self.config.db_name
            coll = self.get_collection(
                self.config.read_coll or COLL_CHUNKS, io="read", db_name=src_db
            )

            # Query for chunks with trust scores
            query = {"trust_score": {"$exists": True}}
            if video_id:
                query["video_id"] = video_id

            chunks = list(
                coll.find(
                    query,
                    {"trust_score": 1, "trust_method": 1, "video_id": 1, "chunk_id": 1},
                )
            )

            if not chunks:
                return {
                    "total_chunks": 0,
                    "avg_trust_score": 0,
                    "trust_method_distribution": {},
                    "high_trust_chunks": 0,
                    "low_trust_chunks": 0,
                }

            # Calculate statistics
            trust_scores = [chunk["trust_score"] for chunk in chunks]
            avg_trust_score = sum(trust_scores) / len(trust_scores)

            # Trust method distribution
            trust_methods = defaultdict(int)
            for chunk in chunks:
                method = chunk.get("trust_method", "unknown")
                trust_methods[method] += 1

            # High/low trust chunks
            high_trust_chunks = len([s for s in trust_scores if s >= 0.7])
            low_trust_chunks = len([s for s in trust_scores if s <= 0.3])

            return {
                "total_chunks": len(chunks),
                "avg_trust_score": avg_trust_score,
                "min_trust_score": min(trust_scores),
                "max_trust_score": max(trust_scores),
                "trust_method_distribution": dict(trust_methods),
                "high_trust_chunks": high_trust_chunks,
                "low_trust_chunks": low_trust_chunks,
                "high_trust_percentage": high_trust_chunks / len(chunks) * 100,
                "low_trust_percentage": low_trust_chunks / len(chunks) * 100,
                "generated_at": time.time(),
            }

        except Exception as e:
            logger.error(f"Failed to generate trust statistics: {e}")
            return {
                "total_chunks": 0,
                "avg_trust_score": 0,
                "error": str(e),
                "generated_at": time.time(),
            }


if __name__ == "__main__":
    stage = TrustStage()
    raise SystemExit(stage.run())
