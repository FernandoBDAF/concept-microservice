# v2 exit runs — EXP-20..23 on the kind cluster lab

**Date:** 2026-07-10 · **Runner:** Claude (same session as the v1.1 exit
run) · **Cluster:** kind, profiles `single` (EXP-20/22/23) and `multinode`
(EXP-21) · **Result:** all four pass; three defects found and fixed
mid-run; two design findings recorded. Tagged `lab-v2.0`.

## EXP-20 · Catalog parity on kind — ✅

Every compose drill re-ran against the cluster through kubectl levers
(mapping: deploy/k8s/README.md):

| Drill | Compose (v1.1 run) | Cluster | Verdict |
|---|---|---|---|
| smoke | 7/7, 0/58 failed, p95 185 ms | 7/7, 0/58 failed, p95 147 ms | ✅ |
| burst 50 VU | 0.17% failed, 1313 iters | **0.00% failed**, 589 iters, p95 1.6 s | ✅ (deltas below) |
| worker outage ×100 | drained ~15 s, 0 loss | consumers=0 observed, drained <1 min, 0 loss (msgs 96–99 in log) | ✅ |
| scale-out ×300 | 49 s → 21 s (1→3 repl) | **49 s → 25 s** | ✅ |
| poison | +6/+6/+6, +3 doc; 2 flavors | identical, x-death `rejected` | ✅ |
| broker outage | partial degradation | **full API outage at ingress** — the finding | ⚠️ documented |
| document E2E | E2E OK, stub, object 115 B | identical through TLS ingress | ✅ |

Calibration deltas (expected, recorded): k6 through TLS ingress +
CPU-limited pods ≈ half the iteration throughput and a fatter tail
(p95 1.6 s under burst vs 178 ms) — the requests-vs-checks distinction
matters: failures stayed at 0.

**Finding (design):** api's `/ready` ANDs PG+Redis+RMQ; Kubernetes acts on
it — broker down ⇒ both api pods NotReady ⇒ Service loses all endpoints ⇒
nginx 503s CRUD that compose kept serving. Auth (checks only PG) kept
serving 201s throughout. Write-up:
[readiness-coupling](2026-07-10-readiness-coupling-broker-outage.md);
CONCEPTUAL_REVIEW §12; decide in v4.

**Fixed en route:**
1. mongodb readiness probe — unquoted `{ping: 1}` YAML-parsed as a map
   (compose quotes it; kustomize doesn't validate; kubectl does).
2. rabbitmq probe economics — `rabbitmq-diagnostics status/
   check_port_connectivity` spawn an Erlang VM per call; under a 500m CPU
   limit they blow their timeouts and liveness kill-loops the broker
   (5 restarts/30 min → the first smoke's 9.5% failures). Now: readiness =
   TCP:5672, liveness = `ping` @15 s timeout, CPU 1000m.
3. `runAsNonRoot` vs named users — kubelet can't verify `USER app`;
   all six Dockerfiles now pin numeric `USER 10001:10001`. Forcing a
   *foreign* UID via securityContext had broken graphrag's pip `--user`
   layout (ModuleNotFoundError) — manifests now respect image users and
   assert only `runAsNonRoot`.

## EXP-21 · Node kill & rescheduling (multinode) — ✅

Under `cluster-sim-load` (10 VU / 5 min, 8846 requests) with an
availability probe at 2 Hz:

| Event | Numbers |
|---|---|
| `kubectl drain lab-worker2` (hosted 1 of 2 api replicas) | done in **12 s**; replica → lab-worker3; **0 probe failures**, PDB `minAvailable: 1` held |
| `docker stop lab-worker3` (hard kill; hosted the fresh replica) | NotReady detected in **23 s**; **10×** probe blips over ~25 s (nginx retried the stale endpoint), then clean on the surviving replica |
| k6 whole-drill damage | **0.03%** (3/8846), p95 220 ms |
| stale state | dead node's pod read `Running`, no deletionTimestamp at T+5 min (unreachable-taint eviction hadn't fired) |
| `kubectl delete pod --force --grace-period=0` | replacement **Running in 8 s** on lab-worker2 |
| `docker start lab-worker3` | node Ready again; deployment back to 2/2 |

Store pinning (kind-multinode overlay: `node-type=infrastructure`) kept all
four StatefulSets + redis on the surviving infra node — no PVC stranding
(kind local-path PVs carry node affinity; an unpinned store on the victim
node would have deadlocked the drill).

Lesson bank: drain vs kill is the whole story — graceful eviction respects
PDBs and moves pods *before* the node goes; a dead kubelet leaves lies in
`kubectl get pods` and only taints, timeouts, or an operator's force-delete
resolve them.

## EXP-22 · Image path — ✅

- `/health` marker built as `:exp22`, pushed to localhost:5001, `kubectl
  set image` → rolling update; `{"build":"exp22"}` live through the
  ingress; registry catalog lists all six repos.
- Availability probe during the rollout: 198/200 — the 2×
  connection blips were the endpoint-propagation race on pod termination.
  **Fixed:** 5 s `preStop` sleep on api/auth; re-probed a full rollout:
  **150/150.**
- `kubectl rollout undo` → `{"status":"ok"}`, image back to `:dev`,
  history shows both revisions. Rollback is one command.

## EXP-23 · Zero-trust proof — ✅ (and a better lesson than scripted)

- Denied paths deny (timeout): scratch pod → postgres/redis/rabbitmq;
  email-worker → postgres. Allowed paths work: DNS from scratch;
  api → auth 200 (through the policy, not around it).
- The flip needed **three** deletions to open worker→postgres, not one:
  ingress-side guards (`lab-infra: default-deny-all, postgres`) weren't
  enough; egress-side (`lab-core: default-deny-all, workers`) *still*
  wasn't enough — **`allow-dns`'s empty podSelector selects every pod, and
  any selecting egress policy imposes default-deny on everything it
  doesn't allow.** Union-of-allows; no deny primitive. Re-applying the
  overlay restored all 15 policies and the denial.
- Incidentally proves kind ≥ v0.23's kindnet (kube-network-policies)
  actually enforces NetworkPolicy — era-1 asserted this; v2 demonstrated
  it.

## Acceptance evidence

- `make cluster-up PROFILE=multinode` from deleted cluster → all Ready in
  **9 m 21 s** (warm image cache; single profile is faster). Wait logic is
  part of the target — "ready" means rollouts complete, jobs Complete,
  certificates Ready.
- `make verify` green locally; CI phase 1 mirrors it (verify + 6 image
  builds + drift check). Drift check proven two-sided: deliberate env-key
  typo → both symmetric errors + exit 1; revert → green.
- Operational notes for future runs: `kubectl port-forward` dies with its
  pod (broker restarts kill it — rerun it, or use `kubectl exec` paths);
  the first apply after cert-manager install can race its webhook (up.sh
  retries); a pod on a killed node reads `Running` — trust node status
  first.
