from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional
import argparse

try:
    from src.domain.services.ingestion.transcripts import get_transcript
    from src.core.config.paths import COLL_RAW_VIDEOS
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
    from src.lib.error_handling.decorators import handle_errors
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", "..")))
    from src.domain.services.ingestion.transcripts import get_transcript
    from src.core.config.paths import COLL_RAW_VIDEOS
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig


@dataclass
class BackfillTranscriptConfig(BaseStageConfig):
    languages: List[str] = field(default_factory=lambda: ["en", "en-US", "en-GB"])
    channel_id: Optional[str] = None
    channel_title: Optional[str] = None

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        langs = getattr(args, "languages", None) or ["en", "en-US", "en-GB"]
        channel_id = getattr(args, "channel_id", None)
        channel_title = getattr(args, "channel_title", None)
        return cls(
            **vars(base),
            languages=langs,
            channel_id=channel_id,
            channel_title=channel_title,
        )


class BackfillTranscriptStage(BaseStage):
    name = "backfill_transcript"
    description = "Fill missing transcript_raw for raw_videos or a single video"
    ConfigCls = BackfillTranscriptConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        p.add_argument("--languages", nargs="*", default=["en", "en-US", "en-GB"])
        p.add_argument("--channel_id", type=str)
        p.add_argument("--channel_title", type=str)

    def iter_docs(self) -> List[Dict[str, Any]]:
        # Read from configured read DB/collection (default raw_videos)
        src_db = self.config.read_db_name or self.config.db_name
        coll = self.get_collection(
            self.config.read_coll or COLL_RAW_VIDEOS, io="read", db_name=src_db
        )
        
        # Build query based on different filtering options
        if self.config.video_id:
            # Single video filter (CLI argument)
            q: Dict[str, Any] = {"video_id": self.config.video_id}
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            q = {
                "video_id": {"$in": self.config.input_video_ids},
                "$or": [
                    {"transcript_raw": {"$exists": False}},
                    {"transcript_raw": None},
                    {"transcript_raw": ""},
                ]
            }
            print(f"[backfill_transcript] Filtering to {len(self.config.input_video_ids)} video(s) from source selection")
        elif self.config.channel_id:
            q = {"channel_id": self.config.channel_id}
        elif self.config.channel_title:
            q = {"channel_title": self.config.channel_title}
        else:
            q = {
                "$or": [
                    {"transcript_raw": {"$exists": False}},
                    {"transcript_raw": None},
                    {"transcript_raw": ""},
                ]
            }
        
        docs = list(coll.find(q, {"video_id": 1}))
        print(f"[backfill_transcript] Selected {len(docs)} video(s) for transcript backfill")
        return docs

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, d: Dict[str, Any]) -> None:
        vid = d["video_id"]
        src_db = self.config.read_db_name or self.config.db_name
        dst_db = self.config.write_db_name or self.config.db_name
        read_coll = self.get_collection(
            self.config.read_coll or COLL_RAW_VIDEOS, io="read", db_name=src_db
        )
        write_coll = self.get_collection(
            self.config.write_coll or COLL_RAW_VIDEOS, io="write", db_name=dst_db
        )
        existing = read_coll.find_one({"video_id": vid}, {"transcript_raw": 1})
        if (existing.get("transcript_raw") or "").strip() and not self.config.upsert_existing:
            self.stats["skipped"] += 1
            self.log(f"Skip existing {vid}")
            return
        url = f"https://www.youtube.com/watch?v={vid}"
        self.log(f"Fetching transcript for {vid} (langs={self.config.languages})")
        print(f"Fetching transcript for {vid} (langs={self.config.languages})")
        items = get_transcript(url, languages=self.config.languages)
        if not items:
            self.stats["failed"] += 1
            self.log(f"No transcript found for {vid}")
            print(f"No transcript found for {vid}")
            return
        text = "\n".join(i.get("text", "") for i in items if i.get("text"))
        if not self.config.dry_run:
            write_coll.update_one(
                {"video_id": vid}, {"$set": {"transcript_raw": text}}, upsert=True
            )
        self.stats["updated"] += 1
        self.log(f"Updated {vid} chars={len(text)}")
        print(f"Updated {vid} chars={len(text)}")


if __name__ == "__main__":
    stage = BackfillTranscriptStage()
    raise SystemExit(stage.run())
