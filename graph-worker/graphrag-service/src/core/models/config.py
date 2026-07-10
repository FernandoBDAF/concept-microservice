from dataclasses import dataclass, field
from typing import Optional, Dict, Any, List


@dataclass
class BaseStageConfig:
    max: Optional[int] = None
    llm: bool = False
    verbose: bool = False
    dry_run: bool = False
    db_name: Optional[str] = None
    # Optional IO overrides (read/write DB and collection names)
    read_db_name: Optional[str] = None
    write_db_name: Optional[str] = None
    read_coll: Optional[str] = None
    write_coll: Optional[str] = None
    upsert_existing: bool = False
    video_id: Optional[str] = None
    # List of video IDs to filter by (from source selection filter)
    # When set, stages should only process documents matching these video_ids
    input_video_ids: Optional[List[str]] = field(default=None)
    concurrency: Optional[int] = 15    # Production default for LLM stages
    # Trace ID for linking transformations across pipeline run
    trace_id: Optional[str] = None

    @classmethod
    def from_args_env(
        cls,
        args: Any,
        env: Dict[str, str],
        default_db: str,
        default_read_coll: Optional[str] = None,
        default_write_coll: Optional[str] = None,
    ) -> "BaseStageConfig":
        # Import paths.py which already handles env vars with defaults
        from src.core.config.paths import DB_NAME

        # Retrieval order: args → default_db param → paths.py constant
        # paths.py constants already check env vars, so we just use them as fallback
        db_name = (
            getattr(args, "db_name", None)
            or env.get("DB_NAME")
            or default_db
            or DB_NAME
        )
        read_db = getattr(args, "read_db_name", None) or env.get("READ_DB_NAME")
        write_db = getattr(args, "write_db_name", None) or env.get("WRITE_DB_NAME")

        # For collections: args → env → default_*_coll param
        read_coll = (
            getattr(args, "read_coll", None)
            or env.get("READ_COLL")
            or default_read_coll
        )
        write_coll = (
            getattr(args, "write_coll", None)
            or env.get("WRITE_COLL")
            or default_write_coll
        )

        # Get input_video_ids from args (passed from source selection filter)
        input_video_ids = getattr(args, "input_video_ids", None)
        
        return cls(
            max=getattr(args, "max", None),
            # remove llm from args
            llm=bool(getattr(args, "llm", False)),
            verbose=bool(getattr(args, "verbose", False)),
            dry_run=bool(getattr(args, "dry_run", False)),
            db_name=db_name,
            read_db_name=read_db,
            write_db_name=write_db,
            read_coll=read_coll,
            write_coll=write_coll,
            upsert_existing=bool(getattr(args, "upsert_existing", False)),
            video_id=getattr(args, "video_id", None),
            input_video_ids=input_video_ids,
            concurrency=(
                int(getattr(args, "concurrency"))
                if getattr(args, "concurrency", None) is not None
                else None
            ),
        )
