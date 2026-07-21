"use client";

// Rung 1 (visibility) + rung 2 (control) for one system on the selected
// target: description, port block, deep links, live state (lab) or an
// unknown + "Check status" affordance (other systems), and the verb buttons
// that open the action modal.

import { useState } from "react";
import { targetVerbs } from "../lib/format";
import type { LabSnapshot, SystemDef, TargetName, Verb } from "../lib/types";
import { useCockpit } from "../lib/store";
import { LabReadout } from "./LabReadout";

export function SystemCard({
  system,
  target,
  targetAvailable,
  labSnap,
}: {
  system: SystemDef;
  target: TargetName;
  targetAvailable: boolean;
  labSnap: LabSnapshot | null; // provided for the lab system on compose/kind
}) {
  const { openAction } = useCockpit();
  const verbs = targetVerbs(system, target);
  const supported = system.targets[target] !== undefined || verbs.scale;
  const links = system.links?.[target] ?? {};
  const linkEntries = Object.entries(links);

  const scaleComponents = (system.scale ?? []).filter((s) => Boolean(s[target]));
  const [component, setComponent] = useState<string>(
    scaleComponents[0]?.component ?? ""
  );
  const [n, setN] = useState<string>("1");

  const showLiveReadout = Boolean(labSnap && labSnap.loaded);

  function launch(verb: Verb, params?: Record<string, string>) {
    openAction({
      kind: "launch",
      request: { system: system.name, target, verb, params },
      system,
      confirm: verb === "down",
    });
  }

  const btnDisabled = !targetAvailable;
  const disabledTitle = targetAvailable
    ? undefined
    : `${target} target unavailable`;

  return (
    <section className="system-card">
      <div className="system-head">
        <div>
          <span className="system-name">{system.name}</span>
          <span className="system-port">{system.port_block}</span>
        </div>
        {!showLiveReadout && (
          <span className="badge dim" title="live state not in the read API">
            UNKNOWN
          </span>
        )}
      </div>
      <div className="system-desc">{system.description}</div>

      {linkEntries.length > 0 && (
        <div className="links system-links">
          {linkEntries.map(([name, url]) => (
            <a key={name} href={url} target="_blank" rel="noreferrer">
              {name}
            </a>
          ))}
        </div>
      )}

      {!supported ? (
        <div className="notice">not defined for the {target} target</div>
      ) : (
        <div className="control-row">
          {verbs.up && (
            <button
              className="btn-action"
              disabled={btnDisabled}
              title={disabledTitle}
              onClick={() => launch("up")}
            >
              Up
            </button>
          )}
          {verbs.down && (
            <button
              className="btn-action btn-action-danger"
              disabled={btnDisabled}
              title={disabledTitle}
              onClick={() => launch("down")}
            >
              Down
            </button>
          )}
          {verbs.status && (
            <button
              className="btn-action"
              disabled={btnDisabled}
              title={disabledTitle}
              onClick={() => launch("status")}
            >
              {showLiveReadout ? "Status" : "Check status"}
            </button>
          )}
          {verbs.scale && scaleComponents.length > 0 && (
            <span className="scale-group">
              <select
                aria-label="scale component"
                className="select"
                value={component}
                onChange={(e) => setComponent(e.target.value)}
              >
                {scaleComponents.map((s) => (
                  <option key={s.component} value={s.component}>
                    {s.component}
                  </option>
                ))}
              </select>
              <input
                aria-label="replica count"
                className="num-input"
                type="number"
                min={1}
                max={10}
                value={n}
                onChange={(e) => setN(e.target.value)}
              />
              <button
                className="btn-action"
                disabled={btnDisabled}
                title={disabledTitle}
                onClick={() =>
                  launch("scale", {
                    component,
                    n: String(Math.min(10, Math.max(1, Number(n) || 1))),
                  })
                }
              >
                Scale
              </button>
            </span>
          )}
        </div>
      )}

      {showLiveReadout && labSnap && (
        <LabReadout target={target} snap={labSnap} />
      )}
    </section>
  );
}
