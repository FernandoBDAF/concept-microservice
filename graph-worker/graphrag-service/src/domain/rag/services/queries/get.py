import os
from dotenv import load_dotenv
import argparse
import json

try:
    from src.infrastructure.database.mongodb import get_mongo_client
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(
        _os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..", ".."))
    )
    from src.infrastructure.database.mongodb import get_mongo_client
from src.core.config.paths import DB_NAME
from src.domain.services.rag.retrieval import structured_search

from bson import ObjectId
from datetime import datetime
from bson.decimal128 import Decimal128

def to_plain(o):
    if isinstance(o, ObjectId):
        return str(o)
    if isinstance(o, datetime):
        return o.isoformat()
    if isinstance(o, Decimal128):
        return float(o.to_decimal())
    return str(o)

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Get documents from MongoDB")
    parser.add_argument("--col", type=str, required=True)
    parser.add_argument("--fields", type=str, required=False)
    parser.add_argument("--filters", type=str, required=False)
    parser.add_argument("--sort_by", type=str, required=False)
    parser.add_argument("--top_k", type=int, required=False)
    return parser.parse_args()


def main() -> None:
    load_dotenv()
    args = parse_args()
    client = get_mongo_client()
    db = client[DB_NAME]
    col = db[args.col]
    fields = args.fields.split(', ') if args.fields else None
    filters = json.loads(args.filters) if args.filters else None
    sort_by = json.loads(args.sort_by) if args.sort_by else None
    hits = structured_search(col, fields=fields, filters=filters, sort_by=sort_by, top_k=args.top_k)
    # print(hits)
    print(json.dumps(hits, default=to_plain, ensure_ascii=False, indent=2))

if __name__ == "__main__":
    main()