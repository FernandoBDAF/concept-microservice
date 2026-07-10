import os
from typing import Any, Dict, List, Optional
import argparse
from dataclasses import dataclass, field

import re

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

from src.core.config.paths import (
    DB_NAME,
    COLL_CLEANED,
    COLL_CHUNKS,
    COLL_RAW_VIDEOS,
)
from src.core.types.text import normalize_newlines, sha256_text, seconds_to_hhmmss
from src.lib.error_handling.decorators import handle_errors
import uuid
from datetime import datetime, timezone


@dataclass
class ChunkConfig(BaseStageConfig):
    # Production-tuned defaults (from yt_clean_enrich.py)
    chunk_strategy: str = "recursive"  # Recursive for better coherence
    token_size: int = 1200             # Larger tokens for coherent passages
    overlap_pct: float = 0.20          # More overlap for context preservation
    split_chars: List[str] = field(default_factory=lambda: [".", "?", "!"])
    semantic_model: Optional[str] = None

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        strategy = (
            (getattr(args, "chunk_strategy", None) or env.get("CHUNK_STRATEGY", "fixed"))
            .strip()
            .lower()
        )
        token_size = int(getattr(args, "token_size", env.get("TOKEN_SIZE", 500)))
        overlap_pct = float(getattr(args, "overlap_pct", env.get("OVERLAP_PCT", 0.15)))
        split_chars_arg = getattr(args, "split_chars", None) or env.get("SPLIT_CHARS", ".")
        split_chars = [s.strip() for s in str(split_chars_arg).split(",") if s.strip()]
        semantic_model = getattr(args, "semantic_model", None) or env.get("SEMANTIC_MODEL")
        return cls(
            **vars(base),
            chunk_strategy=strategy,
            token_size=token_size,
            overlap_pct=overlap_pct,
            split_chars=split_chars,
            semantic_model=semantic_model,
        )


class ChunkStage(BaseStage):
    name = "chunk"
    description = "Chunk cleaned transcripts (compressed preferred) and upsert to chunks collection"
    ConfigCls = ChunkConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        p.add_argument(
            "--chunk_strategy",
            choices=["fixed", "recursive", "semantic"],
            default="fixed",
        )
        p.add_argument("--token_size", type=int, default=500)
        p.add_argument("--overlap_pct", type=float, default=0.15)
        p.add_argument("--split_chars", type=str, default=".")
        p.add_argument("--semantic_model", type=str)

    def iter_docs(self) -> List[Dict[str, Any]]:
        # Read from configured read collection (default cleaned_transcripts) on read DB
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CLEANED
        coll = self.get_collection(src_coll_name, io="read", db_name=src_db)
        q: Dict[str, Any] = {
            "$or": [
                {"compressed_text": {"$exists": True, "$ne": None}},
                {"cleaned_text": {"$exists": True, "$ne": None}},
            ]
        }
        # Apply video filtering from source selection
        if self.config.video_id:
            # Single video filter (CLI argument)
            q["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            q["video_id"] = {"$in": self.config.input_video_ids}
            print(f"[chunk] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        docs = list(coll.find(q, {"video_id": 1, "compressed_text": 1, "cleaned_text": 1}))
        print(f"[chunk] Selected {len(docs)} cleaned doc(s)")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        # Write to configured write collection (default video_chunks) on write DB
        dst_db = self.config.write_db_name or self.config.db_name
        chunks_coll_name = self.config.write_coll or COLL_CHUNKS
        chunks_coll = self.get_collection(chunks_coll_name, io="write", db_name=dst_db)
        video_id = doc.get("video_id")
        if not video_id:
            return
        source_text = (doc.get("compressed_text") or doc.get("cleaned_text") or "").strip()
        if not source_text:
            return
        # Early-exit: skip reprocessing if chunks already exist and upsert_existing is False
        if not self.config.upsert_existing:
            existing_any = chunks_coll.find_one({"video_id": video_id}, {"_id": 1})
            if existing_any:
                self.logger.info(
                    f"[chunk] Skip existing chunks {video_id} (upsert_existing={self.config.upsert_existing})"
                )
                self.stats["skipped"] += 1
                return
        print(
            f"[chunk] Processing video_id={video_id} strategy={self.config.chunk_strategy} token_size={self.config.token_size} overlap_pct={self.config.overlap_pct}"
        )
        # Build chunks from source_text based on strategy
        chunks_plain: List[str] = []
        try:
            if self.config.chunk_strategy == "fixed":
                from langchain.text_splitter import TokenTextSplitter

                splitter = TokenTextSplitter(
                    chunk_size=int(self.config.token_size),
                    chunk_overlap=int(self.config.token_size * float(self.config.overlap_pct)),
                )
                chunks_plain = splitter.split_text(source_text)
            elif self.config.chunk_strategy == "recursive":
                from langchain.text_splitter import RecursiveCharacterTextSplitter

                # Build proper separator hierarchy with fallbacks
                # Start with paragraph breaks, then sentences, then words
                separators = [
                    "\n\n",    # Paragraph breaks first
                    "\n",      # Line breaks
                    ". ",      # Sentence ends (with space to avoid splitting decimals)
                    "? ",      # Question marks
                    "! ",      # Exclamations
                    "; ",      # Semicolons
                    ", ",      # Commas
                    " ",       # Words (fallback)
                    "",        # Characters (last resort)
                ]
                splitter = RecursiveCharacterTextSplitter(
                    separators=separators,
                    chunk_size=int(self.config.token_size),
                    chunk_overlap=int(self.config.token_size * float(self.config.overlap_pct)),
                    length_function=len,  # Use character count
                )
                chunks_plain = splitter.split_text(source_text)
            elif self.config.chunk_strategy == "semantic":
                from langchain_experimental.text_splitter import SemanticChunker
                from langchain_openai import OpenAIEmbeddings

                model_name = self.config.semantic_model or os.getenv(
                    "OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"
                )
                emb = OpenAIEmbeddings(model=model_name)
                splitter = SemanticChunker(embeddings=emb)
                chunks_plain = splitter.split_text(source_text)
        except Exception as e:
            chunks_plain = [source_text]
            print(f"[chunk] Chunking failed ({e}); using single chunk fallback")

        print(f"[chunk] Produced {len(chunks_plain)} chunk(s) before normalization")
        texts = [normalize_newlines(c or "") for c in chunks_plain]
        cue_re = re.compile(
            r"\[(APPLAUSE|SQUEAKING|RUSTLING|MUSIC|LAUGHTER|NOISE|CLICKING)\]",
            re.IGNORECASE,
        )
        display_texts = [cue_re.sub("", t) for t in texts]

        # Ancillary metadata and video info
        age_days = 180
        channel_id = None
        video_title = None
        channel_name = None
        published_at = None
        video_url = f"https://youtube.com/watch?v={video_id}"
        try:
            from datetime import datetime, timezone

            # Fetch raw video metadata from the default db in this case
            src_db = self.config.db_name
            rv = self.get_collection(COLL_RAW_VIDEOS, io="read", db_name=src_db).find_one(
                {"video_id": video_id},
                {
                    "channel_id": 1,
                    "published_at": 1,
                    "title": 1,
                    "channel_title": 1,
                    "duration_seconds": 1,
                },
            )
            if rv and rv.get("published_at"):
                published = rv.get("published_at")
                if isinstance(published, str):
                    try:
                        published_dt = datetime.fromisoformat(published.replace("Z", "+00:00"))
                    except Exception:
                        published_dt = None
                else:
                    published_dt = published
                if published_dt:
                    now = datetime.now(timezone.utc)
                    age_days = max(0, int((now - published_dt).days))
            if rv and rv.get("channel_id"):
                channel_id = rv.get("channel_id")
            if rv:
                video_title = rv.get("title")
                channel_name = rv.get("channel_title") or rv.get("channel_name")
                published_at = rv.get("published_at")
                duration_seconds = rv.get("duration_seconds") or 0
            else:
                duration_seconds = 0
        except Exception:
            duration_seconds = 0

        # Record chunking parameters
        chunking_info: Dict[str, Any] = {
            "strategy": self.config.chunk_strategy,
            "token_size": int(self.config.token_size),
            "overlap_pct": float(self.config.overlap_pct),
        }
        if self.config.chunk_strategy == "recursive":
            chunking_info["separators"] = "paragraph→line→sentence→word→char"
        elif self.config.chunk_strategy == "semantic":
            chunking_info["split_chars"] = self.config.split_chars or ["."]
        if self.config.chunk_strategy == "semantic":
            chunking_info["semantic_model"] = self.config.semantic_model or os.getenv(
                "OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"
            )

        out_docs: List[Dict[str, Any]] = []
        # Detect markdown code fences anywhere in the chunk
        code_fence = re.compile(r"```", re.MULTILINE)
        total_chars = sum(len(t) for t in texts) or 1
        cum_chars = 0
        total_chunks = len(texts)
        for i, (text, disp) in enumerate(zip(texts, display_texts), start=1):
            chunk_hash = sha256_text(text)
            code_present = bool(code_fence.search(text))
            # Estimate timestamps by proportional chars over duration
            start_ratio = cum_chars / total_chars
            cum_chars += len(text)
            end_ratio = cum_chars / total_chars
            ts_start = seconds_to_hhmmss(start_ratio * duration_seconds)
            ts_end = seconds_to_hhmmss(end_ratio * duration_seconds)
            chunk_uuid = str(uuid.uuid4())
            out_docs.append(
                {
                    "chunk_id": chunk_uuid,
                    "video_id": video_id,
                    "video_title": video_title,
                    "channel_name": channel_name,
                    "video_url": video_url,
                    "published_at": published_at,
                    "timestamp_start": ts_start,
                    "timestamp_end": ts_end,
                    "chunk_text": text,
                    "summary": None,
                    "embedding_text": None,
                    "chunk_hash": chunk_hash,
                    "entities": [],
                    "concepts": [],
                    "relations": [],
                    "temporal_references": [],
                    "numerical_data": [],
                    "visual_cues": [],
                    "cross_links": [],
                    "context": {
                        "speaker": None,
                        "sentiment": None,
                        "tone": None,
                        "language": None,
                        "tags": [],
                    },
                    "provenance": {
                        "source_pipeline_stage": "chunk_stage",
                        "created_at": datetime.now(timezone.utc).isoformat(),
                        "version": "2.0",
                        "model_used": None,
                        "embedding_model": None,
                    },
                    "metadata": {
                        "age_days": age_days,
                        "code_present": code_present,
                        "channel_id": channel_id,
                        "chunking": chunking_info,
                        "chunk_index": i - 1,
                        "chunk_count": total_chunks,
                    },
                }
            )

        if not out_docs:
            print(f"[chunk] No chunks generated for {video_id}")
            return

        # If we are reprocessing an existing video, delete old chunks now that new ones are prepared
        if self.config.upsert_existing:
            try:
                res = chunks_coll.delete_many({"video_id": video_id})
                print(
                    f"[chunk] Removed {getattr(res, 'deleted_count', 0)} existing chunk(s) for video_id={video_id}"
                )
            except Exception:
                print(
                    f"[chunk] Warning: failed to delete existing chunks for video_id={video_id}; proceeding with upserts"
                )

        try:
            from pymongo import UpdateOne

            BATCH = 500
            for i in range(0, len(out_docs), BATCH):
                batch = out_docs[i : i + BATCH]
                print(
                    f"[chunk] Writing batch {i//BATCH + 1} with {len(batch)} chunk(s) to {chunks_coll_name}"
                )
                ops = [
                    UpdateOne(
                        {"video_id": d["video_id"], "chunk_id": d["chunk_id"]},
                        {"$set": d},
                        upsert=True,
                    )
                    for d in batch
                ]
                if ops:
                    chunks_coll.bulk_write(ops, ordered=False)
        except Exception:
            for d in out_docs:
                chunks_coll.update_one(
                    {"video_id": d["video_id"], "chunk_id": d["chunk_id"]},
                    {"$set": d},
                    upsert=True,
                )

        print(
            f"[chunk] Chunked {video_id}: {len(out_docs)} chunks (strategy={self.config.chunk_strategy})"
        )


if __name__ == "__main__":
    stage = ChunkStage()
    raise SystemExit(stage.run())
