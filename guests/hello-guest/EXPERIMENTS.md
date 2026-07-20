# hello-guest experiments

Guest-local drills in the lab's format (root `EXPERIMENTS.md` conventions:
Goal / Validates / Steps / Expect / Cleanup, assertions runnable). Required
by HOST_CONTRACT.md §1.5.

Prereqs: the lab stack is up (`make up` at the repo root — the guest's
compose file attaches to `microservices_default`), the hello-guest scrape job
is present in `scripts/compose/prometheus.yml`, and the guest is up:
`docker compose -f guests/hello-guest/docker-compose.yml up -d --build`.

---

## EXP-HG-01 · Guest burst visibility

**Goal:** prove an isolated guest is *observable through the host* — its
traffic shows up in the shared Prometheus/Grafana, and losing a guest
component is visible as a flatline, not silence you have to go looking for.
**Validates:** host scrape path onto the guest network (HOST_CONTRACT §2
"Observability"), metric-name stability (§1.2), compose restart policy /
k8s self-healing.

**Steps (compose)**
1. Baseline — both targets up in the host Prometheus:
   ```bash
   curl -s 'http://localhost:9090/api/v1/targets' \
     | grep -o '"scrapeUrl":"[^"]*hello-guest[^"]*"'
   ```
   Expect two scrape URLs (`hello-guest-web:8080`, `hello-guest-worker:8080`).
2. Flood the web component for ~60 s (curl loop; use `hey`/`k6` if installed):
   ```bash
   for i in $(seq 1 3000); do curl -s -o /dev/null http://localhost:4100/; done
   ```
3. Watch the burst in shared Grafana (http://localhost:3001) — Explore →
   query `rate(hello_guest_web_requests_total[1m])`. Or assert from the CLI:
   ```bash
   curl -s 'http://localhost:9090/api/v1/query?query=rate(hello_guest_web_requests_total[1m])' \
     | grep -v '"value":\[[0-9.]*,"0"\]'
   ```
   (non-zero rate ⇒ the guest burst is visible host-side).
4. Crash the worker mid-flight — from *inside*, so the restart policy sees
   a failure, not an operator stop (`docker kill`/`stop` mark the container
   manually stopped and `unless-stopped` then deliberately does NOT revive
   it — calibrated 2026-07-19, the run caught the original `docker kill`
   step never coming back):
   ```bash
   docker exec hello-guest-worker-1 kill 1
   ```
5. Within ~30 s, verify the flatline *and* the recovery:
   ```bash
   # jobs counter stops increasing while the container is down (the series
   # goes stale/empty until the target returns):
   curl -s 'http://localhost:9090/api/v1/query?query=rate(hello_guest_jobs_total[1m])'
   # restart: unless-stopped revives the crashed container — expect "Up":
   docker compose -f guests/hello-guest/docker-compose.yml ps worker
   ```

**Steps (kind)** — same drill, k8s self-healing instead of compose restart:
1. Bring the guest up per `launch.yaml` `targets.kind.up`, obs overlay applied
   (`kubectl apply -k guests/hello-guest/k8s/obs`).
2. Flood through the shared ingress:
   ```bash
   for i in $(seq 1 3000); do curl -sk -o /dev/null https://hello-guest.lab.local/; done
   ```
3. Kill the worker pod and watch it come back:
   ```bash
   kubectl -n hello-guest delete pod -l app=hello-guest-worker
   kubectl -n hello-guest get pods -w   # new pod Running within ~30 s
   ```

**Expect**
- Step 2's burst is a visible spike on `rate(hello_guest_web_requests_total[1m])`
  in the shared Grafana — a guest is watchable without its own stack.
- While the worker is down, `hello_guest_jobs_total` flatlines (rate → 0
  within one scrape interval + 1 m window) and `up{...worker...}` goes 0.
- The worker returns without operator action (compose `unless-stopped` /
  k8s Deployment), and `hello_guest_jobs_total` **resets to 0** — a restart
  is visible as a counter reset, which `rate()` handles.

**If not:** targets missing in step 1 ⇒ the guest isn't attached to
`microservices_default` or the prometheus.yml job is absent. Rate stays 0 in
k8s ⇒ the obs overlay isn't applied or the ServiceMonitor isn't selected by
the stack (see `k8s/obs/servicemonitors.yaml` note).

**Cleanup**
```bash
docker compose -f guests/hello-guest/docker-compose.yml down
# kind: kubectl delete -k guests/hello-guest/k8s/obs; kubectl delete -k guests/hello-guest/k8s/base
```
