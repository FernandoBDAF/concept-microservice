// Presentation helpers: status → badge class, timestamps, resolved-command
// previews. Kept pure so components stay declarative.

import type {
  ActionState,
  ComposeService,
  ExperimentReport,
  KindWorkload,
  SystemDef,
  TargetName,
  Verb,
} from "./types";

export function composeStateClass(state: string): string {
  if (state === "running") return "ok";
  if (state === "restarting" || state === "paused" || state === "created")
    return "warn";
  return "bad";
}

export function composeHealthClass(health: string): string {
  if (health === "healthy") return "ok";
  if (health === "starting") return "warn";
  if (health === "none") return "dim";
  return "bad";
}

export function kindStatusClass(w: KindWorkload): string {
  const [ready, total] = w.ready.split("/");
  if (w.status === "Running" && ready === total && ready !== "0") return "ok";
  if (w.status === "Pending" || w.status === "ContainerCreating") return "warn";
  if (w.status === "Running") return "warn"; // running but not all ready
  return "bad";
}

export function actionStateClass(state: ActionState): string {
  switch (state) {
    case "succeeded":
      return "ok";
    case "failed":
      return "bad";
    case "running":
      return "warn";
    default:
      return "dim";
  }
}

export function serviceLabel(s: ComposeService): string {
  return `${s.state}/${s.health}`;
}

// ---- scored-experiment report presentation ----

function reportPassed(r: ExperimentReport): number {
  return Math.max(0, r.total - r.failed);
}

// Long form for the modal panel, e.g. "5/6 assertions passed".
export function reportSummary(r: ExperimentReport): string {
  return `${reportPassed(r)}/${r.total} assertions passed`;
}

// Compact form for the history row badge, e.g. "4/4 ✓" or "3/5 ✗".
export function reportBadge(r: ExperimentReport): string {
  return `${reportPassed(r)}/${r.total} ${r.passed ? "✓" : "✗"}`;
}

export function fmtClock(iso?: string): string {
  if (!iso) return "--:--:--";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toTimeString().slice(0, 8);
}

export function fmtDuration(startIso?: string, endIso?: string): string {
  if (!startIso) return "";
  const start = new Date(startIso).getTime();
  const end = endIso ? new Date(endIso).getTime() : Date.now();
  if (Number.isNaN(start) || Number.isNaN(end)) return "";
  const secs = Math.max(0, Math.round((end - start) / 1000));
  if (secs < 60) return `${secs}s`;
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return `${m}m${s.toString().padStart(2, "0")}s`;
}

export function fmtElapsed(startIso: string, now: number): string {
  const start = new Date(startIso).getTime();
  if (Number.isNaN(start)) return "";
  const secs = Math.max(0, Math.round((now - start) / 1000));
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = secs % 60;
  const pad = (n: number) => n.toString().padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

// Which verbs a system exposes on a given target (drives which buttons show).
export function targetVerbs(
  system: SystemDef,
  target: TargetName
): { up: boolean; down: boolean; status: boolean; scale: boolean } {
  const t = system.targets[target];
  const scale = !!system.scale?.some((s) => Boolean(s[target]));
  return {
    up: Boolean(t?.up),
    down: Boolean(t?.down),
    status: Boolean(t?.status),
    scale,
  };
}

// Best-effort preview of the make command an action will resolve to, drawn
// from the registry — the teaching surface even before the POST returns the
// authoritative `command`.
export function previewCommand(
  system: SystemDef,
  target: TargetName,
  verb: Verb,
  params?: { component?: string; n?: string; id?: string }
): string | undefined {
  if (verb === "scale") {
    const entry = system.scale?.find((s) => s.component === params?.component);
    const tmpl = entry?.[target];
    if (!tmpl) return undefined;
    return tmpl.replace("{n}", params?.n ?? "N");
  }
  if (verb === "experiment") {
    return `make experiment E=${params?.id ?? "<id>"}`;
  }
  const t = system.targets[target];
  return t?.[verb as "up" | "down" | "status"];
}
