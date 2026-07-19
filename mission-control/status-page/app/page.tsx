"use client";

import { useCallback, useEffect, useState } from "react";

const CONTROLD =
  process.env.NEXT_PUBLIC_CONTROLD_URL ?? "http://127.0.0.1:4900";
const POLL_MS = 5000;

type TargetName = "compose" | "kind";

interface Target {
  name: TargetName;
  available: boolean;
}

interface ComposeService {
  name: string;
  state: string;
  health: string;
  image: string;
}

interface KindWorkload {
  namespace: string;
  name: string;
  ready: string;
  status: string;
}

interface HealthResult {
  service: string;
  ok: boolean;
  latency_ms: number;
  error?: string;
}

interface LabLink {
  name: string;
  url: string;
  note?: string;
}

interface Snapshot {
  services: ComposeService[];
  workloads: KindWorkload[];
  health: HealthResult[];
  links: LabLink[];
  statusError: string | null;
}

const emptySnapshot: Snapshot = {
  services: [],
  workloads: [],
  health: [],
  links: [],
  statusError: null,
};

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(`${CONTROLD}${path}`, { cache: "no-store" });
  const body = await res.json();
  if (!res.ok) {
    throw new Error(body?.error ?? `controld returned ${res.status}`);
  }
  return body as T;
}

function composeStateClass(state: string): string {
  if (state === "running") return "ok";
  if (state === "restarting" || state === "paused" || state === "created")
    return "warn";
  return "bad";
}

function composeHealthClass(health: string): string {
  if (health === "healthy") return "ok";
  if (health === "starting") return "warn";
  if (health === "none") return "dim";
  return "bad";
}

function kindStatusClass(w: KindWorkload): string {
  const [ready, total] = w.ready.split("/");
  if (w.status === "Running" && ready === total && ready !== "0") return "ok";
  if (w.status === "Pending" || w.status === "ContainerCreating") return "warn";
  if (w.status === "Running") return "warn"; // running but not all ready
  return "bad";
}

export default function StatusPage() {
  const [targets, setTargets] = useState<Target[]>([]);
  const [target, setTarget] = useState<TargetName>("compose");
  const [snap, setSnap] = useState<Snapshot>(emptySnapshot);
  const [lastPoll, setLastPoll] = useState<string>("--:--:--");
  const [controldDown, setControldDown] = useState(false);

  const poll = useCallback(async (current: TargetName) => {
    let fetchedTargets: Target[];
    try {
      fetchedTargets = await getJSON<Target[]>("/api/targets");
      setTargets(fetchedTargets);
      setControldDown(false);
    } catch {
      setControldDown(true);
      setLastPoll(new Date().toTimeString().slice(0, 8));
      return;
    }

    const available = fetchedTargets.find(
      (t) => t.name === current
    )?.available;
    if (!available) {
      setSnap(emptySnapshot);
      setLastPoll(new Date().toTimeString().slice(0, 8));
      return;
    }

    const next: Snapshot = { ...emptySnapshot };
    try {
      if (current === "compose") {
        const status = await getJSON<{ services: ComposeService[] }>(
          `/api/status?target=compose`
        );
        next.services = status.services ?? [];
      } else {
        const status = await getJSON<{ workloads: KindWorkload[] }>(
          `/api/status?target=kind`
        );
        next.workloads = status.workloads ?? [];
      }
    } catch (err) {
      next.statusError = err instanceof Error ? err.message : String(err);
    }
    try {
      const health = await getJSON<{ results: HealthResult[] }>(
        `/api/health?target=${current}`
      );
      next.health = health.results ?? [];
    } catch {
      // health degrades quietly; status error already surfaces problems
    }
    try {
      const links = await getJSON<{ links: LabLink[] }>(
        `/api/links?target=${current}`
      );
      next.links = links.links ?? [];
    } catch {
      // links are static; missing links are not worth an error state
    }
    setSnap(next);
    setLastPoll(new Date().toTimeString().slice(0, 8));
  }, []);

  useEffect(() => {
    poll(target);
    const id = setInterval(() => poll(target), POLL_MS);
    return () => clearInterval(id);
  }, [target, poll]);

  const selected = targets.find((t) => t.name === target);
  const targetUp = selected?.available ?? false;

  return (
    <main className="console">
      <header className="prompt-line">
        <span className="prompt-title">lab-status</span>
        <span className="prompt-sep">::</span>
        <span className="prompt-meta">target={target}</span>
        <span className="prompt-sep">::</span>
        <span className="prompt-meta">poll {lastPoll}</span>
        <span
          className={`tick${controldDown ? " stale" : ""}`}
          aria-hidden="true"
        />
        <span className="prompt-meta" style={{ marginLeft: "auto" }}>
          read-only
        </span>
      </header>

      <nav className="switcher" aria-label="Target">
        {(["compose", "kind"] as TargetName[]).map((name) => {
          const t = targets.find((x) => x.name === name);
          const up = t?.available ?? false;
          return (
            <button
              key={name}
              className={`${name === target ? "active" : ""}${
                up ? "" : " down"
              }`}
              onClick={() => {
                setTarget(name);
                setSnap(emptySnapshot);
              }}
            >
              {name}
              <span className="avail">{up ? "up" : "down"}</span>
            </button>
          );
        })}
      </nav>

      {controldDown && (
        <div className="notice">
          <strong>controld unreachable</strong> — start it with{" "}
          <code>make controld</code> (expects {CONTROLD})
        </div>
      )}

      {!controldDown && !targetUp && (
        <div className="notice">
          <strong>target unavailable</strong> — the {target} target is not
          running. Bring it up with make, then this page picks it up on the
          next poll.
        </div>
      )}

      {!controldDown && targetUp && (
        <>
          <section className="section">
            <div className="section-label">services</div>
            {snap.statusError ? (
              <div className="notice">
                <strong>status error</strong> — {snap.statusError}
              </div>
            ) : target === "compose" ? (
              snap.services.length === 0 ? (
                <div className="notice">no containers reported</div>
              ) : (
                <div className="grid">
                  {snap.services.map((s) => (
                    <div className="card" key={s.name}>
                      <div className="card-head">
                        <span className="card-name">{s.name}</span>
                        <span>
                          <span
                            className={`badge ${composeStateClass(s.state)}`}
                          >
                            {s.state.toUpperCase()}
                          </span>{" "}
                          <span
                            className={`badge ${composeHealthClass(s.health)}`}
                          >
                            {s.health.toUpperCase()}
                          </span>
                        </span>
                      </div>
                      <div className="card-sub" title={s.image}>
                        {s.image}
                      </div>
                    </div>
                  ))}
                </div>
              )
            ) : snap.workloads.length === 0 ? (
              <div className="notice">no workloads reported</div>
            ) : (
              <div className="grid">
                {snap.workloads.map((w) => (
                  <div className="card" key={`${w.namespace}/${w.name}`}>
                    <div className="card-head">
                      <span className="card-name">{w.name}</span>
                      <span className={`badge ${kindStatusClass(w)}`}>
                        {w.status.toUpperCase()} {w.ready}
                      </span>
                    </div>
                    <div className="card-sub">{w.namespace}</div>
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="section">
            <div className="section-label">health</div>
            {snap.health.length === 0 ? (
              <div className="notice">no health results</div>
            ) : (
              <div>
                {snap.health.map((h) => (
                  <div className="health-row" key={h.service}>
                    <span className="health-service">{h.service}</span>
                    <span className={`badge ${h.ok ? "ok" : "bad"}`}>
                      {h.ok ? "OK" : "FAIL"}
                    </span>
                    {h.latency_ms > 0 && (
                      <span className="health-latency">
                        {h.latency_ms.toFixed(1)}ms
                      </span>
                    )}
                    {h.error && (
                      <span className="health-error" title={h.error}>
                        {h.error}
                      </span>
                    )}
                  </div>
                ))}
              </div>
            )}
          </section>

          <section className="section">
            <div className="section-label">links</div>
            {snap.links.length === 0 ? (
              <div className="notice">no links for this target</div>
            ) : (
              <div className="links">
                {snap.links.map((l) => (
                  <span key={`${l.name}-${l.url}`}>
                    <a href={l.url} target="_blank" rel="noreferrer">
                      {l.name}
                    </a>
                    {l.note && <span className="link-note"> ({l.note})</span>}
                  </span>
                ))}
              </div>
            )}
          </section>
        </>
      )}

      <footer className="footer">
        v3 status page · seeds v6 mission control (ADR-005) · controld{" "}
        {CONTROLD} · polls every {POLL_MS / 1000}s
      </footer>
    </main>
  );
}
