import os
from typing import Any, Dict, List, Optional
from dataclasses import dataclass
import argparse
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
from src.core.config.paths import DB_NAME, COLL_RAW_VIDEOS, COLL_CLEANED
from src.core.types.text import normalize_newlines
from src.lib.concurrency import run_llm_concurrent  # Migrated to core library
from src.lib.error_handling.decorators import handle_errors


def _normalize_text(text: str) -> str:
    # Normalize line endings and collapse excessive whitespace
    t = (text or "").replace("\r\n", " ").replace("\r", " ").replace("\n", " ")
    return t


def _split_units(text: str) -> List[str]:
    """Split text into logical units without assuming double newlines.

    Priority:
      1) blank-line separated paragraphs (\n\n)
      2) single newlines
      3) sentence boundaries
      4) whole text
    """
    t = _normalize_text(text)
    if re.search(r"\n{2,}", t):
        return [p.strip() for p in re.split(r"\n{2,}", t) if p.strip()]
    if "." in t:
        parts = [p.strip() for p in t.split(".") if p.strip()]
        buf: List[str] = []
        acc = ""
        for p in parts:
            if len(acc) + len(p) + 1 <= 5000:
                acc = (acc + ". " + p) if acc else p
            else:
                if acc:
                    buf.append(acc)
                acc = p
        if acc:
            buf.append(acc)
        return buf
    # Sentence split
    sentences = [s.strip() for s in re.split(r"(?<=[.!?])\s+", t) if s.strip()]
    if sentences:
        return sentences
    return [t] if t else []


def _llm_clean_text(
    agent_factory,
    video_id: str,
    raw_text: str,
    max_workers: int,
    retries: int = 1,
    backoff_s: float = 0.5,
    qps: Optional[float] = None,
    logger=None,
) -> dict:
    # Remove common single-word interjections/fillers before splitting
    fillers_re = re.compile(
        r"\b(OK|Okay|Yeah|Yep|Nope|Morning|Right|So|Well|Um|Uh|Mm|Mm-hm|Mhm|Like)\b[,.!?]*\s+",
        re.IGNORECASE,
    )
    preprocessed = fillers_re.sub("", raw_text or "")
    chunks = _split_units(preprocessed)
    if not chunks:
        return {"video_id": video_id, "cleaned_text": "", "paragraphs": []}

    # Log progress for large operations
    num_chunks = len(chunks)
    if logger:
        if num_chunks > 5:
            logger.info(
                f"[clean] Processing {video_id}: {num_chunks} chunks "
                f"with {max_workers} workers (this may take a while)..."
            )
        else:
            logger.debug(f"[clean] Processing {video_id}: {num_chunks} chunks")

    def _on_error(e, ch):
        return ch

    import time

    llm_start = time.time()
    cleaned_parts = run_llm_concurrent(
        chunks,
        agent_factory,
        "clean",
        max_workers=max_workers,
        retries=int(retries),
        backoff_s=float(backoff_s),
        qps=qps,
        jitter=True,
        on_error=_on_error,
        preserve_order=True,
    )
    llm_elapsed = time.time() - llm_start
    if logger:
        if num_chunks > 5:
            logger.info(
                f"[clean] Completed LLM calls for {video_id}: "
                f"{num_chunks} chunks in {llm_elapsed:.1f}s "
                f"(avg {llm_elapsed/num_chunks:.2f}s/chunk)"
            )
        else:
            logger.debug(
                f"[clean] Completed LLM calls for {video_id}: "
                f"{num_chunks} chunks in {llm_elapsed:.1f}s"
            )
    cleaned_chunks: List[str] = []
    for i, out in enumerate(cleaned_parts, start=1):
        out = out or ""
        if not out.strip():
            out = chunks[i - 1]
        cleaned_chunks.append(normalize_newlines(out))
        # Chunk cleaning progress logged via handle_doc

    post_processed_chunks = []
    # Post-processing: strip stage cues and artifacts, standardize dashes/whitespace
    for chunck in cleaned_chunks:
        cleaned = normalize_newlines(chunck)
        cleaned = re.sub(
            r"\[(APPLAUSE|SQUEAKING|RUSTLING|MUSIC|LAUGHTER|NOISE|CLICKING)\]",
            "",
            cleaned,
            flags=re.IGNORECASE,
        )
        cleaned = re.sub(r"-{2,}", " — ", cleaned)
        cleaned = re.sub(r"\s{2,}", " ", cleaned)
        post_processed_chunks.append(cleaned)

    # Enforce paragraphization: 6–10 sentences per paragraph (flexible)
    sentences = []
    for blk in post_processed_chunks:
        sentences.extend([s for s in re.split(r"(?<=[.!?])\s+", blk) if s.strip()])
    target_min, target_max = 6, 10
    paragraphs: List[str] = []
    buf: List[str] = []
    for s in sentences:
        buf.append(s.strip())
        if len(buf) >= target_max:
            paragraphs.append(" ".join(buf))
            buf = []
    if buf:
        # last paragraph: allow shorter if needed
        if paragraphs and len(buf) < target_min:
            paragraphs[-1] = paragraphs[-1] + " " + " ".join(buf)
        else:
            paragraphs.append(" ".join(buf))
    cleaned_text = "\n\n".join(paragraphs)
    return {"video_id": video_id, "cleaned_text": cleaned_text}


def build_embedding_text(chunk: Dict[str, Any]) -> str:
    summary = chunk.get("summary", "")
    entities = ", ".join([e["name"] for e in chunk.get("entities", [])[:3]])
    concepts = ", ".join([c["name"] for c in chunk.get("concepts", [])[:3]])
    base_text = chunk.get("chunk_text", "")

    # Concatenate weighted signals
    return f"Summary: {summary}\nKey Entities: {entities}\nKey Concepts: {concepts}\n\nMain Content:\n{base_text}"


@dataclass
class CleanConfig(BaseStageConfig):
    # Production-tuned defaults (from yt_clean_enrich.py)
    use_llm: bool = True              # Always use LLM for quality
    llm_retries: int = 4               # Higher retries for reliability
    llm_backoff_s: float = 10.0        # Longer backoff for stability
    llm_qps: Optional[float] = None
    model_name: Optional[str] = None
    prompt_id: Optional[str] = None   # Dynamic prompt selection from registry

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        use_llm = bool(getattr(args, "llm", True) or (env.get("CLEAN_WITH_LLM") == "1"))
        
        # Set production default for concurrency if not provided
        if base.concurrency is None:
            base.concurrency = 15
        
        return cls(
            **vars(base),
            use_llm=use_llm,
            llm_retries=int(env.get("LLM_RETRIES", "4") or "4"),
            llm_backoff_s=float(env.get("LLM_BACKOFF_S", "10.0") or "10.0"),
            llm_qps=None,
            model_name=env.get("OPENAI_DEFAULT_MODEL"),
        )


class CleanStage(BaseStage):
    name = "clean"
    description = "Clean transcripts into standardized text and paragraphs"
    ConfigCls = CleanConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)

    def iter_docs(self) -> List[Dict[str, Any]]:
        # Read from the default DB by default; allow explicit override via read_db_name/read_coll
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_RAW_VIDEOS
        coll = self.get_collection(src_coll_name, io="read", db_name=src_db)
        
        # Build query with video filtering
        query: Dict[str, Any] = {}
        if self.config.video_id:
            # Single video filter (CLI argument)
            query["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            query["video_id"] = {"$in": self.config.input_video_ids}
            print(f"[clean] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        docs = list(coll.find(query, {"video_id": 1, "transcript_raw": 1}))
        print(f"[clean] Selected {len(docs)} document(s) to process")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        video_id = doc.get("video_id")
        text = (doc.get("transcript_raw") or "").strip()
        if not video_id or not text:
            return
        # Write to configured write collection (default cleaned_transcripts) on write DB
        # Write to default DB unless write_db_name provided
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CLEANED
        cleaned = self.get_collection(dst_coll_name, io="write", db_name=dst_db)
        if not self.config.upsert_existing:
            existing = cleaned.find_one({"video_id": video_id}, {"cleaned_text": 1})
            if existing and (existing.get("cleaned_text") or "").strip():
                self.stats["skipped"] += 1
                self.log(f"Skip existing cleaned {video_id}")
                return

        # Log start of cleaning operation with LLM
        import time

        start_time = time.time()
        from src.domain.agents.ingestion.clean import TranscriptCleanAgent

        # Estimate chunks for progress logging
        text_len = len(text)
        estimated_chunks = max(1, text_len // 8000)  # Rough estimate based on typical chunk size
        self.logger.info(
            f"[clean] Starting LLM cleaning for {video_id} "
            f"(text_len={text_len}, est_chunks={estimated_chunks}, "
            f"workers={self.config.concurrency or 10}, "
            f"prompt_id={self.config.prompt_id or 'default'})"
        )

        # Create agent factory with model and prompt configuration
        agent_factory = lambda: TranscriptCleanAgent(
            model_name=self.config.model_name,
            prompt_id=self.config.prompt_id  # Pass prompt_id for dynamic selection
        )
        payload = _llm_clean_text(
            agent_factory,
            video_id,
            text,
            max_workers=int(self.config.concurrency or 10),
            retries=int(self.config.llm_retries or 1),
            backoff_s=float(self.config.llm_backoff_s or 0.5),
            qps=self.config.llm_qps,
            logger=self.logger,
        )
        # Fallback: if cleaning produced nothing, persist the raw text so downstream stages can proceed.
        if not payload or not (payload.get("cleaned_text") or "").strip():
            payload = {"video_id": video_id, "cleaned_text": text, "paragraphs": [text]}

        elapsed = time.time() - start_time

        if payload.get("cleaned_text") and not (payload.get("cleaned_text") or "").strip():
            self.log(f"No cleaned text for {video_id}")
            return
        if not self.config.dry_run:
            cleaned.update_one({"video_id": video_id}, {"$set": payload}, upsert=True)
        self.stats["updated"] += 1
        self.logger.info(
            f"[clean] Completed {video_id} → {dst_coll_name} "
            f"(llm={self.config.use_llm}, time={elapsed:.1f}s)"
        )


if __name__ == "__main__":
    stage = CleanStage()
    raise SystemExit(stage.run())
