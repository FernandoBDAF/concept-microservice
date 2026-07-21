"use client";

// Live read-API state for the lab system on a compose/kind target — the v3
// services/workloads + health tables, preserved verbatim in look and logic.

import {
  composeHealthClass,
  composeStateClass,
  kindStatusClass,
} from "../lib/format";
import type { LabSnapshot, TargetName } from "../lib/types";

export function LabReadout({
  target,
  snap,
}: {
  target: TargetName;
  snap: LabSnapshot;
}) {
  return (
    <div className="lab-readout">
      <div className="readout-block">
        <div className="readout-label">services</div>
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
                      <span className={`badge ${composeStateClass(s.state)}`}>
                        {s.state.toUpperCase()}
                      </span>{" "}
                      <span className={`badge ${composeHealthClass(s.health)}`}>
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
      </div>

      <div className="readout-block">
        <div className="readout-label">health</div>
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
      </div>
    </div>
  );
}
