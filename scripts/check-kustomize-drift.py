#!/usr/bin/env python3
"""Compose ⇄ kustomize drift check (ADR-002.4).

Compose (docker-compose.yml) is the behavioral reference; the kustomize tree
(deploy/k8s) must declare the same surface. Mechanically compared:

  1. service set     — every compose service has a cluster workload and vice
                       versa (documented exceptions below)
  2. env keys        — per service, identical key sets (cluster-only helper
                       keys that only feed $(VAR) expansion are allowlisted)
  3. infra images    — pulled images (postgres, redis, ...) match exactly;
                       built services must reference localhost:5001/<name>
  4. migration CMs   — the generated ConfigMaps carry every migration file
                       currently in the service dirs (the generator lists
                       files explicitly, so adding a migration must not be
                       silently ignored)

Exit 0 = no drift. Needs: docker compose, kustomize, PyYAML.
"""

import json
import subprocess
import sys
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parent.parent
OVERLAY = ROOT / "deploy/k8s/overlays/kind-local"

# compose service -> cluster workload (kind, name). None = deliberately
# compose-only / cluster-only; string values name the k8s counterpart.
SERVICE_MAP = {
    "api-service": "api-service",
    "auth-service": "auth-service",
    "graphrag-service": "graphrag-service",
    "email-worker": "email-worker",
    "image-worker": "image-worker",
    "profile-worker": "profile-worker",
    "postgres": "postgres",
    "redis": "redis",
    "rabbitmq": "rabbitmq",
    "mongodb": "mongodb",
    "minio": "minio",
    "minio-init": "minio-bucket-init",
    "api-migrate": "api-migrate",
    "auth-migrate": "auth-migrate",
    # observability stays compose-side until v3 (phase brief: out of scope)
    "prometheus": None,
    "grafana": None,
}

# cluster-only env keys: secretKeyRef helpers consumed by $(VAR) expansion in
# URL-shaped values, plus store credentials that compose hardcodes inline.
ALLOW_EXTRA_CLUSTER_ENV = {
    "api-service": {"POSTGRES_PASSWORD"},
    "graphrag-service": {"RABBITMQ_PASSWORD", "MONGO_ROOT_PASSWORD"},
    "email-worker": {"RABBITMQ_PASSWORD"},
    "image-worker": {"RABBITMQ_PASSWORD"},
    "profile-worker": {"RABBITMQ_PASSWORD"},
    "postgres": {"AUTH_DB_PASSWORD", "PGDATA"},
    "rabbitmq": {"RABBITMQ_DEFAULT_USER", "RABBITMQ_DEFAULT_PASS"},
    "api-migrate": {"PGPASSWORD"},
    "auth-migrate": {"PGPASSWORD"},
    "minio-bucket-init": {"MINIO_ROOT_PASSWORD"},
}
# compose-only env keys (inline credentials replaced by Secrets in-cluster)
ALLOW_EXTRA_COMPOSE_ENV = {
    "rabbitmq": {"RABBITMQ_DEFAULT_USER", "RABBITMQ_DEFAULT_PASS"},
    "api-migrate": {"PGPASSWORD"},
    "auth-migrate": {"PGPASSWORD"},
}

errors: list[str] = []


def compose_config() -> dict:
    out = subprocess.run(
        ["docker", "compose", "config", "--format", "json"],
        cwd=ROOT, capture_output=True, text=True, check=True,
    ).stdout
    return json.loads(out)


def kustomize_build() -> list[dict]:
    out = subprocess.run(
        ["kustomize", "build", "--load-restrictor", "LoadRestrictionsNone", str(OVERLAY)],
        capture_output=True, text=True, check=True,
    ).stdout
    return [d for d in yaml.safe_load_all(out) if d]


def workload_containers(doc: dict) -> list[dict]:
    spec = doc.get("spec", {})
    tpl = spec.get("template", {}).get("spec", {})
    return tpl.get("containers", [])


def main() -> int:
    compose = compose_config()
    cluster = kustomize_build()

    workloads = {}  # name -> {env: set, image: str}
    for doc in cluster:
        if doc.get("kind") not in ("Deployment", "StatefulSet", "Job"):
            continue
        name = doc["metadata"]["name"]
        for c in workload_containers(doc):
            env = {e["name"] for e in c.get("env", [])}
            workloads[name] = {"env": env, "image": c["image"]}

    configmaps = {
        doc["metadata"]["name"]: doc
        for doc in cluster
        if doc.get("kind") == "ConfigMap"
    }

    # 1 + 2 + 3: service set, env keys, images
    for svc, spec in compose["services"].items():
        mapped = SERVICE_MAP.get(svc, "<unmapped>")
        if mapped == "<unmapped>":
            errors.append(f"compose service '{svc}' has no entry in SERVICE_MAP")
            continue
        if mapped is None:
            continue
        if mapped not in workloads:
            errors.append(f"compose service '{svc}' -> '{mapped}' missing from kustomize build")
            continue

        w = workloads[mapped]
        compose_env = set((spec.get("environment") or {}).keys())
        cluster_env = w["env"]
        extra_cluster = cluster_env - compose_env - ALLOW_EXTRA_CLUSTER_ENV.get(mapped, set())
        extra_compose = compose_env - cluster_env - ALLOW_EXTRA_COMPOSE_ENV.get(svc, set())
        if extra_cluster:
            errors.append(f"{svc}: cluster-only env keys {sorted(extra_cluster)} (allowlist or fix)")
        if extra_compose:
            errors.append(f"{svc}: compose-only env keys {sorted(extra_compose)} (missing in cluster)")

        if "build" in spec:
            want = f"localhost:5001/{mapped}:"
            if not w["image"].startswith(want):
                errors.append(f"{svc}: built service must use {want}* in-cluster, got {w['image']}")
        else:
            if w["image"] != spec["image"]:
                errors.append(f"{svc}: image mismatch compose={spec['image']} cluster={w['image']}")

    for name in workloads:
        if name not in {m for m in SERVICE_MAP.values() if m}:
            errors.append(f"cluster workload '{name}' has no compose counterpart in SERVICE_MAP")

    # 4: migration ConfigMaps carry every file on disk
    def check_migrations(cm_prefix: str, directory: Path, suffix: str):
        cms = [c for n, c in configmaps.items() if n.startswith(cm_prefix)]
        if not cms:
            errors.append(f"no ConfigMap named {cm_prefix}* in build")
            return
        keys = set(cms[0].get("data", {}).keys())
        files = {p.name for p in directory.glob(f"*{suffix}")}
        missing = files - keys
        if missing:
            errors.append(
                f"{cm_prefix}: migration files not in the generator: {sorted(missing)} "
                f"(add them to deploy/k8s/base/migrations/kustomization.yaml)"
            )

    check_migrations("api-migrations", ROOT / "api-service/migrations", ".up.sql")
    check_migrations("auth-migrations", ROOT / "auth-service/migrations", ".sql")

    if errors:
        print("compose ⇄ kustomize DRIFT:")
        for e in errors:
            print(f"  ✗ {e}")
        return 1
    print(f"no drift: {len([m for m in SERVICE_MAP.values() if m])} workloads, "
          f"env surfaces and images consistent")
    return 0


if __name__ == "__main__":
    sys.exit(main())
