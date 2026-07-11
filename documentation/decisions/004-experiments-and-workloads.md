# ADR-004 — Experiments, chaos, load generation (2026-07-10)

## 004.1 Experiments run guided AND scored
**Context:** guided prose maximizes learning; machine assertions make the
catalog a regression suite and the future UI library's engine. **Decision:**
every experiment keeps Watch/Expect prose and gains machine-checked
assertions; `make experiment E=<id>` returns pass/fail; CI runs a smoke
subset. **Consequences:** v4 defines the assertion runner; experiments become
the lab's test suite officially.

## 004.2 Experiment definitions: YAML + markdown prose
**Context:** the UI (v6) must parse definitions; assertions need structure;
prose carries the teaching. **Decision:** `experiments/<id>.yaml` holds id,
steps, watch refs, assertions (PromQL + thresholds); markdown prose stays
alongside; EXPERIMENTS.md becomes a generated/index view. **Consequences:**
humans keep narrative, machines get structure, one source per experiment.

## 004.3 Chaos Mesh for fault injection
**Context:** stop-a-container drills can't do network faults; Chaos Mesh is
CRD-driven (faults become part of experiment YAML) with its own UI.
**Decision:** Chaos Mesh at v4 for pod-kill, network delay/partition, IO
faults. **Consequences:** network-level drills (api⇄postgres latency)
become possible; one more operator to run; its UI is itself tooling practice.

## 004.4 Small Go AMQP load generator
**Context:** publish.py uses the mgmt HTTP API (~100 msg/s, no
confirms/persistence semantics). **Decision:** `cmd/loadgen` in
operational-workers: envelope-correct AMQP publisher with rate/duration/
confirm flags, containerized. publish.py stays for quick pokes; k6 stays for
HTTP. **Consequences:** honest queue-side load at real rates; reuses the
workers' envelope code; UI/experiments can invoke it anywhere.
