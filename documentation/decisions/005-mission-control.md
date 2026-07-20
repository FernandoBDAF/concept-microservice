# ADR-005 — Mission Control UI (2026-07-10)

## 005.1 Build it: Next.js/React
**Context:** assembling (Backstage/Portainer) is faster to "something" but
doesn't fit the experiment-library concept; owner has real front-end
experience (GraphDash/StagesUI in mycelium, formerly KnowledgeManager).
**Decision:** build a
Next.js/React app. **Consequences:** exactly-fitting cockpit + transferable
UI practice; the v3 thin status page (ADR-001.3) uses this stack to de-risk
it early.

## 005.2 Control plane: API daemon that runs the make targets
**Context:** make is the single source of truth; a UI shelling out directly
is chained to a local checkout; a native-API daemon would drift from make.
**Decision:** `lab-controld` exposes REST/WS but implements actions by
invoking the same make targets/scripts; make stays directly usable.
**Consequences:** one control path, streaming output to the UI, remote/AWS
reach later without a second brain.
**Superseded by ADR-005.6** (2026-07-20): the stream transport shipped as
SSE, not WebSocket; the wrap-make control-plane decision is unchanged.

## 005.3 Experiment format follows ADR-004.2 (YAML + prose)
The UI renders the YAML definitions and embeds each experiment's Watch
dashboards next to its run output; records outcomes to
documentation/experiments/.

## 005.4 Localhost-only now; auth arrives with remote targets
**Context:** loam's UI proves the 127.0.0.1/no-auth pattern locally; an
AWS-triggering control plane can't stay open. **Decision:** bind localhost
with no auth through v6; the AWS-target integration adds an auth story
(minimum: shared token + TLS) as part of its acceptance. **Consequences:**
zero friction locally; a hard gate before remote control exists.

## 005.5 loam relationship: borrow patterns only (for now)
**Context:** both are "local web app over derived state"; loam is itself
under active development, so merge timing is bad. **Decision:** copy loam's
proven ideas (read-only server discipline, live file-watching, degrade
quietly) but keep Mission Control independent; revisit deeper integration
(shared components, embedding) later once both stabilize. **Consequences:**
no coupled release cycles; loam integrates as a *guest system* (v7), not as
UI plumbing.

## 005.6 Streaming transport: SSE, not WebSocket (2026-07-20)
**Supersedes ADR-005.2** (transport only — the daemon-wraps-make decision
stands). **Context:** 005.2 said "REST/WS"; v6 shipped the action stream as
Server-Sent Events (`GET /api/actions/{id}/stream`). Action output is a
one-way stdout/stderr fan-out — WebSocket bidirectionality buys nothing
here. **Decision:** REST + SSE. Native `EventSource` gives reconnection for
free; `?token=` query auth is workable on the one route where a browser
cannot set headers (and is scoped to that route only, Bearer everywhere
else); the auth story stays one middleware, not a second handshake.
**Consequences:** no WebSocket dependency; slow consumers are handled with
bounded ring buffers and explicit dropped-line markers instead of
backpressure plumbing.
