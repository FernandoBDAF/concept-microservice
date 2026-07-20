"use client";

// Rung 3: the experiment catalog. Cards list id/title/needs; the detail modal
// renders steps, watch items (each with the experiment-owning system's
// Grafana-family deep links for the current target beside it), and the
// assertions table. From the detail you can run the scored experiment
// (streaming action) or record an outcome. The owning system is whichever
// registry entry declares an experiments catalog — today lab, but any system
// declaring one works without UI changes.

import { useCallback, useState } from "react";
import { postJSON } from "../lib/api";
import type {
  Experiment,
  OutcomeResult,
  SystemDef,
  TargetName,
} from "../lib/types";
import { useCockpit } from "../lib/store";

export function ExperimentLibrary({
  experiments,
  target,
  experimentSystem,
  onRefresh,
  loadError,
}: {
  experiments: Experiment[];
  target: TargetName;
  experimentSystem: SystemDef | null;
  onRefresh: () => void;
  loadError: string | null;
}) {
  const [selected, setSelected] = useState<Experiment | null>(null);

  return (
    <section className="section">
      <div className="section-label section-label-row">
        <span>experiment library</span>
        <button className="btn-ghost" onClick={onRefresh}>
          ↻ refresh
        </button>
      </div>

      {loadError ? (
        <div className="notice">
          <strong>experiments unavailable</strong> — {loadError}
        </div>
      ) : experiments.length === 0 ? (
        <div className="notice">no experiments reported</div>
      ) : (
        <div className="grid exp-grid">
          {experiments.map((exp) => (
            <button
              key={exp.id}
              className="card exp-card"
              onClick={() => setSelected(exp)}
            >
              <div className="card-head">
                <span className="card-name">{exp.id}</span>
              </div>
              <div className="exp-title">{exp.title}</div>
              {exp.needs?.length > 0 && (
                <div className="chip-row">
                  {exp.needs.map((need) => (
                    <span key={need} className="chip">
                      {need}
                    </span>
                  ))}
                </div>
              )}
            </button>
          ))}
        </div>
      )}

      {selected && (
        <ExperimentDetail
          exp={selected}
          target={target}
          experimentSystem={experimentSystem}
          onClose={() => setSelected(null)}
        />
      )}
    </section>
  );
}

function ExperimentDetail({
  exp,
  target,
  experimentSystem,
  onClose,
}: {
  exp: Experiment;
  target: TargetName;
  experimentSystem: SystemDef | null;
  onClose: () => void;
}) {
  const { openAction, addToast } = useCockpit();
  const [result, setResult] = useState<OutcomeResult>("pass");
  const [notes, setNotes] = useState("");
  const [saving, setSaving] = useState(false);
  const links = experimentSystem?.links?.[target] ?? {};
  const linkEntries = Object.entries(links);

  const runScored = useCallback(() => {
    if (!experimentSystem) {
      addToast({
        tone: "warn",
        message: "no system in the registry declares experiments",
      });
      return;
    }
    openAction({
      kind: "launch",
      request: {
        system: experimentSystem.name,
        target,
        verb: "experiment",
        params: { id: exp.id },
      },
      system: experimentSystem,
      confirm: false,
    });
  }, [experimentSystem, target, exp.id, openAction, addToast]);

  const recordOutcome = useCallback(async () => {
    setSaving(true);
    try {
      await postJSON(`/api/experiments/${exp.id}/outcome`, {
        result,
        notes,
      });
      addToast({
        tone: "info",
        message: `Outcome recorded for ${exp.id}: ${result}`,
      });
      setNotes("");
    } catch (err) {
      addToast({
        tone: "error",
        message: err instanceof Error ? err.message : "Failed to record outcome",
      });
    } finally {
      setSaving(false);
    }
  }, [exp.id, result, notes, addToast]);

  return (
    <div className="overlay" onMouseDown={onClose}>
      <div
        className="modal modal-wide"
        role="dialog"
        aria-modal="true"
        aria-label={exp.title}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="modal-head">
          <span className="modal-title">
            <span className="verb">{exp.id}</span> {exp.title}
          </span>
          <button className="modal-close" aria-label="Close" onClick={onClose}>
            ×
          </button>
        </div>

        <div className="modal-body">
          {exp.needs?.length > 0 && (
            <div className="chip-row">
              {exp.needs.map((need) => (
                <span key={need} className="chip">
                  {need}
                </span>
              ))}
            </div>
          )}

          <div className="detail-block">
            <div className="readout-label">steps</div>
            <ol className="step-list">
              {exp.steps?.map((step, i) => (
                <li key={i}>
                  <code>{step.run}</code>
                  {step.background && (
                    <span className="chip chip-inline">background</span>
                  )}
                </li>
              ))}
            </ol>
          </div>

          <div className="detail-block">
            <div className="readout-label">watch</div>
            {exp.watch?.length ? (
              <ul className="watch-list">
                {exp.watch.map((w, i) => (
                  <li key={i} className="watch-item">
                    <span className="watch-prose">{w}</span>
                    {linkEntries.length > 0 && (
                      <span className="watch-links">
                        {linkEntries.map(([name, url]) => (
                          <a
                            key={name}
                            href={url}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {name}
                          </a>
                        ))}
                      </span>
                    )}
                  </li>
                ))}
              </ul>
            ) : (
              <div className="prompt-meta">no watch items</div>
            )}
          </div>

          <div className="detail-block">
            <div className="readout-label">assertions</div>
            {exp.assertions?.length ? (
              <div className="table-wrap">
                <table className="assert-table">
                  <thead>
                    <tr>
                      <th>type</th>
                      <th>query / url</th>
                      <th>op</th>
                      <th>value</th>
                      <th>timeout</th>
                    </tr>
                  </thead>
                  <tbody>
                    {exp.assertions.map((a, i) => (
                      <tr key={i}>
                        <td>{a.type}</td>
                        <td className="assert-target">
                          <code>{a.query ?? a.url ?? ""}</code>
                        </td>
                        <td>{a.op ?? ""}</td>
                        <td>{a.value !== undefined ? String(a.value) : ""}</td>
                        <td>{a.timeout !== undefined ? String(a.timeout) : ""}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="prompt-meta">no assertions</div>
            )}
          </div>

          <div className="detail-block outcome-block">
            <div className="readout-label">record outcome</div>
            <div className="outcome-radios">
              {(["pass", "fail", "aborted"] as OutcomeResult[]).map((r) => (
                <label key={r} className="radio">
                  <input
                    type="radio"
                    name="outcome"
                    value={r}
                    checked={result === r}
                    onChange={() => setResult(r)}
                  />
                  {r}
                </label>
              ))}
            </div>
            <textarea
              className="note-input"
              rows={3}
              placeholder="notes…"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
            <button
              className="btn-ghost"
              disabled={saving}
              onClick={() => void recordOutcome()}
            >
              {saving ? "saving…" : "Record outcome"}
            </button>
          </div>
        </div>

        <div className="modal-foot">
          <button className="btn-action" onClick={runScored}>
            ▶ Run scored
          </button>
          <span className="prompt-meta file-note">{exp.file}</span>
          <button
            className="btn-ghost"
            style={{ marginLeft: "auto" }}
            onClick={onClose}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
