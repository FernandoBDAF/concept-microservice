"""TTL reaper (ADR-006.4) — kills expired ttl-tagged session leaks.

Contract (HANDOFF §4):
  - Query resource-groups tagging API for resources tagged `ttl` (epoch
    seconds) in THIS region only.
  - For each expired resource, dispatch a per-service delete (ec2, eks
    nodegroup + cluster, rds instance, elbv2, nat gateway, elastic IP are
    the realistic leak surface — sessions tag everything they create with
    ttl).
  - DRY_RUN=true (default): log what WOULD be deleted, delete nothing.
    EXP-55 first proves the dry-run list, then a decoy resource, then flips
    the flag.
  - Never touch anything without a ttl tag; never touch stack=base tags.

Structured logging: one JSON line per considered resource
  ({arn, ttl, expired, action}), plus a JSON summary line. boto3 is the
Lambda runtime's — imported inside the handler so the module imports (and
py_compile / unit tests) without it.
"""
import json
import os
import time

# ── ARN → delete dispatch ────────────────────────────────────────────────────
# Each entry maps (service, resource_type) → a callable that, given the boto3
# session and the parsed resource id, performs the delete. Keep this the single
# source of truth for the reaper's blast radius — it must mirror the IAM policy
# in ../main.tf exactly (HANDOFF §4).


def _del_ec2_instance(sess, rid):
    sess.client("ec2").terminate_instances(InstanceIds=[rid])


def _del_nat_gateway(sess, rid):
    sess.client("ec2").delete_nat_gateway(NatGatewayId=rid)


def _del_elastic_ip(sess, rid):
    # tagging API returns the allocation id (eipalloc-…) as the resource id.
    sess.client("ec2").release_address(AllocationId=rid)


def _del_eks_cluster(sess, rid):
    sess.client("eks").delete_cluster(name=rid)


def _del_eks_nodegroup(sess, rid):
    # rid is "<cluster>/<nodegroup>/<uuid>" — the tagging API keeps the full
    # nodegroup ARN suffix. Cluster + nodegroup names are the first two parts.
    cluster, nodegroup = rid.split("/")[0], rid.split("/")[1]
    sess.client("eks").delete_nodegroup(clusterName=cluster, nodegroupName=nodegroup)


def _del_rds_instance(sess, rid):
    sess.client("rds").delete_db_instance(
        DBInstanceIdentifier=rid,
        SkipFinalSnapshot=True,
        DeleteAutomatedBackups=True,
    )


def _del_elbv2(sess, arn):
    # elbv2 deletes by the full load-balancer ARN, not a bare id.
    sess.client("elbv2").delete_load_balancer(LoadBalancerArn=arn)


# (service, resource_type) → (handler, arg) where arg is "id" (bare resource id)
# or "arn" (the full ARN, needed by elbv2).
_DISPATCH = {
    ("ec2", "instance"): (_del_ec2_instance, "id"),
    ("ec2", "natgateway"): (_del_nat_gateway, "id"),
    ("ec2", "elastic-ip"): (_del_elastic_ip, "id"),
    ("eks", "cluster"): (_del_eks_cluster, "id"),
    ("eks", "nodegroup"): (_del_eks_nodegroup, "id"),
    ("rds", "db"): (_del_rds_instance, "id"),
    ("elasticloadbalancing", "loadbalancer"): (_del_elbv2, "arn"),
}


def parse_arn(arn):
    """Return (service, resource_type, resource_id) or (service, None, None).

    ARN = arn:partition:service:region:account:resource. The resource segment
    may use "type/id", "type:id" (rds), or "type/id/more" (eks nodegroup,
    elbv2). We split off type on the first "/" or ":", keeping the remainder
    (with its slashes) as the id so nested ids survive.
    """
    parts = arn.split(":", 5)
    if len(parts) < 6:
        return (None, None, None)
    service, resource = parts[2], parts[5]
    if "/" in resource:
        rtype, rid = resource.split("/", 1)
    elif ":" in resource:
        rtype, rid = resource.split(":", 1)
    else:
        rtype, rid = resource, ""
    return (service, rtype, rid)


def _tags_map(resource):
    return {t.get("Key"): t.get("Value") for t in resource.get("Tags", [])}


def _iter_resources(sess):
    """Yield tagged resources for tag `ttl` in this region via the paginator."""
    client = sess.client("resourcegroupstaggingapi")
    paginator = client.get_paginator("get_resources")
    for page in paginator.paginate(TagFilters=[{"Key": "ttl"}]):
        for res in page.get("ResourceTagMappingList", []):
            yield res


def handler(event, _context):
    import boto3  # Lambda runtime provides it; keep module import-safe offline.

    dry_run = os.environ.get("DRY_RUN", "true").lower() != "false"
    now = int(time.time())
    sess = boto3.Session()

    considered = 0
    reaped = 0
    for res in _iter_resources(sess):
        considered += 1
        arn = res.get("ResourceARN", "")
        tags = _tags_map(res)
        ttl_raw = tags.get("ttl")
        stack = tags.get("stack")

        service, rtype, rid = parse_arn(arn)
        action = "skip"
        expired = False
        reason = None

        # Guardrails: never touch base-stack resources; require a numeric ttl.
        if stack == "base":
            reason = "stack=base"
        elif ttl_raw is None:
            reason = "no-ttl-tag"
        else:
            try:
                ttl = int(ttl_raw)
            except (TypeError, ValueError):
                reason = "unparseable-ttl"
                ttl = None
            if reason is None:
                expired = ttl < now
                if not expired:
                    reason = "not-expired"
                elif (service, rtype) not in _DISPATCH:
                    reason = "unsupported-service"
                    action = "unsupported"
                else:
                    action = "would-delete" if dry_run else "delete"

        log = {"arn": arn, "ttl": ttl_raw, "expired": expired, "action": action}
        if reason:
            log["reason"] = reason
        print(json.dumps(log))

        if action == "delete":
            handler_fn, argkind = _DISPATCH[(service, rtype)]
            try:
                handler_fn(sess, arn if argkind == "arn" else rid)
                reaped += 1
                print(json.dumps({"arn": arn, "action": "deleted"}))
            except Exception as exc:  # best-effort; hourly reruns catch stragglers
                print(json.dumps({"arn": arn, "action": "delete-failed",
                                  "error": str(exc)}))

    summary = {"reaped": reaped, "dry_run": dry_run, "considered": considered}
    print(json.dumps({"msg": "reaper summary", **summary}))
    return summary
