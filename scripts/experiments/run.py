#!/usr/bin/env python3
"""Scored experiment runner (ADR-004.1).

Contract:
  make experiment E=exp-04   →  python3 scripts/experiments/run.py exp-04
  make experiments           →  python3 scripts/experiments/run.py --list
  (validation)               →  python3 scripts/experiments/run.py --validate

Behavior:
 1. Load experiments/<id>.yaml (schema: experiments/README.md). Reject empty
    `assertions` — a scored run must be falsifiable (EXP-45).
 2. Pre-check `needs` (compose project up / kind cluster / obs stack / guest
    running) and fail fast with a pointed message.
 3. Run `steps` sequentially via subprocess (shell=True, fail-fast; `background:
    true` → Popen without wait, tracked for cleanup).
 4. Poll `assertions` every 5s until each passes or its own timeout lapses:
      promql — GET {PROM_URL}/api/v1/query, first sample vs op/value
      http   — status equality (+ optional json_path == json_equals)
      cli    — exit code 0
 5. Always run `cleanup`. Exit 0 iff every assertion passed.
 6. Append a result block to documentation/experiments/RUNS.md (created with a
    header if absent) and emit a junit-ish XML report to
    .experiment-results/<id>-<timestamp>.xml (gitignored; override the directory
    with $EXPERIMENT_REPORT_DIR) for CI phase 2. The dir is created lazily on the
    first run.

Assertion evaluation is factored into pure functions (compare / parse_duration /
jq_get / extract_promql_sample / evaluate_promql / evaluate_http /
validate_experiment) so it is unit-testable without a live stack — see
scripts/experiments/test_run.py.

Dependencies: stdlib + PyYAML (already a CI dep for drift-check).
"""
from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

import yaml

ROOT = Path(__file__).resolve().parent.parent.parent
EXPERIMENTS_DIR = ROOT / "experiments"
RUNS_MD = ROOT / "documentation" / "experiments" / "RUNS.md"
REPORT_DIR = Path(os.environ.get("EXPERIMENT_REPORT_DIR") or (ROOT / ".experiment-results"))
PROM_URL = os.environ.get("PROM_URL", "http://localhost:9090").rstrip("/")
POLL_INTERVAL = float(os.environ.get("EXPERIMENT_POLL_INTERVAL", "5"))
HTTP_ATTEMPT_TIMEOUT = float(os.environ.get("EXPERIMENT_HTTP_TIMEOUT", "10"))

ID_RE = re.compile(r"^exp-\d{2,}$")
OPS = {
    "==": lambda a, b: abs(a - b) < 1e-9,
    "!=": lambda a, b: abs(a - b) >= 1e-9,
    "<": lambda a, b: a < b,
    "<=": lambda a, b: a <= b,
    ">": lambda a, b: a > b,
    ">=": lambda a, b: a >= b,
}
ASSERTION_TYPES = {"promql", "http", "cli"}
NEEDS_TOKENS = {"compose", "kind", "obs"}  # plus guest:<name>
_MISSING = object()


# ── pure functions (unit-tested) ─────────────────────────────────────────────

def parse_duration(text: Any) -> float:
    """'300s' / '2m' / '1h' / bare seconds → float seconds. Raises ValueError."""
    if isinstance(text, (int, float)):
        return float(text)
    s = str(text).strip().lower()
    m = re.fullmatch(r"(\d+(?:\.\d+)?)\s*(ms|s|m|h)?", s)
    if not m:
        raise ValueError(f"bad duration: {text!r}")
    value = float(m.group(1))
    return value * {"ms": 0.001, "s": 1, "m": 60, "h": 3600, None: 1}[m.group(2)]


def compare(actual: float, op: str, value: float) -> bool:
    if op not in OPS:
        raise ValueError(f"unknown op: {op!r}")
    return OPS[op](float(actual), float(value))


def jq_get(obj: Any, path: str) -> Any:
    """Dot-path lookup (e.g. 'checks.rabbitmq'); returns _MISSING if absent."""
    cur = obj
    for part in path.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        else:
            return _MISSING
    return cur


def extract_promql_sample(parsed: dict) -> Optional[float]:
    """First sample value from a Prometheus /api/v1/query response, else None."""
    if not isinstance(parsed, dict) or parsed.get("status") != "success":
        return None
    result = parsed.get("data", {}).get("result", [])
    if not result:
        return None
    first = result[0]
    # vector: {"value": [ts, "v"]}; scalar: {"value": [ts, "v"]}
    value = first.get("value") if isinstance(first, dict) else None
    if not value or len(value) < 2:
        return None
    try:
        return float(value[1])
    except (TypeError, ValueError):
        return None


def evaluate_promql(parsed: dict, op: str, value: float) -> tuple[bool, float, bool]:
    """(passed, actual, had_sample). An empty vector is treated as 0.0 —
    absence of a queue/counter series reads as zero, which keeps drain-to-zero
    assertions robust while still failing any positive threshold."""
    sample = extract_promql_sample(parsed)
    had_sample = sample is not None
    actual = sample if had_sample else 0.0
    return compare(actual, op, value), actual, had_sample


def evaluate_http(status_code: int, expected_status: int, body_text: str,
                  json_path: Optional[str], json_equals: Any) -> tuple[bool, str]:
    if status_code != expected_status:
        return False, f"status {status_code} != {expected_status}"
    if json_path is None:
        return True, f"status {status_code}"
    try:
        body = json.loads(body_text)
    except (ValueError, TypeError):
        return False, f"status {status_code}, body not JSON"
    got = jq_get(body, json_path)
    if got is _MISSING:
        return False, f"status {status_code}, json path {json_path!r} missing"
    if str(got) != str(json_equals):
        return False, f"status {status_code}, {json_path}={got!r} != {json_equals!r}"
    return True, f"status {status_code}, {json_path}={got!r}"


def validate_experiment(doc: Any, expected_id: Optional[str] = None) -> list[str]:
    """Schema-check a parsed experiment. Returns a list of error strings."""
    errors: list[str] = []
    if not isinstance(doc, dict):
        return [f"top-level must be a mapping, got {type(doc).__name__}"]

    exp_id = doc.get("id")
    if not isinstance(exp_id, str) or not ID_RE.match(exp_id or ""):
        errors.append(f"id must match exp-NN (kebab-case), got {exp_id!r}")
    if expected_id is not None and exp_id != expected_id:
        errors.append(f"id {exp_id!r} != filename id {expected_id!r}")

    if not isinstance(doc.get("title"), str) or not doc.get("title").strip():
        errors.append("title must be a non-empty string")

    needs = doc.get("needs", [])
    if not isinstance(needs, list):
        errors.append("needs must be a list")
    else:
        for n in needs:
            if not (n in NEEDS_TOKENS or (isinstance(n, str) and n.startswith("guest:"))):
                errors.append(f"unknown need {n!r} (want compose|kind|obs|guest:<name>)")

    for key in ("steps", "watch", "cleanup"):
        val = doc.get(key, [])
        if val is None:
            continue
        if not isinstance(val, list):
            errors.append(f"{key} must be a list")
    for key in ("steps", "cleanup"):
        for i, step in enumerate(doc.get(key) or []):
            if not isinstance(step, dict) or not isinstance(step.get("run"), str) or not step["run"].strip():
                errors.append(f"{key}[{i}] must have a non-empty string 'run'")
            if "background" in (step or {}) and not isinstance(step.get("background"), bool):
                errors.append(f"{key}[{i}].background must be a boolean")

    assertions = doc.get("assertions")
    if not isinstance(assertions, list) or len(assertions) == 0:
        errors.append("assertions must be a non-empty list (a scored run must be falsifiable)")
    else:
        for i, a in enumerate(assertions):
            errors.extend(f"assertions[{i}]: {e}" for e in _validate_assertion(a))
    return errors


def _validate_assertion(a: Any) -> list[str]:
    errors: list[str] = []
    if not isinstance(a, dict):
        return [f"must be a mapping, got {type(a).__name__}"]
    atype = a.get("type")
    if atype not in ASSERTION_TYPES:
        errors.append(f"unknown type {atype!r} (want promql|http|cli)")
    if "timeout" not in a:
        errors.append("missing 'timeout'")
    else:
        try:
            parse_duration(a["timeout"])
        except ValueError as e:
            errors.append(str(e))
    if atype == "promql":
        if not isinstance(a.get("query"), str) or not a["query"].strip():
            errors.append("promql needs a non-empty 'query'")
        if a.get("op") not in OPS:
            errors.append(f"promql needs op in {sorted(OPS)}")
        if not isinstance(a.get("value"), (int, float)):
            errors.append("promql needs a numeric 'value'")
    elif atype == "http":
        if not isinstance(a.get("url"), str) or not a["url"].strip():
            errors.append("http needs a non-empty 'url'")
        if not isinstance(a.get("status"), int):
            errors.append("http needs an integer 'status'")
        if ("json_path" in a) != ("json_equals" in a):
            errors.append("http json_path and json_equals must be given together")
    elif atype == "cli":
        if not isinstance(a.get("run"), str) or not a["run"].strip():
            errors.append("cli needs a non-empty 'run'")
    return errors


def assertion_label(a: dict) -> str:
    t = a.get("type")
    if t == "promql":
        return f"promql {a.get('query')} {a.get('op')} {a.get('value')}"
    if t == "http":
        extra = f" [{a['json_path']}=={a['json_equals']}]" if "json_path" in a else ""
        return f"http {a.get('url')} status {a.get('status')}{extra}"
    if t == "cli":
        return f"cli {a.get('run')}"
    return f"{t} (unknown)"


# ── I/O: discovery & loading ─────────────────────────────────────────────────

def discover_experiments() -> list[tuple[str, Path]]:
    out = []
    for p in sorted(EXPERIMENTS_DIR.glob("exp-*.yaml")):
        out.append((p.stem, p))
    return out


def load_experiment(exp_id: str) -> dict:
    path = EXPERIMENTS_DIR / f"{exp_id}.yaml"
    if not path.exists():
        raise FileNotFoundError(f"no such experiment: {path}")
    with open(path) as f:
        return yaml.safe_load(f)


# ── I/O: probes (thin wrappers over the pure evaluators) ─────────────────────

def query_prometheus(query: str) -> Optional[dict]:
    url = f"{PROM_URL}/api/v1/query?" + urllib.parse.urlencode({"query": query})
    try:
        with urllib.request.urlopen(url, timeout=HTTP_ATTEMPT_TIMEOUT) as resp:
            return json.load(resp)
    except (urllib.error.URLError, OSError, ValueError):
        return None


def probe_promql(a: dict) -> tuple[bool, str]:
    parsed = query_prometheus(a["query"])
    if parsed is None:
        return False, "prometheus unreachable / query error"
    passed, actual, had = evaluate_promql(parsed, a["op"], float(a["value"]))
    note = f"actual {actual:g}" + ("" if had else " (no samples → 0)")
    return passed, note


def _http_get(url: str) -> tuple[int, str]:
    try:
        with urllib.request.urlopen(url, timeout=HTTP_ATTEMPT_TIMEOUT) as resp:
            return resp.status, resp.read().decode("utf-8", "replace")
    except urllib.error.HTTPError as e:  # 4xx/5xx still carry a status + body
        return e.code, e.read().decode("utf-8", "replace")
    except (urllib.error.URLError, OSError) as e:
        return 0, f"unreachable: {e}"


def probe_http(a: dict) -> tuple[bool, str]:
    status, body = _http_get(a["url"])
    if status == 0:
        return False, body
    return evaluate_http(status, int(a["status"]), body,
                         a.get("json_path"), a.get("json_equals"))


def probe_cli(a: dict) -> tuple[bool, str]:
    proc = subprocess.run(a["run"], shell=True, cwd=str(ROOT),
                          capture_output=True, text=True)
    ok = proc.returncode == 0
    note = f"exit {proc.returncode}"
    if not ok and proc.stderr.strip():
        note += f": {proc.stderr.strip().splitlines()[-1][:160]}"
    return ok, note


def probe(a: dict) -> tuple[bool, str]:
    return {"promql": probe_promql, "http": probe_http, "cli": probe_cli}[a["type"]](a)


# ── needs precheck ───────────────────────────────────────────────────────────

def _compose_running_services() -> set[str]:
    proc = subprocess.run("docker compose ps --status running --services",
                          shell=True, cwd=str(ROOT), capture_output=True, text=True)
    if proc.returncode != 0:
        return set()
    return {line.strip() for line in proc.stdout.splitlines() if line.strip()}


def precheck_needs(needs: list[str]) -> list[str]:
    failures: list[str] = []
    for need in needs or []:
        try:
            if need == "compose":
                running = _compose_running_services()
                missing = {"rabbitmq", "api-service"} - running
                if missing:
                    failures.append(f"compose: stack not up (missing {sorted(missing)}) — run `make up`")
            elif need == "kind":
                proc = subprocess.run("kubectl config current-context",
                                      shell=True, capture_output=True, text=True)
                if proc.returncode != 0 or "kind" not in proc.stdout:
                    failures.append("kind: no kind cluster context — run `make cluster-up`")
            elif need == "obs":
                proc = subprocess.run("kubectl get ns lab-obs",
                                      shell=True, capture_output=True, text=True)
                if proc.returncode != 0:
                    failures.append("obs: lab-obs namespace absent — run `make obs-up`")
            elif isinstance(need, str) and need.startswith("guest:"):
                name = need.split(":", 1)[1]
                proc = subprocess.run(
                    f"docker compose -f guests/{name}/docker-compose.yml ps --status running --services",
                    shell=True, cwd=str(ROOT), capture_output=True, text=True)
                if proc.returncode != 0 or not proc.stdout.strip():
                    failures.append(f"guest:{name}: not up — run `make guest-up G={name}`")
            else:
                failures.append(f"unknown need {need!r}")
        except OSError as e:
            failures.append(f"{need}: precheck error: {e}")
    return failures


# ── steps / cleanup ──────────────────────────────────────────────────────────

def run_steps(steps: list[dict], bg: list[subprocess.Popen]) -> Optional[str]:
    """Sequential, fail-fast. Returns an error string on the first failure."""
    for i, step in enumerate(steps or []):
        cmd = step["run"]
        if step.get("background"):
            print(f"  step {i + 1} (bg): {cmd}")
            bg.append(subprocess.Popen(cmd, shell=True, cwd=str(ROOT)))
            continue
        print(f"  step {i + 1}: {cmd}")
        proc = subprocess.run(cmd, shell=True, cwd=str(ROOT))
        if proc.returncode != 0:
            return f"step {i + 1} failed (exit {proc.returncode}): {cmd}"
    return None


def run_cleanup(cleanup: list[dict], bg: list[subprocess.Popen]) -> None:
    for proc in bg:
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
    for i, step in enumerate(cleanup or []):
        cmd = step["run"]
        print(f"  cleanup {i + 1}: {cmd}")
        subprocess.run(cmd, shell=True, cwd=str(ROOT))  # best-effort


# ── polling ──────────────────────────────────────────────────────────────────

@dataclass
class AssertionResult:
    index: int
    label: str
    passed: bool
    note: str
    elapsed: float


def poll_assertions(assertions: list[dict]) -> list[AssertionResult]:
    n = len(assertions)
    deadlines = [parse_duration(a["timeout"]) for a in assertions]
    results: list[Optional[AssertionResult]] = [None] * n
    start = time.monotonic()
    while any(r is None for r in results):
        elapsed = time.monotonic() - start
        for i, a in enumerate(assertions):
            if results[i] is not None:
                continue
            passed, note = probe(a)
            if passed:
                results[i] = AssertionResult(i, assertion_label(a), True, note, elapsed)
                print(f"  PASS  {assertion_label(a)}  ({note}, {elapsed:.1f}s)")
            elif elapsed >= deadlines[i]:
                results[i] = AssertionResult(i, assertion_label(a), False, note, elapsed)
                print(f"  FAIL  {assertion_label(a)}  ({note}, timed out at {deadlines[i]:g}s)")
        pending = [deadlines[i] - (time.monotonic() - start) for i in range(n) if results[i] is None]
        if not pending:
            break
        time.sleep(max(0.0, min(POLL_INTERVAL, min(pending))))
    return [r for r in results if r is not None]


# ── reporting ────────────────────────────────────────────────────────────────

def append_runs_md(exp_id: str, title: str, overall: bool, results: list[AssertionResult],
                   step_count: int, duration: float, note: str = "") -> None:
    RUNS_MD.parent.mkdir(parents=True, exist_ok=True)
    if not RUNS_MD.exists():
        RUNS_MD.write_text(
            "# Experiment runs\n\n"
            "Generated by `scripts/experiments/run.py` (ADR-004.1): one block per\n"
            "`make experiment E=<id>` run, newest at the bottom. Prose lives in\n"
            "[EXPERIMENTS.md](../../EXPERIMENTS.md); scored definitions in\n"
            "[experiments/](../../experiments/).\n")
    ts = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    verdict = "PASS" if overall else "FAIL"
    passed = sum(1 for r in results if r.passed)
    lines = [
        f"\n## {exp_id} · {title} — {ts} — {verdict}",
        f"- steps: {step_count} · assertions: {passed}/{len(results)} passed · duration: {duration:.1f}s",
    ]
    if note:
        lines.append(f"- note: {note}")
    for r in results:
        lines.append(f"- [{'PASS' if r.passed else 'FAIL'}] {r.label} → {r.note} ({r.elapsed:.1f}s)")
    with open(RUNS_MD, "a") as f:
        f.write("\n".join(lines) + "\n")


def write_junit(exp_id: str, results: list[AssertionResult], duration: float) -> Path:
    REPORT_DIR.mkdir(parents=True, exist_ok=True)
    failures = sum(1 for r in results if not r.passed)
    suite = ET.Element("testsuite", {
        "name": exp_id, "tests": str(len(results)), "failures": str(failures),
        "errors": "0", "time": f"{duration:.3f}",
        "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S"),
    })
    for r in results:
        case = ET.SubElement(suite, "testcase", {
            "classname": exp_id, "name": r.label, "time": f"{r.elapsed:.3f}"})
        if not r.passed:
            fail = ET.SubElement(case, "failure", {"message": r.note})
            fail.text = f"{r.label}: {r.note}"
    ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    out = REPORT_DIR / f"{exp_id}-{ts}.xml"
    ET.ElementTree(suite).write(out, encoding="utf-8", xml_declaration=True)
    return out


# ── commands ─────────────────────────────────────────────────────────────────

def cmd_list() -> int:
    print(f"{'ID':<8}  {'SCORED':<7}  {'NEEDS':<20}  TITLE")
    for exp_id, path in discover_experiments():
        try:
            doc = yaml.safe_load(path.read_text()) or {}
        except yaml.YAMLError as e:
            print(f"{exp_id:<8}  {'ERR':<7}  {'-':<20}  YAML error: {e}")
            continue
        needs = ",".join(doc.get("needs") or []) or "-"
        scored = "yes" if (doc.get("assertions")) else "no"
        print(f"{exp_id:<8}  {scored:<7}  {needs:<20}  {doc.get('title', '')}")
    return 0


def cmd_validate() -> int:
    ok = True
    for exp_id, path in discover_experiments():
        try:
            doc = yaml.safe_load(path.read_text())
        except yaml.YAMLError as e:
            print(f"FAIL  {path.name}: YAML parse error: {e}")
            ok = False
            continue
        errors = validate_experiment(doc, expected_id=exp_id)
        if errors:
            ok = False
            print(f"FAIL  {path.name}")
            for e in errors:
                print(f"        - {e}")
        else:
            print(f"ok    {path.name}")
    print("all experiments valid" if ok else "validation FAILED")
    return 0 if ok else 1


def cmd_run(exp_id: str) -> int:
    try:
        doc = load_experiment(exp_id)
    except FileNotFoundError as e:
        print(str(e), file=sys.stderr)
        return 2
    errors = validate_experiment(doc, expected_id=exp_id)
    if errors:
        print(f"{exp_id}: invalid definition:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        return 2

    title = doc["title"]
    print(f"== {exp_id} · {title}")

    need_failures = precheck_needs(doc.get("needs") or [])
    if need_failures:
        for f in need_failures:
            print(f"  needs: {f}", file=sys.stderr)
        return 1

    bg: list[subprocess.Popen] = []
    start = time.monotonic()
    results: list[AssertionResult] = []
    step_err: Optional[str] = None
    try:
        print("- steps")
        step_err = run_steps(doc.get("steps") or [], bg)
        if step_err:
            print(f"  {step_err}", file=sys.stderr)
        else:
            print("- assertions")
            results = poll_assertions(doc["assertions"])
    finally:
        print("- cleanup")
        run_cleanup(doc.get("cleanup") or [], bg)

    duration = time.monotonic() - start
    if step_err:  # steps failed fast → record a synthetic failing case
        results = [AssertionResult(0, "steps", False, step_err, duration)]
        overall = False
    else:
        overall = bool(results) and all(r.passed for r in results)
    append_runs_md(exp_id, title, overall, results, len(doc.get("steps") or []),
                   duration, step_err or "")
    report = write_junit(exp_id, results, duration)
    print(f"- report: {report}")
    print(f"== {exp_id}: {'PASS' if overall else 'FAIL'} ({duration:.1f}s)")
    return 0 if overall else 1


def main(argv: Optional[list[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        prog="run.py", description="Scored experiment runner (ADR-004.1)")
    parser.add_argument("experiment", nargs="?", help="experiment id, e.g. exp-02")
    parser.add_argument("--list", action="store_true", help="list ids, titles, needs")
    parser.add_argument("--validate", action="store_true",
                        help="schema-check every experiments/*.yaml and exit")
    args = parser.parse_args(argv)

    if args.list:
        return cmd_list()
    if args.validate:
        return cmd_validate()
    if not args.experiment:
        parser.print_help()
        return 2
    return cmd_run(args.experiment)


if __name__ == "__main__":
    sys.exit(main())
