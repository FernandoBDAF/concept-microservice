"""TTL reaper (ADR-006.4) — SKELETON, ships in DRY_RUN=true.

Contract (HANDOFF §4):
  - Query resource-groups tagging API for resources tagged `ttl` (epoch
    seconds) in THIS region only.
  - For each expired resource, dispatch a per-service delete (ec2, eks
    nodegroup, rds instance, elbv2, nat gateway are the realistic leak
    surface — sessions tag everything they create with ttl).
  - DRY_RUN=true: log what WOULD be deleted, delete nothing. EXP-55 first
    proves the dry-run list, then a decoy resource, then flips the flag.
  - Never touch anything without a ttl tag; never touch stack=base tags.
"""
import json
import os
import time


def handler(event, _context):
    dry_run = os.environ.get("DRY_RUN", "true").lower() != "false"
    now = int(time.time())
    # TODO(v5): boto3 resourcegroupstaggingapi.get_resources(TagFilters=[{"Key": "ttl"}])
    # → parse ARNs, compare int(ttl) < now, dispatch deletes per service.
    print(json.dumps({"msg": "reaper skeleton run", "dry_run": dry_run, "now": now,
                      "todo": "implement per HANDOFF §4 before flipping DRY_RUN"}))
    return {"reaped": 0, "dry_run": dry_run}
