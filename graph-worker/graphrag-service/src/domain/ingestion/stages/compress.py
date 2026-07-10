import os
from dataclasses import dataclass
from typing import Any, Dict, List, Optional
import argparse

try:
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
    from src.core.types.compression import compress_text, postprocess_compressed_text
    from src.core.config.paths import COLL_CLEANED, COLL_ENRICHED
    from src.lib.error_handling.decorators import handle_errors
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", "..")))
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
    from src.core.types.compression import compress_text, postprocess_compressed_text
    from src.core.config.paths import COLL_CLEANED, COLL_ENRICHED


@dataclass
class CompressConfig(BaseStageConfig):
    # Max tokens to keep after compression (higher = longer outputs, lower = tighter summaries)
    target_tokens: int = 1200
    # Compression aggressiveness for LLMLingua (0.0–1.0). Larger values remove more content
    ratio: float = 0.4
    # How to reorder retained context ("sort" ranks by importance; "original" preserves order)
    reorder: str = "sort"
    # LLMLingua model identifier. Multilingual LLMLingua2 is default; switch if you need a different encoder
    model: str = "microsoft/llmlingua-2-bert-base-multilingual-cased-meetingbank"
    # Which collection field to compress: cleaned_text or concatenated enriched segments
    source: str = "cleaned"  # cleaned|enriched
    # Optional post-clean normalization and denoising
    strict_cleanup: bool = False

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        tt = int(getattr(args, "target_tokens", env.get("COMPRESS_TARGET_TOKENS", 1200)))
        ratio = float(getattr(args, "ratio", env.get("COMPRESS_RATIO", 0.4)))
        reorder = getattr(args, "reorder_context", env.get("COMPRESS_REORDER", "sort"))
        # Prefer CLI value if provided, else fall back to env, else default
        model = getattr(args, "model", None) or env.get(
            "COMPRESS_MODEL",
            "microsoft/llmlingua-2-bert-base-multilingual-cased-meetingbank",
        )
        source = getattr(args, "source", env.get("COMPRESS_SOURCE", "cleaned")).strip().lower()
        if source not in ("cleaned", "enriched"):
            source = "cleaned"
        strict_cleanup = bool(
            getattr(args, "strict_cleanup", False)
            or (env.get("COMPRESS_STRICT_CLEANUP", "false").lower() in {"1", "true", "yes", "on"})
        )
        return cls(
            **vars(base),
            target_tokens=tt,
            ratio=ratio,
            reorder=reorder,
            model=model,
            source=source,
            strict_cleanup=strict_cleanup,
        )


class CompressStage(BaseStage):
    name = "compress"
    description = "Compress cleaned transcripts using LLMLingua to remove noise"
    ConfigCls = CompressConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        # Max tokens to keep post-compression (approximate)
        p.add_argument("--target_tokens", type=int, default=1200)
        # Higher ratio → stronger compression (more dropped content)
        p.add_argument("--ratio", type=float, default=0.4)
        # "sort" for importance-ranked order; "original" for narrative continuity
        p.add_argument("--reorder_context", type=str, default="sort")
        # LLMLingua model name to use (see README for options)
        p.add_argument("--model", type=str)
        p.add_argument("--source", type=str, choices=["cleaned", "enriched"], default="cleaned")
        p.add_argument(
            "--strict_cleanup",
            action="store_true",
            help="Apply extra punctuation and lecture-label cleanup after compression",
        )

    def iter_docs(self) -> List[Dict[str, Any]]:
        # Helper function to apply video filtering
        def apply_video_filter(q: Dict[str, Any]) -> None:
            if self.config.video_id:
                # Single video filter (CLI argument)
                q["video_id"] = self.config.video_id
            elif self.config.input_video_ids:
                # Multiple video filter (source selection filter)
                q["video_id"] = {"$in": self.config.input_video_ids}
                print(f"[compress] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        
        if self.config.source == "enriched":
            src_db = self.config.read_db_name or self.config.db_name
            coll = self.get_collection(
                self.config.read_coll or COLL_ENRICHED, io="read", db_name=src_db
            )
            q: Dict[str, Any] = {"cleaned_text": {"$exists": True, "$ne": []}}
            apply_video_filter(q)
            docs = list(coll.find(q, {"video_id": 1, "cleaned_text": 1}))
            print(f"[compress] Selected {len(docs)} doc(s)")
            return docs
        
        src_db = self.config.read_db_name or self.config.db_name
        coll = self.get_collection(self.config.read_coll or COLL_CLEANED, io="read", db_name=src_db)
        q = {"cleaned_text": {"$exists": True, "$ne": None}}
        apply_video_filter(q)
        docs = list(coll.find(q, {"video_id": 1, "cleaned_text": 1}))
        print(f"[compress] Selected {len(docs)} doc(s)")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        video_id = doc.get("video_id")
        if not video_id:
            return
        print(
            f"[compress] Processing video_id={video_id} source={self.config.source} target_tokens={self.config.target_tokens} ratio={self.config.ratio} reorder={self.config.reorder} model={self.config.model}"
        )
        if self.config.source == "enriched":
            source_text = "\n".join([s.get("text", "") for s in doc.get("segments", [])])
        else:
            source_text = doc.get("cleaned_text", "")
        source_text = (source_text or "").strip()
        if not source_text:
            print(f"[compress] Skip empty source text for {video_id}")
            return
        print(f"[compress] Compressing text (len={len(source_text)}) with LLMLingua...")
        try:
            res = compress_text(
                source_text,
                target_tokens=int(self.config.target_tokens),
                ratio=float(self.config.ratio),
                reorder=self.config.reorder,
                model_name=self.config.model,
            )
        except Exception as e:
            print(f"[compress] Compression failed for {video_id}: {e}")
            self.stats["failed"] += 1
            return
        compressed_text = res.get("compressed_text", "")
        meta = res.get("compression_meta", {})
        if not compressed_text:
            print(f"[compress] No compressed_text produced for {video_id}")
            return
        if self.config.strict_cleanup:
            before_len = len(compressed_text)
            compressed_text = postprocess_compressed_text(compressed_text, strict=True)
            print(
                f"[compress] Postprocessed compressed_text (len {before_len} -> {len(compressed_text)})"
            )
        print(
            f"[compress] Compressed length={len(compressed_text)} (delta={len(source_text)-len(compressed_text)})"
        )
        dst_db = self.config.write_db_name or self.config.db_name
        if self.config.source == "enriched":
            coll = self.get_collection(
                self.config.write_coll or COLL_ENRICHED, io="write", db_name=dst_db
            )
        else:
            coll = self.get_collection(
                self.config.write_coll or COLL_CLEANED, io="write", db_name=dst_db
            )
        if not self.config.upsert_existing:
            if coll.find_one({"video_id": video_id, "compressed_text": {"$exists": True}}):
                print(f"[compress] Skip existing compressed_text for {video_id}")
                return
        if not self.config.dry_run:
            coll.update_one(
                {"video_id": video_id},
                {
                    "$set": {
                        "compressed_text": compressed_text,
                        "compression_meta": meta,
                    }
                },
                upsert=True,
            )
        self.stats["updated"] += 1
        print(f"[compress] Saved compressed_text for {video_id} (len={len(compressed_text)})")


if __name__ == "__main__":
    stage = CompressStage()
    raise SystemExit(stage.run())
