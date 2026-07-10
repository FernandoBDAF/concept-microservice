import os
import re
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple
import argparse
from googleapiclient.discovery import build
from googleapiclient.errors import HttpError

# Transcript now handled via LangChain YoutubeLoader in services/transcripts

try:
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", "..")))
    from src.core.base.stage import BaseStage
    from src.core.models.config import BaseStageConfig
from src.core.config.paths import (
    COLL_RAW_VIDEOS,
)
from src.lib.error_handling.decorators import handle_errors


def get_youtube_client() -> Any:
    key = os.getenv("YOUTUBE_API_KEY")
    if not key:
        raise RuntimeError("YOUTUBE_API_KEY is not set")
    return build("youtube", "v3", developerKey=key)


def get_uploads_playlist_id(youtube: Any, channel_id: str) -> Optional[str]:
    # Fetch the uploads playlist for the channel
    resp = youtube.channels().list(part="contentDetails", id=channel_id).execute()
    items = resp.get("items", [])
    if not items:
        return None
    return items[0]["contentDetails"]["relatedPlaylists"].get("uploads")


def list_videos_in_playlist(youtube: Any, playlist_id: str, limit: int) -> List[str]:
    video_ids: List[str] = []
    page_token: Optional[str] = None
    while len(video_ids) < limit:
        try:
            resp = (
                youtube.playlistItems()
                .list(
                    part="contentDetails",
                    playlistId=playlist_id,
                    maxResults=50,
                    pageToken=page_token,
                )
                .execute()
            )
        except HttpError as e:
            print(f"YouTube API error for playlist_id={playlist_id}: {e}")
            break
        except Exception as e:
            print(f"Unexpected error listing playlist items: {e}")
            break
        for it in resp.get("items", []):
            vid = it.get("contentDetails", {}).get("videoId")
            if vid:
                video_ids.append(vid)
                if len(video_ids) >= limit:
                    break
        page_token = resp.get("nextPageToken")
        if not page_token:
            break
    return video_ids


def list_recent_videos_for_channel(youtube: Any, channel_id: str, limit: int) -> List[str]:
    uploads = get_uploads_playlist_id(youtube, channel_id)
    if not uploads:
        return []
    return list_videos_in_playlist(youtube, uploads, limit)


def fetch_video_details(youtube: Any, video_ids: List[str]) -> Dict[str, Dict[str, Any]]:
    details: Dict[str, Dict[str, Any]] = {}
    # Batch in chunks of 50
    for i in range(0, len(video_ids), 50):
        batch = video_ids[i : i + 50]
        resp = (
            youtube.videos()
            .list(
                part="snippet,contentDetails,statistics",
                id=",".join(batch),
                maxResults=50,
            )
            .execute()
        )
        for it in resp.get("items", []):
            vid = it.get("id")
            if not vid:
                continue
            details[vid] = it
    return details


def fetch_playlist_details(youtube: Any, playlist_id: str) -> Dict[str, Any]:
    """Fetch playlist metadata including title and description from YouTube API."""
    try:
        resp = youtube.playlists().list(
            part="snippet",
            id=playlist_id,
            maxResults=1
        ).execute()
        items = resp.get("items", [])
        if items:
            snippet = items[0].get("snippet", {})
            return {
                "playlist_id": playlist_id,
                "playlist_title": snippet.get("title"),
                "playlist_description": snippet.get("description"),
                "playlist_channel_id": snippet.get("channelId"),
                "playlist_channel_title": snippet.get("channelTitle"),
            }
    except HttpError as e:
        print(f"[ingest] YouTube API error fetching playlist details: {e}")
    except Exception as e:
        print(f"[ingest] Error fetching playlist details: {e}")
    # Return minimal info with just the playlist_id on failure
    return {"playlist_id": playlist_id}


_ISO_DURATION_RE = re.compile(r"PT(?:(?P<hours>\d+)H)?(?:(?P<minutes>\d+)M)?(?:(?P<seconds>\d+)S)?")


def parse_iso8601_duration(duration: str) -> Optional[int]:
    if not duration:
        return None
    m = _ISO_DURATION_RE.fullmatch(duration)
    if not m:
        return None
    hours = int(m.group("hours") or 0)
    minutes = int(m.group("minutes") or 0)
    seconds = int(m.group("seconds") or 0)
    return hours * 3600 + minutes * 60 + seconds


def fetch_transcript_text(video_id: str) -> Tuple[Optional[str], Optional[str]]:
    # New implementation delegates to services.transcripts (LangChain)
    try:
        from src.domain.services.ingestion.transcripts import get_transcript

        url = f"https://www.youtube.com/watch?v={video_id}"
        items = get_transcript(url)
        if not items:
            return None, None
        # Concatenate all text segments
        text = "\n".join(seg.get("text", "") for seg in items if seg.get("text"))
        # Language not exposed by loader; return None
        return (text if text.strip() else None), None
    except Exception:
        return None, None


def compute_engagement_score(stats: Dict[str, Any]) -> Optional[float]:
    try:
        views = float(stats.get("viewCount", 0))
        likes = float(stats.get("likeCount", 0))
        comments = float(stats.get("commentCount", 0))
        if views <= 0:
            return None
        return min(1.0, (likes * 2.0 + comments * 3.0) / max(100.0, views))
    except Exception:
        return None


def to_raw_video_doc(
    item: Dict[str, Any],
    transcript_text: Optional[str],
    transcript_lang: Optional[str],
    playlist_info: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    snippet = item.get("snippet", {})
    stats = item.get("statistics", {})
    content = item.get("contentDetails", {})

    published_at = snippet.get("publishedAt")
    title = snippet.get("title")
    description = snippet.get("description")
    channel_id = snippet.get("channelId")
    channel_title = snippet.get("channelTitle")
    tags = snippet.get("tags", []) or []
    thumbnails = (snippet.get("thumbnails", {}) or {}).copy()
    thumb_url = None
    for key in ["maxres", "standard", "high", "medium", "default"]:
        if key in thumbnails and thumbnails[key].get("url"):
            thumb_url = thumbnails[key]["url"]
            break

    doc = {
        "video_id": item.get("id"),
        "title": title,
        "description": description,
        "channel_id": channel_id,
        "channel_title": channel_title,
        "published_at": published_at,
        "duration_seconds": parse_iso8601_duration(content.get("duration")),
        "stats": {
            "viewCount": int(stats.get("viewCount", 0) or 0),
            "likeCount": int(stats.get("likeCount", 0) or 0),
            "commentCount": int(stats.get("commentCount", 0) or 0),
        },
        "keywords": tags,
        "engagement_score": compute_engagement_score(stats),
        "transcript_raw": transcript_text,
        "transcript_language": transcript_lang,
        "thumbnail_url": thumb_url,
        # timezone-aware datetime is stored as BSON date in Mongo
        "fetched_at": datetime.now(timezone.utc),
    }
    # If playlist info was provided (from playlist ingestion), persist it
    if playlist_info:
        doc["playlist_id"] = playlist_info.get("playlist_id")
        doc["playlist_title"] = playlist_info.get("playlist_title")
    return doc


@dataclass
class IngestConfig(BaseStageConfig):
    playlist_id: Optional[str] = None
    channel_id: Optional[str] = None
    video_ids: Optional[List[str]] = None

    @classmethod
    def from_args_env(cls, args: Any, env: Dict[str, str], default_db: Optional[str]):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        vids = getattr(args, "video_ids", None)
        return cls(
            **vars(base),
            playlist_id=getattr(args, "playlist_id", None),
            channel_id=getattr(args, "channel_id", None),
            video_ids=vids,
        )


class IngestStage(BaseStage):
    name = "ingest"
    description = "Ingest YouTube videos → raw_videos"
    ConfigCls = IngestConfig

    def build_parser(self, p: argparse.ArgumentParser) -> None:
        super().build_parser(p)
        g = p.add_mutually_exclusive_group(required=False)
        g.add_argument("--playlist_id", type=str)
        g.add_argument("--channel_id", type=str)
        g.add_argument("--video_ids", nargs="+")

    def setup(self) -> None:
        super().setup()
        self.youtube = get_youtube_client()
        self._details: Dict[str, Dict[str, Any]] = {}
        self._playlist_info: Optional[Dict[str, Any]] = None

    def iter_docs(self) -> List[Dict[str, Any]]:
        # Fetch playlist metadata if ingesting from playlist
        if self.config.playlist_id:
            self._playlist_info = fetch_playlist_details(self.youtube, self.config.playlist_id)
            playlist_title = self._playlist_info.get("playlist_title", "Unknown")
            print(f"[ingest] Playlist: {playlist_title} ({self.config.playlist_id})")

        # Resolve video ids based on config
        if self.config.playlist_id:
            vids = list_videos_in_playlist(
                self.youtube, self.config.playlist_id, int(self.config.max or 100)
            )
        elif self.config.channel_id:
            vids = list_recent_videos_for_channel(
                self.youtube, self.config.channel_id, int(self.config.max or 100)
            )
        elif self.config.video_ids is not None:
            vids = self.config.video_ids or []
            if self.config.max:
                vids = vids[: int(self.config.max)]
        else:
            # Fallback: process entries in raw_videos where channel_id is null (read DB)
            coll = self.get_collection(self.config.read_coll or COLL_RAW_VIDEOS, io="read")
            vids = [
                d.get("video_id")
                for d in coll.find({"channel_id": {"$in": [None, ""]}}, {"video_id": 1})
            ]
            if self.config.max:
                vids = vids[: int(self.config.max)]
            print(f"[ingest] Fallback mode: found {len(vids)} video(s) with missing channel_id")
        if not vids:
            print("[ingest] No videos to ingest.")
            return []
        # Skip videos that already exist when not upserting to save API calls
        if not self.config.upsert_existing:
            coll = self.get_collection(self.config.read_coll or COLL_RAW_VIDEOS, io="read")
            existing = {
                d.get("video_id") for d in coll.find({"video_id": {"$in": vids}}, {"video_id": 1})
            }
            if existing:
                before = len(vids)
                vids = [v for v in vids if v not in existing]
                print(
                    f"[ingest] Skipping {len(existing)} existing video(s); {len(vids)}/{before} remain"
                )
            if not vids:
                return []
        # Pre-fetch details in batch for efficiency; single batch fetch for all remaining vids
        details = fetch_video_details(self.youtube, vids)
        # Retry once for any missing IDs (partial API response)
        missing = [v for v in vids if v not in details]
        if missing:
            retry = fetch_video_details(self.youtube, missing)
            if retry:
                details.update(retry)
                print(f"[ingest] Retried {len(missing)}; recovered {len(retry)} detail(s)")
        self._details = details
        print(f"[ingest] Collected {len(vids)} video id(s); details for {len(self._details)}")
        return [{"video_id": v} for v in vids]

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> None:
        vid = doc.get("video_id")
        if not vid:
            return
        # Collections on write DB for persistence
        coll = self.get_collection(self.config.write_coll or COLL_RAW_VIDEOS, io="write")
        # Skip existing unless upsert
        if not self.config.upsert_existing:
            if coll.find_one({"video_id": vid}):
                print(f"[ingest] Skip existing {vid}")
                self.stats["skipped"] += 1
                return
        item = self._details.get(vid)
        coll = self.get_collection(self.config.write_coll or COLL_RAW_VIDEOS, io="write")
        if not item:
            # Create a minimal entry with channel_id=None
            try:
                coll.update_one(
                    {"video_id": vid},
                    {
                        "$setOnInsert": {"video_id": vid},
                        "$set": {
                            "channel_id": None,
                            "fetched_at": datetime.now(timezone.utc),
                        },
                    },
                    upsert=True,
                )
                self.stats["updated"] += 1
                print(f"[ingest] Created minimal stub for {vid} (no details)")
            except Exception as e:
                self.stats["failed"] += 1
                print(f"[ingest] Error creating stub for {vid}: {e}")
            return
        try:

            transcript_text, transcript_lang = fetch_transcript_text(vid)
        except Exception as e:
            print(f"[ingest] Error fetching transcript for {vid}: {e}")
            self.stats["transcript_failed"] += 1
            transcript_text, transcript_lang = None, None
        raw_doc = to_raw_video_doc(item, transcript_text, transcript_lang, self._playlist_info)
        try:
            coll.update_one({"video_id": raw_doc["video_id"]}, {"$set": raw_doc}, upsert=True)
            self.stats["updated"] += 1
            print(f"[ingest] Upserted {vid}: title='{(raw_doc.get('title') or '')[:60]}'")
        except Exception as e:
            self.stats["failed"] += 1
            print(f"[ingest] Error upserting {vid}: {e}")


if __name__ == "__main__":
    stage = IngestStage()
    raise SystemExit(stage.run())
