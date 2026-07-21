"use client";

// Cockpit header: title, live target switcher (chips from /api/targets — an
// available target is selectable, an unavailable one is disabled; either way
// its note, when present, shows as a tooltip — for aws this is a live probe
// note explaining the session is up or WHY it isn't), the session bar, and the
// settings popover.

import type { Target, TargetName } from "../lib/types";
import { SessionBar } from "./SessionBar";
import { SettingsPopover } from "./SettingsPopover";

export function Header({
  targets,
  target,
  onSelectTarget,
  lastPoll,
  controldDown,
  authError,
}: {
  targets: Target[];
  target: TargetName;
  onSelectTarget: (t: TargetName) => void;
  lastPoll: string;
  controldDown: boolean;
  authError: boolean;
}) {
  return (
    <header className="cockpit-header">
      <div className="prompt-line">
        <span className="prompt-title">mission-control</span>
        <span className="prompt-sep">::</span>
        <span className="prompt-meta">target={target}</span>
        <span className="prompt-sep">::</span>
        <span className="prompt-meta">poll {lastPoll}</span>
        <span
          className={`tick${controldDown ? " stale" : ""}`}
          aria-hidden="true"
        />
        <div className="header-right">
          <SessionBar />
          <SettingsPopover />
        </div>
      </div>

      <nav className="switcher" aria-label="Target">
        {targets.length === 0 ? (
          <span className="prompt-meta">no targets reported</span>
        ) : (
          targets.map((t) => {
            const active = t.name === target;
            const disabled = !t.available;
            // Show the note as a tooltip whenever the probe supplies one — in
            // both states — falling back to a generic reason only when a
            // disabled target reports no note.
            const title =
              t.note ?? (disabled ? `${t.name} target not available` : undefined);
            return (
              <button
                key={t.name}
                className={`${active ? "active" : ""}${
                  disabled ? " down" : ""
                }`}
                disabled={disabled}
                title={title}
                onClick={() => !disabled && onSelectTarget(t.name)}
              >
                {t.name}
                <span className="avail">{t.available ? "up" : "down"}</span>
              </button>
            );
          })
        )}
      </nav>

      {authError && (
        <div className="notice auth-notice">
          <strong>401 unauthorized</strong> — controld requires a token. Open{" "}
          <em>settings</em> and paste the controld token.
        </div>
      )}
    </header>
  );
}
