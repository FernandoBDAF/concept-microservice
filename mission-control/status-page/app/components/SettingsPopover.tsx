"use client";

// Settings popover: the controld bearer token (persisted to localStorage) and
// the base URL controld is expected at. The token is applied to every fetch
// and folded into the SSE query string.

import { useEffect, useRef, useState } from "react";
import { CONTROLD_BASE } from "../lib/api";
import { useCockpit } from "../lib/store";

export function SettingsPopover() {
  const { token, saveToken } = useCockpit();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(token);
  const ref = useRef<HTMLDivElement | null>(null);

  useEffect(() => setDraft(token), [token]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open]);

  return (
    <div className="settings" ref={ref}>
      <button
        className={`btn-ghost${token ? " has-token" : ""}`}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        title={token ? "Token configured" : "No token configured"}
      >
        ⚙ settings
      </button>
      {open && (
        <div className="popover settings-popover">
          <label className="field-label" htmlFor="controld-token">
            controld token
          </label>
          <input
            id="controld-token"
            className="text-input"
            type="password"
            placeholder="Bearer token (optional)"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
          />
          <div className="field-hint">
            base: <code>{CONTROLD_BASE}</code>
          </div>
          <div className="popover-actions">
            <button
              className="btn-ghost"
              onClick={() => {
                saveToken(draft.trim());
                setOpen(false);
              }}
            >
              Save
            </button>
            <button
              className="btn-ghost"
              onClick={() => {
                setDraft("");
                saveToken("");
              }}
            >
              Clear
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
