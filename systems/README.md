# Systems registry (ADR-005 · phase v6)

The machine face of the host contract: one YAML per launchable system.
`lab-controld` reads this directory at startup; the Mission Control UI's
catalog IS this registry (guests onboard by dropping a file here — v7's
mycelium/loam become config, not code).

## Schema v0

```yaml
name: lab                  # unique id (kebab-case)
description: one line
port_block: core           # or 41xx/42xx/43xx per HOST_CONTRACT.md
targets:                   # which lab targets can run it, and how.
  compose:                 # commands SHOULD be make targets (ADR-005.2: the
    up: make up            # daemon wraps make, never reimplements it) — but
    down: make down        # the registry content IS the whitelist, so non-make
    status: make ps        # commands (e.g. kubectl for guests) are allowed.
  kind:
    up: make cluster-up
    down: make cluster-down
    status: make cluster-status
  aws:                     # present only if the system supports sessions
    up: make aws-up
    down: make aws-down
    status: make aws-plan
scale:                     # optional: scalable components
  - component: email-worker
    compose: make scale S=email-worker N={n}
    kind: make cluster-scale S=email-worker N={n}
links:                     # per-target deep links (status page shows them);
  compose: { grafana: "http://localhost:3001", rabbitmq: "http://localhost:15672" }
  kind: { grafana: "https://grafana.lab.local" }
  aws: { cost-explorer: "https://console.aws.amazon.com/cost-management/home" }
experiments: experiments/  # dir of scored YAML defs relevant to this system
```

Rules: commands are exec'd verbatim from the repo root with streamed
stdout; `{n}`-style placeholders are filled from validated action params;
anything not listed here is not invokable (the registry doubles as the
daemon's action whitelist — see mission-control/controld/registry.go and
actions.go: `make` is preferred, but the enforced rule is the whitelist
itself, not a make-only restriction). One system per file — a second
`---` YAML document fails the load. `port_block` is informational only
(documenting the HOST_CONTRACT.md allocation); the loader does not
validate it against the `core|41xx|42xx|43xx` set.
