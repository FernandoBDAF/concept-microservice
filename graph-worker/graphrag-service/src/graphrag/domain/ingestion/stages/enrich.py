import os
from dataclasses import dataclass
from typing import Any, Dict, List, Tuple, Optional
import argparse

from dotenv import load_dotenv
from pymongo import MongoClient

try:
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig

    # Ensure these are available in the normal import path as well
    from src.core.types.text import normalize_newlines
    from src.lib.concurrency import (
        run_llm_concurrent,
    )  # Migrated to core library
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", "..")))
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
    from src.core.types.text import normalize_newlines
    from src.lib.concurrency import (
        run_llm_concurrent,
    )  # Migrated to core library
from src.core.config.paths import (
    DB_NAME,
    COLL_CLEANED,
    COLL_ENRICHED,
    COLL_RAW_VIDEOS,
    COLL_CHUNKS,
)


from src.core.types.enrichment import normalize_enrich_payload_for_chunk
from src.domain.stages.ingestion.clean import build_embedding_text
from src.lib.error_handling.decorators import handle_errors


"""LLM-only enrichment stage: no heuristic fallback paths.

Future Enhancements for Video-Level Tagging:
- Currently chunks inherit context.tags from chunk-level LLM extraction
- For improved filtering and topic discovery:
  1. Extract video-level topics from raw_videos metadata or full transcript summary
  2. Backfill metadata.tags to all chunks belonging to that video
  3. Update chunk creation pipeline to propagate video tags automatically
- This would enable better planner filter selection (e.g., "machine learning", "RAG", "embeddings")
  instead of relying on granular chunk-level tags that may miss the big picture
"""


@dataclass
class EnrichConfig(BaseStageConfig):
    # Production-tuned defaults (from yt_clean_enrich.py)
    use_llm: bool = True               # Always use LLM for quality
    llm_retries: int = 4               # Higher retries for reliability
    llm_backoff_s: float = 10.0        # Longer backoff for stability
    llm_qps: Optional[float] = None
    model_name: Optional[str] = None

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        # Map ENRICH_MAX into base.max if no CLI max given
        if getattr(base, "max", None) is None:
            m = env.get("ENRICH_MAX")
            if (m or "").strip().isdigit():
                base.max = int(m)
        # Accept CLI overrides (fallback to env)
        retries_cli = getattr(args, "llm_retries", None)
        backoff_cli = getattr(args, "llm_backoff_s", None)
        qps_cli = getattr(args, "llm_qps", None)
        model_cli = getattr(args, "model_name", None)
        llm_retries = int(
            retries_cli if retries_cli is not None else (env.get("LLM_RETRIES", "4") or 4)
        )
        llm_backoff_s = float(
            backoff_cli if backoff_cli is not None else (env.get("LLM_BACKOFF_S", "10.0") or 10.0)
        )
        llm_qps_env = env.get("LLM_QPS")
        llm_qps = (
            float(qps_cli) if qps_cli is not None else (float(llm_qps_env) if llm_qps_env else None)
        )
        model_name = model_cli or env.get("OPENAI_MODEL")
        
        # Set production default for concurrency if not provided
        if base.concurrency is None:
            base.concurrency = 15
        
        return cls(
            **vars(base),
            use_llm=True,
            llm_retries=llm_retries,
            llm_backoff_s=llm_backoff_s,
            llm_qps=llm_qps,
            model_name=model_name,
        )


class EnrichStage(BaseStage):
    name = "enrich"
    description = "Enrich cleaned transcripts into segments with tags and metadata"
    ConfigCls = EnrichConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        # Optional LLM tuning flags
        p.add_argument("--llm_retries", type=int)
        p.add_argument("--llm_backoff_s", type=float)
        p.add_argument("--llm_qps", type=float)
        p.add_argument("--model_name", type=str)

    def iter_docs(self) -> List[Dict[str, Any]]:
        # When upserting existing, reprocess all chunks for the selection (ignore summary filter)
        if self.config.upsert_existing:
            q: Dict[str, Any] = {}
        else:
            # Only select chunks that have not been enriched yet
            q = {
                "$or": [
                    {"summary": {"$exists": False}},
                    {"summary": None},
                    {"summary": ""},
                ]
            }
        
        # Apply video filtering from source selection
        if self.config.video_id:
            # Single video filter (CLI argument)
            q["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            q["video_id"] = {"$in": self.config.input_video_ids}
            print(f"[enrich] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        # Read from configured read collection (default video_chunks) on read DB
        src_db = self.config.read_db_name or self.config.db_name
        coll_name = self.config.read_coll or COLL_CHUNKS
        coll = self.get_collection(coll_name, io="read", db_name=src_db)
        docs = list(coll.find(q, {"chunk_id": 1, "video_id": 1, "chunk_text": 1}))
        print(f"[enrich] Selected {len(docs)} chunk(s) to enrich")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        # Write to configured write collection (default video_chunks) on write DB
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CHUNKS
        chunks = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        video_id = doc.get("video_id")
        chunk_id = doc.get("chunk_id")
        text = (doc.get("chunk_text") or "").strip()
        if not video_id or not chunk_id:
            raise RuntimeError("Missing identifiers for chunk enrichment")
        if not text:
            raise RuntimeError(f"No chunk_text for chunk_id={chunk_id}")
        print(f"[enrich] Start video_id={video_id} chunk_id={chunk_id} text_len={len(text)}")
        source = "llm"
        text = normalize_newlines(text)
        # LLM-only enrichment (single call)
        try:
            from src.domain.agents.ingestion.enrich import EnrichmentAgent

            print(f"[enrich] Calling structured annotation for chunk_id={chunk_id}")
            agent = EnrichmentAgent()
            structured = agent.annotate_chunk_structured(text)
        except Exception as e:
            print(f"Enrich LLM error video_id={video_id}: {e}")
            raise

        payload = normalize_enrich_payload_for_chunk(structured)

        # Discretize confidences (high/medium/low) alongside raw values
        def _disc(x: float) -> str:
            try:
                v = float(x)
            except Exception:
                return "low"
            if v >= 0.8:
                return "high"
            if v >= 0.5:
                return "medium"
            return "low"

        try:
            for e in payload.get("entities", []) or []:
                e["relevance_level"] = _disc(e.get("relevance", 0.0))
            for c in payload.get("concepts", []) or []:
                c["confidence_level"] = _disc(c.get("confidence", 0.0))
            for r in payload.get("relations", []) or []:
                r["confidence_level"] = _disc(r.get("confidence", 0.0))
            # quality_score (weighted mean)
            ents = [float(e.get("relevance", 0.0) or 0.0) for e in payload.get("entities", [])]
            cons = [float(c.get("confidence", 0.0) or 0.0) for c in payload.get("concepts", [])]
            rels = [float(r.get("confidence", 0.0) or 0.0) for r in payload.get("relations", [])]

            def _avg(xs):
                return (sum(xs) / len(xs)) if xs else None

            avg_e = _avg(ents)
            avg_c = _avg(cons)
            avg_r = _avg(rels)
            w_e, w_r, w_c = 0.4, 0.4, 0.2
            parts = []
            if avg_e is not None:
                parts.append((avg_e, w_e))
            if avg_r is not None:
                parts.append((avg_r, w_r))
            if avg_c is not None:
                parts.append((avg_c, w_c))
            if parts:
                numer = sum(v * w for v, w in parts)
                denom = sum(w for _, w in parts) or 1.0
                payload["quality_score"] = max(0.0, min(1.0, numer / denom))
            else:
                payload["quality_score"] = None
        except Exception:
            pass
        # provenance and model
        payload.setdefault("provenance", {})
        prov = payload["provenance"] if isinstance(payload["provenance"], dict) else {}
        prov.update(
            {
                "source_pipeline_stage": "enrich_agent_v2",
                "version": "2.0",
                "model_used": os.getenv("OPENAI_DEFAULT_MODEL") or "gpt-4o-mini",
            }
        )
        try:
            from datetime import datetime, timezone

            if not prov.get("created_at"):
                prov["created_at"] = datetime.now(timezone.utc).isoformat()
        except Exception:
            pass
        payload["provenance"] = prov
        print(f"[enrich] Upserting structured fields for chunk_id={chunk_id} into {dst_coll_name}")
        # Always produce embedding_text from enriched content
        try:
            payload["embedding_text"] = build_embedding_text(
                {
                    "summary": payload.get("summary", ""),
                    "entities": payload.get("entities", []),
                    "concepts": payload.get("concepts", []),
                    "chunk_text": text,
                }
            )
        except Exception:
            payload["embedding_text"] = text

        chunks.update_one({"chunk_id": chunk_id}, {"$set": payload}, upsert=True)
        self.stats["updated"] += 1
        print(f"Enriched chunk {chunk_id} for video_id={video_id}")

    def run(self, config: Optional[BaseStageConfig] = None) -> int:
        # Override to enable LLM concurrency across chunks
        if config is None:
            self.parse_args()
            self.config = self.ConfigCls.from_args_env(self.args, dict(os.environ), DB_NAME)
        else:
            self.config = config
        self.setup()

        try:
            docs = list(self.iter_docs())
            if self.config.max is not None:
                docs = docs[: int(self.config.max)]
            total = len(docs)
            if total == 0:
                print("[enrich] Nothing to process")
                self.finalize()
                return 0

            texts = [normalize_newlines((d.get("chunk_text") or "").strip()) for d in docs]

            print(
                f"[enrich] Launching concurrent LLM calls for {total} chunk(s) (workers={int(self.config.concurrency or 8)})"
            )

            from src.domain.agents.ingestion.enrich import EnrichmentAgent

            results = run_llm_concurrent(
                texts,
                agent_factory=lambda: EnrichmentAgent(),
                method_name="annotate_chunk_structured",
                max_workers=int(self.config.concurrency or 8),
                retries=int(self.config.llm_retries or 4),
                backoff_s=float(self.config.llm_backoff_s or 5.0),
                qps=self.config.llm_qps,
                jitter=False,
                on_error=lambda e, t: {},
                preserve_order=True,
            )

            chunks_coll_name = self.config.write_coll or COLL_CHUNKS
            dst_db = self.config.write_db_name or self.config.db_name
            chunks_coll = self.get_collection(chunks_coll_name, io="write", db_name=dst_db)
            for idx, (doc, structured) in enumerate(zip(docs, results), start=1):
                video_id = doc.get("video_id")
                chunk_id = doc.get("chunk_id")
                if not (video_id and chunk_id):
                    print(f"[enrich] Skip invalid doc at index {idx}")
                    self.stats["failed"] += 1
                    continue
                payload = normalize_enrich_payload_for_chunk(structured or {})
                payload.setdefault("provenance", {})
                prov = payload["provenance"] if isinstance(payload["provenance"], dict) else {}
                prov.update(
                    {
                        "source_pipeline_stage": "enrich_agent_v2",
                        "version": "2.0",
                        "model_used": os.getenv("BEDROCK_MODEL_ID")
                        or os.getenv("OPENAI_DEFAULT_MODEL")
                        or "gpt-4o-mini",
                    }
                )
                try:
                    from datetime import datetime, timezone

                    if not prov.get("created_at"):
                        prov["created_at"] = datetime.now(timezone.utc).isoformat()
                except Exception:
                    pass
                payload["provenance"] = prov

                print(
                    f"[enrich] Upserting {idx}/{total} chunk_id={chunk_id} into {chunks_coll_name}"
                )
                chunks_coll.update_one({"chunk_id": chunk_id}, {"$set": payload}, upsert=True)
                self.stats["updated"] += 1

            self.finalize()
            return 0
        except Exception as e:
            print(f"[enrich] Fatal error: {e}")
            return 1


if __name__ == "__main__":
    stage = EnrichStage()
    raise SystemExit(stage.run())
