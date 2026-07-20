"use client";

// Mission Control — the single-page cockpit. Grows from the v3 read-only
// status page (ADR-001.3): rung 1 visibility, rung 2 control, rung 3
// experiment library, session recorder, and run history — one route,
// componentized. Polling stays at 5s for targets/status/health; the systems
// and experiment catalogs are fetched once + on target change + manual
// refresh (never polled).

import { useCallback, useEffect, useState } from "react";
import { ApiError, CONTROLD_BASE, getJSON } from "./lib/api";
import { CockpitProvider } from "./lib/store";
import {
  emptyLabSnapshot,
  type ComposeService,
  type Experiment,
  type HealthResult,
  type KindWorkload,
  type LabSnapshot,
  type SystemDef,
  type Target,
  type TargetName,
} from "./lib/types";
import { Header } from "./components/Header";
import { SystemCard } from "./components/SystemCard";
import { ExperimentLibrary } from "./components/ExperimentLibrary";
import { HistoryPanel } from "./components/HistoryPanel";

const POLL_MS = 5000;

function clock(): string {
  return new Date().toTimeString().slice(0, 8);
}

function Cockpit() {
  const [targets, setTargets] = useState<Target[]>([]);
  const [target, setTarget] = useState<TargetName>("compose");
  const [labSnap, setLabSnap] = useState<LabSnapshot>(emptyLabSnapshot);
  const [systems, setSystems] = useState<SystemDef[]>([]);
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [systemsError, setSystemsError] = useState<string | null>(null);
  const [expError, setExpError] = useState<string | null>(null);
  const [lastPoll, setLastPoll] = useState<string>("--:--:--");
  const [controldDown, setControldDown] = useState(false);
  const [authError, setAuthError] = useState(false);

  const is401 = (err: unknown) => err instanceof ApiError && err.status === 401;

  // 5s poll: targets always; lab live state (status + health) only when the
  // selected target is compose/kind and available (the read API covers those).
  // The caller's AbortSignal cancels a poll that outlives its target selection
  // so stale responses never land in state.
  const poll = useCallback(
    async (current: TargetName, signal: AbortSignal) => {
      let fetchedTargets: Target[];
      try {
        fetchedTargets = await getJSON<Target[]>("/api/targets", { signal });
        setTargets(fetchedTargets);
        setControldDown(false);
        setAuthError(false);
      } catch (err) {
        if (signal.aborted) return;
        if (is401(err)) setAuthError(true);
        else setControldDown(true);
        setLastPoll(clock());
        return;
      }

      const available =
        fetchedTargets.find((t) => t.name === current)?.available ?? false;
      const readable = current === "compose" || current === "kind";

      if (!readable || !available) {
        setLabSnap({ ...emptyLabSnapshot, loaded: false });
        setLastPoll(clock());
        return;
      }

      const next: LabSnapshot = { ...emptyLabSnapshot, loaded: true };
      try {
        if (current === "compose") {
          const status = await getJSON<{ services: ComposeService[] }>(
            "/api/status?target=compose",
            { signal }
          );
          next.services = status.services ?? [];
        } else {
          const status = await getJSON<{ workloads: KindWorkload[] }>(
            "/api/status?target=kind",
            { signal }
          );
          next.workloads = status.workloads ?? [];
        }
      } catch (err) {
        if (signal.aborted) return;
        if (is401(err)) setAuthError(true);
        next.statusError = err instanceof Error ? err.message : String(err);
      }
      try {
        const health = await getJSON<{ results: HealthResult[] }>(
          `/api/health?target=${current}`,
          { signal }
        );
        next.health = health.results ?? [];
      } catch {
        // health degrades quietly; status error already surfaces problems
      }
      if (signal.aborted) return;
      setLabSnap(next);
      setLastPoll(clock());
    },
    []
  );

  useEffect(() => {
    const ctrl = new AbortController();
    let inFlight = false;
    const run = async () => {
      if (inFlight) return; // previous poll still running — skip this tick
      inFlight = true;
      try {
        await poll(target, ctrl.signal);
      } finally {
        inFlight = false;
      }
    };
    void run();
    const id = window.setInterval(() => void run(), POLL_MS);
    return () => {
      window.clearInterval(id);
      ctrl.abort(); // drop any in-flight poll for the old target
    };
  }, [target, poll]);

  // Catalogs: fetch once + on target change + manual refresh (never polled).
  const loadCatalog = useCallback(async () => {
    try {
      const s = await getJSON<SystemDef[]>("/api/systems");
      setSystems(s);
      setSystemsError(null);
    } catch (err) {
      if (is401(err)) setAuthError(true);
      setSystemsError(err instanceof Error ? err.message : String(err));
    }
    try {
      const e = await getJSON<Experiment[]>("/api/experiments");
      setExperiments(e);
      setExpError(null);
    } catch (err) {
      if (is401(err)) setAuthError(true);
      setExpError(err instanceof Error ? err.message : String(err));
    }
  }, []);

  useEffect(() => {
    void loadCatalog();
  }, [target, loadCatalog]);

  const selectTarget = useCallback((t: TargetName) => {
    setTarget(t);
    setLabSnap({ ...emptyLabSnapshot, loaded: false });
  }, []);

  const targetAvailable =
    targets.find((t) => t.name === target)?.available ?? false;
  // The system whose registry entry declares an experiments catalog owns the
  // scored-run verb (today that's lab; any system declaring one works).
  const experimentSystem = systems.find((s) => Boolean(s.experiments)) ?? null;

  return (
    <main className="console">
      <Header
        targets={targets}
        target={target}
        onSelectTarget={selectTarget}
        lastPoll={lastPoll}
        controldDown={controldDown}
        authError={authError}
      />

      {controldDown && (
        <div className="notice">
          <strong>controld unreachable</strong> — start it with{" "}
          <code>make controld</code> (expects {CONTROLD_BASE}). The cockpit
          stays read-only until it responds.
        </div>
      )}

      {!controldDown && !targetAvailable && (
        <div className="notice">
          <strong>{target} target unavailable</strong> — actions are disabled;
          bring the target up and the cockpit picks it up on the next poll.
        </div>
      )}

      <section className="section">
        <div className="section-label section-label-row">
          <span>systems</span>
          <button className="btn-ghost" onClick={() => void loadCatalog()}>
            ↻ refresh
          </button>
        </div>
        {systemsError ? (
          <div className="notice">
            <strong>systems unavailable</strong> — {systemsError}
          </div>
        ) : systems.length === 0 ? (
          <div className="notice">no systems reported</div>
        ) : (
          <div className="system-grid">
            {systems.map((system) => (
              <SystemCard
                key={system.name}
                system={system}
                target={target}
                targetAvailable={targetAvailable}
                labSnap={system.name === "lab" ? labSnap : null}
              />
            ))}
          </div>
        )}
      </section>

      <ExperimentLibrary
        experiments={experiments}
        target={target}
        experimentSystem={experimentSystem}
        onRefresh={() => void loadCatalog()}
        loadError={expError}
      />

      <HistoryPanel />

      <footer className="footer">
        mission control · v6 cockpit (ADR-005) · controld {CONTROLD_BASE} ·
        targets/status/health poll every {POLL_MS / 1000}s
      </footer>
    </main>
  );
}

export default function Page() {
  return (
    <CockpitProvider>
      <Cockpit />
    </CockpitProvider>
  );
}
