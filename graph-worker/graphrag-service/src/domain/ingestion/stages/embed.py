import os
import time
from dataclasses import dataclass
from typing import Any, Dict, List, Optional
import argparse

import requests

try:
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(
        _os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", ".."))
    )
    from src.infrastructure.database.mongodb import get_mongo_client
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig

from src.infrastructure.llm.rate_limit import RateLimiter
from src.core.config.paths import COLL_CHUNKS
VECTOR_DIM = int(os.getenv("ATLAS_EMBEDDING_DIM", os.getenv("EMBEDDING_DIMENSIONS", 1024)))
from src.domain.stages.ingestion.clean import build_embedding_text
from src.lib.error_handling.decorators import handle_errors


def embed_texts(texts: List[str]) -> List[List[float]]:
    api_key = os.getenv("VOYAGE_API_KEY")
    if not api_key:
        raise RuntimeError("VOYAGE_API_KEY is not set")
    model = os.getenv("VOYAGE_EMBED_MODEL", "voyage-2")
    limiter = RateLimiter()
    try:
        import voyageai  # type: ignore

        client = voyageai.Client(
            api_key=api_key,
            max_retries=int(os.getenv("VOYAGE_MAX_RETRIES", "4")),
            timeout=int(os.getenv("VOYAGE_TIMEOUT", "60")),
        )
        limiter.wait()
        res = client.embed(texts, model=model, input_type="document")
        return list(res.embeddings)
    except Exception:
        pass

    max_retries = int(os.getenv("VOYAGE_MAX_RETRIES", "4"))
    backoff_base = float(os.getenv("VOYAGE_BACKOFF_BASE", "1.5"))
    for attempt in range(max_retries + 1):
        try:
            limiter.wait()
            r = requests.post(
                "https://api.voyageai.com/v1/embeddings",
                headers={
                    "Authorization": f"Bearer {api_key}",
                    "Content-Type": "application/json",
                },
                json={"model": model, "input": texts},
                timeout=60,
            )
            if r.status_code in (429, 500, 502, 503, 504):
                retry_after = r.headers.get("Retry-After")
                if retry_after:
                    try:
                        sleep_s = float(retry_after)
                    except Exception:
                        sleep_s = backoff_base**attempt
                else:
                    sleep_s = backoff_base**attempt
                if attempt < max_retries:
                    limiter.delay(max(0.5, min(30.0, sleep_s)))
                    time.sleep(max(0.5, min(30.0, sleep_s)))
                    continue
            r.raise_for_status()
            payload = r.json()
            if "data" in payload:
                return [v.get("embedding", []) for v in payload.get("data", [])]
            if "embeddings" in payload:
                return payload.get("embeddings", [])
            return []
        except requests.HTTPError as e:
            if (
                getattr(e.response, "status_code", 0) in (429, 500, 502, 503, 504)
                and attempt < max_retries
            ):
                time.sleep(max(0.5, min(30.0, backoff_base**attempt)))
                continue
            try:
                code = getattr(e.response, "status_code", None)
                print(f"Voyage embeddings HTTP error {code}; skipping batch")
            except Exception:
                pass
            return []
        except Exception:
            if attempt < max_retries:
                time.sleep(max(0.5, min(15.0, backoff_base**attempt)))
                continue
            return []
    print("Voyage embeddings: exhausted retries; skipping batch")
    return []


@dataclass
class EmbedConfig(BaseStageConfig):
    embed_source: str = "chunk"  # chunk | summary
    # New options
    use_hybrid_embedding_text: bool = True
    unit_normalize_embeddings: bool = True
    emit_multi_vectors: bool = False

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        src = getattr(args, "embed_source", None) or env.get("EMBED_SOURCE", "chunk")
        src = str(src).strip().lower()
        if src not in {"chunk", "summary"}:
            src = "chunk"
        use_hybrid = getattr(args, "use_hybrid_embedding_text", None)
        if use_hybrid is None:
            use_hybrid = env.get("EMBED_USE_HYBRID", "true").lower() in {
                "1",
                "true",
                "yes",
                "on",
            }
        unit_norm = getattr(args, "unit_normalize_embeddings", None)
        if unit_norm is None:
            unit_norm = env.get("EMBED_UNIT_NORMALIZE", "true").lower() in {
                "1",
                "true",
                "yes",
                "on",
            }
        multi_vec = getattr(args, "emit_multi_vectors", None)
        if multi_vec is None:
            multi_vec = env.get("EMBED_MULTI_VECTORS", "false").lower() in {
                "1",
                "true",
                "yes",
                "on",
            }
        return cls(
            **vars(base),
            embed_source=src,
            use_hybrid_embedding_text=bool(use_hybrid),
            unit_normalize_embeddings=bool(unit_norm),
            emit_multi_vectors=bool(multi_vec),
        )


class EmbedStage(BaseStage):
    name = "embed"
    description = "Generate embeddings for existing chunks without embeddings"
    ConfigCls = EmbedConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        p.add_argument("--embed_source", choices=["chunk", "summary"], default="chunk")
        p.add_argument("--use_hybrid_embedding_text", action="store_true")
        p.add_argument(
            "--no_use_hybrid_embedding_text",
            dest="use_hybrid_embedding_text",
            action="store_false",
        )
        p.add_argument("--unit_normalize_embeddings", action="store_true")
        p.add_argument(
            "--no_unit_normalize_embeddings",
            dest="unit_normalize_embeddings",
            action="store_false",
        )
        p.add_argument("--emit_multi_vectors", action="store_true")

    def iter_docs(self) -> List[Dict[str, Any]]:
        q: Dict[str, Any] = {"embedding": {"$exists": False}}
        
        # Apply video filtering from source selection
        if self.config.video_id:
            # Single video filter (CLI argument)
            q["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            q["video_id"] = {"$in": self.config.input_video_ids}
            print(f"[embed] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        src_db = self.config.read_db_name or self.config.db_name
        coll = self.get_collection(
            self.config.read_coll or COLL_CHUNKS, io="read", db_name=src_db
        )
        docs = list(
            coll.find(
                q,
                {
                    "video_id": 1,
                    "chunk_id": 1,
                    "chunk_text": 1,
                    "summary": 1,
                    "relations": 1,
                    "concepts": 1,
                    "context.tags": 1,
                },
            )
        )
        print(f"[embed] Selected {len(docs)} chunk(s) missing embeddings"
        )
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        dst_db = self.config.write_db_name or self.config.db_name
        coll = self.get_collection(
            self.config.write_coll or COLL_CHUNKS, io="write", db_name=dst_db
        )
        video_id = doc.get("video_id")
        chunk_id = doc.get("chunk_id")
        src_field = "chunk_text" if self.config.embed_source == "chunk" else "summary"
        # Build hybrid embedding text if configured
        if self.config.use_hybrid_embedding_text:
            try:
                hybrid = build_embedding_text(doc)
            except Exception:
                hybrid = (doc.get(src_field) or "").strip()
            text = hybrid
        else:
            text = (doc.get(src_field) or "").strip()
        set_embedding_text = text
        if not (video_id and chunk_id and text):
            return
        if not self.config.upsert_existing:
            existing = coll.find_one(
                {"video_id": video_id, "chunk_id": chunk_id}, {"embedding": 1}
            )
            if existing and existing.get("embedding"):
                return
        vecs = embed_texts([text])
        vec = vecs[0] if vecs else []
        if not vec:
            print(f"[embed] Empty embedding for {chunk_id}; skipping")
            return
        # Unit normalize if configured (why do that? would that change the vector signal?)
        if self.config.unit_normalize_embeddings and vec:
            try:
                import math

                norm = math.sqrt(sum(v * v for v in vec)) or 1.0
                vec = [v / norm for v in vec]
                vector_norm = 1.0
            except Exception:
                vector_norm = None
        else:
            try:
                import math

                vector_norm = math.sqrt(sum(v * v for v in vec))
            except Exception:
                vector_norm = None
        coll.update_one(
            {"video_id": video_id, "chunk_id": chunk_id},
            {
                "$set": {
                    "embedding_text": set_embedding_text,
                    "embedding": vec,
                    "embedding_model": os.getenv("VOYAGE_EMBED_MODEL", "voyage-2"),
                    "embedding_dim": VECTOR_DIM,
                    "vector_norm": vector_norm,
                }
            },
            upsert=True,
        )
        self.stats["updated"] += 1
        print(f"[embed] Embedded {chunk_id}")

        # Optional multi-vector embeddings (relations/concepts/tags)
        if self.config.emit_multi_vectors:
            try:
                parts: Dict[str, str] = {}
                rels = doc.get("relations") or []
                if rels:
                    parts["embedding_relations_text"] = ", ".join(
                        [
                            f"{r.get('subject','')} {r.get('predicate','')} {r.get('object','')}".strip()
                            for r in rels
                        ]
                    )
                cons = doc.get("concepts") or []
                if cons:
                    names = [c.get("name", "") for c in cons if c.get("name")]
                    if names:
                        parts["embedding_concepts_text"] = ", ".join(names)
                tags = (doc.get("context") or {}).get("tags") or []
                if tags:
                    parts["embedding_tags_text"] = ", ".join([str(t) for t in tags])

                update_extra: Dict[str, Any] = {}
                for key, txt in parts.items():
                    text_clean = (txt or "").strip()
                    if not text_clean:
                        continue
                    v = embed_texts([text_clean])
                    vv = v[0] if v else []
                    vnorm = None
                    if vv:
                        try:
                            import math

                            if self.config.unit_normalize_embeddings:
                                n = math.sqrt(sum(x * x for x in vv)) or 1.0
                                vv = [x / n for x in vv]
                                vnorm = 1.0
                            else:
                                vnorm = math.sqrt(sum(x * x for x in vv))
                        except Exception:
                            vnorm = None
                        # map key to vector field
                        vec_field = key.replace("_text", "")
                        update_extra[vec_field] = vv
                        update_extra[vec_field + "_dim"] = VECTOR_DIM
                        update_extra[vec_field + "_model"] = os.getenv(
                            "VOYAGE_EMBED_MODEL", "voyage-2"
                        )
                        update_extra[vec_field + "_norm"] = vnorm
                        update_extra[key] = text_clean

                if update_extra:
                    coll.update_one(
                        {"video_id": video_id, "chunk_id": chunk_id},
                        {"$set": update_extra},
                        upsert=True,
                    )
            except Exception:
                pass


if __name__ == "__main__":
    stage = EmbedStage()
    raise SystemExit(stage.run())
