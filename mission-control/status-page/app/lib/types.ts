// Shared types for the Mission Control cockpit.
// These mirror the FINAL controld API contract (v6-HANDOFF §4, ADR-005).

export type TargetName = "compose" | "kind" | "aws";

export interface Target {
  name: TargetName;
  available: boolean;
  note?: string;
}

// ---- read API (existing v3 shapes) ----

export interface ComposeService {
  name: string;
  state: string;
  health: string;
  image: string;
}

export interface KindWorkload {
  namespace: string;
  name: string;
  ready: string;
  status: string;
}

export interface HealthResult {
  service: string;
  ok: boolean;
  latency_ms: number;
  error?: string;
}

export interface LabLink {
  name: string;
  url: string;
  note?: string;
}

// Read-API snapshot for the lab system on a compose/kind target.
export interface LabSnapshot {
  services: ComposeService[];
  workloads: KindWorkload[];
  health: HealthResult[];
  statusError: string | null;
  loaded: boolean;
}

export const emptyLabSnapshot: LabSnapshot = {
  services: [],
  workloads: [],
  health: [],
  statusError: null,
  loaded: false,
};

// ---- systems catalog (/api/systems) ----

export interface ScaleEntry {
  component: string;
  compose?: string;
  kind?: string;
  aws?: string;
}

export type TargetVerbs = {
  up?: string;
  down?: string;
  status?: string;
};

export interface SystemDef {
  name: string;
  description: string;
  port_block: string;
  targets: Partial<Record<TargetName, TargetVerbs>>;
  scale?: ScaleEntry[];
  links?: Partial<Record<TargetName, Record<string, string>>>;
  experiments?: string;
}

// ---- actions (/api/actions) ----

export type Verb = "up" | "down" | "status" | "scale" | "experiment";

export interface ActionParams {
  confirm?: "true";
  component?: string;
  n?: string;
  id?: string;
}

export interface ActionRequest {
  system: string;
  target: TargetName;
  verb: Verb;
  params?: ActionParams;
}

// controld records are born "running" and finalize to succeeded/failed; there
// is no queued/pending state.
export type ActionState = "running" | "succeeded" | "failed";

// ---- scored-experiment report (present only on completed scored runs) ----

export interface AssertionResult {
  name: string;
  passed: boolean;
  detail?: string;
}

export interface ExperimentReport {
  passed: boolean; // overall
  total: number; // total assertions
  failed: number;
  assertions: AssertionResult[];
}

export interface ActionRecord {
  id: string;
  request: ActionRequest;
  command: string;
  state: ActionState;
  exit_code?: number;
  started_at: string;
  ended_at?: string;
  // Present only on completed scored experiment runs; absent for older runs,
  // non-experiment verbs, or a runner that crashed before writing a report.
  report?: ExperimentReport;
}

// ---- experiments (/api/experiments) ----

export interface ExperimentStep {
  run: string;
  background?: boolean;
}

export interface ExperimentAssertion {
  type: string;
  query?: string;
  url?: string;
  op?: string;
  value?: string | number;
  timeout?: string | number;
  [k: string]: unknown;
}

export interface Experiment {
  id: string;
  title: string;
  needs: string[];
  steps: ExperimentStep[];
  watch: string[];
  assertions: ExperimentAssertion[];
  file: string;
}

export type OutcomeResult = "pass" | "fail" | "aborted";

// ---- sessions (/api/sessions) ----

export interface Session {
  id: string;
  title: string;
  started_at: string;
}
