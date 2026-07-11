# ADR-002 — Local cluster platform (2026-07-10)

## 002.1 Kustomize now; one Helm chart later as an exercise
**Context:** era-1 raw YAML drifted; Helm was an unfinished era-1 aspiration;
the review (§10) called config duplication a top issue. **Decision:** kustomize
base + overlays (kind-local now, aws later) for all lab manifests; write **one
Helm chart for one service in v8** purely as a learning exercise.
**Consequences:** plain readable YAML day-to-day; Helm literacy still gets a
dedicated slot; kube-prometheus-stack (ADR-003.1) provides incidental Helm
exposure earlier.

## 002.2 kind, with single-node and multinode profiles
**Context:** era-1 used kind (configs preserved in legacy_project); node-kill
drills need multiple nodes. **Decision:** kind as the substrate; default
single-node profile for daily work, multinode profile spun up only for
node-failure/rescheduling drills. **Consequences:** upstream-faithful behavior;
RAM cost contained to the drills that need it; era-1 configs get mined, not
rewritten from scratch.

## 002.3 Local registry container for images
**Context:** era-1 used `kind load`; ECR arrives at v5. **Decision:** run a
local registry (localhost:5001) wired into kind; make targets build/tag/push
to it; imagePullPolicy and tags behave like production. **Consequences:** real
push/pull semantics from day one; the v5 switch to ECR changes a registry URL,
not the workflow.

## 002.4 Compose stays first-class, with a mechanical drift check
**Context:** compose is the fast inner loop and most experiments run on it;
dual-maintenance with kustomize is the known cost. **Decision:** keep both
first-class; add a CI/contract check that compose and kustomize declare the
same images/env surface so drift is caught mechanically (lands with CI,
ADR-010.2). **Consequences:** experiments keep their fast loop; the drift
class that caused the profile-tasks incident gets a guard instead of a hope.
